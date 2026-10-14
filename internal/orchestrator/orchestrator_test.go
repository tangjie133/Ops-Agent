package orchestrator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

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

func copyGit(src, dest string) error {
	cmd := exec.Command("git", "clone", "--local", src, dest)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("copyGit: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

type fakeGH struct {
	last     github.PRCreateOpts
	calls    int
	comments int
}

func (f *fakeGH) PRCreate(ctx context.Context, opts github.PRCreateOpts) (string, error) {
	f.calls++
	f.last = opts
	return "https://example.test/pr/1", nil
}

func (f *fakeGH) IssueComment(ctx context.Context, repo string, num int, body string) error {
	f.comments++
	return nil
}

func enabledCfg() *config.Config {
	cfg := config.Default()
	cfg.Task.IssueToPR.Enabled = true
	return cfg
}

func TestRunCommentsIssue(t *testing.T) {
	src := initGitRepo(t)
	tasks, err := task.Load(filepath.Join(t.TempDir(), "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	gh := &fakeGH{}
	orch := New(Deps{
		Tasks: tasks, Exec: &executor.Fake{Summary: "介绍完成"},
		Publish: publisher.New(gh), Config: enabledCfg(),
		MemoryPath: filepath.Join(t.TempDir(), "mem.jsonl"),
		Clone: func(ctx context.Context, repo, dest string) error {
			if repo != "tangjie133/test" {
				t.Fatalf("repo=%s", repo)
			}
			return copyGit(src, dest)
		},
		LoadIssue: func(ctx context.Context, repo string, number int, kind task.SourceKind) (string, string, error) {
			if number != 51 || kind != task.SourceKindIssue {
				t.Fatalf("%s %d %s", repo, number, kind)
			}
			return "介绍库", "简单介绍库功能", nil
		},
	})
	out, err := orch.RunIssueToPR(context.Background(), RunRequest{
		Repo: "tangjie133/test", Number: 51, TriggerHash: "t1", SourceKind: task.SourceKindIssue,
	})
	if err != nil || out.Skipped || gh.calls != 0 || gh.comments != 1 {
		t.Fatalf("out=%+v gh=%+v err=%v", out, gh, err)
	}
	if out.Task.Status != task.StatusDone || !out.Task.CommentPosted || out.Task.PRURL != "" || out.Task.StopReason != "" {
		t.Fatalf("%+v", out.Task)
	}
}

func TestRunIssueToPRSkipsDuplicateTrigger(t *testing.T) {
	src := initGitRepo(t)
	tasks, _ := task.Load(filepath.Join(t.TempDir(), "tasks.json"))
	gh := &fakeGH{}
	orch := New(Deps{
		Tasks: tasks, Exec: &executor.Fake{Summary: "ok"}, Publish: publisher.New(gh),
		Config:     enabledCfg(),
		MemoryPath: filepath.Join(t.TempDir(), "mem.jsonl"),
		Clone:      func(ctx context.Context, repo, dest string) error { return copyGit(src, dest) },
		LoadIssue: func(ctx context.Context, repo string, number int, kind task.SourceKind) (string, string, error) {
			return "t", "b", nil
		},
	})
	req := RunRequest{Repo: "o/r", Number: 8, Title: "fix", TriggerHash: "same"}
	if _, err := orch.RunIssueToPR(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	out, err := orch.RunIssueToPR(context.Background(), req)
	if err != nil || !out.Skipped {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	if gh.comments != 1 {
		t.Fatalf("comments=%d", gh.comments)
	}
}

func TestRetrySameTriggerCommentsAgain(t *testing.T) {
	src := initGitRepo(t)
	tasks, _ := task.Load(filepath.Join(t.TempDir(), "tasks.json"))
	gh := &fakeGH{}
	orch := New(Deps{
		Tasks: tasks, Exec: &executor.Fake{Summary: "ok"}, Publish: publisher.New(gh),
		Config:     enabledCfg(),
		MemoryPath: filepath.Join(t.TempDir(), "mem.jsonl"),
		Clone:      func(ctx context.Context, repo, dest string) error { return copyGit(src, dest) },
		LoadIssue: func(ctx context.Context, repo string, number int, kind task.SourceKind) (string, string, error) {
			return "t", "b", nil
		},
	})
	req := RunRequest{Repo: "o/r", Number: 51, TriggerHash: "tui:same", SourceKind: task.SourceKindIssue}
	if _, err := orch.RunIssueToPR(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	tk, ok := tasks.GetByKey(task.MakeTaskKey("o/r", task.TypeIssueToPR, task.SourceKindIssue, 51))
	if !ok {
		t.Fatal("missing task")
	}
	tk.Status = task.StatusCreated
	tk.StopReason = ""
	tk.ActiveRunID = ""
	if err := tasks.Update(tk); err != nil {
		t.Fatal(err)
	}
	out, err := orch.RunIssueToPR(context.Background(), req)
	if err != nil || out.Skipped || gh.comments != 2 {
		t.Fatalf("out=%+v comments=%d err=%v", out, gh.comments, err)
	}
}

func TestTimeoutDoesNotComment(t *testing.T) {
	src := initGitRepo(t)
	tasks, _ := task.Load(filepath.Join(t.TempDir(), "tasks.json"))
	gh := &fakeGH{}
	cfg := enabledCfg()
	cfg.Task.Budget.MaxDuration = "1ns"
	orch := New(Deps{
		Tasks: tasks, Exec: &executor.Fake{}, Publish: publisher.New(gh), Config: cfg,
		MemoryPath: filepath.Join(t.TempDir(), "mem.jsonl"),
		Clone:      func(ctx context.Context, repo, dest string) error { return copyGit(src, dest) },
		LoadIssue: func(ctx context.Context, repo string, number int, kind task.SourceKind) (string, string, error) {
			return "t", "b", nil
		},
	})
	out, err := orch.RunIssueToPR(context.Background(), RunRequest{Repo: "o/r", Number: 1, TriggerHash: "to"})
	if err != nil {
		t.Fatal(err)
	}
	if gh.comments != 0 || out.Task.StopReason != "超过任务时限" {
		t.Fatalf("comments=%d reason=%q", gh.comments, out.Task.StopReason)
	}
}

func TestCloneFailureStops(t *testing.T) {
	tasks, _ := task.Load(filepath.Join(t.TempDir(), "tasks.json"))
	gh := &fakeGH{}
	orch := New(Deps{
		Tasks: tasks, Exec: &executor.Fake{}, Publish: publisher.New(gh), Config: enabledCfg(),
		MemoryPath: filepath.Join(t.TempDir(), "mem.jsonl"),
		Clone:      func(ctx context.Context, repo, dest string) error { return fmt.Errorf("clone denied") },
		LoadIssue: func(ctx context.Context, repo string, number int, kind task.SourceKind) (string, string, error) {
			return "t", "b", nil
		},
	})
	out, err := orch.RunIssueToPR(context.Background(), RunRequest{Repo: "o/r", Number: 2, TriggerHash: "c"})
	if err != nil {
		t.Fatal(err)
	}
	if gh.comments != 0 || !strings.Contains(out.Task.StopReason, "clone denied") {
		t.Fatalf("%+v", out.Task)
	}
}

func TestDirtyWorkspaceCommentAndDraftPR(t *testing.T) {
	src := initGitRepo(t)
	tasks, _ := task.Load(filepath.Join(t.TempDir(), "tasks.json"))
	gh := &fakeGH{}
	var pushed string
	orch := New(Deps{
		Tasks: tasks, Exec: &executor.Fake{Changed: true, Summary: "已修 read"},
		Publish: publisher.New(gh), Config: enabledCfg(),
		MemoryPath: filepath.Join(t.TempDir(), "mem.jsonl"),
		Clone:      func(ctx context.Context, repo, dest string) error { return copyGit(src, dest) },
		LoadIssue: func(ctx context.Context, repo string, number int, kind task.SourceKind) (string, string, error) {
			return "read 溢出", "size 被忽略", nil
		},
		VerifyArduino: func(ctx context.Context, workDir string) error { return nil },
		CommitPush: func(ctx context.Context, workDir, branch string) error {
			pushed = branch
			return nil
		},
		DefaultBranch: func(ctx context.Context, repo string) (string, error) { return "main", nil },
	})
	out, err := orch.RunIssueToPR(context.Background(), RunRequest{
		Repo: "tangjie133/test2", Number: 2, TriggerHash: "lib", SourceKind: task.SourceKindIssue,
	})
	if err != nil || out.Skipped || gh.comments != 1 || gh.calls != 1 {
		t.Fatalf("out=%+v gh=%+v err=%v", out, gh, err)
	}
	if pushed != "ops-agent/issue-2" || gh.last.Head != "ops-agent/issue-2" || !gh.last.Draft || gh.last.Title != "read 溢出" {
		t.Fatalf("pushed=%s last=%+v", pushed, gh.last)
	}
	if !out.Task.CommentPosted || out.Task.PRURL != "https://example.test/pr/1" {
		t.Fatalf("%+v", out.Task)
	}
}

func TestArduinoFailDoesNotComment(t *testing.T) {
	src := initGitRepo(t)
	tasks, _ := task.Load(filepath.Join(t.TempDir(), "tasks.json"))
	gh := &fakeGH{}
	orch := New(Deps{
		Tasks: tasks, Exec: &executor.Fake{Changed: true, Summary: "x"},
		Publish: publisher.New(gh), Config: enabledCfg(),
		MemoryPath: filepath.Join(t.TempDir(), "mem.jsonl"),
		Clone:      func(ctx context.Context, repo, dest string) error { return copyGit(src, dest) },
		LoadIssue:  func(ctx context.Context, repo string, number int, kind task.SourceKind) (string, string, error) { return "t", "b", nil },
		VerifyArduino: func(ctx context.Context, workDir string) error {
			return fmt.Errorf("示例编译失败: examples/A/A.ino\nerror: overflow")
		},
	})
	out, err := orch.RunIssueToPR(context.Background(), RunRequest{Repo: "o/r", Number: 2, TriggerHash: "bad"})
	if err != nil {
		t.Fatal(err)
	}
	if gh.comments != 0 || gh.calls != 0 || !strings.Contains(out.Task.StopReason, "overflow") {
		t.Fatalf("%+v comments=%d prs=%d", out.Task, gh.comments, gh.calls)
	}
}

func TestHighRiskDoesNotPublish(t *testing.T) {
	src := initGitRepo(t)
	tasks, _ := task.Load(filepath.Join(t.TempDir(), "tasks.json"))
	gh := &fakeGH{}
	orch := New(Deps{
		Tasks: tasks,
		Exec: executor.ExecutorFunc(func(ctx context.Context, req executor.Request) (executor.Result, error) {
			p := filepath.Join(req.WorkDir, ".github", "workflows", "ci.yml")
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return executor.Result{}, err
			}
			return executor.Result{Summary: "x", Changed: true}, os.WriteFile(p, []byte("on: push\n"), 0o644)
		}),
		Publish: publisher.New(gh), Config: enabledCfg(),
		MemoryPath: filepath.Join(t.TempDir(), "mem.jsonl"),
		Clone:      func(ctx context.Context, repo, dest string) error { return copyGit(src, dest) },
		LoadIssue:  func(ctx context.Context, repo string, number int, kind task.SourceKind) (string, string, error) { return "t", "b", nil },
		VerifyArduino: func(ctx context.Context, workDir string) error { return nil },
	})
	out, err := orch.RunIssueToPR(context.Background(), RunRequest{Repo: "o/r", Number: 3, TriggerHash: "risk"})
	if err != nil {
		t.Fatal(err)
	}
	if gh.comments != 0 || gh.calls != 0 || !strings.Contains(out.Task.StopReason, "高风险") {
		t.Fatalf("%+v", out.Task)
	}
}
