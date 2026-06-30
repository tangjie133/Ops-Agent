package tui

import (
	"log"
	"strings"
)

// LogLineMsg 已弃用：后台日志请走 backgroundLogSink。保留类型供兼容/诊断。
type LogLineMsg struct {
	Line string
}

type uiLogWriter struct{}

func (w *uiLogWriter) Write(p []byte) (int, error) {
	line := strings.TrimSpace(string(p))
	if line != "" {
		bgLog.append(logKindWebhook, line)
	}
	return len(p), nil
}

// NewUILogger 创建写入日志文件 + 节流刷新 TUI 日志区的 logger。
func NewUILogger() *log.Logger {
	return log.New(&uiLogWriter{}, "", log.Ltime)
}
