package tui

import (
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

const logPlaceholder = "（无日志）"

const logRenderMax = 80 // viewport 仅显示数行，不必对 200 条全量渲染

type logKind int

const (
	logKindDefault logKind = iota
	logKindWebhook
	logKindWebhookEvent
	logKindWorker
	logKindInvestigator
	logKindError
)

type logEntry struct {
	kind logKind
	text string
}

// logPanel 独立日志区：smee / webhook / Worker 等后台输出，不与对话混排。
// 内部存纯文本；渲染时上色。Alt Screen 下请 Ctrl+Y 复制或查看日志文件。
type logPanel struct {
	viewport viewport.Model
	entries  []logEntry
	visible  bool
}

func newLogPanel() logPanel {
	return logPanel{
		viewport: viewport.New(60, 4),
		visible:  true,
	}
}

func (m *Model) appendLogKind(kind logKind, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	atBottom := len(m.log.entries) == 0 || m.log.viewport.AtBottom()
	m.log.entries = append(m.log.entries, logEntry{kind: kind, text: truncateLogDisplay(text)})
	m.log.entries = trimLogEntries(m.log.entries, maxLogEntries)
	queueLogPersist(text)
	m.syncLogViewport(atBottom)
	m.markDirty()
}

func (m *Model) persistLogLine(text string) {
	queueLogPersist(text)
}

func (m *Model) logPlainText() string {
	if len(m.log.entries) == 0 {
		return ""
	}
	lines := make([]string, len(m.log.entries))
	for i, e := range m.log.entries {
		lines[i] = e.text
	}
	return strings.Join(lines, "\n")
}

func (m *Model) copyLogsToClipboard() (int, error) {
	text := m.logPlainText()
	if text == "" {
		return 0, fmt.Errorf("日志为空")
	}
	if err := clipboard.WriteAll(text); err != nil {
		return 0, err
	}
	return len(m.log.entries), nil
}

func styleLogEntry(kind logKind, text string) string {
	switch kind {
	case logKindWebhookEvent:
		return styleWebhookEvent.Render(text)
	case logKindWebhook:
		return styleWebhookLog.Render(text)
	case logKindWorker:
		return styleWorkerEvent.Render(text)
	case logKindInvestigator:
		return styleInvestigatorLog.Render(text)
	case logKindError:
		return styleStatusErr.Render(text)
	default:
		return styleWebhookLog.Render(text)
	}
}

func (m *Model) syncLogViewport(stickBottom bool) {
	entries := m.log.entries
	if len(entries) > logRenderMax {
		entries = entries[len(entries)-logRenderMax:]
	}
	var b strings.Builder
	if len(entries) == 0 {
		b.WriteString(logPlaceholder)
	} else {
		for i, e := range entries {
			if i > 0 {
				b.WriteByte('\n')
			}
			// 日志区用纯文本，避免 lipgloss 对数百条逐行上色拖慢 View。
			b.WriteString(e.text)
		}
	}
	m.log.viewport.SetContent(b.String())
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
	b.WriteString(styleLogHeader.Render("── 日志 ── Ctrl+Y 复制"))
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
