package tui

import (
	"log"
	"strings"
)

// LogLineMsg 由 webhook/smee 等后台组件投递，写入底部日志区。
type LogLineMsg struct {
	Line string
}

type uiLogWriter struct {
	send func(LogLineMsg)
}

func (w *uiLogWriter) Write(p []byte) (int, error) {
	line := strings.TrimSpace(string(p))
	if line != "" {
		w.send(LogLineMsg{Line: line})
	}
	return len(p), nil
}

// NewUILogger 创建写入 TUI 输出区的 logger（仅时间，不含日期）。
func NewUILogger(send func(LogLineMsg)) *log.Logger {
	return log.New(&uiLogWriter{send: send}, "", log.Ltime)
}
