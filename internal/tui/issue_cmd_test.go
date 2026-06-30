package tui

import (
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

func TestParseIssueArgsWithRepoHash(t *testing.T) {
	store, _ := todo.Load(t.TempDir() + "/todo.json")
	repo, num, err := parseIssueArgs([]string{"/issue", "tangjie133/test#30"}, store, "tangjie133/Ops-Agent")
	if err != "" || repo != "tangjie133/test" || num != 30 {
		t.Fatalf("repo=%q num=%d err=%q", repo, num, err)
	}
}

func TestParseIssueArgsFromTodo(t *testing.T) {
	store, _ := todo.Load(t.TempDir() + "/todo.json")
	_ = store.Upsert(todo.Item{Repo: "tangjie133/test", Number: 30, Title: "x", Status: todo.StatusInTodo})

	repo, num, err := parseIssueArgs([]string{"/issue", "30"}, store, "tangjie133/Ops-Agent")
	if err != "" || repo != "tangjie133/test" || num != 30 {
		t.Fatalf("repo=%q num=%d err=%q", repo, num, err)
	}
}

func TestParseIssueArgsAmbiguous(t *testing.T) {
	store, _ := todo.Load(t.TempDir() + "/todo.json")
	_ = store.Upsert(todo.Item{Repo: "a/r1", Number: 1, Title: "x", Status: todo.StatusInTodo})
	_ = store.Upsert(todo.Item{Repo: "a/r2", Number: 1, Title: "y", Status: todo.StatusInTodo})

	_, _, err := parseIssueArgs([]string{"/issue", "1"}, store, "a/r1")
	if err == "" {
		t.Fatal("expected ambiguity error")
	}
}
