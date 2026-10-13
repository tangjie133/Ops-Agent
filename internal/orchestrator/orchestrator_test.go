package orchestrator

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
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/publisher"
	"github.com/ZzedJay/Ops-Agent/internal/task"
)

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"git", "init", "-b", "main"},
		{"git", "config", "user.email", "ops@example.com"},
		{"git", "config", "user.name", "ops"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	add := exec.Command("git", "add", ".")
	add.Dir = dir
	if out, err := add.CombinedOutput(); err != nil {
		t.Fatalf("add: %s", out)
	}
	commit := exec.Command("git", "commit", "-m", "init")
	commit.Dir = dir
	if out, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("commit: %s", out)
	}
	return dir
}

type fakeGH struct {
	last  github.PRCreateOpts
	calls int
}

func (f *fakeGH) PRCreate(ctx context.Context, opts github.PRCreateOpts) (string, error) {
	f.calls++
	f.last = opts
	return "https://example.test/pr/1", nil
}

func (f *fakeGH) IssueComment(ctx context.Context, repo string, num int, body string) error {
	return nil
}

func enabledCfg() *config.Config {
	cfg := config.Default()
	cfg.Task.IssueToPR.Enabled = true
	return cfg
}

func openPolicy() budget.Policy {
	return budget.Policy{MaxSteps: 12, MaxRetries: 3, MaxDuration: time.Hour, MaxCommands: 20, MaxLLMCalls: 12}
}

func TestRunIssueToPRCreatesDraft(t *testing.T) {
	src := initGitRepo(t)
	tasks, err := task.Load(filepath.Join(t.TempDir(), "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	gh := &fakeGH{}
	orch := New(Deps{
		Tasks:      tasks,
		Exec:       &executor.Fake{Changed: true},
		Publish:    publisher.New(gh),
		Config:     enabledCfg(),
		Policy:     openPolicy(),
		MemoryPath: filepath.Join(t.TempDir(), "mem.jsonl"),
	})
	out, err := orch.RunIssueToPR(context.Background(), RunRequest{
		Repo: "o/r", Number: 7, Title: "fix", TriggerHash: "t1", LocalRepo: src, Base: "main",
	})
	if err != nil || out.Skipped || out.PRURL == "" {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	if !gh.last.Draft {
		t.Fatal("must be draft")
	}
	if out.Task.Status != task.StatusDone {
		t.Fatalf("status=%s", out.Task.Status)
	}
}

func TestRunIssueToPRSkipsDuplicateTrigger(t *testing.T) {
	src := initGitRepo(t)
	tasks, _ := task.Load(filepath.Join(t.TempDir(), "tasks.json"))
	gh := &fakeGH{}
	orch := New(Deps{
		Tasks: tasks, Exec: &executor.Fake{Changed: true}, Publish: publisher.New(gh),
		Config: enabledCfg(), Policy: openPolicy(), MemoryPath: filepath.Join(t.TempDir(), "mem.jsonl"),
	})
	req := RunRequest{Repo: "o/r", Number: 8, Title: "fix", TriggerHash: "same", LocalRepo: src, Base: "main"}
	if _, err := orch.RunIssueToPR(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	out, err := orch.RunIssueToPR(context.Background(), req)
	if err != nil || !out.Skipped {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	if gh.calls != 1 {
		t.Fatalf("calls=%d", gh.calls)
	}
}

func TestRunIssueToPRStopsOnBudget(t *testing.T) {
	src := initGitRepo(t)
	tasks, _ := task.Load(filepath.Join(t.TempDir(), "tasks.json"))
	gh := &fakeGH{}
	orch := New(Deps{
		Tasks: tasks, Exec: &executor.Fake{Changed: true}, Publish: publisher.New(gh),
		Config: enabledCfg(), Policy: budget.Policy{MaxSteps: 0, MaxRetries: 3, MaxDuration: time.Hour, MaxCommands: 20, MaxLLMCalls: 12},
		MemoryPath: filepath.Join(t.TempDir(), "mem.jsonl"),
	})
	out, err := orch.RunIssueToPR(context.Background(), RunRequest{
		Repo: "o/r", Number: 9, Title: "fix", TriggerHash: "b", LocalRepo: src, Base: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.PRURL != "" || gh.calls != 0 {
		t.Fatalf("should not publish: %+v calls=%d", out, gh.calls)
	}
	if out.Task.Status != task.StatusNeedsManualIntervention {
		t.Fatalf("status=%s", out.Task.Status)
	}
}
