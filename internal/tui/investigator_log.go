package tui

import tea "github.com/charmbracelet/bubbletea"

// InvestigatorLogMsg Investigator 调试日志，写入底部日志区。
type InvestigatorLogMsg struct {
	Line string
}

func (m *Model) bindProgramSend(send func(tea.Msg)) {
	m.programSend = send
	m.investigatorLogFn = func(line string) {
		if line != "" {
			send(InvestigatorLogMsg{Line: line})
		}
	}
}
