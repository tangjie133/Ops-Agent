package tui

import "fmt"

// renderSettingsPage 只显示模型、webhook 和代理的当前值。
func (m *Model) renderSettingsPage() string {
	if m.cfg == nil {
		return "设置\n没有配置\nEsc 返回"
	}
	ai := "不可达"
	if m.aiOK {
		ai = "可达"
	}
	wh := "关"
	if m.cfg.Webhook.Enabled {
		wh = "开 " + m.cfg.Webhook.Listen
	}
	proxy := "关"
	if m.cfg.Proxy.Enabled {
		proxy = "开 " + m.cfg.Proxy.EffectiveHTTPS()
	}
	return fmt.Sprintf("设置  Esc 返回\n\n模型 ai.model：%s（%s）\nWebhook webhook.enabled：%s\n代理 proxy.https_proxy：%s\n",
		m.cfg.AI.Model, ai, wh, proxy)
}
