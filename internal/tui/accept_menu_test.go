package tui

import "testing"

func TestIsAcceptMenuCommand(t *testing.T) {
	if !isAcceptMenuCommand("/accept") || !isAcceptMenuCommand(" /ACCEPT ") || !isAcceptMenuCommand("/验收") {
		t.Fatal("expected /accept")
	}
	if isAcceptMenuCommand("/accept auto") || isAcceptMenuCommand("/mode") {
		t.Fatal("expected only bare /accept")
	}
}
