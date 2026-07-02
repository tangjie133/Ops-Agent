package config

import "testing"

func TestLibTestSummary(t *testing.T) {
	c := LibTestConfig{Enabled: true, AutoRun: true, Standard: "arduino-library"}
	c.Normalize()
	if c.Summary() != "自动 · arduino-library · push+release" {
		t.Fatalf("got %q", c.Summary())
	}
	c.AutoRun = false
	if c.RunModeLabel() != "手动" {
		t.Fatal("expected manual")
	}
}
