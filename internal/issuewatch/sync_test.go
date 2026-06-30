package issuewatch

import (
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

func TestRemoveClosed(t *testing.T) {
	store, _ := todo.Load(t.TempDir() + "/todo.json")
	_ = store.Upsert(todo.Item{Repo: "o/r", Number: 1, Title: "x", Status: todo.StatusInTodo})

	res, err := RemoveClosed(store, "o/r", 1)
	if err != nil || !res.Removed {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	it, _ := store.Get("o/r", 1)
	if it.Status != todo.StatusDone {
		t.Fatalf("status=%s", it.Status)
	}

	res, err = RemoveClosed(store, "o/r", 1)
	if err != nil || res.Removed || res.Reason != "already inactive" {
		t.Fatalf("res=%+v", res)
	}

	res, err = RemoveClosed(store, "o/r", 99)
	if err != nil || res.Removed || res.Reason != "not in todo" {
		t.Fatalf("res=%+v", res)
	}
}
func TestEnqueueOnComment(t *testing.T) {
	cfg := config.Default()
	cfg.IssueWatch.Labels = nil
	cfg.IssueWatch.RequireUnassigned = false

	store, _ := todo.Load(t.TempDir() + "/todo.json")

	iss := github.Issue{Number: 10, Title: "old issue", State: "OPEN", URL: "https://github.com/o/r/issues/10"}
	res, err := EnqueueOnComment(cfg, store, "o/r", iss)
	if err != nil || !res.Added {
		t.Fatalf("res=%+v err=%v", res, err)
	}

	res, err = EnqueueOnComment(cfg, store, "o/r", iss)
	if err != nil || res.Added || res.Reason != "already active" {
		t.Fatalf("res=%+v", res)
	}

	closed := github.Issue{Number: 11, Title: "closed", State: "CLOSED"}
	res, err = EnqueueOnComment(cfg, store, "o/r", closed)
	if err != nil || res.Added || res.Reason != "issue closed" {
		t.Fatalf("res=%+v", res)
	}
}

func TestReopenAfterClosed(t *testing.T) {
	cfg := config.Default()
	cfg.IssueWatch.Labels = nil
	cfg.IssueWatch.RequireUnassigned = false

	store, _ := todo.Load(t.TempDir() + "/todo.json")
	_ = store.Upsert(todo.Item{Repo: "o/r", Number: 2, Title: "old", Status: todo.StatusDone})

	iss := github.Issue{Number: 2, Title: "reopened", State: "OPEN", URL: "https://github.com/o/r/issues/2"}
	res, err := Reopen(cfg, store, "o/r", iss)
	if err != nil || !res.Added {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	it, _ := store.Get("o/r", 2)
	if it.Status != todo.StatusInTodo || it.Title != "reopened" {
		t.Fatalf("item=%+v", it)
	}
}
