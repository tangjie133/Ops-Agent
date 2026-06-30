package config

import (
	"fmt"
	"net/url"
	"strings"
)

// ProxyConfig 供 git/gh 克隆仓库时使用的 HTTP(S) 代理（如翻墙）。
type ProxyConfig struct {
	Enabled    bool   `yaml:"enabled"`
	HTTPSProxy string `yaml:"https_proxy"`
	HTTPProxy  string `yaml:"http_proxy"`
	NoProxy    string `yaml:"no_proxy"`
}

func (p *ProxyConfig) Normalize() {
	p.HTTPSProxy = strings.TrimSpace(p.HTTPSProxy)
	p.HTTPProxy = strings.TrimSpace(p.HTTPProxy)
	p.NoProxy = strings.TrimSpace(p.NoProxy)
}

// EffectiveHTTPS 返回实际用于 HTTPS 的代理地址。
func (p *ProxyConfig) EffectiveHTTPS() string {
	if p.HTTPSProxy != "" {
		return p.HTTPSProxy
	}
	return p.HTTPProxy
}

func (p *ProxyConfig) Summary() string {
	if !p.Enabled {
		return "关闭"
	}
	ep := p.EffectiveHTTPS()
	if ep == "" {
		return "已启用（未设置地址）"
	}
	return "开 · " + FormatProxyDisplay(ep)
}

func FormatProxyDisplay(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "—"
	}
	if len(raw) > 40 {
		return raw[:37] + "..."
	}
	return raw
}

func ValidateProxyURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("无效的代理 URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "socks5" {
		return fmt.Errorf("代理须为 http、https 或 socks5")
	}
	if u.Host == "" {
		return fmt.Errorf("缺少 host")
	}
	return nil
}

func (p *ProxyConfig) Validate() error {
	if !p.Enabled {
		return nil
	}
	if err := ValidateProxyURL(p.HTTPSProxy); err != nil {
		return fmt.Errorf("https_proxy: %w", err)
	}
	if err := ValidateProxyURL(p.HTTPProxy); err != nil {
		return fmt.Errorf("http_proxy: %w", err)
	}
	return nil
}

// ProxyConnField 代理配置项（TUI 菜单）。
type ProxyConnField struct {
	Title       string
	Description string
	Placeholder string
}

func ProxyConnFields() []ProxyConnField {
	return []ProxyConnField{
		{
			Title:       "HTTPS Proxy",
			Description: "克隆 GitHub 仓库时使用，如 http://127.0.0.1:7890（Clash/v2ray 本地端口）",
			Placeholder: "http://127.0.0.1:7890",
		},
		{
			Title:       "HTTP Proxy",
			Description: "可选；留空则 HTTPS 请求复用 HTTPS Proxy",
			Placeholder: "http://127.0.0.1:7890",
		},
		{
			Title:       "No Proxy",
			Description: "不走代理的域名，逗号分隔，如 127.0.0.1,localhost",
			Placeholder: "127.0.0.1,localhost",
		},
	}
}

func ProxyConnectionIntro() string {
	return "仅作用于 Ops-Agent 发起的 gh repo clone / git pull；修改后自动保存，可测试 GitHub 连通性"
}
