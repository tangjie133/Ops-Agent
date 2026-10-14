package orchestrator

import (
	"context"

	"github.com/ZzedJay/Ops-Agent/internal/task"
)

// ProcessCreated 处理队列中第一条 created 任务。没有任务时 ran 为 false。
func ProcessCreated(ctx context.Context, orch *Orchestrator, store *task.FileStore) (bool, error) {
	var next *task.Task
	for _, tk := range store.List() {
		if tk.Status == task.StatusCreated {
			item := tk
			next = &item
			break
		}
	}
	if next == nil {
		return false, nil
	}
	out, err := orch.RunIssueToPR(ctx, RunRequest{
		Repo: next.Repo, Number: next.SourceNumber, Title: next.Title, URL: next.URL,
		SourceKind: next.SourceKind, TriggerHash: "tui:" + next.ID,
	})
	if err != nil {
		return true, err
	}
	if out.Skipped {
		return false, nil
	}
	if out.Task.Status == task.StatusFailed {
		return true, err
	}
	return true, nil
}
