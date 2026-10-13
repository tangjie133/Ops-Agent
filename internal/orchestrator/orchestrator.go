package orchestrator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ZzedJay/Ops-Agent/internal/budget"
	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/executor"
	"github.com/ZzedJay/Ops-Agent/internal/memory"
	"github.com/ZzedJay/Ops-Agent/internal/publisher"
	"github.com/ZzedJay/Ops-Agent/internal/safety"
	"github.com/ZzedJay/Ops-Agent/internal/skill"
	"github.com/ZzedJay/Ops-Agent/internal/task"
	"github.com/ZzedJay/Ops-Agent/internal/verifier"
	"github.com/ZzedJay/Ops-Agent/internal/workspace"
)

// Deps 是编排器依赖的可替换组件。
type Deps struct {
	Tasks      *task.FileStore
	Exec       executor.Executor
	Publish    *publisher.Publisher
	Config     *config.Config
	Policy     budget.Policy
	MemoryPath string
}

// RunRequest 是一次 issue_to_pr 执行请求。
type RunRequest struct {
	Repo        string
	Number      int
	Title       string
	URL         string
	Body        string
	TriggerHash string
	LocalRepo   string
	Base        string
}

// Outcome 是编排结果。
type Outcome struct {
	Task    task.Task
	Skipped bool
	PRURL   string
	Verdict safety.Verdict
}

// Orchestrator 驱动单个任务走完验证和 draft PR。
type Orchestrator struct {
	deps Deps
}

// New 创建编排器。
func New(deps Deps) *Orchestrator {
	return &Orchestrator{deps: deps}
}

// RunIssueToPR 执行一条 Issue 到 draft PR 的闭环。
func (o *Orchestrator) RunIssueToPR(ctx context.Context, req RunRequest) (Outcome, error) {
	if o.deps.Tasks == nil {
		return Outcome{}, fmt.Errorf("task store required")
	}
	createdTask := task.Task{
		Repo: req.Repo, Type: task.TypeIssueToPR, SourceKind: task.SourceKindIssue,
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
	meter := budget.NewMeter(time.Now())
	meter.NoteStep()
	if !meter.Allow(o.deps.Policy, time.Now()) {
		return o.stop(tk, run.ID, task.StatusNeedsManualIntervention, "budget exceeded")
	}
	if req.LocalRepo == "" {
		return Outcome{}, fmt.Errorf("local repo required")
	}
	root := filepath.Join(os.TempDir(), "ops-agent-task-workspaces")
	branch := fmt.Sprintf("ops-agent/issue-%d", req.Number)
	sess, err := workspace.AllocateFromLocal(req.LocalRepo, root, branch)
	if err != nil {
		return o.stop(tk, run.ID, task.StatusFailed, err.Error())
	}
	tk.WorkspacePath = sess.Dir
	tk.Branch = branch
	if err := o.step(tk.ID, task.StatusPlanning, task.StatusExecuting); err != nil {
		return Outcome{}, err
	}
	_, err = o.deps.Exec.Run(ctx, executor.Request{
		WorkDir: sess.Dir, Repo: req.Repo, Number: req.Number, Title: req.Title,
		Body: req.Body + "\n" + skill.Directive(),
	})
	if err != nil {
		return o.stop(tk, run.ID, task.StatusFailed, err.Error())
	}
	if err := o.deps.Tasks.Transition(tk.ID, task.StatusVerifying); err != nil {
		return Outcome{}, err
	}
	verify := verifier.Run(ctx, verifier.Opts{RepoPath: sess.Dir})
	files, deleted := diffStats(sess.Dir)
	report := safety.Evaluate(safety.Input{
		DiffFiles: files, Deleted: deleted, VerifyOK: verify.OK,
		Retries: 0, MaxRetries: o.deps.Policy.MaxRetries,
	})
	if err := o.deps.Tasks.Transition(tk.ID, task.StatusEvaluatingSafety); err != nil {
		return Outcome{}, err
	}
	if report.Verdict != safety.VerdictPublishAsDraft {
		status := task.StatusNeedsManualIntervention
		if report.Verdict == safety.VerdictNeedsManualReview {
			status = task.StatusAwaitingHumanReview
		}
		out, err := o.stop(tk, run.ID, status, string(report.Verdict))
		out.Verdict = report.Verdict
		return out, err
	}
	if err := o.deps.Tasks.Transition(tk.ID, task.StatusPublishing); err != nil {
		return Outcome{}, err
	}
	url, err := o.deps.Publish.CreateDraftPR(ctx, publisher.DraftPRInput{
		Repo: req.Repo, Number: req.Number, Title: req.Title, Head: branch, Base: req.Base,
		Body: req.Body, TaskID: tk.ID, RunID: run.ID,
	})
	if err != nil {
		return o.stop(tk, run.ID, task.StatusFailed, err.Error())
	}
	tk.PRURL = url
	tk.Status = task.StatusPublishing
	if err := o.deps.Tasks.Update(tk); err != nil {
		return Outcome{}, err
	}
	if err := o.step(tk.ID, task.StatusLearning, task.StatusDone); err != nil {
		return Outcome{}, err
	}
	_ = memory.Append(o.deps.MemoryPath, memory.Event{TaskID: tk.ID, RunID: run.ID, Kind: "done", Message: skill.Name})
	_ = o.deps.Tasks.ClearActiveRun(tk.ID)
	fresh, _ := o.deps.Tasks.GetByKey(tk.Key)
	return Outcome{Task: fresh, PRURL: url, Verdict: report.Verdict}, nil
}

func (o *Orchestrator) step(id string, states ...task.Status) error {
	for _, st := range states {
		if err := o.deps.Tasks.Transition(id, st); err != nil {
			return err
		}
	}
	return nil
}

func (o *Orchestrator) stop(tk task.Task, runID string, status task.Status, msg string) (Outcome, error) {
	if err := o.deps.Tasks.Transition(tk.ID, status); err != nil {
		return Outcome{}, err
	}
	_ = memory.Append(o.deps.MemoryPath, memory.Event{TaskID: tk.ID, RunID: runID, Kind: string(status), Message: msg})
	_ = o.deps.Tasks.ClearActiveRun(tk.ID)
	fresh, _ := o.deps.Tasks.GetByKey(tk.Key)
	return Outcome{Task: fresh}, nil
}

func diffStats(dir string) (files, deleted int) {
	cmd := exec.Command("git", "-C", dir, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return 0, 0
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		files++
		if strings.HasPrefix(line, "D") || strings.HasPrefix(line, " D") {
			deleted++
		}
	}
	return files, deleted
}
