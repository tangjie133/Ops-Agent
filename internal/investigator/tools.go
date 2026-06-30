package investigator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/rag"
	"github.com/ZzedJay/Ops-Agent/internal/repovalidate"
)

// Toolbox 在克隆仓库沙箱内执行工具，并支持抓取外部文档与本地 RAG。
type Toolbox struct {
	repoPath string
	cfg      config.InvestigatorConfig
	ragCfg   config.RAGConfig
	ragIdx   *rag.Index
	proxy    config.ProxyConfig
	log      Logger
}

func (t *Toolbox) SetLogger(log Logger) {
	t.log = log
}

func NewToolbox(repoPath string, invCfg config.InvestigatorConfig, ragCfg config.RAGConfig, ragIdx *rag.Index, proxy config.ProxyConfig) *Toolbox {
	invCfg.Normalize()
	ragCfg.Normalize()
	proxy.Normalize()
	return &Toolbox{repoPath: repoPath, cfg: invCfg, ragCfg: ragCfg, ragIdx: ragIdx, proxy: proxy}
}

func (t *Toolbox) RepoPath() string {
	return t.repoPath
}

func (t *Toolbox) Run(ctx context.Context, a Action) (string, error) {
	switch a.Action {
	case ActionSearch:
		return t.search(ctx, a.Query)
	case ActionRead:
		return t.readFile(a.Path, a.StartLine, a.EndLine)
	case ActionListDir:
		return t.listDir(a.Path)
	case ActionFetchURL:
		return t.fetchURL(ctx, a.URL)
	case ActionWebSearch:
		return t.webSearch(ctx, a.Query)
	case ActionRAGSearch:
		return t.ragSearch(a.Query)
	case ActionRepoValidate:
		return t.repoValidate(a.Query)
	default:
		return "", fmt.Errorf("unsupported tool action %q", a.Action)
	}
}

func (t *Toolbox) resolvePath(rel string) (string, error) {
	rel = normalizeRelPath(rel)
	if rel == "" {
		return t.repoPath, nil
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

func normalizeRelPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimPrefix(p, "/")
	return filepath.ToSlash(filepath.Clean(p))
}

func (t *Toolbox) listDir(rel string) (string, error) {
	dir, err := t.resolvePath(rel)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", rel)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	base := normalizeRelPath(rel)
	if base == "." {
		base = ""
	}
	for _, e := range entries {
		if e.Name() == ".git" {
			continue
		}
		name := e.Name()
		if base != "" {
			name = base + "/" + name
		}
		if e.IsDir() {
			name += "/"
		}
		b.WriteString(name)
		b.WriteByte('\n')
		if b.Len() > 8000 {
			b.WriteString("…(truncated)\n")
			break
		}
	}
	return strings.TrimSpace(b.String()), nil
}

func (t *Toolbox) readFile(rel string, start, end int) (string, error) {
	full, err := t.resolvePath(rel)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(full)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("is a directory")
	}

	data, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	if len(data) > t.cfg.ReadFileMaxBytes {
		data = data[:t.cfg.ReadFileMaxBytes]
	}

	lines := strings.Split(string(data), "\n")
	total := len(lines)
	if start <= 0 {
		start = 1
	}
	if end <= 0 || end < start {
		end = start + t.cfg.ReadFileMaxLines - 1
	}
	if end-start+1 > t.cfg.ReadFileMaxLines {
		end = start + t.cfg.ReadFileMaxLines - 1
	}
	if start > total {
		return fmt.Sprintf("file %s has %d lines; start_line %d out of range", rel, total, start), nil
	}
	if end > total {
		end = total
	}

	var b strings.Builder
	fmt.Fprintf(&b, "file: %s (total %d lines)\n", rel, total)
	for i := start; i <= end; i++ {
		fmt.Fprintf(&b, "%4d| %s\n", i, lines[i-1])
	}
	if end < total {
		b.WriteString("…(more lines below)\n")
	}
	return b.String(), nil
}

func (t *Toolbox) search(ctx context.Context, query string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("empty query")
	}
	hits := grepContent(ctx, t.repoPath, query, t.cfg.SearchMaxHits)
	if len(hits) == 0 {
		return "no matches", nil
	}
	var b strings.Builder
	for _, h := range hits {
		fmt.Fprintf(&b, "%s:%d: %s\n", h.Path, h.Line, h.Text)
	}
	return strings.TrimSpace(b.String()), nil
}

func (t *Toolbox) ragSearch(query string) (string, error) {
	if !t.ragCfg.On() || t.ragIdx == nil {
		return "", fmt.Errorf("local knowledge RAG disabled or index not ready")
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("empty query")
	}
	logf(t.log, "Investigator rag_search (知识库): %q", query)
	hits := t.ragIdx.Search(query, t.ragCfg.SearchTopK)
	logf(t.log, "Investigator rag_search · %d hits", len(hits))
	if len(hits) == 0 {
		return "no results (检查 knowledge/datasheets 与 standards 是否已放入并重建索引)", nil
	}
	return rag.FormatHits(hits), nil
}

func (t *Toolbox) repoValidate(standardName string) (string, error) {
	standardsDir := filepath.Join(config.KnowledgeDir(t.ragCfg), "standards")
	standardName = strings.TrimSpace(standardName)
	if standardName == "" {
		standardName = t.ragCfg.DefaultStandard
	}
	if standardName == "" {
		names, _ := repovalidate.ListStandards(standardsDir)
		if len(names) == 0 {
			return "", fmt.Errorf("no standards in %s", standardsDir)
		}
		standardName = names[0]
	}
	logf(t.log, "Investigator repo_validate: standard=%q path=%s", standardName, t.repoPath)
	std, err := repovalidate.LoadStandard(standardsDir, standardName)
	if err != nil {
		return "", err
	}
	report := repovalidate.Validate(t.repoPath, std)
	return report.Format(), nil
}

type grepHit struct {
	Path string
	Line int
	Text string
}
