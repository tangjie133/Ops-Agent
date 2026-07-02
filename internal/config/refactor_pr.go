package config

import (
	"fmt"
	"strings"
)

const (
	RefactorPRTriggerManual    = "manual"             // TUI f + 确认框
	RefactorPRTriggerApproval  = "on_comment_approval" // Issue 评论 /approve-pr
)

// RefactorPRConfig Issue 确认后重构并开 PR（与 Issue 评论 mode 独立配置）。
type RefactorPRConfig struct {
	Enabled      bool     `yaml:"enabled"`
	Trigger      string   `yaml:"trigger"` // manual | on_comment_approval | both
	BranchPrefix string   `yaml:"branch_prefix"`
	TestCommands []string `yaml:"test_commands"` // 空则自动 go test ./...
	MaxSteps     int      `yaml:"max_steps"`     // 0 = 使用 ai.investigator.max_steps
}

func (c *RefactorPRConfig) Normalize() {
	switch c.Trigger {
	case "", RefactorPRTriggerManual:
		c.Trigger = RefactorPRTriggerManual
	case RefactorPRTriggerApproval, "both":
		// keep
	default:
		c.Trigger = RefactorPRTriggerManual
	}
	if strings.TrimSpace(c.BranchPrefix) == "" {
		c.BranchPrefix = "ops-agent/issue-"
	}
	if c.MaxSteps < 0 {
		c.MaxSteps = 0
	}
}

func (c *RefactorPRConfig) BranchName(issueNum int) string {
	return c.BranchPrefix + fmt.Sprintf("%d", issueNum)
}

func (c *RefactorPRConfig) ManualEnabled() bool {
	if !c.Enabled {
		return false
	}
	return c.Trigger == RefactorPRTriggerManual || c.Trigger == "both"
}

func (c *RefactorPRConfig) CommentApprovalEnabled() bool {
	if !c.Enabled {
		return false
	}
	return c.Trigger == RefactorPRTriggerApproval || c.Trigger == "both"
}

func (c *RefactorPRConfig) Summary() string {
	if !c.Enabled {
		return "关闭"
	}
	switch c.Trigger {
	case RefactorPRTriggerApproval:
		return "启用 · 评论 /approve-pr"
	case "both":
		return "启用 · f 确认 + /approve-pr"
	default:
		return "启用 · TUI f 确认"
	}
}

func (c *RefactorPRConfig) EnabledLabel() string {
	if c.Enabled {
		return "已启用"
	}
	return "已禁用"
}

func (c *RefactorPRConfig) TriggerLabel() string {
	if !c.Enabled {
		return "—（需先启用）"
	}
	switch c.Trigger {
	case RefactorPRTriggerApproval:
		return "/approve-pr 评论"
	case "both":
		return "f + /approve-pr"
	default:
		return "TUI f 确认"
	}
}

// CycleTrigger 在 manual → on_comment_approval → both 间循环。
func (c *RefactorPRConfig) CycleTrigger() {
	switch c.Trigger {
	case RefactorPRTriggerManual:
		c.Trigger = RefactorPRTriggerApproval
	case RefactorPRTriggerApproval:
		c.Trigger = "both"
	default:
		c.Trigger = RefactorPRTriggerManual
	}
}

func (c *RefactorPRConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	switch c.Trigger {
	case RefactorPRTriggerManual, RefactorPRTriggerApproval, "both":
		return nil
	default:
		return fmt.Errorf("refactor_pr.trigger: 无效值 %q", c.Trigger)
	}
}
