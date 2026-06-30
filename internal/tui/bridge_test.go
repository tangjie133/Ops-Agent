package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestProgramBridgeDropLogWhenFull(t *testing.T) {
	var got int
	bridge := newProgramBridge(func(tea.Msg) { got++ })
	// fill buffer
	for i := 0; i < externalMsgBuffer+10; i++ {
		bridge.Post(LogLineMsg{Line: "x"})
	}
	if got > externalMsgBuffer {
		t.Fatalf("forwarded too many: %d", got)
	}
}

func TestTrimLogEntries(t *testing.T) {
	entries := make([]logEntry, 500)
	for i := range entries {
		entries[i] = logEntry{text: "x"}
	}
	out := trimLogEntries(entries, maxLogEntries)
	if len(out) != maxLogEntries {
		t.Fatalf("len=%d", len(out))
	}
}
