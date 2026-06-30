package netproxy

import (
	"os"
	"os/exec"
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

func TestEnvironProxy(t *testing.T) {
	cfg := config.ProxyConfig{
		Enabled:    true,
		HTTPSProxy: "http://127.0.0.1:7890",
		NoProxy:    "localhost",
	}
	env := Environ([]string{"PATH=/usr/bin", "HTTPS_PROXY=old"}, cfg)
	if containsEnv(env, "HTTPS_PROXY=old") {
		t.Fatal("should replace old proxy")
	}
	if !containsEnv(env, "HTTPS_PROXY=http://127.0.0.1:7890") {
		t.Fatalf("env=%v", env)
	}
}

func containsEnv(env []string, want string) bool {
	for _, e := range env {
		if e == want {
			return true
		}
	}
	return false
}

func TestConfigureCmd(t *testing.T) {
	cfg := config.ProxyConfig{Enabled: true, HTTPSProxy: "http://127.0.0.1:9999"}
	cmd := exec.Command("env")
	ConfigureCmd(cmd, cfg)
	found := false
	for _, e := range cmd.Env {
		if e == "HTTPS_PROXY=http://127.0.0.1:9999" {
			found = true
		}
	}
	if !found {
		t.Fatalf("cmd env=%v", cmd.Env)
	}
	_ = os.Environ()
}
