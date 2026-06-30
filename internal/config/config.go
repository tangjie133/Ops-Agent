package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ModeManual = "manual"
	ModeSemi   = "semi"
	ModeFull   = "full"
)

type Config struct {
	IssueWatch      IssueWatchConfig      `yaml:"issue_watch"`
	Webhook         WebhookConfig         `yaml:"webhook"`
	IssueAutomation IssueAutomationConfig `yaml:"issue_automation"`
	LibTest         LibTestConfig         `yaml:"lib_test"`
	Notify          NotifyConfig          `yaml:"notify"`
	AI              AIConfig              `yaml:"ai"`
	Proxy           ProxyConfig           `yaml:"proxy"`
	CI              CIConfig              `yaml:"ci"`
}

// IssueWatchConfig 定义 issue 入待办的过滤规则（由 webhook 触发，非轮询）。
type IssueWatchConfig struct {
	Enabled           bool       `yaml:"enabled"`
	Labels            []string   `yaml:"labels"`
	RequireUnassigned bool       `yaml:"require_unassigned"`
	Todo              TodoConfig `yaml:"todo"`
}

type WebhookConfig struct {
	Enabled   bool                `yaml:"enabled"`
	Listen    string              `yaml:"listen"`
	Path      string              `yaml:"path"`
	Secret    string              `yaml:"secret"`
	PublicURL string              `yaml:"public_url"` // GitHub Payload URL（smee.io 频道等）
	Tunnel    WebhookTunnelConfig `yaml:"tunnel"`
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
	Provider     string              `yaml:"provider"`
	BaseURL      string              `yaml:"base_url"`
	Model        string              `yaml:"model"`
	APIKey       string              `yaml:"api_key"`
	Investigator InvestigatorConfig  `yaml:"investigator"`
	RepoContext  RepoContextConfig   `yaml:"repo_context"`
	RAG          RAGConfig           `yaml:"rag"`
}

func (c *AIConfig) normalize() {
	c.Investigator.Normalize()
	c.RepoContext.Normalize()
	c.RAG.Normalize()
}

type CIConfig struct {
	PRCheckOnEvents []string `yaml:"pr_check_on_events"`
}

func Default() *Config {
	return &Config{
		IssueWatch: IssueWatchConfig{
			Enabled:           true,
			Labels:            []string{"ops", "needs-triage"},
			RequireUnassigned: true,
			Todo:              TodoConfig{MaxItems: 50},
		},
		Webhook: WebhookConfig{
			Enabled: true,
			Listen:  "127.0.0.1:8765",
			Path:    "/webhooks/github",
			Secret:  "",
			Tunnel: WebhookTunnelConfig{
				Smee: SmeeTunnelConfig{Enabled: true},
			},
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
		LibTest: LibTestConfig{
			Enabled:  true,
			Standard: "arduino-library",
			AutoRun:  true,
			OnPush:   true,
			OnRelease: true,
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
			Investigator: InvestigatorConfig{
				MaxSteps: 12,
			},
			RepoContext: RepoContextConfig{
				Enabled: true,
			},
			RAG: RAGConfig{
				InjectTopK:      4,
				SearchTopK:      8,
				DefaultStandard: "arduino-library",
			},
		},
		CI: CIConfig{
			PRCheckOnEvents: []string{"pull_request"},
		},
	}
}

func Load() (*Config, error) {
	cfg := Default()

	path, err := resolveConfigPath()
	if err != nil {
		return nil, err
	}
	loadedConfigPath = path
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

	if cfg.Webhook.Listen == "" {
		cfg.Webhook.Listen = "127.0.0.1:8765"
	}
	if cfg.Webhook.Path == "" {
		cfg.Webhook.Path = "/webhooks/github"
	} else if cfg.Webhook.Path[0] != '/' {
		cfg.Webhook.Path = "/" + cfg.Webhook.Path
	}
	if cfg.IssueAutomation.Mode == "" {
		cfg.IssueAutomation.Mode = ModeSemi
	}

	cfg.AI.normalize()
	cfg.Proxy.Normalize()
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
	cfg.Proxy.HTTPSProxy = os.ExpandEnv(cfg.Proxy.HTTPSProxy)
	cfg.Proxy.HTTPProxy = os.ExpandEnv(cfg.Proxy.HTTPProxy)
	cfg.Proxy.NoProxy = os.ExpandEnv(cfg.Proxy.NoProxy)
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

// IsValidMode 判断是否为支持的自动化模式。
func IsValidMode(mode string) bool {
	switch strings.ToLower(mode) {
	case ModeManual, ModeSemi, ModeFull:
		return true
	default:
		return false
	}
}

// ModeTitle 返回模式的中文名称。
func ModeTitle(mode string) string {
	switch mode {
	case ModeManual:
		return "手动"
	case ModeFull:
		return "全自动"
	default:
		return "半自动"
	}
}

// ModeDescription 返回模式的简要说明。
func ModeDescription(mode string) string {
	switch mode {
	case ModeManual:
		return "Issue 入待办后仅展示，不自动分析或回复"
	case ModeFull:
		return "自动 AI 分析并回复 issue，无需人工确认"
	default:
		return "自动 AI 分析并生成回复草稿，发送前需人工确认"
	}
}

func (c *IssueAutomationConfig) ModeSummary() string {
	return fmt.Sprintf("%s (%s) — %s", c.ModeLabel(), ModeTitle(c.Mode), ModeDescription(c.Mode))
}

// FormatModesHelp 列出全部模式及说明；current 为当前模式时在对应行前加 *。
func FormatModesHelp(current string) string {
	modes := []string{ModeManual, ModeSemi, ModeFull}
	var b strings.Builder
	b.WriteString("模式说明:\n")
	for _, mode := range modes {
		prefix := "  "
		if mode == current {
			prefix = "* "
		}
		b.WriteString(fmt.Sprintf("%s%s (%s) — %s\n", prefix, mode, ModeTitle(mode), ModeDescription(mode)))
	}
	return strings.TrimRight(b.String(), "\n")
}

func TodoStorePath() string {
	if p := os.Getenv("OPS_AGENT_DATA"); p != "" {
		return filepath.Join(p, "todo.json")
	}
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "ops-agent", "todo.json")
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share", "ops-agent", "todo.json")
	}
	return "todo.json"
}

// LogFilePath TUI 会话日志（纯文本，可 tail -f 或编辑器打开）。
func LogFilePath() string {
	if p := os.Getenv("OPS_AGENT_DATA"); p != "" {
		return filepath.Join(p, "logs", "tui.log")
	}
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "ops-agent", "logs", "tui.log")
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share", "ops-agent", "logs", "tui.log")
	}
	return filepath.Join("logs", "tui.log")
}

// DiagLogFilePath TUI 诊断日志（性能/桥接/慢 Update，tail -f 排查卡顿）。
func DiagLogFilePath() string {
	if p := os.Getenv("OPS_AGENT_DATA"); p != "" {
		return filepath.Join(p, "logs", "tui-diag.log")
	}
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "ops-agent", "logs", "tui-diag.log")
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share", "ops-agent", "logs", "tui-diag.log")
	}
	return filepath.Join("logs", "tui-diag.log")
}

// WebhookAddr 返回 webhook 监听地址（listen + path）。
func (c *Config) WebhookAddr() string {
	return c.Webhook.Listen + c.Webhook.Path
}
