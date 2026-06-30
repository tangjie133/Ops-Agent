package tui

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
