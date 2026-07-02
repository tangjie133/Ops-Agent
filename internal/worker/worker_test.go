package worker

import (
	"context"
	"fmt"
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

type stubAnalyzer struct {
	draft string
	err   error
}

func (s stubAnalyzer) AnalyzeIssue(_ context.Context, _ string, _ int) (string, error) {
	return s.draft, s.err
}

type stubPoster struct {
	posted []string
	err    error
}

func (s *stubPoster) IssueComment(_ context.Context, repo string, num int, body string) error {
	if s.err != nil {
		return s.err
	}
	s.posted = append(s.posted, fmt.Sprintf("%s#%d:%s", repo, num, body))
	return nil
}

func TestProcessSemiMode(t *testing.T) {
	cfg := config.Default()
	cfg.IssueAutomation.SetMode(config.ModeSemi)

	store, _ := todo.Load(t.TempDir() + "/todo.json")
	_ = store.Upsert(todo.Item{Repo: "o/r", Number: 1, Title: "x", Status: todo.StatusInTodo})

	w := NewWithDeps(cfg, store, stubAnalyzer{draft: "reply draft"}, &stubPoster{})
	res, err := w.Process(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Ready || res.Draft != "reply draft" {
		t.Fatalf("res=%+v", res)
	}
	got, ok := store.Get("o/r", 1)
	if !ok || got.Status != todo.StatusReady || got.Draft != "reply draft" {
		t.Fatalf("status=%v draft=%q", got.Status, got.Draft)
	}
}

func TestProcessManualSkips(t *testing.T) {
	cfg := config.Default()
	cfg.IssueAutomation.SetMode(config.ModeManual)

	store, _ := todo.Load(t.TempDir() + "/todo.json")
	_ = store.Upsert(todo.Item{Repo: "o/r", Number: 2, Status: todo.StatusInTodo})

	w := NewWithDeps(cfg, store, stubAnalyzer{draft: "x"}, &stubPoster{})
	res, err := w.Process(context.Background())
	if err != nil || (res != nil && res.Ready) {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	got, _ := store.Get("o/r", 2)
	if got.Status != todo.StatusInTodo {
		t.Fatalf("status=%v", got.Status)
	}
}

func TestProcessFullAutoPost(t *testing.T) {
	cfg := config.Default()
	cfg.IssueAutomation.SetMode(config.ModeFull)

	store, _ := todo.Load(t.TempDir() + "/todo.json")
	_ = store.Upsert(todo.Item{Repo: "o/r", Number: 3, Title: "y", Status: todo.StatusInTodo})

	poster := &stubPoster{}
	w := NewWithDeps(cfg, store, stubAnalyzer{draft: "auto reply"}, poster)
	res, err := w.Process(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Posted || len(poster.posted) != 1 {
		t.Fatalf("res=%+v posted=%v", res, poster.posted)
	}
	got, _ := store.Get("o/r", 3)
	if got.Status != todo.StatusPosted {
		t.Fatalf("status=%v", got.Status)
	}
}

func TestProcessFullAutoPostReady(t *testing.T) {
	cfg := config.Default()
	cfg.IssueAutomation.SetMode(config.ModeFull)

	store, _ := todo.Load(t.TempDir() + "/todo.json")
	_ = store.Upsert(todo.Item{Repo: "o/r", Number: 5, Title: "ready", Status: todo.StatusReady, Draft: "draft body"})

	poster := &stubPoster{}
	w := NewWithDeps(cfg, store, stubAnalyzer{draft: "unused"}, poster)
	res, err := w.Process(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Posted || len(poster.posted) != 1 {
		t.Fatalf("res=%+v posted=%v", res, poster.posted)
	}
	got, _ := store.Get("o/r", 5)
	if got.Status != todo.StatusPosted {
		t.Fatalf("status=%v", got.Status)
	}
}

func TestPostDraft(t *testing.T) {
	cfg := config.Default()
	store, _ := todo.Load(t.TempDir() + "/todo.json")
	_ = store.Upsert(todo.Item{Repo: "o/r", Number: 4, Title: "z", Status: todo.StatusReady, Draft: "confirmed"})

	poster := &stubPoster{}
	w := NewWithDeps(cfg, store, nil, poster)
	if err := w.PostDraft(context.Background(), "o/r", 4); err != nil {
		t.Fatal(err)
	}
	if len(poster.posted) != 1 {
		t.Fatalf("posted=%v", poster.posted)
	}
	got, _ := store.Get("o/r", 4)
	if got.Status != todo.StatusPosted {
		t.Fatalf("status=%v", got.Status)
	}
}
