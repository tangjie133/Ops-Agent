package tui

// investigator_log.go — Investigator 阶段状态与日志 sink（轮询读入 UI，不 flooding Post）。

import (
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const invStatusMinInterval = 1 * time.Second

var invStatusState struct {
	mu   sync.Mutex
	line string
}

// invStatusMsg 已弃用：阶段 2 由 pollInvStatus 在 refresh tick 读取。
type invStatusMsg struct {
	Line string
}

// investigatorLogSink Investigator 日志只写文件；状态由轮询 tick 读入 UI。
type investigatorLogSink struct {
	mu   sync.Mutex
	last time.Time
}

func newInvestigatorLogSink() *investigatorLogSink {
	return &investigatorLogSink{}
}

func (s *investigatorLogSink) log(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	queueLogPersist(line)

	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.last) < invStatusMinInterval {
		return
	}
	s.last = time.Now()
	invStatusState.mu.Lock()
	invStatusState.line = truncateLogDisplay(line)
	invStatusState.mu.Unlock()
}

func (s *investigatorLogSink) reset() {
	s.mu.Lock()
	s.last = time.Time{}
	s.mu.Unlock()
	invStatusState.mu.Lock()
	invStatusState.line = ""
	invStatusState.mu.Unlock()
}

func pollInvStatus() string {
	invStatusState.mu.Lock()
	defer invStatusState.mu.Unlock()
	return invStatusState.line
}

func (m *Model) bindInvestigatorLog() {
	sink := newInvestigatorLogSink()
	m.invLogSink = sink
	m.investigatorLogFn = sink.log
}

// maintainSpinner 已合并到 refresh tick（500ms 更新 spinner 帧，避免 250ms 独立 tick 洪峰）。
func (m *Model) maintainSpinner() tea.Cmd {
	return nil
}

func (m *Model) clearInvStatus() {
	m.invStatus = ""
	if m.invLogSink != nil {
		m.invLogSink.reset()
	}
}
