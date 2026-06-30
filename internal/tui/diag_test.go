package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestDiagMsgType(t *testing.T) {
	if got := diagMsgType(refreshTickMsg{}); got != "refreshTick" {
		t.Fatalf("got %q", got)
	}
	if got := diagMsgType(workerTickMsg{}); got != "workerTick" {
		t.Fatalf("got %q", got)
	}
	if got := diagMsgType(tea.KeyMsg{Type: tea.KeyCtrlC}); got != "Key:ctrl+c" {
		t.Fatalf("got %q", got)
	}
}
