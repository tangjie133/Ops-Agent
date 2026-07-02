package pr

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/netproxy"
)

const maxLogLines = 30
const maxDiffStatBytes = 12000

// BranchInfo 当前分支相对默认分支的 PR 上下文。
type BranchInfo struct {
	Repo          string
	HeadBranch    string
	BaseBranch    string
	BaseRef       string // 用于 git log/diff 的 ref（优先 origin/base）
	ExistingPR    *github.PullRequest
	Commits       string
	DiffStat      string
	CommitCount   int
	NeedsPush     bool
	UnpushedCount int    // 未 push 的提交数；-1 表示无 upstream 且远程无该分支
	PushHint      string // 提交前提示
}

// GatherBranchInfo 收集当前 cwd 仓库的分支、提交与 diff 统计。
func GatherBranchInfo(ctx context.Context, gh *github.Client, proxy config.ProxyConfig, repo string) (*BranchInfo, error) {
	return GatherBranchInfoInDir(ctx, gh, proxy, repo, "")
}

// GatherBranchInfoInDir 在指定目录收集分支、提交与 diff 统计（dir 空则用进程 cwd）。
func GatherBranchInfoInDir(ctx context.Context, gh *github.Client, proxy config.ProxyConfig, repo, dir string) (*BranchInfo, error) {
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
	head, err := gitCurrentBranch(ctx, proxy, dir)
	if err != nil {
		return nil, err
	}
	if head == base {
		return nil, fmt.Errorf("当前在默认分支 %s，请先 checkout 功能分支", base)
	}

	baseRef, err := resolveDiffBaseRef(ctx, proxy, dir, base)
	if err != nil {
		return nil, err
	}

	info := &BranchInfo{
		Repo:       repo,
		HeadBranch: head,
		BaseBranch: base,
		BaseRef:    baseRef,
	}

	if pr, err := gh.PRViewCurrent(ctx, repo); err == nil && pr != nil {
		info.ExistingPR = pr
	}

	commits, err := gitLogOneline(ctx, proxy, dir, baseRef, head, maxLogLines)
	if err != nil {
		return nil, fmt.Errorf("读取提交记录失败: %w", err)
	}
	info.Commits = commits
	info.CommitCount = countCommitLines(commits)

	diffStat, err := gitDiffStat(ctx, proxy, dir, baseRef, head, maxDiffStatBytes)
	if err != nil {
		return nil, fmt.Errorf("读取 diff 统计失败: %w", err)
	}
	info.DiffStat = diffStat

	needsPush, unpushed, hint := gitBranchPushState(ctx, proxy, dir, head)
	info.NeedsPush = needsPush
	info.UnpushedCount = unpushed
	info.PushHint = hint

	if info.CommitCount == 0 && info.ExistingPR == nil {
		return nil, fmt.Errorf("相对 %s 无新提交，无法创建 PR（请先 commit 或确认 base 分支正确）", base)
	}

	return info, nil
}

func resolveDiffBaseRef(ctx context.Context, proxy config.ProxyConfig, dir, base string) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		return "", fmt.Errorf("empty base branch")
	}
	originRef := "origin/" + base
	if gitRefExists(ctx, proxy, dir, originRef) {
		return originRef, nil
	}
	if gitRefExists(ctx, proxy, dir, base) {
		return base, nil
	}
	return "", fmt.Errorf("找不到分支 %s 或 %s，请先 git fetch origin", base, originRef)
}

func gitRefExists(ctx context.Context, proxy config.ProxyConfig, dir, ref string) bool {
	_, err := gitRun(ctx, proxy, dir, "rev-parse", "--verify", ref+"^{commit}")
	return err == nil
}

func countCommitLines(commits string) int {
	commits = strings.TrimSpace(commits)
	if commits == "" {
		return 0
	}
	return strings.Count(commits, "\n") + 1
}

// gitBranchPushState 检查当前分支相对远程的 push 状态。
func gitBranchPushState(ctx context.Context, proxy config.ProxyConfig, dir, head string) (needsPush bool, unpushed int, hint string) {
	upOut, upErr := gitRun(ctx, proxy, dir, "rev-parse", "--abbrev-ref", "@{upstream}")
	if upErr != nil {
		remoteOut, remoteErr := gitRun(ctx, proxy, dir, "ls-remote", "--heads", "origin", head)
		if remoteErr != nil || strings.TrimSpace(string(remoteOut)) == "" {
			hint = fmt.Sprintf("分支 %s 尚未 push，请先: git push -u origin %s", head, head)
			return true, -1, hint
		}
		hint = fmt.Sprintf("分支 %s 在远程已存在但未设置 upstream，建议: git branch -u origin/%s", head, head)
		return false, 0, hint
	}

	upstream := strings.TrimSpace(string(upOut))
	countOut, countErr := gitRun(ctx, proxy, dir, "rev-list", "--count", upstream+"..HEAD")
	if countErr != nil {
		hint = fmt.Sprintf("无法比较 %s 与 HEAD，请确认已 push: git push -u origin %s", upstream, head)
		return true, -1, hint
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(countOut)))
	if n > 0 {
		hint = fmt.Sprintf("有 %d 个提交未 push 到 %s，请先: git push", n, upstream)
		return true, n, hint
	}
	return false, 0, ""
}

func gitCurrentBranch(ctx context.Context, proxy config.ProxyConfig, dir string) (string, error) {
	out, err := gitRun(ctx, proxy, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git branch: %w", err)
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" || branch == "HEAD" {
		return "", fmt.Errorf("detached HEAD，无法创建 PR")
	}
	return branch, nil
}

func gitLogOneline(ctx context.Context, proxy config.ProxyConfig, dir, base, head string, limit int) (string, error) {
	rangeRef := base + ".." + head
	out, err := gitRun(ctx, proxy, dir, "log", rangeRef, "--oneline", fmt.Sprintf("-%d", limit))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitDiffStat(ctx context.Context, proxy config.ProxyConfig, dir, base, head string, maxBytes int) (string, error) {
	rangeRef := base + "..." + head
	out, err := gitRun(ctx, proxy, dir, "diff", rangeRef, "--stat")
	if err != nil {
		return "", err
	}
	s := strings.TrimSpace(string(out))
	if len(s) > maxBytes {
		s = s[:maxBytes] + "\n…（diff 统计已截断）"
	}
	return s, nil
}

func gitRun(ctx context.Context, proxy config.ProxyConfig, dir string, args ...string) ([]byte, error) {
	gitArgs := args
	if dir != "" {
		gitArgs = append([]string{"-C", dir}, args...)
	}
	cmd := exec.CommandContext(ctx, "git", gitArgs...)
	netproxy.ConfigureCmd(cmd, proxy)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("%w (%s)", err, strings.TrimSpace(string(out)))
	}
	return out, nil
}
