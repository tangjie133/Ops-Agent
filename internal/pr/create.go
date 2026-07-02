package pr

import (
	"context"
	"fmt"

	"github.com/ZzedJay/Ops-Agent/internal/github"
)

// Submit 创建新 PR 或更新已有 PR 的 title/body。
func Submit(ctx context.Context, gh *github.Client, d *Draft) (string, error) {
	if gh == nil || d == nil {
		return "", fmt.Errorf("nil client or draft")
	}
	if d.ExistingPR > 0 {
		if err := gh.PREdit(ctx, d.Repo, d.ExistingPR, d.Title, d.Body); err != nil {
			return "", fmt.Errorf("更新 PR #%d: %w", d.ExistingPR, err)
		}
		if d.ExistingURL != "" {
			return d.ExistingURL, nil
		}
		pr, err := gh.PRView(ctx, d.Repo, d.ExistingPR)
		if err != nil {
			return fmt.Sprintf("已更新 PR #%d", d.ExistingPR), nil
		}
		return pr.URL, nil
	}
	if d.NeedsPush {
		if d.PushHint != "" {
			return "", fmt.Errorf(d.PushHint)
		}
		return "", fmt.Errorf("分支 %s 尚未 push 到远程，请先 git push -u origin %s", d.HeadBranch, d.HeadBranch)
	}
	url, err := gh.PRCreate(ctx, github.PRCreateOpts{
		Repo:  d.Repo,
		Title: d.Title,
		Body:  d.Body,
		Base:  d.BaseBranch,
		Head:  d.HeadBranch,
	})
	if err != nil {
		return "", err
	}
	return url, nil
}
