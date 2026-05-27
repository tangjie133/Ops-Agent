package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	ModeManual = "manual"
	ModeSemi   = "semi"
	ModeFull   = "full"
)

type Config struct {
	IssueWatch      IssueWatchConfig      `yaml:"issue_watch"`
	IssueAutomation IssueAutomationConfig `yaml:"issue_automation"`
	Notify          NotifyConfig          `yaml:"notify"`
	AI              AIConfig              `yaml:"ai"`
	CI              CIConfig              `yaml:"ci"`
}

type IssueWatchConfig struct {
	Enabled           bool          `yaml:"enabled"`
	Interval          time.Duration `yaml:"interval"`
	Repo              string        `yaml:"repo"`
	Labels            []string      `yaml:"labels"`
	RequireUnassigned bool          `yaml:"require_unassigned"`
	Todo              TodoConfig    `yaml:"todo"`
}

type TodoConfig struct {
	MaxItems int `yaml:"max_items"`
}

type IssueAutomationConfig struct {
	Mode               string          `yaml:"mode"`
	AutoAnalyze        bool            `yaml:"auto_analyze"`
	ConfirmBeforeReply bool            `yaml:"confirm_before_reply"`
	AutoReply          AutoReplyConfig `yaml:"auto_reply"`
	NotifyOnReady      bool            `yaml:"notify_on_ready"`
	NotifyOnPosted     bool            `yaml:"notify_on_posted"`
}

type AutoReplyConfig struct {
	OnlyLabels         []string `yaml:"only_labels"`
	MaxCommentsPerHour int      `yaml:"max_comments_per_hour"`
	CommentFooter      string   `yaml:"comment_footer"`
}

type NotifyConfig struct {
	OnFailure bool                     `yaml:"on_failure"`
	Channels  map[string]ChannelConfig `yaml:"channels"`
}

type ChannelConfig struct {
	Enabled    bool   `yaml:"enabled"`
	WebhookURL string `yaml:"webhook_url"`
}

type AIConfig struct {
	Provider string `yaml:"provider"`
	BaseURL  string `yaml:"base_url"`
	Model    string `yaml:"model"`
	APIKey   string `yaml:"api_key"`
}

type CIConfig struct {
	PRCheckOnEvents []string          `yaml:"pr_check_on_events"`
	IssueScan       IssueScanCIConfig `yaml:"issue_scan"`
}

type IssueScanCIConfig struct {
	Enabled     bool `yaml:"enabled"`
	FailOnMatch bool `yaml:"fail_on_match"`
}

func Default() *Config {
	return &Config{
		IssueWatch: IssueWatchConfig{
			Enabled:           true,
			Interval:          5 * time.Minute,
			Labels:            []string{"ops", "needs-triage"},
			RequireUnassigned: true,
			Todo:              TodoConfig{MaxItems: 50},
		},
		IssueAutomation: IssueAutomationConfig{
			Mode:               ModeSemi,
			AutoAnalyze:        true,
			ConfirmBeforeReply: true,
			AutoReply: AutoReplyConfig{
				MaxCommentsPerHour: 10,
				CommentFooter:      "---\n_Posted by Ops-Agent (auto)_",
			},
		},
		Notify: NotifyConfig{
			OnFailure: true,
			Channels: map[string]ChannelConfig{
				"slack":    {Enabled: false},
				"feishu":   {Enabled: false},
				"dingtalk": {Enabled: false},
			},
		},
		AI: AIConfig{
			Provider: "openai-compatible",
			BaseURL:  "http://127.0.0.1:8080/v1",
			Model:    "qwen2.5-coder",
			APIKey:   "local",
		},
		CI: CIConfig{
			PRCheckOnEvents: []string{"pull_request"},
			IssueScan: IssueScanCIConfig{
				Enabled:     false,
				FailOnMatch: false,
			},
		},
	}
}

func Load() (*Config, error) {
	cfg := Default()

	path, err := resolveConfigPath()
	if err != nil {
		return nil, err
	}
	if path == "" {
		expandEnv(cfg)
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	if cfg.IssueWatch.Interval <= 0 {
		cfg.IssueWatch.Interval = 5 * time.Minute
	}
	if cfg.IssueAutomation.Mode == "" {
		cfg.IssueAutomation.Mode = ModeSemi
	}

	expandEnv(cfg)
	return cfg, nil
}

func resolveConfigPath() (string, error) {
	if p := os.Getenv("OPS_AGENT_CONFIG"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("OPS_AGENT_CONFIG not found: %s", p)
	}

	local := ".ops-agent.yaml"
	if _, err := os.Stat(local); err == nil {
		return local, nil
	}

	user, err := os.UserConfigDir()
	if err != nil {
		return "", nil
	}
	global := filepath.Join(user, "ops-agent", "config.yaml")
	if _, err := os.Stat(global); err == nil {
		return global, nil
	}

	return "", nil
}

func expandEnv(cfg *Config) {
	cfg.AI.BaseURL = os.ExpandEnv(cfg.AI.BaseURL)
	cfg.AI.APIKey = os.ExpandEnv(cfg.AI.APIKey)
	for name, ch := range cfg.Notify.Channels {
		ch.WebhookURL = os.ExpandEnv(ch.WebhookURL)
		cfg.Notify.Channels[name] = ch
	}
}

func (c *IssueAutomationConfig) ModeLabel() string {
	switch c.Mode {
	case ModeManual:
		return "manual"
	case ModeFull:
		return "full"
	default:
		return "semi"
	}
}

func (c *IssueAutomationConfig) SetMode(mode string) {
	switch strings.ToLower(mode) {
	case ModeManual:
		c.Mode = ModeManual
		c.AutoAnalyze = false
	case ModeFull:
		c.Mode = ModeFull
		c.AutoAnalyze = true
		c.ConfirmBeforeReply = false
	default:
		c.Mode = ModeSemi
		c.AutoAnalyze = true
		c.ConfirmBeforeReply = true
	}
}
