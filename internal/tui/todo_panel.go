package tui

// todo_panel.go — 左侧任务列表渲染与键盘导航。

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ZzedJay/Ops-Agent/internal/task"
)

var todoSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func taskBusy(st task.Status) bool {
	switch st {
	case task.StatusCollectingContext, task.StatusPlanning, task.StatusExecuting,
		task.StatusVerifying, task.StatusEvaluatingSafety, task.StatusPublishing, task.StatusLearning:
		return true
	default:
		return false
	}
}

func (m *Model) hasAnalyzingTodo() bool {
	for _, it := range m.activeTodos() {
		if taskBusy(it.Status) {
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

// formatTodoEntry 使用 TaskRows。选中时展开阶段、日志、原因和 PR。
func formatTodoEntry(it task.Task, width int, selected bool, spinnerFrame int) []string {
	_ = spinnerFrame
	if width < 12 {
		width = 12
	}
	rows := TaskRows(it, selected, nil)
	marker := " "
	if selected {
		marker = ">"
	}
	rows[0] = marker + " " + truncateASCII(rows[0], width-2)
	for i := 1; i < len(rows); i++ {
		rows[i] = "  " + truncateASCII(rows[i], width-2)
	}
	return rows
}

func (m *Model) renderLeftColumn() string {
	todoW := m.todoPanelWidth()
	var b strings.Builder
	b.WriteString(styleTodoHeader.Render("任务"))
	b.WriteString("\n")
	b.WriteString(m.renderTodoPanel())
	return lipgloss.NewStyle().Width(todoW).Height(m.bodyHeight()).Render(b.String())
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

func statusSymbol(st task.Status, spinnerFrame int) string {
	switch st {
	case task.StatusDone:
		return "✓"
	case task.StatusFailed, task.StatusNeedsManualIntervention:
		return "✗"
	case task.StatusCancelled, task.StatusRolledBack:
		return "—"
	case task.StatusAwaitingApproval, task.StatusAwaitingHumanReview:
		return "●"
	default:
		if taskBusy(st) {
			return analyzingSpinner(spinnerFrame)
		}
		return "○"
	}
}

// todoWatchSummary 汇总任务涉及的仓库数。
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

func (m *Model) activeTodos() []task.Task {
	if m.tasks == nil {
		return nil
	}
	var active []task.Task
	for _, it := range m.tasks.List() {
		switch it.Status {
		case task.StatusDone, task.StatusCancelled, task.StatusRolledBack:
			continue
		}
		active = append(active, it)
	}
	return active
}

func todoIndexByKey(active []task.Task, repo string, num int) int {
	for i, it := range active {
		if it.Repo == repo && it.SourceNumber == num {
			return i
		}
	}
	return -1
}

func (m *Model) captureTodoAnchor() {
	active := m.activeTodos()
	if m.todoSel >= 0 && m.todoSel < len(active) {
		it := active[m.todoSel]
		m.todoAnchorRepo = it.Repo
		m.todoAnchorNum = it.SourceNumber
	}
}

func (m *Model) ensureTodoSelection() {
	active := m.activeTodos()
	if len(active) == 0 {
		m.todoSel = -1
		m.todoAnchorRepo = ""
		m.todoAnchorNum = 0
		return
	}
	if m.todoAnchorRepo != "" {
		if idx := todoIndexByKey(active, m.todoAnchorRepo, m.todoAnchorNum); idx >= 0 {
			m.todoSel = idx
			return
		}
		m.todoAnchorRepo = ""
		m.todoAnchorNum = 0
	}
	if m.todoSel < 0 || m.todoSel >= len(active) {
		m.todoSel = 0
	}
	m.captureTodoAnchor()
}

func (m *Model) dismissSelectedTodo() {
	active := m.activeTodos()
	if m.todoSel < 0 || m.todoSel >= len(active) {
		return
	}
	it := active[m.todoSel]
	ok, err := m.tasks.CancelIssue(it.Repo, it.SourceNumber)
	if err != nil || !ok {
		m.appendOutput(fmt.Sprintf("取消失败: %v", err))
		return
	}
	m.appendOutput(fmt.Sprintf("已取消 %s#%d", it.Repo, it.SourceNumber))
	m.ensureTodoSelection()
}

const listEntryLines = 2

func panelScrollStart(sel, total, maxLines int) int {
	if total <= 0 || maxLines <= 0 {
		return 0
	}
	visible := maxLines / listEntryLines
	if visible < 1 {
		visible = 1
	}
	if total <= visible {
		return 0
	}
	start := 0
	if sel >= visible {
		start = sel - visible + 1
	}
	maxStart := total - visible
	if start > maxStart {
		start = maxStart
	}
	return start
}

func panelWrapSel(sel, delta, n int) int {
	if n <= 0 {
		return -1
	}
	if sel < 0 {
		if delta >= 0 {
			return 0
		}
		return n - 1
	}
	return (sel + delta + n*1024) % n
}

func (m *Model) moveTodoSel(delta int) {
	n := len(m.activeTodos())
	m.todoSel = panelWrapSel(m.todoSel, delta, n)
	if n > 0 {
		m.captureTodoAnchor()
	} else {
		m.todoAnchorRepo = ""
		m.todoAnchorNum = 0
	}
	m.markDirty()
}

func (m *Model) todoUp()   { m.moveTodoSel(-1) }
func (m *Model) todoDown() { m.moveTodoSel(1) }

func (m *Model) focusSelectedTodo() tea.Cmd {
	active := m.activeTodos()
	if m.todoSel < 0 || m.todoSel >= len(active) {
		return nil
	}
	it := active[m.todoSel]
	return m.runCommand(fmt.Sprintf("/issue %s#%d", it.Repo, it.SourceNumber))
}
