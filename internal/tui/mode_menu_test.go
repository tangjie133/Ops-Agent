package tui

import (
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

func TestIsModeMenuCommand(t *testing.T) {
	if !isModeMenuCommand("/mode") || !isModeMenuCommand(" /MODE ") {
		t.Fatal("expected /mode to match")
	}
	if isModeMenuCommand("/mode semi") || isModeMenuCommand("/status") {
		t.Fatal("expected only bare /mode")
	}
}

func TestApplyAutomationMode(t *testing.T) {
	cfg := config.Default()
	cfg.IssueAutomation.SetMode(config.ModeManual)

	out := applyAutomationMode(cfg, config.ModeSemi)
	if cfg.IssueAutomation.Mode != config.ModeSemi {
		t.Fatalf("expected semi, got %s", cfg.IssueAutomation.Mode)
	}
	if out == "" {
		t.Fatal("expected output")
	}

	out = applyAutomationMode(cfg, config.ModeSemi)
	if out == "" || cfg.IssueAutomation.Mode != config.ModeSemi {
		t.Fatal("expected keep message")
	}
}

func TestModeMenuIndex(t *testing.T) {
	if modeMenuIndex(config.ModeFull) != 2 {
		t.Fatalf("got %d", modeMenuIndex(config.ModeFull))
	}
}
