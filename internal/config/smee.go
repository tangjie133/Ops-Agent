package config

import (
	"fmt"
	"net/url"
	"strings"
)

// SmeeTunnelConfig 内嵌 smee.io 客户端，将公网 webhook 转发到本地 listen+path。
type SmeeTunnelConfig struct {
	Enabled bool `yaml:"enabled"`
}

// WebhookTunnelConfig 公网接入隧道（当前支持 smee）。
type WebhookTunnelConfig struct {
	Smee SmeeTunnelConfig `yaml:"smee"`
}

// NormalizeSmeeChannelURL 去掉 smee 频道 URL 上多余的 path（如误填的 /webhook）。
func NormalizeSmeeChannelURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	host := strings.ToLower(u.Host)
	if host != "smee.io" && !strings.HasSuffix(host, ".smee.io") {
		return raw
	}
	seg := strings.Trim(u.Path, "/")
	if seg == "" {
		return strings.TrimRight(raw, "/")
	}
	id := strings.Split(seg, "/")[0]
	u.Path = "/" + id
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.Scheme+"://"+u.Host+u.Path, "/")
}

// SmeeChannelURL 返回 smee 频道地址；使用 public_url 作为 channel。
func (w *WebhookConfig) SmeeChannelURL() string {
	return NormalizeSmeeChannelURL(w.PublicURL)
}

// IsSmeeChannelURL 判断 URL 是否为 smee.io 频道（内嵌隧道仅支持 smee）。
func IsSmeeChannelURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	host := strings.ToLower(u.Host)
	return host == "smee.io" || strings.HasSuffix(host, ".smee.io")
}

// SmeeTunnelActive 是否应启动内嵌 smee 客户端。
func (w *WebhookConfig) SmeeTunnelActive() bool {
	if !w.Enabled || !w.Tunnel.Smee.Enabled {
		return false
	}
	u := w.SmeeChannelURL()
	return u != "" && IsSmeeChannelURL(u)
}

// PayloadURL 供 GitHub Webhooks 填写的公网地址。
func (w *WebhookConfig) PayloadURL() string {
	if u := w.SmeeChannelURL(); u != "" {
		return u
	}
	return strings.TrimSpace(w.PublicURL)
}

// ValidateSmeeChannelURL 校验 smee 频道 URL。
func ValidateSmeeChannelURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("smee 频道 URL 不能为空")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("无效的 URL: %w", err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("URL 须以 http:// 或 https:// 开头")
	}
	if u.Host == "" {
		return fmt.Errorf("URL 缺少主机名")
	}
	return nil
}

func (w *WebhookConfig) SmeeToggleLabel() string {
	if w.Tunnel.Smee.Enabled {
		return "已启用"
	}
	return "已禁用"
}

// SmeeHint 返回 smee 隧道配置提示（空表示无问题）。
func (w *WebhookConfig) SmeeHint() string {
	if !w.Tunnel.Smee.Enabled {
		return ""
	}
	u := w.SmeeChannelURL()
	if u == "" {
		return "请配置 Public URL（https://smee.io/…）"
	}
	if !IsSmeeChannelURL(u) {
		return "Public URL 需改为 smee.io 频道"
	}
	return ""
}

func (w *WebhookConfig) SmeeStatusLabel() string {
	if !w.Tunnel.Smee.Enabled {
		return "已禁用"
	}
	if hint := w.SmeeHint(); hint != "" {
		return "已启用 · " + hint
	}
	return "已启用 · 就绪"
}
