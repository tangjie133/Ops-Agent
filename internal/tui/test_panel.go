package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/ZzedJay/Ops-Agent/internal/libtest"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

type leftPanelFocus int

const (
	focusTodo leftPanelFocus = iota
	focusTest
)

func (m *Model) leftPanelSplit() (todoLines, testLines int) {
	body := m.bodyHeight()
	testLines = body / 3
	if testLines < 6 {
		testLines = 6
	}
	if testLines > body/2 {
		testLines = body / 2
	}
	todoLines = body - testLines - 2
	if todoLines < 5 {
		todoLines = 5
	}
	return todoLines, testLines
}

func (m *Model) activeLibTests() []libtest.Item {
	var active []libtest.Item
	for _, it := range m.libTestStore.List() {
		if it.Status == libtest.StatusDismissed {
			continue
		}
		active = append(active, it)
	}
	return active
}

func (m *Model) ensureTestSelection() {
	active := m.activeLibTests()
	if len(active) == 0 {
		m.testSel = -1
		return
	}
	if m.testSel < 0 || m.testSel >= len(active) {
		m.testSel = 0
	}
}

func (m *Model) testUp() {
	active := m.activeLibTests()
	if len(active) == 0 {
		m.testSel = -1
		return
	}
	if m.testSel <= 0 {
		m.testSel = 0
		return
	}
	m.testSel--
	m.markDirty()
}

func (m *Model) testDown() {
	active := m.activeLibTests()
	if len(active) == 0 {
		m.testSel = -1
		return
	}
	if m.testSel < 0 {
		m.testSel = 0
		return
	}
	if m.testSel >= len(active)-1 {
		m.testSel = len(active) - 1
		return
	}
	m.testSel++
	m.markDirty()
}

func formatTestEntry(it libtest.Item, width int, selected bool, spinnerFrame int) []string {
	if width < 12 {
		width = 12
	}
	marker := " "
	if selected {
		marker = ">"
	}
	ref := it.Repo
	if it.Ref != "" && it.Ref != "HEAD" {
		ref = fmt.Sprintf("%s@%s", it.Repo, it.Ref)
	}
	head := fmt.Sprintf("%s %s %s", marker, testStatusSymbol(it.Status, spinnerFrame), truncateASCII(ref, width-4))
	sub := truncateASCII(it.Title, width-3)
	if sub == "" {
		sub = it.Trigger
	}
	return []string{head, "   " + sub}
}

func testStatusSymbol(st libtest.Status, spinnerFrame int) string {
	switch st {
	case libtest.StatusChecking:
		return analyzingSpinner(spinnerFrame)
	case libtest.StatusPass:
		return "✓"
	case libtest.StatusFail:
		return "✗"
	case libtest.StatusPending:
		return "○"
	default:
		return "—"
	}
}

func (m *Model) hasCheckingLibTest() bool {
	for _, it := range m.activeLibTests() {
		if it.Status == libtest.StatusChecking {
			return true
		}
	}
	return false
}

func (m *Model) renderTestPanel(maxLines int) string {
	active := m.activeLibTests()
	if len(active) == 0 {
		return styleTodoItem.Render("  (无)")
	}
	m.ensureTestSelection()
	lineWidth := m.todoPanelWidth() - 2
	var lines []string
	for i, it := range active {
		entry := formatTestEntry(it, lineWidth, i == m.testSel && m.leftFocus == focusTest, m.spinnerFrame)
		if len(lines)+len(entry) > maxLines {
			remaining := len(active) - i
			if remaining > 0 {
				lines = append(lines, styleTodoItem.Render(fmt.Sprintf("  …+%d", remaining)))
			}
			break
		}
		for j, line := range entry {
			style := styleTodoItem
			if i == m.testSel && m.leftFocus == focusTest && j == 0 {
				style = styleTestSelected
			} else if j == 0 && it.Status == libtest.StatusChecking {
				style = styleTodoAnalyzing
			} else if j == 0 && it.Status == libtest.StatusFail {
				style = styleTestFail
			} else if j == 0 && it.Status == libtest.StatusPass {
				style = styleTestPass
			}
			lines = append(lines, style.Render(line))
		}
	}
	return strings.Join(lines, "\n")
}

func (m *Model) renderTodoPanelLimited(maxLines int) string {
	active := m.activeTodos()
	if len(active) == 0 {
		return styleTodoItem.Render("  (无)")
	}
	m.ensureTodoSelection()
	lineWidth := m.todoPanelWidth() - 2
	var lines []string
	for i, it := range active {
		entry := formatTodoEntry(it, lineWidth, i == m.todoSel && m.leftFocus == focusTodo, m.spinnerFrame)
		if len(lines)+len(entry) > maxLines {
			remaining := len(active) - i
			if remaining > 0 {
				lines = append(lines, styleTodoItem.Render(fmt.Sprintf("  …+%d", remaining)))
			}
			break
		}
		for j, line := range entry {
			style := styleTodoItem
			if i == m.todoSel && m.leftFocus == focusTodo && j == 0 {
				style = styleTodoSelected
			} else if j == 0 && it.Status == todo.StatusAnalyzing {
				style = styleTodoAnalyzing
			}
			lines = append(lines, style.Render(line))
		}
	}
	return strings.Join(lines, "\n")
}

func (m *Model) renderLeftColumn() string {
	todoMax, testMax := m.leftPanelSplit()
	todoW := m.todoPanelWidth()

	var b strings.Builder
	todoTitle := "待办"
	if m.leftFocus == focusTodo {
		todoTitle = "待办 ▸"
	}
	b.WriteString(styleTodoHeader.Render(todoTitle))
	b.WriteString("\n")
	b.WriteString(m.renderTodoPanelLimited(todoMax))
	b.WriteString("\n\n")
	testTitle := "验收"
	if m.leftFocus == focusTest {
		testTitle = "验收 ▸"
	}
	b.WriteString(styleTestHeader.Render(testTitle))
	b.WriteString("\n")
	b.WriteString(m.renderTestPanel(testMax))

	return lipgloss.NewStyle().Width(todoW).Height(m.bodyHeight()).Render(b.String())
}

func (m *Model) dismissSelectedTest() {
	active := m.activeLibTests()
	if m.testSel < 0 || m.testSel >= len(active) {
		return
	}
	it := active[m.testSel]
	if err := m.libTestStore.Transition(it.Repo, it.Ref, libtest.StatusDismissed); err != nil {
		m.appendOutput(fmt.Sprintf("忽略验收项失败: %v", err))
		return
	}
	m.appendOutput(fmt.Sprintf("已忽略验收 %s@%s", it.Repo, it.Ref))
	m.ensureTestSelection()
}

func (m *Model) showSelectedTestReport() {
	active := m.activeLibTests()
	if m.testSel < 0 || m.testSel >= len(active) {
		return
	}
	it := active[m.testSel]
	if strings.TrimSpace(it.Report) == "" {
		m.appendOutput(fmt.Sprintf("%s@%s 尚无验收报告（pending 或正在验收）", it.Repo, it.Ref))
		return
	}
	m.appendOutput(fmt.Sprintf("── 验收 %s@%s ──\n%s", it.Repo, it.Ref, truncateForDisplay(it.Report, 8000)))
	if it.Workspace != "" {
		m.appendOutput("工作目录: " + it.Workspace)
	}
}
