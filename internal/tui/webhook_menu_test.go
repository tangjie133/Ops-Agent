package tui

import (
	"strings"
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

func TestIsWebhookMenuCommand(t *testing.T) {
	if !isWebhookMenuCommand("/webhook") {
		t.Fatal("expected match")
	}
	if isWebhookMenuCommand("/webhook x") {
		t.Fatal("expected no match")
	}
}

func TestPersistConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPS_AGENT_CONFIG", dir+"/cfg.yaml")
	cfg := config.Default()
	msg := persistConfig(cfg)
	if !strings.Contains(msg, "已保存") {
		t.Fatalf("msg=%q", msg)
	}
}

func TestModeMenuActivatePersists(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPS_AGENT_CONFIG", dir+"/cfg.yaml")
	cfg := config.Default()
	m := &Model{cfg: cfg}
	m.modeMenuSel = int(modeItemFull)
	if !m.modeMenuActivate() {
		t.Fatal("expected activate")
	}
	if cfg.IssueAutomation.Mode != config.ModeFull {
		t.Fatalf("mode=%s", cfg.IssueAutomation.Mode)
	}
	if !strings.Contains(m.menuNotice, "已保存") {
		t.Fatalf("notice=%q", m.menuNotice)
	}
}
