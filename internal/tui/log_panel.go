package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

const logPlaceholder = "（无日志）"

// logPanel 独立日志区：smee / webhook / Worker 等后台输出，不与对话混排。
type logPanel struct {
	viewport viewport.Model
	content  string
	visible  bool
}

func newLogPanel() logPanel {
	return logPanel{
		viewport: viewport.New(60, 4),
		visible:  true,
	}
}

func (m *Model) appendLog(s string) {
	if strings.TrimSpace(s) == "" {
		return
	}
	atBottom := m.log.content == "" || m.log.viewport.AtBottom()
	if m.log.content != "" {
		m.log.content += "\n"
	}
	m.log.content += s
	m.syncLogViewport(atBottom)
}

func (m *Model) syncLogViewport(stickBottom bool) {
	content := m.log.content
	if content == "" {
		content = logPlaceholder
	}
	m.log.viewport.SetContent(content)
	if stickBottom {
		m.log.viewport.GotoBottom()
	}
}

func (m *Model) toggleLogPanel() {
	m.log.visible = !m.log.visible
	m.layout()
}

func (m *Model) bodyHeight() int {
	h := m.height - headerLineCount - m.activeFooterLines()
	if h < 3 {
		h = 3
	}
	return h
}

func (m *Model) logHeight() int {
	if !m.log.visible {
		return 0
	}
	body := m.bodyHeight()
	h := body / 5
	if h < 4 {
		h = 4
	}
	if h > 8 {
		h = 8
	}
	if h > body-4 {
		h = max(3, body/3)
	}
	return h
}

func (m *Model) chatHeight() int {
	body := m.bodyHeight()
	lh := m.logHeight()
	extra := 0
	if lh > 0 {
		extra = 1 // 「── 日志 ──」标题行
	}
	ch := body - lh - extra
	if ch < 3 {
		ch = 3
	}
	return ch
}

func (m *Model) renderLogSection(outW int) string {
	if m.logHeight() <= 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(styleLogHeader.Render("── 日志 ──"))
	b.WriteString("\n")
	logView := lipgloss.NewStyle().Width(outW).Height(m.logHeight()).Render(m.log.viewport.View())
	b.WriteString(logView)
	return b.String()
}

func (m *Model) chatAreaTop() int {
	return headerLineCount
}

func (m *Model) chatAreaBottom() int {
	return headerLineCount + m.chatHeight()
}

func (m *Model) logAreaTop() int {
	if m.logHeight() <= 0 {
		return -1
	}
	return m.chatAreaBottom() + 1
}

func (m *Model) logAreaBottom() int {
	if m.logHeight() <= 0 {
		return -1
	}
	return headerLineCount + m.bodyHeight()
}

func (m *Model) isInLogArea(y int) bool {
	top, bottom := m.logAreaTop(), m.logAreaBottom()
	if top < 0 {
		return false
	}
	return y >= top && y < bottom
}
