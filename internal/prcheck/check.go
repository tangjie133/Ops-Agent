package prcheck

// check.go — 对当前或指定 PR 执行规则检测（merge 状态、CI 等）。

import (
	"context"
	"fmt"

	"github.com/ZzedJay/Ops-Agent/internal/github"
)

// Result PR 检测结果。
type Result struct {
	OK       bool
	Repo     string
	PRNumber int
	PRURL    string
	PRTitle  string
	Failures []string
}

type Options struct {
	Repo     string // 空 = 当前 git 仓库
	PRNumber int    // 0 = 当前分支 PR
}

// Check 执行 PR 规则检测并返回失败项列表。
func Check(ctx context.Context, gh *github.Client, opts Options) (*Result, error) {
	repo := opts.Repo
	if repo == "" {
		var err error
		repo, err = gh.RepoFromCwd(ctx)
		if err != nil {
			return nil, err
		}
	}

	num := opts.PRNumber
	if num == 0 {
		pr, err := gh.PRViewCurrent(ctx, repo)
		if err != nil {
			return nil, fmt.Errorf("no PR for current branch: %w", err)
		}
		num = pr.Number
	}

	pr, err := gh.PRView(ctx, repo, num)
	if err != nil {
		return nil, err
	}

	failures := evaluate(pr)
	return &Result{
		Repo:     repo,
		PRNumber: pr.Number,
		PRURL:    pr.URL,
		PRTitle:  pr.Title,
		OK:       len(failures) == 0,
		Failures: failures,
	}, nil
}
