package tui

import (
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/task"
)

func TestParseIssueArgsWithRepoHash(t *testing.T) {
	store, _ := task.Load(t.TempDir() + "/tasks.json")
	repo, num, err := parseIssueArgs([]string{"/issue", "tangjie133/test#30"}, store, "tangjie133/Ops-Agent")
	if err != "" || repo != "tangjie133/test" || num != 30 {
		t.Fatalf("repo=%q num=%d err=%q", repo, num, err)
	}
}

func TestParseIssueArgsFromTodo(t *testing.T) {
	store, _ := task.Load(t.TempDir() + "/tasks.json")
	_, _, _ = store.Ensure(task.Task{Repo: "tangjie133/test", Type: task.TypeIssueToPR, SourceKind: task.SourceKindIssue, SourceNumber: 30, Title: "x"})

	repo, num, err := parseIssueArgs([]string{"/issue", "30"}, store, "tangjie133/Ops-Agent")
	if err != "" || repo != "tangjie133/test" || num != 30 {
		t.Fatalf("repo=%q num=%d err=%q", repo, num, err)
	}
}

func TestParseIssueArgsAmbiguous(t *testing.T) {
	store, _ := task.Load(t.TempDir() + "/tasks.json")
	_, _, _ = store.Ensure(task.Task{Repo: "a/r1", Type: task.TypeIssueToPR, SourceKind: task.SourceKindIssue, SourceNumber: 1, Title: "x"})
	_, _, _ = store.Ensure(task.Task{Repo: "a/r2", Type: task.TypeIssueToPR, SourceKind: task.SourceKindIssue, SourceNumber: 1, Title: "y"})

	_, _, err := parseIssueArgs([]string{"/issue", "1"}, store, "a/r1")
	if err == "" {
		t.Fatal("expected ambiguity error")
	}
}
