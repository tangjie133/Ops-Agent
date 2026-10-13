package orchestrator

import (
	"fmt"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/task"
)

// EnqueueIssue 在 issue_to_pr 启用时创建或复用任务，不立即开始执行。
func EnqueueIssue(cfg *config.Config, store *task.FileStore, repo string, iss github.Issue, triggerHash string) (bool, task.Task, error) {
	_ = triggerHash
	if cfg == nil || !cfg.Task.IssueToPR.Enabled {
		return false, task.Task{}, nil
	}
	if store == nil {
		return false, task.Task{}, fmt.Errorf("task store required")
	}
	return store.Ensure(task.Task{
		Repo:         repo,
		Type:         task.TypeIssueToPR,
		SourceKind:   task.SourceKindIssue,
		SourceNumber: iss.Number,
		Title:        iss.Title,
		URL:          iss.URL,
	})
}
