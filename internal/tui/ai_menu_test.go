package tui

import "testing"

func TestIsAIMenuCommand(t *testing.T) {
	if !isAIMenuCommand("/model") || !isAIMenuCommand("/ai") || !isAIMenuCommand(" /MODEL ") {
		t.Fatal("expected /model /ai")
	}
	if isAIMenuCommand("/model x") || isAIMenuCommand("/status") {
		t.Fatal("expected bare command only")
	}
}
