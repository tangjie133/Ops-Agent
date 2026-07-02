package tui

// poll_ui.go — 500ms 轮询 tick：按 mtime 重载 store、合并 Investigator 状态、驱动 spinner。
import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const refreshInterval = 500 * time.Millisecond

type refreshTickMsg struct{}

func (m *Model) refreshTickCmd() tea.Cmd {
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg {
		return refreshTickMsg{}
	})
}

func (m *Model) handleRefreshTick() tea.Cmd {
	dirty := false
	var workerCmd tea.Cmd

	if changed, _ := m.store.ReloadIfChanged(); changed {
		dirty = true
		workerCmd = m.triggerWorkerIfNeeded()
	}
	if changed, _ := m.libTestStore.ReloadIfChanged(); changed {
		dirty = true
	}

	if inv := pollInvStatus(); inv != m.invStatus {
		m.invStatus = inv
		dirty = true
	}

	prevTodoSel := m.todoSel
	m.ensureTodoSelection()
	m.ensureTestSelection()
	if m.todoSel != prevTodoSel {
		dirty = true
	}

	if m.hasAnalyzingTodo() || m.hasCheckingLibTest() {
		m.spinnerFrame++
		m.spinnerActive = true
		dirty = true
	} else if m.spinnerActive {
		m.spinnerActive = false
		dirty = true
	}

	if dirty {
		m.markDirty()
	}

	if workerCmd != nil {
		return tea.Batch(m.refreshTickCmd(), workerCmd)
	}
	return m.refreshTickCmd()
}
