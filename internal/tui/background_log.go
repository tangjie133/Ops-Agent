package tui

import (
	"strings"
	"sync"
)

// backgroundLogSink 后台日志（smee/webhook 等）只写文件 + 内存缓冲，由 refresh tick 批量刷入 UI。
type backgroundLogSink struct {
	mu      sync.Mutex
	pending []logEntry
}

var bgLog backgroundLogSink

func initBackgroundLog() {}

func (s *backgroundLogSink) append(kind logKind, line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	queueLogPersist(line)
	s.mu.Lock()
	s.pending = append(s.pending, logEntry{kind: kind, text: truncateLogDisplay(line)})
	s.mu.Unlock()
}

func (s *backgroundLogSink) drain() []logEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.pending) == 0 {
		return nil
	}
	out := append([]logEntry(nil), s.pending...)
	s.pending = s.pending[:0]
	return out
}

func (m *Model) mergeBackgroundLogs() {
	pending := bgLog.drain()
	if len(pending) == 0 {
		return
	}
	atBottom := len(m.log.entries) == 0 || m.log.viewport.AtBottom()
	m.log.entries = append(m.log.entries, pending...)
	m.log.entries = trimLogEntries(m.log.entries, maxLogEntries)
	m.syncLogViewport(atBottom)
}
