package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

func (m *Model) activeTodos() []todo.Item {
	items := m.store.List()
	var active []todo.Item
	for _, it := range items {
		switch it.Status {
		case todo.StatusDismissed, todo.StatusDone:
			continue
		}
		active = append(active, it)
	}
	return active
}

func (m *Model) ensureTodoSelection() {
	active := m.activeTodos()
	if len(active) == 0 {
		m.todoSel = -1
		return
	}
	if m.todoSel < 0 || m.todoSel >= len(active) {
		m.todoSel = 0
	}
}

func (m *Model) todoUp() {
	active := m.activeTodos()
	if len(active) == 0 {
		m.todoSel = -1
		return
	}
	if m.todoSel <= 0 {
		m.todoSel = 0
		return
	}
	m.todoSel--
}

func (m *Model) todoDown() {
	active := m.activeTodos()
	if len(active) == 0 {
		m.todoSel = -1
		return
	}
	if m.todoSel < 0 {
		m.todoSel = 0
		return
	}
	if m.todoSel >= len(active)-1 {
		m.todoSel = len(active) - 1
		return
	}
	m.todoSel++
}

func (m *Model) dismissSelectedTodo() {
	active := m.activeTodos()
	if m.todoSel < 0 || m.todoSel >= len(active) {
		return
	}
	it := active[m.todoSel]
	if err := m.store.Transition(it.Repo, it.Number, todo.StatusDismissed); err != nil {
		m.appendOutput(fmt.Sprintf("忽略失败: %v", err))
		return
	}
	m.appendOutput(fmt.Sprintf("已忽略 #%d", it.Number))
	m.ensureTodoSelection()
}

func (m *Model) focusSelectedTodo() tea.Cmd {
	active := m.activeTodos()
	if m.todoSel < 0 || m.todoSel >= len(active) {
		return nil
	}
	num := active[m.todoSel].Number
	return m.runCommand(fmt.Sprintf("/issue %d", num))
}

func statusSymbol(st todo.Status) string {
	switch st {
	case todo.StatusAnalyzing:
		return "…"
	case todo.StatusReady:
		return "●"
	case todo.StatusPosted, todo.StatusDone:
		return "✓"
	case todo.StatusFailed:
		return "✗"
	case todo.StatusDismissed:
		return "—"
	default:
		return "○"
	}
}
