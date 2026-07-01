package tui

// webhook_menu.go — /webhook 配置菜单与子菜单渲染。
import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

const (
	webhookMenuLevelRoot       = 1
	webhookMenuLevelIssue      = 2
	webhookMenuLevelConnection = 3
)

type webhookRootItem int

const (
	webhookRootReceive webhookRootItem = iota
	webhookRootIssueRules
	webhookRootConnection
	webhookRootSmee
)

type webhookIssueItem int

const (
	webhookIssueEnabled webhookIssueItem = iota
	webhookIssueUnassigned
	webhookIssueLabels
)

type webhookConnectionItem int

const (
	webhookConnListen webhookConnectionItem = iota
	webhookConnPath
	webhookConnSecret
	webhookConnPublicURL
)

func isWebhookMenuCommand(line string) bool {
	return strings.EqualFold(strings.TrimSpace(line), "/webhook")
}

func (m *Model) openWebhookMenu() {
	m.webhookMenuOpen = true
	m.webhookMenuLevel = webhookMenuLevelRoot
	m.webhookMenuSel = 0
	m.webhookEditField = webhookEditNone
	m.menuNotice = ""
	m.input.Blur()
	m.layout()
}

func (m *Model) closeWebhookMenu() {
	m.webhookMenuOpen = false
	m.webhookMenuLevel = webhookMenuLevelRoot
	m.webhookMenuSel = 0
	m.webhookEditField = webhookEditNone
	m.connInput.Blur()
	m.flushMenuNotice()
	m.input.Focus()
	m.layout()
}

func (m *Model) flushMenuNotice() {
	if m.menuNotice == "" {
		return
	}
	m.appendOutput(m.menuNotice)
	m.menuNotice = ""
}

func (m *Model) setMenuNotice(msg string) {
	m.menuNotice = msg
}

func (m *Model) webhookMenuItemCount() int {
	switch m.webhookMenuLevel {
	case webhookMenuLevelIssue:
		return 3
	case webhookMenuLevelConnection:
		return 4
	default:
		return 4
	}
}

func (m *Model) webhookMenuGoBack() {
	if m.webhookEditField >= 0 {
		m.cancelWebhookConnEdit()
		return
	}
	switch m.webhookMenuLevel {
	case webhookMenuLevelIssue:
		m.webhookMenuLevel = webhookMenuLevelRoot
		m.webhookMenuSel = int(webhookRootIssueRules)
	case webhookMenuLevelConnection:
		m.webhookMenuLevel = webhookMenuLevelRoot
		m.webhookMenuSel = int(webhookRootConnection)
	default:
		m.closeWebhookMenu()
		return
	}
	m.layout()
}

func (m *Model) handleWebhookMenuKey(msg string) bool {
	if m.webhookEditField >= 0 {
		return false
	}
	switch msg {
	case "esc", "b":
		if m.webhookMenuLevel != webhookMenuLevelRoot {
			m.webhookMenuGoBack()
			return true
		}
		m.closeWebhookMenu()
		return true
	case "j", "down":
		n := m.webhookMenuItemCount()
		m.webhookMenuSel = (m.webhookMenuSel + 1) % n
		return true
	case "k", "up":
		n := m.webhookMenuItemCount()
		m.webhookMenuSel = (m.webhookMenuSel - 1 + n) % n
		return true
	case "enter":
		return m.webhookMenuActivate()
	default:
		if len(msg) == 1 && msg[0] >= '1' && msg[0] <= '9' {
			idx := int(msg[0] - '1')
			if idx < m.webhookMenuItemCount() {
				m.webhookMenuSel = idx
				return m.webhookMenuActivate()
			}
		}
	}
	return false
}

func (m *Model) webhookMenuActivate() bool {
	switch m.webhookMenuLevel {
	case webhookMenuLevelRoot:
		switch webhookRootItem(m.webhookMenuSel) {
		case webhookRootReceive:
			m.toggleWebhookEnabled()
		case webhookRootIssueRules:
			m.webhookMenuLevel = webhookMenuLevelIssue
			m.webhookMenuSel = 0
			m.layout()
		case webhookRootConnection:
			m.webhookMenuLevel = webhookMenuLevelConnection
			m.webhookMenuSel = 0
			m.layout()
		case webhookRootSmee:
			m.toggleSmeeTunnel()
		}
	case webhookMenuLevelIssue:
		switch webhookIssueItem(m.webhookMenuSel) {
		case webhookIssueEnabled:
			m.cfg.IssueWatch.Enabled = !m.cfg.IssueWatch.Enabled
			m.saveWebhookSetting("Issue 监视", false)
		case webhookIssueUnassigned:
			m.cfg.IssueWatch.RequireUnassigned = !m.cfg.IssueWatch.RequireUnassigned
			m.saveWebhookSetting("未指派过滤", false)
		case webhookIssueLabels:
			next := (config.LabelPresetIndex(m.cfg.IssueWatch.Labels) + 1) % len(config.LabelPresets())
			config.ApplyLabelPreset(m.cfg, next)
			m.saveWebhookSetting("Label 策略", false)
		}
	case webhookMenuLevelConnection:
		m.startWebhookConnEdit(webhookConnectionItem(m.webhookMenuSel))
	}
	return true
}

func (m *Model) saveWebhookSetting(label string, restart bool) {
	msg := persistConfig(m.cfg) + " · " + label + " → " + m.cfg.ConnectionSummary()
	if restart && m.whRuntime != nil && m.cfg.Webhook.Enabled {
		if err := m.whRuntime.Restart(m.cfg); err != nil {
			msg += " · 重启失败: " + err.Error()
		} else {
			msg += " · 监听 " + m.whRuntime.ListenURL()
		}
	}
	m.setMenuNotice(msg)
}

func (m *Model) toggleSmeeTunnel() {
	m.cfg.Webhook.Tunnel.Smee.Enabled = !m.cfg.Webhook.Tunnel.Smee.Enabled
	msg := persistConfig(m.cfg) + " · Smee 隧道 " + m.cfg.Webhook.SmeeToggleLabel()
	if hint := m.cfg.Webhook.SmeeHint(); hint != "" {
		msg += " · " + hint
	}
	if m.whRuntime != nil && m.cfg.Webhook.Enabled {
		if err := m.whRuntime.Restart(m.cfg); err != nil {
			msg += " · 重启失败: " + err.Error()
		} else if m.cfg.Webhook.SmeeTunnelActive() {
			msg += " · 已连接"
		}
	}
	m.setMenuNotice(msg)
}

func (m *Model) toggleWebhookEnabled() {
	m.cfg.Webhook.Enabled = !m.cfg.Webhook.Enabled
	msg := persistConfig(m.cfg) + " · Webhook 接收 → " + m.cfg.WebhookSummary()
	if m.whRuntime != nil {
		if err := m.whRuntime.Restart(m.cfg); err != nil {
			msg += " · 重启失败: " + err.Error()
		} else if m.cfg.Webhook.Enabled {
			msg += " · 监听 " + m.whRuntime.ListenURL()
		} else {
			msg += " · 监听已停止"
		}
	}
	m.setMenuNotice(msg)
}

func (m *Model) renderWebhookMenu() string {
	switch m.webhookMenuLevel {
	case webhookMenuLevelIssue:
		return m.renderWebhookIssueMenu()
	case webhookMenuLevelConnection:
		return m.renderWebhookConnectionMenu()
	default:
		return m.renderWebhookRootMenu()
	}
}

func (m *Model) renderWebhookRootMenu() string {
	items := []menuItem{
		{"1", "Webhook 接收", m.webhookEnabledLabel(), "开启后在本机 listen 地址接收 GitHub webhook"},
		{"2", "Issue 入队规则", "进入子菜单 →", "配置哪些 issue 写入待办列表"},
		{"3", "连接配置", m.webhookConnectionBrief(), "listen / path / secret / Public URL"},
		{"4", "Smee 隧道", m.cfg.Webhook.SmeeToggleLabel(), "内嵌 smee 客户端；Public URL 填 smee.io 频道后自动转发"},
	}
	return m.renderWebhookMenuBox("Webhook 配置", items, "Enter 操作 · 2/3 子菜单 · j/k 移动 · Esc 关闭", config.WebhookConnectionIntro())
}

func (m *Model) renderWebhookConnectionMenu() string {
	w := m.cfg.Webhook
	listenURL := w.LocalURL()
	if m.whRuntime != nil && m.cfg.Webhook.Enabled {
		listenURL = m.whRuntime.ListenURL()
	}
	fields := config.WebhookConnFields()
	items := []menuItem{
		{"1", fields[0].Title, m.webhookConnDisplayValue(webhookConnListen), fields[0].Description},
		{"2", fields[1].Title, m.webhookConnDisplayValue(webhookConnPath), fields[1].Description},
		{"3", fields[2].Title, m.webhookConnDisplayValue(webhookConnSecret), fields[2].Description},
		{"4", fields[3].Title, m.webhookConnDisplayValue(webhookConnPublicURL), fields[3].Description},
	}
	return m.renderWebhookMenuBox("连接配置", items, m.connectionMenuHint(listenURL), config.WebhookConnectionIntro())
}

type menuItem struct {
	key   string
	title string
	value string
	desc  string
}

func (m *Model) renderWebhookIssueMenu() string {
	preset := config.LabelPresets()[config.LabelPresetIndex(m.cfg.IssueWatch.Labels)]
	items := []menuItem{
		{"1", "Issue 监视", m.issueWatchEnabledLabel(), "开启后 webhook 命中的 issue 才会写入待办"},
		{"2", "仅未指派入队", m.requireUnassignedLabel(), "已指派给他人的 issue 不入队"},
		{"3", "Label 过滤", preset.Title, "命中任一配置 label 的 issue 才入队；空列表表示不过滤"},
	}
	return m.renderWebhookMenuBox("Issue 入队规则", items, "Enter 切换 · Esc/B 返回 · j/k 移动", "")
}

func (m *Model) renderWebhookMenuBox(title string, items []menuItem, hint string, intro string) string {
	lines := []string{
		styleModeMenuTitle.Render(title),
	}
	if intro != "" {
		lines = append(lines, styleModeMenuDesc.Render(intro))
	}
	if m.webhookMenuLevel == webhookMenuLevelConnection {
		lines = append(lines, "当前: "+m.cfg.ConnectionSummary())
	} else {
		lines = append(lines, "当前: "+m.cfg.WebhookSummary())
		if hint := m.cfg.Webhook.SmeeHint(); hint != "" {
			lines = append(lines, styleStatusWarn.Render("⚠ "+hint))
		}
	}
	if m.menuNotice != "" {
		lines = append(lines, styleModeMenuDesc.Render("↳ "+truncateMenuNotice(m.menuNotice, menuNoticeWidth(m.width))))
	}
	lines = append(lines, "")

	for i, it := range items {
		marker := "  "
		style := styleModeMenuItem
		if i == m.webhookMenuSel {
			marker = "> "
			style = styleModeMenuSelected
		}
		line := fmt.Sprintf("%s[%s] %s — %s", marker, it.key, it.title, it.value)
		lines = append(lines, style.Render(line))
		if it.desc != "" && i == m.webhookMenuSel {
			lines = append(lines, styleModeMenuDesc.Render("     "+truncateMenuNotice(it.desc, menuNoticeWidth(m.width)-5)))
		}
	}

	lines = append(lines, "", styleModeMenuHint.Render(hint))
	return m.renderMenuBox(lines)
}

func truncateMenuNotice(s string, max int) string {
	if max < 8 {
		max = 8
	}
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func menuNoticeWidth(termWidth int) int {
	w := min(termWidth-4, 68)
	if w < 40 {
		w = 40
	}
	return w
}

func (m *Model) renderMenuBox(lines []string) string {
	content := strings.Join(lines, "\n")
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(0, 1).
		Width(min(m.width-2, 72))

	rendered := box.Render(content)
	return padBlock(rendered, m.activeFooterLines()-1, m.width)
}

func padBlock(block string, lines, width int) string {
	rows := strings.Split(block, "\n")
	if len(rows) > lines {
		// 保留顶部标题与菜单项，优先截掉中间说明行
		if lines >= 3 {
			head := rows[:2]
			tail := rows[len(rows)-(lines-2):]
			rows = append(head, tail...)
		} else {
			rows = rows[:lines]
		}
	}
	for len(rows) < lines {
		rows = append(rows, strings.Repeat(" ", width))
	}
	for i, row := range rows {
		if lipgloss.Width(row) < width {
			rows[i] = row + strings.Repeat(" ", width-lipgloss.Width(row))
		}
	}
	return strings.Join(rows, "\n")
}

func (m *Model) webhookEnabledLabel() string {
	if m.cfg.Webhook.Enabled {
		return "已启用"
	}
	return "已禁用"
}

func (m *Model) issueWatchEnabledLabel() string {
	if m.cfg.IssueWatch.Enabled {
		return "已启用"
	}
	return "已禁用"
}

func (m *Model) requireUnassignedLabel() string {
	if m.cfg.IssueWatch.RequireUnassigned {
		return "是"
	}
	return "否"
}

func (m *Model) webhookConnectionBrief() string {
	brief := m.cfg.Webhook.Listen
	if pub := config.FormatWebhookPublicURLDisplay(m.cfg.Webhook.PublicURL); pub != "未设置" {
		brief += " · " + pub
	} else {
		brief += " · Secret:" + config.FormatWebhookSecretDisplay(m.cfg.Webhook.Secret)
	}
	return brief
}
