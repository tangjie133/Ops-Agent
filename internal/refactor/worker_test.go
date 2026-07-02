package refactor

import (
	"context"
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

func TestIsApprovePRStyleActions(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{`{"action":"done","body":"ok"}`, ActionDone},
		{`{"action":"edit_file","path":"a.go","content":"package main"}`, ActionEditFile},
		{`{"action":"edit_file","path":"a.go","old":"x","new":"y"}`, ActionEditFile},
	}
	for _, tc := range cases {
		a, err := ParseAction(tc.raw)
		if err != nil {
			t.Fatalf("ParseAction: %v", err)
		}
		if a.Action != tc.want {
			t.Fatalf("got %q want %q", a.Action, tc.want)
		}
	}
}

func TestRunRequiresFixConfirmed(t *testing.T) {
	cfg := config.Default()
	cfg.IssueAutomation.RefactorPR.Enabled = true

	store, _ := todo.Load(t.TempDir() + "/todo.json")
	_ = store.Upsert(todo.Item{Repo: "o/r", Number: 1, Title: "x", Status: todo.StatusReady})

	w := New(cfg, store, nil)
	_, err := w.Run(context.Background(), Request{Repo: "o/r", Number: 1})
	if err == nil || err.Error() != "状态须为 fix_confirmed，当前 ready" {
		t.Fatalf("err=%v", err)
	}
}

func TestProcessNextDisabled(t *testing.T) {
	cfg := config.Default()
	store, _ := todo.Load(t.TempDir() + "/todo.json")
	_ = store.Upsert(todo.Item{Repo: "o/r", Number: 1, Status: todo.StatusFixConfirmed})

	w := New(cfg, store, nil)
	res, err := w.ProcessNext(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Done {
		t.Fatalf("res=%+v", res)
	}
}

func TestBranchNameConfig(t *testing.T) {
	cfg := config.RefactorPRConfig{BranchPrefix: "fix/issue-"}
	cfg.Normalize()
	if got := cfg.BranchName(42); got != "fix/issue-42" {
		t.Fatalf("got %q", got)
	}
}
