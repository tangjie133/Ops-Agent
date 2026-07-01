package libtest

// runner.go — 克隆仓库到 workspace 并执行格式规范与 demo 检测。

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/repovalidate"
)

// RunCheck 克隆/更新到 workspace 并执行格式 + demo 检测。
func RunCheck(ctx context.Context, gh *github.Client, cfg *config.Config, item Item) (workspace string, report string, pass bool, err error) {
	if cfg == nil {
		return "", "", false, fmt.Errorf("nil config")
	}
	cfg.LibTest.Normalize()
	cfg.AI.RAG.Normalize()

	standardsDir := filepath.Join(config.KnowledgeDir(cfg.AI.RAG), "standards")
	std, err := repovalidate.LoadStandard(standardsDir, cfg.LibTest.Standard)
	if err != nil {
		return "", "", false, err
	}

	root := cfg.LibTest.WorkspacesRoot()
	dest := filepath.Join(root, filepath.FromSlash(item.Repo))
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", "", false, err
	}

	if err := cloneOrUpdate(ctx, gh, cfg, item.Repo, item.Ref, dest); err != nil {
		return "", "", false, err
	}

	formatReport := repovalidate.Validate(dest, std)
	demoReport := CheckDemos(dest, cfg.LibTest, std)

	pass = formatReport.OK && demoReport.OK
	var b strings.Builder
	b.WriteString(formatReport.Format())
	b.WriteString("\n\n")
	b.WriteString(demoReport.Format())
	if !pass {
		b.WriteString("\n\n── 汇总 ──\n验收未通过（格式或 demo 不符合规范），请修改后重新 push。")
	} else {
		b.WriteString("\n\n── 汇总 ──\n验收通过。")
	}
	return dest, strings.TrimSpace(b.String()), pass, nil
}

func cloneOrUpdate(ctx context.Context, gh *github.Client, cfg *config.Config, repo, ref, dest string) error {
	if gh == nil {
		return fmt.Errorf("github client required")
	}
	if isGitRepo(dest) {
		if err := gh.GitPull(ctx, dest, cfg.Proxy); err != nil {
			_ = os.RemoveAll(dest)
		} else if ref != "" && ref != "HEAD" {
			return gh.GitCheckout(ctx, dest, ref, cfg.Proxy)
		} else {
			return nil
		}
	}
	if err := gh.CloneRepo(ctx, repo, dest); err != nil {
		return err
	}
	if ref != "" && ref != "HEAD" {
		return gh.GitCheckout(ctx, dest, ref, cfg.Proxy)
	}
	return nil
}

func isGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// Enqueue 新库事件入队。
func Enqueue(store *FileStore, cfg config.LibTestConfig, repo, ref, trigger, title string) (bool, error) {
	cfg.Normalize()
	if !cfg.Enabled {
		return false, nil
	}
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return false, fmt.Errorf("empty repo")
	}
	if ref == "" {
		ref = "HEAD"
	}
	if !store.ShouldEnqueue(repo, ref) {
		return false, nil
	}
	active := 0
	for _, it := range store.List() {
		switch it.Status {
		case StatusDismissed:
			continue
		default:
			active++
		}
	}
	if active >= cfg.MaxQueueItems {
		return false, fmt.Errorf("libtest queue full")
	}
	item := Item{
		Repo:    repo,
		Ref:     ref,
		Trigger: trigger,
		Title:   title,
		Status:  StatusPending,
	}
	return true, store.Upsert(item)
}
