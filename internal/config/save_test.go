package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveCreatesFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPS_AGENT_CONFIG", filepath.Join(dir, "cfg.yaml"))

	cfg := Default()
	cfg.Webhook.PublicURL = "https://smee.io/test"
	cfg.Webhook.Secret = "secret"

	path, err := Save(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Webhook.PublicURL != "https://smee.io/test" {
		t.Fatalf("public_url=%q", loaded.Webhook.PublicURL)
	}
}

func TestWebhookLocalURL(t *testing.T) {
	w := WebhookConfig{Listen: "127.0.0.1:8765", Path: "/webhooks/github"}
	if w.LocalURL() != "http://127.0.0.1:8765/webhooks/github" {
		t.Fatal(w.LocalURL())
	}
}
