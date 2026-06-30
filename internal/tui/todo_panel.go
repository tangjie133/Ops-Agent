package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

const todoSpinnerInterval = 80 * time.Millisecond

var todoSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (m *Model) spinnerTickCmd() tea.Cmd {
	return tea.Tick(todoSpinnerInterval, func(time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

func (m *Model) hasAnalyzingTodo() bool {
	for _, it := range m.activeTodos() {
		if it.Status == todo.StatusAnalyzing {
			return true
		}
	}
	return false
}

func analyzingSpinner(frame int) string {
	if len(todoSpinnerFrames) == 0 {
		return "…"
	}
	if frame < 0 {
		frame = -frame
	}
	return todoSpinnerFrames[frame%len(todoSpinnerFrames)]
}

// formatTodoEntry 两行展示：第一行 仓库#编号，第二行 标题。
func formatTodoEntry(it todo.Item, width int, selected bool, spinnerFrame int) []string {
	if width < 12 {
		width = 12
	}
	marker := " "
	if selected {
		marker = ">"
	}
	ref := todoFullRef(it.Repo, it.Number)
	head := fmt.Sprintf("%s %s %s", marker, statusSymbol(it.Status, spinnerFrame), truncateASCII(ref, width-4))
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

func statusSymbol(st todo.Status, spinnerFrame int) string {
	switch st {
	case todo.StatusAnalyzing:
		return analyzingSpinner(spinnerFrame)
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

// todoWatchSummary 汇总待办涉及的仓库数（Webhook 按 payload 跨库，非 cwd 单库）。
func (m *Model) todoWatchSummary() string {
	repos := make(map[string]struct{})
	for _, it := range m.activeTodos() {
		if it.Repo != "" {
			repos[it.Repo] = struct{}{}
		}
	}
	switch len(repos) {
	case 0:
		return "0 仓库"
	case 1:
		for r := range repos {
			return r
		}
	default:
		return fmt.Sprintf("%d 仓库", len(repos))
	}
	return "—"
}

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
