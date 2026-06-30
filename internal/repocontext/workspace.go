package repocontext

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/netproxy"
)

// Workspace 管理本地仓库克隆缓存（供 Investigator 工具使用）。
type Workspace struct {
	cacheRoot string
	gh        *github.Client
	cfg       config.RepoContextConfig
	proxy     config.ProxyConfig
}

func NewWorkspace(cfg config.RepoContextConfig, proxy config.ProxyConfig, gh *github.Client) *Workspace {
	cfg.Normalize()
	proxy.Normalize()
	return &Workspace{
		cacheRoot: config.ReposCacheDir(),
		gh:        gh,
		cfg:       cfg,
		proxy:     proxy,
	}
}

func (w *Workspace) Enabled() bool {
	return w.cfg.Enabled
}

// Prepare 确保 repo 已克隆/更新到本地缓存，返回工作目录。
func (w *Workspace) Prepare(ctx context.Context, repo string) (string, error) {
	if !w.cfg.Enabled {
		return "", fmt.Errorf("repo_context disabled")
	}
	if w.gh == nil {
		return "", fmt.Errorf("github client required")
	}
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return "", fmt.Errorf("empty repo")
	}

	dest := filepath.Join(w.cacheRoot, filepath.FromSlash(repo))
	if err := os.MkdirAll(w.cacheRoot, 0o755); err != nil {
		return "", err
	}

	if isGitRepo(dest) {
		if err := w.gitPull(ctx, dest); err != nil {
			_ = os.RemoveAll(dest)
		} else {
			return dest, nil
		}
	}

	if err := w.gh.CloneRepo(ctx, repo, dest); err != nil {
		return "", err
	}
	return dest, nil
}

func isGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

func (w *Workspace) gitPull(ctx context.Context, dir string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "pull", "--ff-only", "--quiet")
	netproxy.ConfigureCmd(cmd, w.proxy)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git pull: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}
