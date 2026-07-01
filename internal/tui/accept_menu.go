package tui

// accept_menu.go — /accept 验收配置菜单（与 Issue /webhook 流程独立）。
import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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
	m.markDirty()
}

func (m *Model) closeAcceptMenu() {
	m.acceptMenuOpen = false
	m.flushMenuNotice()
	m.input.Focus()
	m.layout()
	m.markDirty()
}

func (m *Model) saveAcceptSetting(label string) {
	m.cfg.LibTest.Normalize()
	msg := persistConfig(m.cfg) + " · " + label + " → " + m.cfg.LibTest.Summary()
	m.setMenuNotice(msg)
	m.markDirty()
}

func (m *Model) handleAcceptMenuKey(msg string) (handled bool, cmd tea.Cmd) {
	switch msg {
	case "esc":
		m.closeAcceptMenu()
		return true, nil
	case "j", "down":
		m.acceptMenuSel = (m.acceptMenuSel + 1) % 2
		m.markDirty()
		return true, nil
	case "k", "up":
		m.acceptMenuSel = (m.acceptMenuSel - 1 + 2) % 2
		m.markDirty()
		return true, nil
	case "enter":
		return m.acceptMenuActivate()
	default:
		if len(msg) == 1 && msg[0] >= '1' && msg[0] <= '2' {
			m.acceptMenuSel = int(msg[0] - '1')
			return m.acceptMenuActivate()
		}
	}
	return false, nil
}

func (m *Model) acceptMenuActivate() (bool, tea.Cmd) {
	m.cfg.LibTest.Normalize()
	switch acceptMenuItem(m.acceptMenuSel) {
	case acceptItemEnabled:
		wasEnabled := m.cfg.LibTest.Enabled
		m.cfg.LibTest.Enabled = !m.cfg.LibTest.Enabled
		m.saveAcceptSetting("验收功能")
		if m.cfg.LibTest.Enabled && !wasEnabled {
			return true, m.libTestTickCmd()
		}
		return true, nil
	case acceptItemAutoRun:
		if !m.cfg.LibTest.Enabled {
			m.setMenuNotice("请先启用验收功能")
			m.markDirty()
			return true, nil
		}
		m.cfg.LibTest.AutoRun = !m.cfg.LibTest.AutoRun
		m.saveAcceptSetting("执行方式")
		return true, nil
	}
	return true, nil
}

func (m *Model) renderAcceptMenu() string {
	m.cfg.LibTest.Normalize()
	autoLabel := m.cfg.LibTest.RunModeLabel()
	if !m.cfg.LibTest.Enabled {
		autoLabel = "—（需先启用）"
	}
	items := []struct {
		key, title, value, desc string
	}{
		{"1", "验收功能", m.cfg.LibTest.EnabledLabel(), "关闭后不再接收 push/release 验收入队，也不执行验收"},
		{"2", "执行方式", autoLabel, "自动：入队后后台验收；手动：仅入队，选中后按 Enter 验收"},
	}

	var lines []string
	lines = append(lines, styleModeMenuTitle.Render("验收配置"))
	lines = append(lines, styleModeMenuDesc.Render("与待办 /mode、/webhook（Issue 入队）完全独立；仅控制 lib_test。"))
	lines = append(lines, fmt.Sprintf("当前: %s", m.cfg.LibTest.Summary()))
	if m.menuNotice != "" {
		lines = append(lines, styleModeMenuDesc.Render("↳ "+truncateMenuNotice(m.menuNotice, menuNoticeWidth(m.width))))
	}
	lines = append(lines, "")

	for i, it := range items {
		marker := "  "
		style := styleModeMenuItem
		if i == m.acceptMenuSel {
			marker = "> "
			style = styleModeMenuSelected
		}
		line := fmt.Sprintf("%s[%s] %s — %s", marker, it.key, it.title, it.value)
		lines = append(lines, style.Render(line))
		if it.desc != "" && i == m.acceptMenuSel {
			lines = append(lines, styleModeMenuDesc.Render("     "+truncateMenuNotice(it.desc, menuNoticeWidth(m.width)-5)))
		}
	}

	lines = append(lines, "", styleModeMenuHint.Render("Enter 切换 · j/k 移动 · Esc 关闭"))
	return m.renderMenuBox(lines)
}
