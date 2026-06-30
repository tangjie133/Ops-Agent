package tui

import (
	"strings"
)

type acceptMenuItem int

const (
	acceptItemEnabled acceptMenuItem = iota
	acceptItemAutoRun
)

func isAcceptMenuCommand(line string) bool {
	line = strings.TrimSpace(line)
	return strings.EqualFold(line, "/accept") || line == "/验收"
}

func (m *Model) openAcceptMenu() {
	m.acceptMenuOpen = true
	m.acceptMenuSel = 0
	m.menuNotice = ""
	m.input.Blur()
	m.layout()
}

func (m *Model) closeAcceptMenu() {
	m.acceptMenuOpen = false
	m.flushMenuNotice()
	m.input.Focus()
	m.layout()
}

func (m *Model) saveAcceptSetting(label string) {
	// 仅修改 cfg.LibTest；Save 写入完整配置，不影响 issue_watch / issue_automation。
	m.cfg.LibTest.Normalize()
	msg := persistConfig(m.cfg) + " · " + label + " → " + m.cfg.LibTest.Summary()
	m.setMenuNotice(msg)
}

func (m *Model) handleAcceptMenuKey(msg string) bool {
	switch msg {
	case "esc":
		m.closeAcceptMenu()
		return true
	case "j", "down":
		m.acceptMenuSel = (m.acceptMenuSel + 1) % 2
		return true
	case "k", "up":
		m.acceptMenuSel = (m.acceptMenuSel - 1 + 2) % 2
		return true
	case "enter":
		return m.acceptMenuActivate()
	default:
		if len(msg) == 1 && msg[0] >= '1' && msg[0] <= '2' {
			m.acceptMenuSel = int(msg[0] - '1')
			return m.acceptMenuActivate()
		}
	}
	return false
}

func (m *Model) acceptMenuActivate() bool {
	m.cfg.LibTest.Normalize()
	switch acceptMenuItem(m.acceptMenuSel) {
	case acceptItemEnabled:
		m.cfg.LibTest.Enabled = !m.cfg.LibTest.Enabled
		m.saveAcceptSetting("验收功能")
	case acceptItemAutoRun:
		if !m.cfg.LibTest.Enabled {
			m.setMenuNotice("请先启用验收功能")
			return true
		}
		m.cfg.LibTest.AutoRun = !m.cfg.LibTest.AutoRun
		m.saveAcceptSetting("执行方式")
	}
	return true
}

func (m *Model) renderAcceptMenu() string {
	m.cfg.LibTest.Normalize()
	autoLabel := m.cfg.LibTest.RunModeLabel()
	if !m.cfg.LibTest.Enabled {
		autoLabel = "—（需先启用）"
	}
	items := []menuItem{
		{"1", "验收功能", m.cfg.LibTest.EnabledLabel(), "关闭后不再接收 push/release 验收入队，也不执行验收"},
		{"2", "执行方式", autoLabel, "自动：入队后后台验收；手动：仅入队，选中后按 Enter 验收"},
	}
	return m.renderWebhookMenuBox("验收配置", items, "Enter 切换 · j/k 移动 · Esc 关闭", "与待办 /mode、/webhook（Issue 入队）完全独立；仅控制 lib_test。")
}
