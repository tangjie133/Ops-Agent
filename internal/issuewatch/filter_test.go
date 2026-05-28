package issuewatch

import (
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
)

func TestLabelMatchOR(t *testing.T) {
	labels := []github.Label{{Name: "ops"}, {Name: "bug"}}
	if !labelMatch(labels, []string{"ops"}) {
		t.Fatal("expected ops match")
	}
	if !labelMatch(labels, []string{"bug"}) {
		t.Fatal("expected bug match")
	}
	if labelMatch(labels, []string{"feature"}) {
		t.Fatal("expected no match")
	}
	if !labelMatch(labels, nil) {
		t.Fatal("empty required should match all")
	}
}

func TestMatchesUnassigned(t *testing.T) {
	cfg := config.Default()
	cfg.IssueWatch.RequireUnassigned = true
	cfg.IssueWatch.Labels = []string{"ops"}

	withAssignee := github.Issue{State: "OPEN", Labels: []github.Label{{Name: "ops"}}, Assignees: []github.User{{Login: "u"}}}
	if Matches(withAssignee, cfg) {
		t.Fatal("assigned issue should not match")
	}

	open := github.Issue{State: "OPEN", Labels: []github.Label{{Name: "ops"}}}
	if !Matches(open, cfg) {
		t.Fatal("expected match")
	}
}
