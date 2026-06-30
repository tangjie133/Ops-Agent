package config

import "testing"

func TestNormalizeSmeeChannelURL(t *testing.T) {
	got := NormalizeSmeeChannelURL("https://smee.io/N6BMyoHea1WUggZM/webhook")
	want := "https://smee.io/N6BMyoHea1WUggZM"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSmeeTunnelActive(t *testing.T) {
	cfg := Default()
	if cfg.Webhook.SmeeTunnelActive() {
		t.Fatal("expected inactive without public_url")
	}
	cfg.Webhook.PublicURL = "https://smee.io/test"
	if !cfg.Webhook.SmeeTunnelActive() {
		t.Fatal("expected active with smee public_url")
	}
	cfg.Webhook.PublicURL = "https://example.ngrok-free.dev/webhook"
	if cfg.Webhook.SmeeTunnelActive() {
		t.Fatal("ngrok URL should not activate smee tunnel")
	}
	cfg.Webhook.PublicURL = "https://smee.io/test"
	cfg.Webhook.Tunnel.Smee.Enabled = false
	if cfg.Webhook.SmeeTunnelActive() {
		t.Fatal("expected inactive when disabled")
	}
}

func TestValidateSmeeChannelURL(t *testing.T) {
	if err := ValidateSmeeChannelURL("https://smee.io/abc"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSmeeChannelURL(""); err == nil {
		t.Fatal("expected error for empty")
	}
}
