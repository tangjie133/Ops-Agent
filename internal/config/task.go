package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// TaskConfig 是统一任务编排相关配置。
type TaskConfig struct {
	StorePath   string          `yaml:"store_path"`
	Idempotency bool            `yaml:"idempotency"`
	IssueToPR   IssueToPRConfig `yaml:"issue_to_pr"`
	Workspace   WorkspaceConfig `yaml:"workspace"`
	Budget      BudgetConfig    `yaml:"budget"`
	Feedback    FeedbackConfig  `yaml:"feedback"`
}

// IssueToPRConfig 控制 Issue 到 draft PR 是否作为主流程。
type IssueToPRConfig struct {
	Enabled bool `yaml:"enabled"`
}

// WorkspaceConfig 控制任务工作区隔离与清理。
type WorkspaceConfig struct {
	Mode         string `yaml:"mode"`
	KeepFailed   bool   `yaml:"keep_failed"`
	CleanupAfter string `yaml:"cleanup_after"`
	Root         string `yaml:"root"`
}

// BudgetConfig 限制单个任务的自动化消耗。
type BudgetConfig struct {
	MaxSteps    int    `yaml:"max_steps"`
	MaxRetries  int    `yaml:"max_retries"`
	MaxDuration string `yaml:"max_duration"`
	MaxCommands int    `yaml:"max_commands"`
	MaxLLMCalls int    `yaml:"max_llm_calls"`
}

// FeedbackConfig 控制外部反馈的最小分类策略。
type FeedbackConfig struct {
	IgnoreKnownBotNoise     bool `yaml:"ignore_known_bot_noise"`
	EscalateOnUnknownReview bool `yaml:"escalate_on_unknown_review"`
}

// Normalize 补齐任务编排配置的空缺默认值，不覆盖显式 false。
func (c *TaskConfig) Normalize() {
	if c.Workspace.Mode == "" {
		c.Workspace.Mode = "worktree"
	}
	if c.Workspace.CleanupAfter == "" {
		c.Workspace.CleanupAfter = "72h"
	}
	if c.Budget.MaxSteps <= 0 {
		c.Budget.MaxSteps = 12
	}
	if c.Budget.MaxRetries <= 0 {
		c.Budget.MaxRetries = 3
	}
	if c.Budget.MaxDuration == "" {
		c.Budget.MaxDuration = "30m"
	}
	if c.Budget.MaxCommands <= 0 {
		c.Budget.MaxCommands = 20
	}
	if c.Budget.MaxLLMCalls <= 0 {
		c.Budget.MaxLLMCalls = 12
	}
}

// TaskStorePath 返回任务队列文件。配置里的 store_path 优先。
func TaskStorePath(cfg *Config) string {
	if cfg != nil && cfg.Task.StorePath != "" {
		return cfg.Task.StorePath
	}
	if p := os.Getenv("OPS_AGENT_DATA"); p != "" {
		return filepath.Join(p, "tasks.json")
	}
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "ops-agent", "tasks.json")
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share", "ops-agent", "tasks.json")
	}
	return "tasks.json"
}
