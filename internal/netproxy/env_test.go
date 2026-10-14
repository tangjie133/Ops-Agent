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

func TestEnvironOverridesGitGlobalProxy(t *testing.T) {
	cfg := config.ProxyConfig{Enabled: true, HTTPSProxy: "http://192.168.1.178:7897"}
	env := Environ([]string{"PATH=/usr/bin"}, cfg)
	if !containsEnv(env, "GIT_CONFIG_KEY_0=http.proxy") || !containsEnv(env, "GIT_CONFIG_VALUE_0=http://192.168.1.178:7897") {
		t.Fatalf("git http.proxy not set from ops config: %v", env)
	}
	if !containsEnv(env, "GIT_CONFIG_KEY_1=https.proxy") || !containsEnv(env, "GIT_CONFIG_VALUE_1=http://192.168.1.178:7897") {
		t.Fatalf("git https.proxy not set from ops config: %v", env)
	}
	if !containsEnv(env, "GIT_CONFIG_COUNT=2") {
		t.Fatalf("GIT_CONFIG_COUNT missing: %v", env)
	}
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
