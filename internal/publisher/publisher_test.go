package publisher

import (
	"context"
	"strings"
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/github"
)

type fakeGH struct {
	last github.PRCreateOpts
}

func (f *fakeGH) PRCreate(ctx context.Context, opts github.PRCreateOpts) (string, error) {
	f.last = opts
	return "https://example.test/pr/1", nil
}

func (f *fakeGH) IssueComment(ctx context.Context, repo string, num int, body string) error {
	return nil
}

func TestCreateDraftPR(t *testing.T) {
	gh := &fakeGH{}
	url, err := New(gh).CreateDraftPR(context.Background(), DraftPRInput{
		Repo: "o/r", Number: 4, Title: "fix", Head: "ops-agent/issue-4", Base: "main",
		Body: "body", TaskID: "t1", RunID: "r1",
	})
	if err != nil || url == "" {
		t.Fatal(err)
	}
	if !gh.last.Draft {
		t.Fatal("must be draft")
	}
	if !strings.Contains(gh.last.Body, "Handled by Ops-Agent") || !strings.Contains(gh.last.Body, "Task: t1") || !strings.Contains(gh.last.Body, "Run: r1") {
		t.Fatalf("body=%s", gh.last.Body)
	}
}
