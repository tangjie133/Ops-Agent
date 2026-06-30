package todo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileStoreUpsertAndTransition(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.json")
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	item := Item{Repo: "o/r", Number: 1, Title: "hello", Status: StatusInTodo}
	if err := s.Upsert(item); err != nil {
		t.Fatal(err)
	}
	if s.ActiveCount() != 1 {
		t.Fatalf("active=%d", s.ActiveCount())
	}
	if err := s.Transition("o/r", 1, StatusReady); err != nil {
		t.Fatal(err)
	}
	got, ok := s.Get("o/r", 1)
	if !ok || got.Status != StatusReady {
		t.Fatalf("status=%v ok=%v", got.Status, ok)
	}

	s2, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	got, ok = s2.Get("o/r", 1)
	if !ok || got.Status != StatusReady {
		t.Fatalf("reload status=%v", got.Status)
	}
}

func TestFileStoreReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.json")
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Upsert(Item{Repo: "o/r", Number: 2, Title: "a", Status: StatusInTodo}); err != nil {
		t.Fatal(err)
	}

	s2, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s2.Reload(); err != nil {
		t.Fatal(err)
	}
	if _, ok := s2.Get("o/r", 2); !ok {
		t.Fatal("expected item after reload")
	}
	if err := s2.Reload(); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(path)
	if err := s2.Reload(); err != nil {
		t.Fatal(err)
	}
	if s2.ActiveCount() != 0 {
		t.Fatalf("expected empty after missing file, got %d", s2.ActiveCount())
	}
}

func TestFileStoreShouldEnqueue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.json")
	s, _ := Load(path)

	if !s.ShouldEnqueue("o/r", 1) {
		t.Fatal("new item should enqueue")
	}
	_ = s.Upsert(Item{Repo: "o/r", Number: 1, Title: "a", Status: StatusInTodo})
	if s.ShouldEnqueue("o/r", 1) {
		t.Fatal("in_todo should not re-enqueue")
	}
	_ = s.Transition("o/r", 1, StatusDismissed)
	if s.ShouldEnqueue("o/r", 1) {
		t.Fatal("dismissed should not re-enqueue")
	}
}

func TestFileStoreDismissedNotActive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.json")
	s, _ := Load(path)
	_ = s.Upsert(Item{Repo: "o/r", Number: 2, Title: "x", Status: StatusDismissed})
	if s.ActiveCount() != 0 {
		t.Fatalf("expected 0 active, got %d", s.ActiveCount())
	}
	_ = os.WriteFile(path, []byte("[]"), 0o644)
}
