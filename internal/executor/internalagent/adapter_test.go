package internalagent

import (
	"context"
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/executor"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

type stubEngine struct{}

func (stubEngine) Run(ctx context.Context, repoPath string, item todo.Item, issue *github.Issue) (string, error) {
	if item.Number != 3 || issue.Title != "bug" {
		return "", context.Canceled
	}
	return "ok", nil
}

func TestAdapterMapsRequest(t *testing.T) {
	res, err := Adapter{Engine: stubEngine{}}.Run(context.Background(), executor.Request{
		WorkDir: t.TempDir(), Repo: "o/r", Number: 3, Title: "bug", Body: "body",
	})
	if err != nil || res.Summary != "ok" || !res.Changed {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}
