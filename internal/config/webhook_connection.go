package config

import (
	"fmt"
	"net"
	"strings"
)

// ValidateWebhookListen 校验 host:port 监听地址。
func ValidateWebhookListen(listen string) error {
	listen = strings.TrimSpace(listen)
	if listen == "" {
		return fmt.Errorf("监听地址不能为空")
	}
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return fmt.Errorf("格式应为 host:port，例如 127.0.0.1:8765")
	}
	if host == "" || port == "" {
		return fmt.Errorf("host 与 port 均不能为空")
	}
	return nil
}

// NormalizeWebhookPath 规范化 webhook 路径。
func NormalizeWebhookPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/webhooks/github"
	}
	if path[0] != '/' {
		return "/" + path
	}
	return path
}

// ValidateWebhookPath 校验 webhook 路径。
func ValidateWebhookPath(path string) error {
	path = NormalizeWebhookPath(path)
	if path == "" || path[0] != '/' {
		return fmt.Errorf("路径必须以 / 开头")
	}
	return nil
}

func FormatWebhookSecretDisplay(secret string) string {
	if secret == "" {
		return "未设置"
	}
	if len(secret) <= 8 {
		return secret
	}
	return secret[:4] + "…" + secret[len(secret)-2:]
}

func FormatWebhookPublicURLDisplay(publicURL string) string {
	if publicURL == "" {
		return "未设置"
	}
	if len(publicURL) > 40 {
		return publicURL[:37] + "..."
	}
	return publicURL
}

// WebhookConnField 连接配置项说明（供 TUI 菜单与 yaml 注释对齐）。
type WebhookConnField struct {
	Title       string
	Description string
	Placeholder string
}

// WebhookConnFields 按顺序：listen、path、secret、public_url。
func WebhookConnFields() []WebhookConnField {
	return []WebhookConnField{
		{
			Title: "监听地址",
			Description: "本机 webhook HTTP 服务绑定的 host:port；GitHub 事件经 smee 转发后到达此处",
			Placeholder: "127.0.0.1:8765",
		},
		{
			Title: "Webhook 路径",
			Description: "与 listen 组成本地 URL（如 /webhooks/github）；smee 转发目标须包含此路径",
			Placeholder: "/webhooks/github",
		},
		{
			Title: "Secret",
			Description: "与 GitHub Webhook 配置的 Secret 一致，用于校验 X-Hub-Signature-256；留空则仅本地调试",
			Placeholder: "与 GitHub 侧 secret 相同",
		},
		{
			Title: "Public URL",
			Description: "GitHub Payload URL：填 smee 频道根地址（如 https://smee.io/ID），不要加 /webhook；/webhook 仅用于本机 path",
			Placeholder: "https://smee.io/your-id",
		},
	}
}

// WebhookConnectionIntro 连接配置总览。
func WebhookConnectionIntro() string {
	return "流程: GitHub(Organization/仓库 Webhook) → Payload URL(smee.io) → 内嵌 Smee → 本机 listen+path；按 payload 跨仓库同步"
}

// ConnectionSummary 连接配置一行摘要。
func (c *Config) ConnectionSummary() string {
	w := c.Webhook
	return w.Listen + w.Path + " · Secret:" + FormatWebhookSecretDisplay(w.Secret) + " · URL:" + FormatWebhookPublicURLDisplay(w.PublicURL)
}
