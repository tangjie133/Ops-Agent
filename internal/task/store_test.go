package task

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(s.List()); n != 0 {
		t.Fatalf("tasks=%d", n)
	}
}

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

func TestBeginRunAllowsRetryWhenCreatedAgain(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, tk, err := s.Ensure(Task{Repo: "o/r", Type: TypeIssueToPR, SourceKind: SourceKindIssue, SourceNumber: 3})
	if err != nil {
		t.Fatal(err)
	}
	r1, skipped, err := s.BeginRun(tk.Key, "tui:"+tk.ID)
	if err != nil || skipped {
		t.Fatalf("first skipped=%v err=%v", skipped, err)
	}
	if err := s.ClearActiveRun(tk.ID); err != nil {
		t.Fatal(err)
	}
	tk, _ = s.GetByKey(tk.Key)
	tk.Status = StatusCreated
	if err := s.Update(tk); err != nil {
		t.Fatal(err)
	}
	r2, skipped, err := s.BeginRun(tk.Key, "tui:"+tk.ID)
	if err != nil || skipped {
		t.Fatalf("retry skipped=%v err=%v", skipped, err)
	}
	if r2.ID == "" || r2.ID == r1.ID {
		t.Fatalf("r1=%s r2=%s", r1.ID, r2.ID)
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
