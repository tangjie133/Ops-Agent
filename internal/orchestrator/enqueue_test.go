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
	cfg.Task.IssueToPR.Enabled = false
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

func TestEnqueuePRReusesTask(t *testing.T) {
	store, err := task.Load(filepath.Join(t.TempDir(), "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Task.IssueToPR.Enabled = true
	_, first, err := EnqueuePR(cfg, store, "o/r", 7, "title", "url")
	if err != nil {
		t.Fatal(err)
	}
	created, second, err := EnqueuePR(cfg, store, "o/r", 7, "title", "url")
	if err != nil {
		t.Fatal(err)
	}
	if created || second.ID != first.ID || second.SourceKind != task.SourceKindPR {
		t.Fatalf("created=%v first=%s second=%+v", created, first.ID, second)
	}
}
