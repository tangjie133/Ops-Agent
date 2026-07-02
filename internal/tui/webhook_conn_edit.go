package tui

// webhook_conn_edit.go — Webhook 连接字段（listen/path/secret 等）的内联编辑。

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

const webhookEditNone = -1

func (m *Model) initConnInput() {
	if m.connInput.Width > 0 {
		return
	}
	ti := textinput.New()
	ti.CharLimit = 512
	ti.Width = 60
	m.connInput = ti
}

func (m *Model) webhookConnFieldLabel(field webhookConnectionItem) string {
	return config.WebhookConnFields()[int(field)].Title
}

func (m *Model) webhookConnPlaceholder(field webhookConnectionItem) string {
	return config.WebhookConnFields()[int(field)].Placeholder
}

func (m *Model) webhookConnDescription(field webhookConnectionItem) string {
	return config.WebhookConnFields()[int(field)].Description
}

func (m *Model) webhookConnCurrentValue(field webhookConnectionItem) string {
	w := m.cfg.Webhook
	switch field {
	case webhookConnListen:
		return w.Listen
	case webhookConnPath:
		return w.Path
	case webhookConnSecret:
		return w.Secret
	case webhookConnPublicURL:
		return w.PublicURL
	default:
		return ""
	}
}

func (m *Model) startWebhookConnEdit(field webhookConnectionItem) {
	m.initConnInput()
	m.webhookEditField = int(field)
	m.connInput.SetValue(m.webhookConnCurrentValue(field))
	m.connInput.Placeholder = m.webhookConnPlaceholder(field)
	m.connInput.Focus()
	if m.width > 8 {
		m.connInput.Width = max(20, m.width-8)
	}
	m.layout()
}

func (m *Model) cancelWebhookConnEdit() {
	m.webhookEditField = webhookEditNone
	m.connInput.Blur()
	m.layout()
}

func (m *Model) commitWebhookConnEdit() {
	field := webhookConnectionItem(m.webhookEditField)
	val := strings.TrimSpace(m.connInput.Value())
	restart := false
	label := m.webhookConnFieldLabel(field)

	switch field {
	case webhookConnListen:
		if err := config.ValidateWebhookListen(val); err != nil {
			m.setMenuNotice("监听地址无效: " + err.Error())
			m.cancelWebhookConnEdit()
			return
		}
		m.cfg.Webhook.Listen = val
		restart = true
	case webhookConnPath:
		val = config.NormalizeWebhookPath(val)
		if err := config.ValidateWebhookPath(val); err != nil {
			m.setMenuNotice("路径无效: " + err.Error())
			m.cancelWebhookConnEdit()
			return
		}
		m.cfg.Webhook.Path = val
		restart = true
	case webhookConnSecret:
		m.cfg.Webhook.Secret = val
	case webhookConnPublicURL:
		if val != "" {
			if err := config.ValidateSmeeChannelURL(val); err != nil {
				m.setMenuNotice("Public URL 无效: " + err.Error())
				m.cancelWebhookConnEdit()
				return
			}
			normalized := config.NormalizeSmeeChannelURL(val)
			if normalized != val {
				val = normalized
				label = "Public URL（已去掉多余路径）"
			}
		}
		m.cfg.Webhook.PublicURL = val
		restart = true
		if val != "" && !config.IsSmeeChannelURL(val) {
			label = "Public URL（非 smee，隧道不会启动）"
		}
	default:
		m.cancelWebhookConnEdit()
		return
	}

	m.cancelWebhookConnEdit()
	msg := persistConfig(m.cfg) + " · " + label + " 已生效"
	if restart && m.whRuntime != nil && m.cfg.Webhook.Enabled {
		if err := m.whRuntime.Restart(m.cfg); err != nil {
			msg += " · 重启失败: " + err.Error()
		} else {
			msg += " · 监听 " + m.whRuntime.ListenURL()
		}
	} else if restart && !m.cfg.Webhook.Enabled {
		msg += " · 开启 Webhook 接收后监听生效"
	}
	m.setMenuNotice(msg)
}

func (m *Model) handleWebhookConnEdit(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.cancelWebhookConnEdit()
			return m, nil
		case "enter":
			m.commitWebhookConnEdit()
			return m, textinput.Blink
		}
	}
	var cmd tea.Cmd
	m.connInput, cmd = m.connInput.Update(msg)
	m.markDirty()
	return m, cmd
}

func (m *Model) renderWebhookConnEditBar() string {
	if m.webhookEditField < 0 {
		return ""
	}
	field := webhookConnectionItem(m.webhookEditField)
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(styleModeMenuTitle.Render("编辑 "+m.webhookConnFieldLabel(field)))
	b.WriteString("\n")
	b.WriteString(styleModeMenuDesc.Render(m.webhookConnDescription(field)))
	b.WriteString("\n")
	b.WriteString(m.connInput.View())
	b.WriteString("\n")
	b.WriteString(styleModeMenuHint.Render("Enter 保存并生效 · Esc 取消编辑"))
	return b.String()
}

func (m *Model) webhookConnDisplayValue(field webhookConnectionItem) string {
	switch field {
	case webhookConnListen:
		return m.cfg.Webhook.Listen
	case webhookConnPath:
		return m.cfg.Webhook.Path
	case webhookConnSecret:
		return config.FormatWebhookSecretDisplay(m.cfg.Webhook.Secret)
	case webhookConnPublicURL:
		return config.FormatWebhookPublicURLDisplay(m.cfg.Webhook.PublicURL)
	default:
		return ""
	}
}

func (m *Model) connectionMenuHint(listenURL string) string {
	if m.webhookEditField >= 0 {
		return fmt.Sprintf("编辑中 · Enter 保存并生效 · 本地 %s", listenURL)
	}
	return fmt.Sprintf("Enter 编辑 → 填完按 Enter 即保存 · 关闭菜单 Esc · 本地 %s", listenURL)
}
