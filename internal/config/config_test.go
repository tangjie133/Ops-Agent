package config

import (
	"strings"
	"testing"
)

func TestDefaultMode(t *testing.T) {
	cfg := Default()
	if cfg.IssueAutomation.Mode != ModeSemi {
		t.Fatalf("expected semi, got %s", cfg.IssueAutomation.Mode)
	}
}

func TestSetMode(t *testing.T) {
	cfg := Default()
	cfg.IssueAutomation.SetMode(ModeManual)
	if cfg.IssueAutomation.Mode != ModeManual || cfg.IssueAutomation.AutoAnalyze {
		t.Fatal("manual mode should disable auto_analyze")
	}
	cfg.IssueAutomation.SetMode(ModeFull)
	if cfg.IssueAutomation.Mode != ModeFull || cfg.IssueAutomation.ConfirmBeforeReply {
		t.Fatal("full mode should skip confirm")
	}
}

func TestModeDescription(t *testing.T) {
	if ModeDescription(ModeManual) == "" || ModeDescription(ModeSemi) == "" || ModeDescription(ModeFull) == "" {
		t.Fatal("mode descriptions should not be empty")
	}
	if !IsValidMode(ModeSemi) || IsValidMode("invalid") {
		t.Fatal("IsValidMode mismatch")
	}
	if !strings.Contains(FormatModesHelp(ModeSemi), "* semi") {
		t.Fatal("FormatModesHelp should mark current mode")
	}
}
