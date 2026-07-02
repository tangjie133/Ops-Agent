package pr

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/netproxy"
)

const maxLogLines = 30
const maxDiffStatBytes = 12000

// BranchInfo 当前分支相对默认分支的 PR 上下文。
type BranchInfo struct {
	Repo       string
	HeadBranch string
	BaseBranch string
	ExistingPR *github.PullRequest
	Commits    string
	DiffStat   string
}

// GatherBranchInfo 收集当前 cwd 仓库的分支、提交与 diff 统计。
func GatherBranchInfo(ctx context.Context, gh *github.Client, proxy config.ProxyConfig, repo string) (*BranchInfo, error) {
	if gh == nil {
		return nil, fmt.Errorf("nil github client")
	}
	if repo == "" {
		var err error
		repo, err = gh.RepoFromCwd(ctx)
		if err != nil {
			return nil, err
		}
	}
	base, err := gh.RepoDefaultBranch(ctx, repo)
	if err != nil {
		return nil, err
	}
	head, err := gitCurrentBranch(ctx, proxy)
	if err != nil {
		return nil, err
	}
	if head == base {
		return nil, fmt.Errorf("当前在默认分支 %s，请先 checkout 功能分支", base)
	}

	info := &BranchInfo{
		Repo:       repo,
		HeadBranch: head,
		BaseBranch: base,
	}

	if pr, err := gh.PRViewCurrent(ctx, repo); err == nil && pr != nil {
		info.ExistingPR = pr
	}

	info.Commits, _ = gitLogOneline(ctx, proxy, base, head, maxLogLines)
	info.DiffStat, _ = gitDiffStat(ctx, proxy, base, head, maxDiffStatBytes)
	return info, nil
}

func gitCurrentBranch(ctx context.Context, proxy config.ProxyConfig) (string, error) {
	out, err := gitRun(ctx, proxy, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git branch: %w", err)
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" || branch == "HEAD" {
		return "", fmt.Errorf("detached HEAD，无法创建 PR")
	}
	return branch, nil
}

func gitLogOneline(ctx context.Context, proxy config.ProxyConfig, base, head string, limit int) (string, error) {
	rangeRef := base + ".." + head
	out, err := gitRun(ctx, proxy, "log", rangeRef, "--oneline", fmt.Sprintf("-%d", limit))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitDiffStat(ctx context.Context, proxy config.ProxyConfig, base, head string, maxBytes int) (string, error) {
	rangeRef := base + "..." + head
	out, err := gitRun(ctx, proxy, "diff", rangeRef, "--stat")
	if err != nil {
		return "", err
	}
	s := strings.TrimSpace(string(out))
	if len(s) > maxBytes {
		s = s[:maxBytes] + "\n…（diff 统计已截断）"
	}
	return s, nil
}

func gitRun(ctx context.Context, proxy config.ProxyConfig, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	netproxy.ConfigureCmd(cmd, proxy)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("%w (%s)", err, strings.TrimSpace(string(out)))
	}
	return out, nil
}
