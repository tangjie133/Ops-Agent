package tui

import (
	"os"
	"strings"
	"testing"
)

func TestCommandSourceDropsMenus(t *testing.T) {
	data, err := os.ReadFile("commands.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, name := range []string{"openModeMenu", "openAIMenu", "openProxyMenu", "openWebhookMenu"} {
		if strings.Contains(text, name) {
			t.Fatalf("still references %s", name)
		}
	}
}
