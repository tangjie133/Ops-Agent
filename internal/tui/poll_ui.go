package tui

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

	if changed, _ := m.store.ReloadIfChanged(); changed {
		dirty = true
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

	return m.refreshTickCmd()
}
