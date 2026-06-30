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

func TestApplyAutomationModePersists(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPS_AGENT_CONFIG", dir+"/cfg.yaml")
	cfg := config.Default()
	out := applyAutomationMode(cfg, config.ModeFull)
	if !strings.Contains(out, "已保存") {
		t.Fatalf("out=%q", out)
	}
	if cfg.IssueAutomation.Mode != config.ModeFull {
		t.Fatalf("mode=%s", cfg.IssueAutomation.Mode)
	}
}
