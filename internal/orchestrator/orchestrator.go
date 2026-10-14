package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ZzedJay/Ops-Agent/internal/arduino"
	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/executor"
	"github.com/ZzedJay/Ops-Agent/internal/memory"
	"github.com/ZzedJay/Ops-Agent/internal/publisher"
	"github.com/ZzedJay/Ops-Agent/internal/safety"
	"github.com/ZzedJay/Ops-Agent/internal/task"
)

// Deps 是编排器依赖的可替换组件。
type Deps struct {
	Tasks      *task.FileStore
	Exec       executor.Executor
	Publish    *publisher.Publisher
	Config     *config.Config
	MemoryPath string
	Clone          func(ctx context.Context, repo, dest string) error
	LoadIssue      func(ctx context.Context, repo string, number int, kind task.SourceKind) (title, body string, err error)
	VerifyArduino  func(ctx context.Context, workDir string) error
	CommitPush     func(ctx context.Context, workDir, branch string) error
	DefaultBranch  func(ctx context.Context, repo string) (string, error)
}

// RunRequest 是一次 Issue 或 PR 执行请求。
type RunRequest struct {
	Repo        string
	Number      int
	Title       string
	URL         string
	Body        string
	TriggerHash string
	Base        string
	SourceKind  task.SourceKind
}

// Outcome 是编排结果。
type Outcome struct {
	Task    task.Task
	Skipped bool
	PRURL   string
	Message string
}

// Orchestrator 驱动单个任务走完只读调查和 Issue 评论。
type Orchestrator struct {
	deps Deps
}

// New 创建编排器。
func New(deps Deps) *Orchestrator {
	return &Orchestrator{deps: deps}
}

// RunIssueToPR 读取目标仓库并在来源 Issue 或 PR 上评论。
func (o *Orchestrator) RunIssueToPR(ctx context.Context, req RunRequest) (Outcome, error) {
	if o.deps.Tasks == nil {
		return Outcome{}, fmt.Errorf("task store required")
	}
	kind := req.SourceKind
	if kind == "" {
		kind = task.SourceKindIssue
	}
	createdTask := task.Task{
		Repo: req.Repo, Type: task.TypeIssueToPR, SourceKind: kind,
		SourceNumber: req.Number, Title: req.Title, URL: req.URL,
	}
	_, tk, err := o.deps.Tasks.Ensure(createdTask)
	if err != nil {
		return Outcome{}, err
	}
	run, skipped, err := o.deps.Tasks.BeginRun(tk.Key, req.TriggerHash)
	if err != nil {
		return Outcome{}, err
	}
	if skipped {
		return Outcome{Task: tk, Skipped: true}, nil
	}
	if err := o.deps.Tasks.Transition(tk.ID, task.StatusCollectingContext); err != nil {
		return Outcome{}, err
	}

	dur := 30 * time.Minute
	if o.deps.Config != nil {
		if d, perr := time.ParseDuration(o.deps.Config.Task.Budget.MaxDuration); perr == nil && d > 0 {
			dur = d
		}
	}
	deadline := time.Now().Add(dur)
	if !deadline.After(time.Now()) {
		return o.stop(tk, run.ID, task.StatusFailed, "超过任务时限")
	}
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	title, body := req.Title, req.Body
	if o.deps.LoadIssue != nil {
		t, b, lerr := o.deps.LoadIssue(ctx, req.Repo, req.Number, kind)
		if lerr != nil {
			return o.stop(tk, run.ID, task.StatusFailed, lerr.Error())
		}
		title, body = t, b
	}
	if o.deps.Clone == nil {
		return o.stop(tk, run.ID, task.StatusFailed, "clone required")
	}
	root, err := os.MkdirTemp("", "ops-agent-task-")
	if err != nil {
		return o.stop(tk, run.ID, task.StatusFailed, err.Error())
	}
	failed := true
	defer func() {
		keep := false
		if o.deps.Config != nil {
			keep = o.deps.Config.Task.Workspace.KeepFailed
		}
		if failed && keep {
			return
		}
		_ = os.RemoveAll(root)
	}()
	dest := filepath.Join(root, "src")
	if err := o.deps.Clone(ctx, req.Repo, dest); err != nil {
		return o.stop(tk, run.ID, task.StatusFailed, err.Error())
	}
	cur, ok := o.deps.Tasks.GetByKey(tk.Key)
	if !ok {
		return Outcome{}, fmt.Errorf("task missing after clone")
	}
	cur.WorkspacePath = dest
	if err := o.deps.Tasks.Update(cur); err != nil {
		return Outcome{}, err
	}
	if err := o.deps.Tasks.Transition(tk.ID, task.StatusPlanning); err != nil {
		return Outcome{}, err
	}
	if err := o.deps.Tasks.Transition(tk.ID, task.StatusExecuting); err != nil {
		return Outcome{}, err
	}
	res, err := o.deps.Exec.Run(ctx, executor.Request{
		WorkDir: dest, Repo: req.Repo, Number: req.Number, Title: title, Body: body,
	})
	if err != nil {
		msg := err.Error()
		if ctx.Err() == context.DeadlineExceeded {
			msg = "超过任务时限"
		}
		return o.stop(tk, run.ID, task.StatusFailed, msg)
	}
	deleted, paths, dirty, err := porcelain(dest)
	if err != nil {
		return o.stop(tk, run.ID, task.StatusFailed, err.Error())
	}
	prURL := ""
	branch := ""
	if dirty {
		if err := o.deps.Tasks.Transition(tk.ID, task.StatusVerifying); err != nil {
			return Outcome{}, err
		}
		verify := o.deps.VerifyArduino
		if verify == nil {
			verify = arduino.Verify
		}
		if verr := verify(ctx, dest); verr != nil {
			return o.stop(tk, run.ID, task.StatusFailed, verr.Error())
		}
		if err := o.deps.Tasks.Transition(tk.ID, task.StatusEvaluatingSafety); err != nil {
			return Outcome{}, err
		}
		rep := safety.Evaluate(safety.Input{Deleted: deleted, Paths: paths, VerifyOK: true})
		if rep.Verdict != safety.VerdictPublishAsDraft {
			return o.stop(tk, run.ID, task.StatusFailed, "高风险："+rep.Reason)
		}
		branch = fmt.Sprintf("ops-agent/issue-%d", req.Number)
		if kind == task.SourceKindPR {
			branch = fmt.Sprintf("ops-agent/pr-%d", req.Number)
		}
		push := o.deps.CommitPush
		if push == nil {
			var proxy config.ProxyConfig
			if o.deps.Config != nil {
				proxy = o.deps.Config.Proxy
			}
			push = func(ctx context.Context, workDir, b string) error {
				return commitAndPush(ctx, workDir, b, proxy)
			}
		}
		if perr := push(ctx, dest, branch); perr != nil {
			return o.stop(tk, run.ID, task.StatusFailed, perr.Error())
		}
	}
	if err := o.deps.Tasks.Transition(tk.ID, task.StatusPublishing); err != nil {
		return Outcome{}, err
	}
	if o.deps.Publish == nil {
		return o.stop(tk, run.ID, task.StatusFailed, "publisher required")
	}
	if err := o.deps.Publish.CommentIssue(ctx, req.Repo, req.Number, res.Summary, tk.ID, run.ID); err != nil {
		return o.stop(tk, run.ID, task.StatusFailed, err.Error())
	}
	fresh, ok := o.deps.Tasks.GetByKey(tk.Key)
	if !ok {
		return Outcome{}, fmt.Errorf("task missing after comment")
	}
	fresh.CommentPosted = true
	fresh.Title = title
	fresh.WorkspacePath = dest
	fresh.Branch = branch
	if err := o.deps.Tasks.Update(fresh); err != nil {
		return Outcome{}, err
	}
	if dirty {
		base := req.Base
		if base == "" && o.deps.DefaultBranch != nil {
			if b, berr := o.deps.DefaultBranch(ctx, req.Repo); berr == nil {
				base = b
			}
		}
		if base == "" {
			base = "main"
		}
		url, derr := o.deps.Publish.CreateDraftPR(ctx, publisher.DraftPRInput{
			Repo: req.Repo, Number: req.Number, Title: title, Head: branch, Base: base,
			Body: res.Summary, TaskID: tk.ID, RunID: run.ID,
		})
		if derr != nil {
			return o.stop(tk, run.ID, task.StatusFailed, "发布失败: "+derr.Error())
		}
		prURL = url
		posted, ok := o.deps.Tasks.GetByKey(tk.Key)
		if !ok {
			return Outcome{}, fmt.Errorf("task missing after pr")
		}
		posted.CommentPosted = true
		posted.PRURL = prURL
		posted.Title = title
		posted.WorkspacePath = dest
		posted.Branch = branch
		if err := o.deps.Tasks.Update(posted); err != nil {
			return Outcome{}, err
		}
	}
	if err := o.deps.Tasks.Transition(tk.ID, task.StatusDone); err != nil {
		return Outcome{}, err
	}
	_ = memory.Append(o.deps.MemoryPath, memory.Event{TaskID: tk.ID, RunID: run.ID, Kind: "done", Message: "已发出"})
	_ = o.deps.Tasks.ClearActiveRun(tk.ID)
	failed = false
	done, _ := o.deps.Tasks.GetByKey(tk.Key)
	return Outcome{Task: done, PRURL: done.PRURL}, nil
}

func (o *Orchestrator) stop(tk task.Task, runID string, status task.Status, msg string) (Outcome, error) {
	if err := o.deps.Tasks.Transition(tk.ID, status); err != nil {
		return Outcome{}, err
	}
	fresh, ok := o.deps.Tasks.GetByKey(tk.Key)
	if !ok {
		return Outcome{}, fmt.Errorf("task missing after stop")
	}
	fresh.StopReason = msg
	fresh.WorkspacePath = tk.WorkspacePath
	if err := o.deps.Tasks.Update(fresh); err != nil {
		return Outcome{}, err
	}
	_ = memory.Append(o.deps.MemoryPath, memory.Event{TaskID: tk.ID, RunID: runID, Kind: string(status), Message: msg})
	_ = o.deps.Tasks.ClearActiveRun(tk.ID)
	fresh, _ = o.deps.Tasks.GetByKey(tk.Key)
	return Outcome{Task: fresh, Message: msg}, nil
}
