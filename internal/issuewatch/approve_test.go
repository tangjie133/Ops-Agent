package issuewatch

import (
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

func TestConfirmFixPR(t *testing.T) {
	store, _ := todo.Load(t.TempDir() + "/todo.json")
	_ = store.Upsert(todo.Item{Repo: "o/r", Number: 1, Title: "x", Status: todo.StatusReady})

	res, err := ConfirmFixPR(store, "o/r", 1)
	if err != nil || !res.Confirmed {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	it, _ := store.Get("o/r", 1)
	if it.Status != todo.StatusFixConfirmed {
		t.Fatalf("status=%s", it.Status)
	}

	res, err = ConfirmFixPR(store, "o/r", 1)
	if err != nil || res.Confirmed || res.Reason != "already confirmed" {
		t.Fatalf("res=%+v", res)
	}

	res, err = ConfirmFixPR(store, "o/r", 99)
	if err != nil || res.Confirmed || res.Reason != "not in todo" {
		t.Fatalf("res=%+v", res)
	}
}
