package tui

import (
	"fmt"
	"strings"

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
	m.appendOutput(fmt.Sprintf("已忽略 %s#%d", it.Repo, it.Number))
	m.ensureTodoSelection()
}

func (m *Model) focusSelectedTodo() tea.Cmd {
	active := m.activeTodos()
	if m.todoSel < 0 || m.todoSel >= len(active) {
		return nil
	}
	it := active[m.todoSel]
	return m.runCommand(fmt.Sprintf("/issue %s#%d", it.Repo, it.Number))
}

// formatTodoEntry 两行展示：第一行 仓库#编号，第二行 标题。
func formatTodoEntry(it todo.Item, width int, selected bool) []string {
	if width < 12 {
		width = 12
	}
	marker := " "
	if selected {
		marker = ">"
	}
	ref := todoFullRef(it.Repo, it.Number)
	head := fmt.Sprintf("%s %s %s", marker, statusSymbol(it.Status), truncateASCII(ref, width-4))
	title := truncateASCII(it.Title, width-3)
	return []string{head, "   " + title}
}

func todoFullRef(repo string, number int) string {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return fmt.Sprintf("#%d", number)
	}
	return fmt.Sprintf("%s#%d", repo, number)
}

func truncateASCII(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return s[:max-1] + "…"
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
