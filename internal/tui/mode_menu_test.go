package tui

import (
	"strings"
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

func TestModeMenuIndex(t *testing.T) {
	if modeMenuIndex(config.ModeFull) != 2 {
		t.Fatalf("got %d", modeMenuIndex(config.ModeFull))
	}
}

func TestModeMenuActivateRefactor(t *testing.T) {
	cfg := config.Default()
	m := &Model{cfg: cfg}

	m.modeMenuSel = int(modeItemRefactorEnabled)
	if !m.modeMenuActivate() {
		t.Fatal("expected activate")
	}
	if !cfg.IssueAutomation.RefactorPR.Enabled {
		t.Fatal("expected enabled")
	}

	m.modeMenuSel = int(modeItemRefactorTrigger)
	if !m.modeMenuActivate() {
		t.Fatal("expected trigger cycle")
	}
	if cfg.IssueAutomation.RefactorPR.Trigger != config.RefactorPRTriggerApproval {
		t.Fatalf("trigger=%q", cfg.IssueAutomation.RefactorPR.Trigger)
	}
}

func TestIssueModeLabel(t *testing.T) {
	cfg := config.Default()
	cfg.IssueAutomation.SetMode(config.ModeSemi)
	if !strings.Contains(issueModeLabel(cfg, config.ModeSemi), "当前") {
		t.Fatal("expected current marker")
	}
	if strings.Contains(issueModeLabel(cfg, config.ModeManual), "当前") {
		t.Fatal("unexpected current marker")
	}
}
