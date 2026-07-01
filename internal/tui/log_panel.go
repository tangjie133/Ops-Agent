package tui

// log_panel.go — 日志写入 tui.log、Ctrl+Y 复制；界面布局高度计算（已无内嵌日志面板）。
import (
	"fmt"
	"os"
	"strings"

	"github.com/atotto/clipboard"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

type logKind int

const (
	logKindDefault logKind = iota
	logKindWebhook
	logKindWebhookEvent
	logKindWorker
	logKindInvestigator
	logKindError
)

// appendLogKind 写入 tui.log；不在界面展示（请 tail 日志文件或 Ctrl+Y 复制）。
func (m *Model) appendLogKind(kind logKind, text string) {
	_ = kind
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	queueLogPersist(text)
}

func (m *Model) persistLogLine(text string) {
	queueLogPersist(text)
}

func (m *Model) copyLogsToClipboard() (int, error) {
	path := config.LogFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, fmt.Errorf("日志文件不存在: %s", path)
		}
		return 0, err
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return 0, fmt.Errorf("日志为空")
	}
	if err := clipboard.WriteAll(text); err != nil {
		return 0, err
	}
	return strings.Count(text, "\n") + 1, nil
}

func (m *Model) bodyHeight() int {
	h := m.height - headerLineCount - m.activeFooterLines()
	if h < 3 {
		h = 3
	}
	return h
}

func (m *Model) chatHeight() int {
	return m.bodyHeight()
}

func (m *Model) chatAreaTop() int {
	return headerLineCount
}

func (m *Model) chatAreaBottom() int {
	return headerLineCount + m.chatHeight()
}

func (m *Model) isInOutputArea(y int) bool {
	if m.height == 0 {
		return true
	}
	top := m.chatAreaTop()
	bottom := m.chatAreaBottom()
	return y >= top && y < bottom
}
