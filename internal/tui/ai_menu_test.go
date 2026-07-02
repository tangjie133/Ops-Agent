package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

func TestIsAIMenuCommand(t *testing.T) {
	if !isAIMenuCommand("/model") || !isAIMenuCommand("/ai") || !isAIMenuCommand(" /MODEL ") {
		t.Fatal("expected /model /ai")
	}
	if isAIMenuCommand("/model x") || isAIMenuCommand("/status") {
		t.Fatal("expected bare command only")
	}
}

func TestAIConnEditMarksViewDirty(t *testing.T) {
	cfg := config.Default()
	m := &Model{cfg: cfg, width: 120, height: 40}
	m.openAIMenu()
	m.aiMenuLevel = aiMenuLevelConnection
	m.startAIConnEdit(aiConnBaseURL)

	m.storeCachedView("cached")
	if _, ok := m.tryCachedView(); !ok {
		t.Fatal("expected warm cache before edit")
	}

	_, _ = m.handleAIConnEdit(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if _, ok := m.tryCachedView(); ok {
		t.Fatal("expected cache miss after typing in AI conn edit")
	}
}
