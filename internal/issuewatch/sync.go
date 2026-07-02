package issuewatch

// sync.go — Issue 关闭/重开时同步待办状态。

import (
	"fmt"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

// RemoveResult 描述 GitHub 关闭 issue 时的待办同步结果。
type RemoveResult struct {
	Removed bool
	Item    todo.Item
	Reason  string
}

// RemoveClosed 将 GitHub 已关闭/删除的 issue 从活跃待办中移除（标记 done）。
func RemoveClosed(store *todo.FileStore, repo string, number int) (*RemoveResult, error) {
	if repo == "" {
		return nil, fmt.Errorf("empty repo")
	}
	it, ok := store.Get(repo, number)
	if !ok {
		return &RemoveResult{Reason: "not in todo"}, nil
	}
	switch it.Status {
	case todo.StatusDone, todo.StatusDismissed:
		return &RemoveResult{Reason: "already inactive"}, nil
	}
	if err := store.Transition(repo, number, todo.StatusDone); err != nil {
		return nil, err
	}
	it.Status = todo.StatusDone
	return &RemoveResult{Removed: true, Item: it}, nil
}

// EnqueueOnComment 在 issue/PR 收到新评论时入队（含历史条目重新激活）。
func EnqueueOnComment(cfg *config.Config, store *todo.FileStore, repo string, iss github.Issue) (*EnqueueResult, error) {
	if iss.State != "" && iss.State != "OPEN" {
		return &EnqueueResult{Reason: "issue closed"}, nil
	}
	if it, ok := store.Get(repo, iss.Number); ok {
		switch it.Status {
		case todo.StatusInTodo, todo.StatusAnalyzing:
			return &EnqueueResult{Reason: "already active"}, nil
		case todo.StatusReady, todo.StatusPosted, todo.StatusFailed, todo.StatusPROpened:
			return reactivateForComment(store, repo, iss, it)
		case todo.StatusFixConfirmed, todo.StatusRefactoring:
			return &EnqueueResult{Reason: "fix in progress"}, nil
		case todo.StatusDone, todo.StatusDismissed:
			return Reopen(cfg, store, repo, iss)
		}
	}
	return Enqueue(cfg, store, repo, iss)
}

// reactivateForComment 将已处理/失败的条目重置为 in_todo，供 Worker 根据新评论重新分析。
func reactivateForComment(store *todo.FileStore, repo string, iss github.Issue, it todo.Item) (*EnqueueResult, error) {
	it.Title = iss.Title
	it.URL = iss.URL
	it.Labels = issueLabels(iss)
	it.Status = todo.StatusInTodo
	it.Draft = ""
	if err := store.Upsert(it); err != nil {
		return nil, err
	}
	return &EnqueueResult{Added: true, Item: it}, nil
}

func issueLabels(iss github.Issue) []string {
	labels := make([]string, len(iss.Labels))
	for i, l := range iss.Labels {
		labels[i] = l.Name
	}
	return labels
}

// Reopen 将 GitHub 重新打开的 issue 写回待办（含曾关闭的条目）。
func Reopen(cfg *config.Config, store *todo.FileStore, repo string, iss github.Issue) (*EnqueueResult, error) {
	if !cfg.IssueWatch.Enabled {
		return &EnqueueResult{Reason: "issue_watch disabled"}, nil
	}
	if repo == "" {
		return nil, fmt.Errorf("empty repo")
	}
	if !Matches(iss, cfg) {
		return &EnqueueResult{Reason: "rule mismatch"}, nil
	}

	if it, ok := store.Get(repo, iss.Number); ok {
		switch it.Status {
		case todo.StatusInTodo, todo.StatusAnalyzing, todo.StatusReady, todo.StatusPosted, todo.StatusFailed:
			return &EnqueueResult{Reason: "already active"}, nil
		case todo.StatusDone, todo.StatusDismissed:
			if store.ActiveCount() >= maxItems(cfg) {
				return &EnqueueResult{Reason: "todo cap reached"}, nil
			}
			labels := make([]string, len(iss.Labels))
			for i, l := range iss.Labels {
				labels[i] = l.Name
			}
			it.Title = iss.Title
			it.URL = iss.URL
			it.Labels = labels
			it.Status = todo.StatusInTodo
			if err := store.Upsert(it); err != nil {
				return nil, err
			}
			return &EnqueueResult{Added: true, Item: it}, nil
		}
	}

	if !store.ShouldEnqueue(repo, iss.Number) {
		return &EnqueueResult{Reason: "already queued or dismissed"}, nil
	}
	return Enqueue(cfg, store, repo, iss)
}
