package tui

import (
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

func TestCompleteCommandPrefix(t *testing.T) {
	got := computeCompletions("/st", nil)
	if len(got) == 0 || got[0].Text != "/status" {
		t.Fatalf("got %v", got)
	}
}

func TestCompleteModeCommand(t *testing.T) {
	got := computeCompletions("/mode", nil)
	if len(got) != 1 || got[0].Text != "/mode" {
		t.Fatalf("got %v", got)
	}
	if len(computeCompletions("/mode ", nil)) != 0 {
		t.Fatal("mode menu has no sub-args")
	}
}

func TestCompleteIssueFromTodos(t *testing.T) {
	todos := []todo.Item{{Repo: "o/r", Number: 42, Title: "hello world"}}
	got := computeCompletions("/issue ", todos)
	if len(got) != 1 || got[0].Text != "/issue o/r#42" {
		t.Fatalf("got %v", got)
	}
}

func TestGhostSuffix(t *testing.T) {
	comps := computeCompletions("/web", nil)
	if ghostSuffix("/web", comps) != "hook" {
		t.Fatalf("suffix=%q", ghostSuffix("/web", comps))
	}
}

func TestNoCompletionForPlainText(t *testing.T) {
	if len(computeCompletions("hello", nil)) != 0 {
		t.Fatal("expected no completions")
	}
}
