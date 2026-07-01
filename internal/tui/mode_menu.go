package tui

// mode_menu.go — /mode 自动化模式选择菜单（manual/semi/full）。

import (
	"fmt"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

var modeMenuOptions = []string{config.ModeManual, config.ModeSemi, config.ModeFull}

func isModeMenuCommand(line string) bool {
	return strings.EqualFold(strings.TrimSpace(line), "/mode")
}

func modeMenuIndex(mode string) int {
	for i, opt := range modeMenuOptions {
		if opt == mode {
			return i
		}
	}
	return 1
}

func applyAutomationMode(cfg *config.Config, mode string) string {
	if cfg.IssueAutomation.Mode == mode {
		return fmt.Sprintf("保持: %s\n%s", cfg.IssueAutomation.ModeSummary(), config.ModeDescription(mode))
	}
	cfg.IssueAutomation.SetMode(mode)
	msg := fmt.Sprintf("已切换为: %s\n%s", cfg.IssueAutomation.ModeSummary(), config.ModeDescription(cfg.IssueAutomation.Mode))
	return msg + "\n" + persistConfig(cfg)
}

func (m *Model) openModeMenu() {
	m.modeMenuOpen = true
	m.modeMenuSel = modeMenuIndex(m.cfg.IssueAutomation.Mode)
	m.menuNotice = ""
	m.input.Blur()
	m.layout()
}

func (m *Model) closeModeMenu() {
	m.modeMenuOpen = false
	m.flushMenuNotice()
	m.input.Focus()
	m.layout()
}

func (m *Model) handleModeMenuKey(msg string) bool {
	switch msg {
	case "esc":
		m.closeModeMenu()
		return true
	case "j", "down":
		m.modeMenuSel = (m.modeMenuSel + 1) % len(modeMenuOptions)
		return true
	case "k", "up":
		n := len(modeMenuOptions)
		m.modeMenuSel = (m.modeMenuSel - 1 + n) % n
		return true
	case "enter":
		mode := modeMenuOptions[m.modeMenuSel]
		m.closeModeMenu()
		m.appendOutput(applyAutomationMode(m.cfg, mode))
		return true
	case "1", "2", "3":
		m.modeMenuSel = int(msg[0] - '1')
		mode := modeMenuOptions[m.modeMenuSel]
		m.closeModeMenu()
		m.appendOutput(applyAutomationMode(m.cfg, mode))
		return true
	}
	return false
}

func (m *Model) renderModeMenu() string {
	var lines []string
	lines = append(lines, styleModeMenuTitle.Render("选择自动化模式"))
	lines = append(lines, fmt.Sprintf("当前: %s", m.cfg.IssueAutomation.ModeSummary()))
	lines = append(lines, "")

	for i, mode := range modeMenuOptions {
		key := fmt.Sprintf("%d", i+1)
		label := fmt.Sprintf("[%s] %s (%s)", key, mode, config.ModeTitle(mode))
		if mode == m.cfg.IssueAutomation.Mode {
			label += " · 当前"
		}
		marker := "  "
		style := styleModeMenuItem
		if i == m.modeMenuSel {
			marker = "> "
			style = styleModeMenuSelected
		}
		lines = append(lines, style.Render(marker+label))
		lines = append(lines, styleModeMenuDesc.Render("     "+config.ModeDescription(mode)))
	}

	lines = append(lines, "", styleModeMenuHint.Render("1/2/3 直接选择 · j/k 移动 · Enter 确认 · Esc 取消"))
	return m.renderMenuBox(lines)
}
