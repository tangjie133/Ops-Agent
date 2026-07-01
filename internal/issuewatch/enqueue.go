package issuewatch

// enqueue.go — Webhook 触发时将匹配的 Issue 写入待办队列。

import (
	"fmt"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

// EnqueueResult 描述 webhook 入队结果。
type EnqueueResult struct {
	Added   bool
	Item    todo.Item
	Reason  string // skipped 原因
}

// Enqueue 将单个 issue 按规则写入待办（跨仓库，repo 来自 webhook payload）。
func Enqueue(cfg *config.Config, store *todo.FileStore, repo string, iss github.Issue) (*EnqueueResult, error) {
	if !cfg.IssueWatch.Enabled {
		return &EnqueueResult{Reason: "issue_watch disabled"}, nil
	}
	if repo == "" {
		return nil, fmt.Errorf("empty repo")
	}
	if !Matches(iss, cfg) {
		return &EnqueueResult{Reason: "rule mismatch"}, nil
	}
	if !store.ShouldEnqueue(repo, iss.Number) {
		return &EnqueueResult{Reason: "already queued or dismissed"}, nil
	}
	if store.ActiveCount() >= maxItems(cfg) {
		return &EnqueueResult{Reason: "todo cap reached"}, nil
	}

	labels := make([]string, len(iss.Labels))
	for i, l := range iss.Labels {
		labels[i] = l.Name
	}
	item := todo.Item{
		Repo:   repo,
		Number: iss.Number,
		Title:  iss.Title,
		URL:    iss.URL,
		Labels: labels,
		Status: todo.StatusInTodo,
	}
	if err := store.Upsert(item); err != nil {
		return nil, err
	}
	return &EnqueueResult{Added: true, Item: item}, nil
}

func maxItems(cfg *config.Config) int {
	max := cfg.IssueWatch.Todo.MaxItems
	if max <= 0 {
		return 50
	}
	return max
}
