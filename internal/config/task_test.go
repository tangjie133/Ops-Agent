package config

import "testing"

func TestTaskConfigDefaults(t *testing.T) {
	cfg := Default()
	cfg.Task.Normalize()
	if !cfg.Task.Idempotency {
		t.Fatal("idempotency default true")
	}
	if cfg.Task.IssueToPR.Enabled {
		t.Fatal("issue_to_pr must default off")
	}
	if cfg.Task.Workspace.Mode != "worktree" {
		t.Fatalf("mode=%s", cfg.Task.Workspace.Mode)
	}
	if cfg.Task.Workspace.CleanupAfter != "72h" {
		t.Fatalf("cleanup=%s", cfg.Task.Workspace.CleanupAfter)
	}
	if !cfg.Task.Workspace.KeepFailed {
		t.Fatal("keep_failed")
	}
	if cfg.Task.Budget.MaxSteps != 12 || cfg.Task.Budget.MaxRetries != 3 || cfg.Task.Budget.MaxDuration != "30m" || cfg.Task.Budget.MaxCommands != 20 || cfg.Task.Budget.MaxLLMCalls != 12 {
		t.Fatalf("budget %+v", cfg.Task.Budget)
	}
	if !cfg.Task.Feedback.IgnoreKnownBotNoise || !cfg.Task.Feedback.EscalateOnUnknownReview {
		t.Fatalf("feedback %+v", cfg.Task.Feedback)
	}
}
