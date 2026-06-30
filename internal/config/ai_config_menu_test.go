package config

import (
	"strings"
	"testing"
)

func TestValidateAIBaseURL(t *testing.T) {
	if err := ValidateAIBaseURL("http://127.0.0.1:8080/v1"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateAIBaseURL(""); err == nil {
		t.Fatal("expected error")
	}
	if err := ValidateAIBaseURL("ftp://x"); err == nil {
		t.Fatal("expected scheme error")
	}
}

func TestNormalizeAIBaseURL(t *testing.T) {
	if got := NormalizeAIBaseURL("http://x/v1/"); got != "http://x/v1" {
		t.Fatalf("got %q", got)
	}
}

func TestAISummary(t *testing.T) {
	cfg := Default()
	s := cfg.AISummary()
	if s == "" || !strings.Contains(s, "8080") || !strings.Contains(s, cfg.AI.Model) {
		t.Fatalf("summary=%q", s)
	}
}
