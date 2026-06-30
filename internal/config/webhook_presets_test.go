package config

import "testing"

func TestLabelPresetIndex(t *testing.T) {
	if LabelPresetIndex(nil) != 0 {
		t.Fatal("nil labels should match all")
	}
	if LabelPresetIndex([]string{"ops", "needs-triage"}) != 1 {
		t.Fatal("expected ops-triage preset")
	}
	if LabelPresetIndex([]string{"ops"}) != 2 {
		t.Fatal("expected ops preset")
	}
}

func TestApplyLabelPreset(t *testing.T) {
	cfg := Default()
	ApplyLabelPreset(cfg, 0)
	if len(cfg.IssueWatch.Labels) != 0 {
		t.Fatalf("labels=%v", cfg.IssueWatch.Labels)
	}
	ApplyLabelPreset(cfg, 2)
	if len(cfg.IssueWatch.Labels) != 1 || cfg.IssueWatch.Labels[0] != "ops" {
		t.Fatalf("labels=%v", cfg.IssueWatch.Labels)
	}
}

func TestWebhookSummary(t *testing.T) {
	cfg := Default()
	if cfg.WebhookSummary() == "" {
		t.Fatal("expected summary")
	}
}
