package config

import (
	"fmt"
	"net/url"
	"strings"
)

// AIConnField 模型连接配置项说明（供 TUI 菜单与 yaml 对齐）。
type AIConnField struct {
	Title       string
	Description string
	Placeholder string
}

// AIConnFields 按顺序：base_url、model、api_key。
func AIConnFields() []AIConnField {
	return []AIConnField{
		{
			Title:       "Base URL",
			Description: "llama-server 或 OpenAI 兼容 API 根地址，需含 /v1（如 http://127.0.0.1:8080/v1）",
			Placeholder: "http://127.0.0.1:8080/v1",
		},
		{
			Title:       "Model",
			Description: "模型名称，与 llama-server --model 或 /v1/models 列表一致",
			Placeholder: "qwen2.5-coder",
		},
		{
			Title:       "API Key",
			Description: "Bearer token；本地 llama-server 可填 local 或留空",
			Placeholder: "local",
		},
	}
}

// AIConnectionIntro 模型配置总览。
func AIConnectionIntro() string {
	return "Worker 与 Agent 通过 OpenAI 兼容 HTTP 调用本地模型；修改后自动保存并重新检测连通性"
}

// AISummary 一行摘要。
func (c *Config) AISummary() string {
	a := c.AI
	model := a.Model
	if model == "" {
		model = "—"
	}
	base := FormatAIBaseURLDisplay(a.BaseURL)
	return base + " · " + model + " · Key:" + FormatAIAPIKeyDisplay(a.APIKey)
}

func FormatAIBaseURLDisplay(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return "未设置"
	}
	if len(baseURL) > 36 {
		return baseURL[:33] + "..."
	}
	return baseURL
}

func FormatAIAPIKeyDisplay(key string) string {
	key = strings.TrimSpace(key)
	if key == "" || key == "local" {
		return "local"
	}
	if len(key) <= 8 {
		return key
	}
	return key[:4] + "…" + key[len(key)-2:]
}

func NormalizeAIBaseURL(raw string) string {
	return strings.TrimSuffix(strings.TrimSpace(raw), "/")
}

func ValidateAIBaseURL(raw string) error {
	raw = NormalizeAIBaseURL(raw)
	if raw == "" {
		return fmt.Errorf("base_url 不能为空")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("无效的 URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("须为 http 或 https")
	}
	if u.Host == "" {
		return fmt.Errorf("缺少 host")
	}
	return nil
}

func ValidateAIModel(model string) error {
	if strings.TrimSpace(model) == "" {
		return fmt.Errorf("model 不能为空")
	}
	return nil
}
