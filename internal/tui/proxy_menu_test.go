package tui

import "testing"

func TestIsProxyMenuCommand(t *testing.T) {
	for _, cmd := range []string{"/proxy", "/vpn", "/网络", " /PROXY "} {
		if !isProxyMenuCommand(cmd) {
			t.Fatalf("want proxy command %q", cmd)
		}
	}
	if isProxyMenuCommand("/proxy x") || isProxyMenuCommand("/model") {
		t.Fatal("should not match")
	}
}
