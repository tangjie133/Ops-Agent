package orchestrator

import (
	"path/filepath"
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/task"
)

func TestEnqueueIssueDisabled(t *testing.T) {
	store, err := task.Load(filepath.Join(t.TempDir(), "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	created, _, err := EnqueueIssue(cfg, store, "o/r", github.Issue{Number: 1, Title: "a"}, "wh:1")
	if err != nil || created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	if _, ok := store.GetByKey(task.MakeTaskKey("o/r", task.TypeIssueToPR, task.SourceKindIssue, 1)); ok {
		t.Fatal("disabled path must not create a task")
	}
}

func TestEnqueueIssueCreatesTask(t *testing.T) {
	store, err := task.Load(filepath.Join(t.TempDir(), "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := enabledCfg()
	created, tk, err := EnqueueIssue(cfg, store, "o/r", github.Issue{Number: 4, Title: "bug", URL: "u"}, "wh:4")
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	if tk.Status != task.StatusCreated || tk.Title != "bug" {
		t.Fatalf("%+v", tk)
	}
	created, _, err = EnqueueIssue(cfg, store, "o/r", github.Issue{Number: 4, Title: "bug"}, "wh:5")
	if err != nil || created {
		t.Fatalf("second created=%v err=%v", created, err)
	}
}
