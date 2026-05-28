package issuewatch

import (
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

func TestEnqueueAddsItem(t *testing.T) {
	cfg := config.Default()
	cfg.IssueWatch.Labels = nil
	store, _ := todo.Load(t.TempDir() + "/todo.json")

	res, err := Enqueue(cfg, store, "o/r", github.Issue{
		Number: 7,
		Title:  "test",
		State:  "OPEN",
		URL:    "https://github.com/o/r/issues/7",
	})
	if err != nil || !res.Added {
		t.Fatalf("err=%v res=%+v", err, res)
	}
}

func TestEnqueueSkipDismissed(t *testing.T) {
	cfg := config.Default()
	cfg.IssueWatch.Labels = nil
	store, _ := todo.Load(t.TempDir() + "/todo.json")
	_ = store.Upsert(todo.Item{Repo: "o/r", Number: 1, Status: todo.StatusDismissed})

	res, err := Enqueue(cfg, store, "o/r", github.Issue{Number: 1, State: "OPEN"})
	if err != nil || res.Added || res.Reason == "" {
		t.Fatalf("expected skip, res=%+v err=%v", res, err)
	}
}
