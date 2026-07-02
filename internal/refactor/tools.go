package refactor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/investigator"
	"github.com/ZzedJay/Ops-Agent/internal/netproxy"
	"github.com/ZzedJay/Ops-Agent/internal/rag"
)

// Toolbox 在克隆仓库沙箱内执行调查、改文件与白名单命令。
type Toolbox struct {
	inv     *investigator.Toolbox
	repoPath string
	proxy    config.ProxyConfig
	log      investigator.Logger
}

func NewToolbox(repoPath string, invCfg config.InvestigatorConfig, ragCfg config.RAGConfig, ragIdx *rag.Index, proxy config.ProxyConfig) *Toolbox {
	return &Toolbox{
		inv:      investigator.NewToolbox(repoPath, invCfg, ragCfg, ragIdx, proxy),
		repoPath: repoPath,
		proxy:    proxy,
	}
}

func (t *Toolbox) SetLogger(log investigator.Logger) {
	t.log = log
	if t.inv != nil {
		t.inv.SetLogger(log)
	}
}

func (t *Toolbox) Edited() bool {
	return t.inv != nil // placeholder; track via git status in worker
}

func (t *Toolbox) Run(ctx context.Context, a Action) (string, error) {
	switch a.Action {
	case ActionEditFile:
		return t.editFile(a.Path, a.Content, a.Old, a.New)
	case ActionRunCmd:
		return t.runCmd(ctx, a.Command)
	case ActionDone:
		return "", fmt.Errorf("done is terminal")
	default:
		ia, err := toInvestigatorAction(a)
		if err != nil {
			return "", err
		}
		return t.inv.Run(ctx, ia)
	}
}

func toInvestigatorAction(a Action) (investigator.Action, error) {
	switch a.Action {
	case ActionSearch, ActionRead, ActionListDir, ActionFetchURL, ActionWebSearch, ActionRAGSearch, ActionRepoValidate:
		return investigator.Action{
			Action:    a.Action,
			Query:     a.Query,
			Path:      a.Path,
			URL:       a.URL,
			StartLine: a.StartLine,
			EndLine:   a.EndLine,
			Body:      a.Body,
		}, nil
	default:
		return investigator.Action{}, fmt.Errorf("unsupported read action %q", a.Action)
	}
}

func (t *Toolbox) resolvePath(rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	rel = strings.TrimPrefix(rel, "./")
	rel = strings.TrimPrefix(rel, "/")
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "." {
		rel = ""
	}
	if strings.Contains(rel, "..") {
		return "", fmt.Errorf("invalid path")
	}
	full := filepath.Join(t.repoPath, filepath.FromSlash(rel))
	absRepo, err := filepath.Abs(t.repoPath)
	if err != nil {
		return "", err
	}
	absFull, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	if absFull != absRepo && !strings.HasPrefix(absFull, absRepo+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes repository")
	}
	return absFull, nil
}

func (t *Toolbox) editFile(rel, content, old, new string) (string, error) {
	full, err := t.resolvePath(rel)
	if err != nil {
		return "", err
	}

	if _, statErr := os.Stat(full); statErr == nil {
		if old != "" || new != "" {
			return t.patchFile(full, rel, old, new)
		}
		return "", fmt.Errorf("edit_file: %s 已存在，禁止用 content 整文件覆盖（会误删未改代码）；请 read_file 后用 old/new 只改必要片段", rel)
	} else if !os.IsNotExist(statErr) {
		return "", statErr
	}

	if content == "" {
		return "", fmt.Errorf("edit_file: 新建文件须提供 content")
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("created %s (%d bytes)", rel, len(content)), nil
}

func (t *Toolbox) patchFile(full, rel, old, new string) (string, error) {
	data, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	text := string(data)
	count := strings.Count(text, old)
	if count == 0 {
		return "", fmt.Errorf("edit_file: old 片段在 %s 中未找到（须与 read_file 原文完全一致，含缩进与换行）", rel)
	}
	if count > 1 {
		return "", fmt.Errorf("edit_file: old 片段在 %s 中出现 %d 次，请扩大上下文使匹配唯一", rel, count)
	}
	updated := strings.Replace(text, old, new, 1)
	if err := os.WriteFile(full, []byte(updated), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("patched %s (%d → %d bytes)", rel, len(data), len(updated)), nil
}

var allowedCmdPrefixes = []string{
	"go test",
	"go build",
	"go vet",
	"make ",
	"npm test",
	"npm run ",
	"yarn test",
	"yarn run ",
	"cargo test",
	"cargo build",
	"pytest",
	"python -m pytest",
	"python3 -m pytest",
	"git status",
	"git diff",
}

func (t *Toolbox) runCmd(ctx context.Context, command string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", fmt.Errorf("empty command")
	}
	if !allowedCommand(command) {
		return "", fmt.Errorf("command not allowed: %q", command)
	}
	cmd := exec.CommandContext(ctx, "bash", "-lc", command)
	cmd.Dir = t.repoPath
	netproxy.ConfigureCmd(cmd, t.proxy)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if len(text) > 12000 {
		text = text[:12000] + "\n…(truncated)"
	}
	if err != nil {
		if text == "" {
			return "", fmt.Errorf("command failed: %w", err)
		}
		return text, fmt.Errorf("command failed: %w", err)
	}
	if text == "" {
		return "ok (no output)", nil
	}
	return text, nil
}

func allowedCommand(command string) bool {
	lower := strings.ToLower(strings.TrimSpace(command))
	for _, p := range allowedCmdPrefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}
