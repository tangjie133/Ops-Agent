package headless

import (
	"context"
	"os"
	"path/filepath"

	"github.com/ZzedJay/Ops-Agent/internal/budget"
	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/executor/internalagent"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/orchestrator"
	"github.com/ZzedJay/Ops-Agent/internal/publisher"
	"github.com/ZzedJay/Ops-Agent/internal/refactor"
	"github.com/ZzedJay/Ops-Agent/internal/task"
)

// ShouldRunIssueToPR 判断是否走 opt-in 的 issue_to_pr headless 入口。
func ShouldRunIssueToPR() bool {
	return os.Getenv("OPS_AGENT_ISSUE_TO_PR") == "1"
}

func runIssueToPR(cfg *config.Config) int {
	store, err := task.Load(taskStorePath(cfg))
	if err != nil {
		return 1
	}
	if !hasCreated(store) {
		return 0
	}
	local := os.Getenv("OPS_AGENT_LOCAL_REPO")
	if local == "" {
		return 1
	}
	gh := github.NewClient()
	policy, err := budget.FromConfig(cfg.Task.Budget)
	if err != nil {
		return 1
	}
	orch := orchestrator.New(orchestrator.Deps{
		Tasks:      store,
		Exec:       internalagent.Adapter{Engine: refactor.NewEngine(cfg, gh)},
		Publish:    publisher.New(gh),
		Config:     cfg,
		Policy:     policy,
		MemoryPath: filepath.Join(filepath.Dir(taskStorePath(cfg)), "memory.jsonl"),
	})
	_, err = ProcessCreated(context.Background(), orch, store, local)
	if err != nil {
		return 1
	}
	return 0
}

// ProcessCreated 处理队列中第一条 created 任务。没有任务时 ran 为 false。
func ProcessCreated(ctx context.Context, orch *orchestrator.Orchestrator, store *task.FileStore, localRepo string) (bool, error) {
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
	out, err := orch.RunIssueToPR(ctx, orchestrator.RunRequest{
		Repo: next.Repo, Number: next.SourceNumber, Title: next.Title, URL: next.URL,
		TriggerHash: "headless:" + next.ID, LocalRepo: localRepo, Base: "main",
	})
	if err != nil {
		return true, err
	}
	if out.Task.Status == task.StatusFailed {
		return true, err
	}
	return true, nil
}

func hasCreated(store *task.FileStore) bool {
	for _, tk := range store.List() {
		if tk.Status == task.StatusCreated {
			return true
		}
	}
	return false
}

func taskStorePath(cfg *config.Config) string {
	if cfg != nil && cfg.Task.StorePath != "" {
		return cfg.Task.StorePath
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "tasks.json"
	}
	return filepath.Join(home, ".local", "share", "ops-agent", "tasks.json")
}
