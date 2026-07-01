package tui

// background_log.go — Webhook/Worker 后台日志统一入口，仅写文件不 Post TUI。

import "strings"

var bgLog backgroundLogSink

type backgroundLogSink struct{}

func initBackgroundLog() {}

func (s *backgroundLogSink) append(kind logKind, line string) {
	_ = kind
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	queueLogPersist(line)
}
