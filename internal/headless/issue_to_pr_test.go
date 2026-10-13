package headless

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ZzedJay/Ops-Agent/internal/budget"
	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/executor"
	"github.com/ZzedJay/Ops-Agent/internal/executor/internalagent"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/orchestrator"
	"github.com/ZzedJay/Ops-Agent/internal/publisher"
	"github.com/ZzedJay/Ops-Agent/internal/task"
)

func TestAdapterImplementsExecutor(t *testing.T) {
	var _ executor.Executor = internalagent.Adapter{}
}

func TestShouldRunIssueToPR(t *testing.T) {
	t.Setenv("OPS_AGENT_ISSUE_TO_PR", "1")
	if !ShouldRunIssueToPR() {
		t.Fatal("expected issue-to-pr mode")
	}
	t.Setenv("OPS_AGENT_ISSUE_TO_PR", "")
	if ShouldRunIssueToPR() {
		t.Fatal("expected off")
	}
}

func TestRunIssueToPREmptyQueue(t *testing.T) {
	cfg := config.Default()
	cfg.Task.StorePath = filepath.Join(t.TempDir(), "tasks.json")
	if code := runIssueToPR(cfg); code != 0 {
		t.Fatalf("code=%d", code)
	}
}

type fakeGH struct {
	calls int
}

func (f *fakeGH) PRCreate(ctx context.Context, opts github.PRCreateOpts) (string, error) {
	f.calls++
	return "https://example.test/pr/1", nil
}

func (f *fakeGH) IssueComment(ctx context.Context, repo string, num int, body string) error {
	return nil
}

func TestProcessCreatedUsesOrchestrator(t *testing.T) {
	store, err := task.Load(filepath.Join(t.TempDir(), "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = store.Ensure(task.Task{Repo: "o/r", Type: task.TypeIssueToPR, SourceKind: task.SourceKindIssue, SourceNumber: 3, Title: "bug"}); err != nil {
		t.Fatal(err)
	}
	gh := &fakeGH{}
	orch := orchestrator.New(orchestrator.Deps{
		Tasks: store, Exec: &executor.Fake{Changed: true}, Publish: publisher.New(gh), Config: config.Default(),
		Policy:     budget.Policy{MaxSteps: 12, MaxRetries: 3, MaxDuration: time.Hour, MaxCommands: 20, MaxLLMCalls: 12},
		MemoryPath: filepath.Join(t.TempDir(), "mem.jsonl"),
	})
	ran, err := ProcessCreated(context.Background(), orch, store, initRepo(t))
	if err != nil || !ran || gh.calls != 1 {
		t.Fatalf("ran=%v calls=%d err=%v", ran, gh.calls, err)
	}
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{{"git", "init", "-b", "main"}, {"git", "config", "user.email", "ops@example.com"}, {"git", "config", "user.name", "ops"}} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s", out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	add := exec.Command("git", "add", ".")
	add.Dir = dir
	_, _ = add.CombinedOutput()
	commit := exec.Command("git", "commit", "-m", "init")
	commit.Dir = dir
	if out, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("%s", out)
	}
	return dir
}
