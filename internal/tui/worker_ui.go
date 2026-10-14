package tui

// worker_ui.go — 定时检查任务队列并交给编排器。

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ZzedJay/Ops-Agent/internal/ai"
	"github.com/ZzedJay/Ops-Agent/internal/task"
)

func (m *Model) refreshAIHealth() {
	health := ai.CheckHealth(m.bgCtx(), m.cfg.AI)
	m.aiOK = health.Reachable
	if !health.Reachable {
		m.aiWarn = health.Message
	}
}

const workerInterval = 8 * time.Second

type workerTickMsg struct{}

func (m *Model) workerTickCmd() tea.Cmd {
	return tea.Tick(workerInterval, func(time.Time) tea.Msg {
		return workerTickMsg{}
	})
}

func (m *Model) triggerWorkerIfNeeded() tea.Cmd {
	if m.workerBusy || !m.aiOK || !m.hasTaskWork() {
		return nil
	}
	return m.runTaskCmd()
}

func (m *Model) hasTaskWork() bool {
	if m.tasks == nil || m.cfg == nil || !m.cfg.Task.IssueToPR.Enabled {
		return false
	}
	for _, tk := range m.tasks.List() {
		if tk.Status == task.StatusCreated {
			return true
		}
	}
	return false
}
