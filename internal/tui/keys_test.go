package tui

import "testing"

func TestRouteMainKey(t *testing.T) {
	if RouteMainKey(true, "j") != KeyMoveDown || RouteMainKey(true, "k") != KeyMoveUp {
		t.Fatal("empty input should move")
	}
	if RouteMainKey(true, "s") != KeyOpenSettings || RouteMainKey(true, "esc") != KeyQuit {
		t.Fatal("empty input settings or quit")
	}
	if RouteMainKey(false, "j") != KeyInsert || RouteMainKey(false, "s") != KeyInsert {
		t.Fatal("typed text stays in the input")
	}
}

func TestRouteSettingsKey(t *testing.T) {
	if RouteSettingsKey("esc") != KeyCloseSettings {
		t.Fatal("settings esc closes the page")
	}
}
