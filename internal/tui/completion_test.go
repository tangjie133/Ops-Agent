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

func TestCompleteModeArgs(t *testing.T) {
	got := computeCompletions("/mode ", nil)
	if len(got) != 3 {
		t.Fatalf("expected 3 mode args, got %v", got)
	}
	got = computeCompletions("/mode s", nil)
	if len(got) == 0 || got[0].Text != "/mode semi" {
		t.Fatalf("got %v", got)
	}
}

func TestCompleteIssueFromTodos(t *testing.T) {
	todos := []todo.Item{{Number: 42, Title: "hello world"}}
	got := computeCompletions("/issue ", todos)
	if len(got) != 1 || got[0].Text != "/issue 42" {
		t.Fatalf("got %v", got)
	}
}

func TestGhostSuffix(t *testing.T) {
	comps := computeCompletions("/che", nil)
	if ghostSuffix("/che", comps) != "ck" {
		t.Fatalf("suffix=%q", ghostSuffix("/che", comps))
	}
}

func TestNoCompletionForPlainText(t *testing.T) {
	if len(computeCompletions("hello", nil)) != 0 {
		t.Fatal("expected no completions")
	}
}
