package task

import "testing"

func TestMakeTaskKeyStable(t *testing.T) {
	a := MakeTaskKey("Owner/Repo", TypeIssueToPR, SourceKindIssue, 45)
	b := MakeTaskKey("owner/repo", TypeIssueToPR, SourceKindIssue, 45)
	if a != b {
		t.Fatalf("expected case-insensitive repo, %q vs %q", a, b)
	}
	if a != "owner/repo|issue_to_pr|issue|45" {
		t.Fatalf("got %q", a)
	}
}

func TestMakeRunKeyIncludesTrigger(t *testing.T) {
	tk := MakeTaskKey("o/r", TypeIssueToPR, SourceKindIssue, 1)
	r1 := MakeRunKey(tk, "comment:99")
	r2 := MakeRunKey(tk, "comment:99")
	r3 := MakeRunKey(tk, "comment:100")
	if r1 != r2 {
		t.Fatal("same trigger must match")
	}
	if r1 == r3 {
		t.Fatal("different trigger must differ")
	}
}
