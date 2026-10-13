package task

import (
	"path/filepath"
	"testing"
)

func TestEnsureReusesTaskKey(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	in := Task{Repo: "o/r", Type: TypeIssueToPR, SourceKind: SourceKindIssue, SourceNumber: 45, Title: "bug"}
	created, t1, err := s.Ensure(in)
	if err != nil || !created {
		t.Fatalf("first ensure created=%v err=%v", created, err)
	}
	created, t2, err := s.Ensure(in)
	if err != nil || created {
		t.Fatalf("second ensure created=%v err=%v", created, err)
	}
	if t1.ID != t2.ID {
		t.Fatalf("ids %s vs %s", t1.ID, t2.ID)
	}
}

func TestBeginRunSkipsDuplicateTrigger(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, tk, err := s.Ensure(Task{Repo: "o/r", Type: TypeIssueToPR, SourceKind: SourceKindIssue, SourceNumber: 1})
	if err != nil {
		t.Fatal(err)
	}
	r1, skipped, err := s.BeginRun(tk.Key, "wh:1")
	if err != nil || skipped {
		t.Fatalf("first run skipped=%v err=%v", skipped, err)
	}
	_, skipped, err = s.BeginRun(tk.Key, "wh:1")
	if err != nil || !skipped {
		t.Fatalf("dup run skipped=%v err=%v", skipped, err)
	}
	if r1.ID == "" {
		t.Fatal("empty run id")
	}
}

func TestBeginRunRejectsParallel(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, tk, err := s.Ensure(Task{Repo: "o/r", Type: TypeIssueToPR, SourceKind: SourceKindIssue, SourceNumber: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, skipped, err := s.BeginRun(tk.Key, "a"); err != nil || skipped {
		t.Fatal(err)
	}
	_, skipped, err := s.BeginRun(tk.Key, "b")
	if err == nil || skipped {
		t.Fatalf("expected parallel error, skipped=%v err=%v", skipped, err)
	}
}

func TestTransitionRejectsIllegal(t *testing.T) {
	if CanTransition(StatusDone, StatusExecuting) {
		t.Fatal("done must be terminal")
	}
	if !CanTransition(StatusCreated, StatusCollectingContext) {
		t.Fatal("created -> collecting_context")
	}
}
