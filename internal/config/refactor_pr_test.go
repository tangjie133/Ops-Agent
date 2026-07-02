package config

import "testing"

func TestRefactorPRCycleTrigger(t *testing.T) {
	c := RefactorPRConfig{Enabled: true, Trigger: RefactorPRTriggerManual}
	c.CycleTrigger()
	if c.Trigger != RefactorPRTriggerApproval {
		t.Fatalf("got %q", c.Trigger)
	}
	c.CycleTrigger()
	if c.Trigger != "both" {
		t.Fatalf("got %q", c.Trigger)
	}
	c.CycleTrigger()
	if c.Trigger != RefactorPRTriggerManual {
		t.Fatalf("got %q", c.Trigger)
	}
}

func TestRefactorPRLabels(t *testing.T) {
	off := RefactorPRConfig{}
	if off.EnabledLabel() != "已禁用" {
		t.Fatal(off.EnabledLabel())
	}
	if off.TriggerLabel() != "—（需先启用）" {
		t.Fatal(off.TriggerLabel())
	}

	on := RefactorPRConfig{Enabled: true, Trigger: "both"}
	if on.Summary() != "启用 · f 确认 + /approve-pr" {
		t.Fatal(on.Summary())
	}
}
