package worker

import (
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

func TestProcessSemiMode(t *testing.T) {
	cfg := config.Default()
	cfg.IssueAutomation.SetMode(config.ModeSemi)

	store, _ := todo.Load(t.TempDir() + "/todo.json")
	_ = store.Upsert(todo.Item{Repo: "o/r", Number: 1, Title: "x", Status: todo.StatusInTodo})

	w := New(cfg, store)
	n, err := w.Process(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("processed=%d", n)
	}
	got, ok := store.Get("o/r", 1)
	if !ok || got.Status != todo.StatusReady || got.Draft == "" {
		t.Fatalf("status=%v draft=%q", got.Status, got.Draft)
	}
}

func TestProcessManualSkips(t *testing.T) {
	cfg := config.Default()
	cfg.IssueAutomation.SetMode(config.ModeManual)

	store, _ := todo.Load(t.TempDir() + "/todo.json")
	_ = store.Upsert(todo.Item{Repo: "o/r", Number: 2, Status: todo.StatusInTodo})

	w := New(cfg, store)
	n, err := w.Process(t.Context())
	if err != nil || n != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	got, _ := store.Get("o/r", 2)
	if got.Status != todo.StatusInTodo {
		t.Fatalf("status=%v", got.Status)
	}
}
