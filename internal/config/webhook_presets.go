package config

import (
	"fmt"
	"sort"
)

// LabelPreset Issue 入队 label 过滤预设。
type LabelPreset struct {
	ID          string
	Title       string
	Description string
	Labels      []string
}

// LabelPresets 返回可选的 label 过滤策略。
func LabelPresets() []LabelPreset {
	return []LabelPreset{
		{
			ID:          "all",
			Title:       "全部",
			Description: "不过滤 label，所有 open issue 均可入队",
			Labels:      nil,
		},
		{
			ID:          "ops-triage",
			Title:       "ops + needs-triage",
			Description: "命中 ops 或 needs-triage 任一标签",
			Labels:      []string{"ops", "needs-triage"},
		},
		{
			ID:          "ops",
			Title:       "仅 ops",
			Description: "仅命中 ops 标签",
			Labels:      []string{"ops"},
		},
	}
}

// LabelPresetIndex 返回当前 labels 对应的预设下标。
func LabelPresetIndex(labels []string) int {
	presets := LabelPresets()
	for i, p := range presets {
		if labelsEqual(labels, p.Labels) {
			return i
		}
	}
	return 1 // 默认 ops-triage
}

// ApplyLabelPreset 应用 label 预设。
func ApplyLabelPreset(cfg *Config, index int) {
	presets := LabelPresets()
	if index < 0 || index >= len(presets) {
		return
	}
	cfg.IssueWatch.Labels = append([]string(nil), presets[index].Labels...)
}

// CurrentLabelPresetTitle 返回当前 label 策略名称。
func CurrentLabelPresetTitle(cfg *Config) string {
	return LabelPresets()[LabelPresetIndex(cfg.IssueWatch.Labels)].Title
}

func labelsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aa := append([]string(nil), a...)
	bb := append([]string(nil), b...)
	sort.Strings(aa)
	sort.Strings(bb)
	for i := range aa {
		if aa[i] != bb[i] {
			return false
		}
	}
	return true
}

// WebhookSummary 一行摘要，供菜单展示。
func (c *Config) WebhookSummary() string {
	wh := "关"
	if c.Webhook.Enabled {
		wh = "开"
	}
	iw := "关"
	if c.IssueWatch.Enabled {
		iw = "开"
	}
	unassigned := "否"
	if c.IssueWatch.RequireUnassigned {
		unassigned = "是"
	}
	return fmt.Sprintf("Webhook %s · Issue监视 %s · 未指派 %s · Labels: %s",
		wh, iw, unassigned, CurrentLabelPresetTitle(c))
}
