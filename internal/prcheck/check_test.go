package prcheck

import (
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/github"
)

func TestResultOK(t *testing.T) {
	res := &Result{OK: true, Repo: "o/r", PRNumber: 1}
	if !res.OK {
		t.Fatal("expected ok")
	}
	if res.FormatReport() == "" {
		t.Fatal("empty report")
	}
}

func TestEvaluateConflictAndChecks(t *testing.T) {
	pr := &github.PullRequest{
		Mergeable:        "CONFLICTING",
		MergeStateStatus: "DIRTY",
		StatusCheckRollup: github.StatusCheckRollup{
			State: "FAILURE",
			Contexts: []github.CheckContext{
				{Context: "ci/test", State: "FAILURE"},
			},
		},
	}
	failures := evaluate(pr)
	if len(failures) < 2 {
		t.Fatalf("expected multiple failures, got %v", failures)
	}
}

func TestEvaluatePending(t *testing.T) {
	pr := &github.PullRequest{
		Mergeable: "MERGEABLE",
		StatusCheckRollup: github.StatusCheckRollup{
			State: "PENDING",
		},
	}
	failures := evaluate(pr)
	if len(failures) != 1 || failures[0] == "" {
		t.Fatalf("expected pending failure, got %v", failures)
	}
}

func TestEvaluateSuccess(t *testing.T) {
	pr := &github.PullRequest{
		Mergeable: "MERGEABLE",
		StatusCheckRollup: github.StatusCheckRollup{
			State: "SUCCESS",
		},
	}
	if len(evaluate(pr)) != 0 {
		t.Fatal("expected no failures")
	}
}

func TestToAlert(t *testing.T) {
	res := &Result{
		OK:       false,
		Repo:     "o/r",
		PRNumber: 7,
		PRURL:    "https://github.com/o/r/pull/7",
		Failures: []string{"conflict"},
	}
	a := res.ToAlert("https://github.com/o/r/actions/runs/1")
	if a.PRNumber != 7 || len(a.Failures) != 1 {
		t.Fatalf("unexpected alert: %+v", a)
	}
}
