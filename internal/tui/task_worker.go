package tui

// task_worker.go — TUI 打开期间处理已入队任务。

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ZzedJay/Ops-Agent/internal/arduino"
	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/executor/internalagent"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/issuecomment"
	"github.com/ZzedJay/Ops-Agent/internal/orchestrator"
	"github.com/ZzedJay/Ops-Agent/internal/publisher"
	"github.com/ZzedJay/Ops-Agent/internal/task"
)

type taskDoneMsg struct {
	ran bool
	err error
}

// runTaskCmd 处理任务队列中的下一条 created 任务。
func (m *Model) runTaskCmd() tea.Cmd {
	if m.tasks == nil || m.gh == nil {
		return nil
	}
	m.workerBusy = true
	ctx := m.bgCtx()
	cfg := m.cfg
	gh := m.gh
	tasks := m.tasks
	return func() tea.Msg {
		storePath := config.TaskStorePath(cfg)
		orch := orchestrator.New(orchestrator.Deps{
			Tasks:      tasks,
			Exec:       internalagent.New(cfg),
			Publish:    publisher.New(gh),
			Config:     cfg,
			MemoryPath: filepath.Join(filepath.Dir(storePath), "memory.jsonl"),
			Clone:         gh.CloneRepo,
			LoadIssue:     loadIssueContext(gh),
			VerifyArduino: arduino.Verify,
			DefaultBranch: gh.RepoDefaultBranch,
		})
		ran, err := orchestrator.ProcessCreated(ctx, orch, tasks)
		return taskDoneMsg{ran: ran, err: err}
	}
}

func loadIssueContext(gh *github.Client) func(ctx context.Context, repo string, number int, kind task.SourceKind) (string, string, error) {
	return func(ctx context.Context, repo string, number int, kind task.SourceKind) (string, string, error) {
		if kind == task.SourceKindPR {
			pr, err := gh.PRView(ctx, repo, number)
			if err != nil {
				return "", "", err
			}
			return pr.Title, pr.Body, nil
		}
		iss, err := gh.IssueView(ctx, repo, number)
		if err != nil {
			return "", "", err
		}
		sel := issuecomment.SelectRecent(iss.Comments, 5, "Handled by Ops-Agent")
		var b strings.Builder
		b.WriteString(iss.Body)
		for _, c := range sel.Comments {
			fmt.Fprintf(&b, "\n\n评论 @%s:\n%s", c.Author.Login, c.Body)
		}
		return iss.Title, b.String(), nil
	}
}

func (m *Model) handleTaskDone(msg taskDoneMsg) tea.Cmd {
	m.workerBusy = false
	if msg.err != nil {
		m.appendLogKind(logKindError, "Task: "+msg.err.Error())
	} else if msg.ran {
		m.appendLogKind(logKindWorker, "Task: 已处理一条任务")
	}
	m.markDirty()
	return nil
}
