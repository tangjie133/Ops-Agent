package tui

// mode_menu.go — /mode Issue 自动化与 refactor_pr 配置菜单。

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

const modeMenuItemCount = 5

type modeMenuItem int

const (
	modeItemManual modeMenuItem = iota
	modeItemSemi
	modeItemFull
	modeItemRefactorEnabled
	modeItemRefactorTrigger
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

func (m *Model) openModeMenu() {
	m.modeMenuOpen = true
	m.modeMenuSel = modeMenuIndex(m.cfg.IssueAutomation.Mode)
	m.menuNotice = ""
	m.input.Blur()
	m.layout()
	m.markDirty()
}

func (m *Model) closeModeMenu() {
	m.modeMenuOpen = false
	m.flushMenuNotice()
	m.input.Focus()
	m.layout()
	m.markDirty()
}

func (m *Model) saveAutomationSetting(label, value string) {
	m.cfg.IssueAutomation.RefactorPR.Normalize()
	msg := persistConfig(m.cfg) + " · " + label + " → " + value
	m.setMenuNotice(msg)
	m.markDirty()
}

func (m *Model) handleModeMenuKey(msg string) (handled bool, cmd tea.Cmd) {
	switch msg {
	case "esc":
		m.closeModeMenu()
		return true, nil
	case "j", "down":
		m.modeMenuSel = (m.modeMenuSel + 1) % modeMenuItemCount
		m.markDirty()
		return true, nil
	case "k", "up":
		m.modeMenuSel = (m.modeMenuSel - 1 + modeMenuItemCount) % modeMenuItemCount
		m.markDirty()
		return true, nil
	case "enter":
		m.modeMenuActivate()
		return true, m.triggerWorkerIfNeeded()
	default:
		if len(msg) == 1 && msg[0] >= '1' && msg[0] <= '5' {
			m.modeMenuSel = int(msg[0] - '1')
			m.modeMenuActivate()
			return true, m.triggerWorkerIfNeeded()
		}
	}
	return false, nil
}

func (m *Model) modeMenuActivate() bool {
	m.cfg.IssueAutomation.RefactorPR.Normalize()
	switch modeMenuItem(m.modeMenuSel) {
	case modeItemManual, modeItemSemi, modeItemFull:
		mode := modeMenuOptions[m.modeMenuSel]
		if m.cfg.IssueAutomation.Mode != mode {
			m.cfg.IssueAutomation.SetMode(mode)
		}
		m.saveAutomationSetting("Issue 模式", m.cfg.IssueAutomation.ModeSummary())
		return true
	case modeItemRefactorEnabled:
		m.cfg.IssueAutomation.RefactorPR.Enabled = !m.cfg.IssueAutomation.RefactorPR.Enabled
		m.saveAutomationSetting("修库 PR", m.cfg.IssueAutomation.RefactorPR.Summary())
		return true
	case modeItemRefactorTrigger:
		if !m.cfg.IssueAutomation.RefactorPR.Enabled {
			m.setMenuNotice("请先启用「修库 PR」")
			m.markDirty()
			return true
		}
		m.cfg.IssueAutomation.RefactorPR.CycleTrigger()
		m.saveAutomationSetting("修库 PR 触发", m.cfg.IssueAutomation.RefactorPR.TriggerLabel())
		return true
	}
	return false
}

func (m *Model) renderModeMenu() string {
	m.cfg.IssueAutomation.RefactorPR.Normalize()
	refactorTrigger := m.cfg.IssueAutomation.RefactorPR.TriggerLabel()

	items := []struct {
		key, title, value, desc string
	}{
		{"1", "Issue · manual", issueModeLabel(m.cfg, config.ModeManual), config.ModeDescription(config.ModeManual)},
		{"2", "Issue · semi", issueModeLabel(m.cfg, config.ModeSemi), config.ModeDescription(config.ModeSemi)},
		{"3", "Issue · full", issueModeLabel(m.cfg, config.ModeFull), config.ModeDescription(config.ModeFull)},
		{"4", "修库 PR", m.cfg.IssueAutomation.RefactorPR.EnabledLabel(), "确认后在分支重构、测试并开 PR（与 Issue 评论 mode 独立）"},
		{"5", "修库 PR 触发", refactorTrigger, "TUI f 确认 · Issue 评论 /approve-pr · 或两者"},
	}

	var lines []string
	lines = append(lines, styleModeMenuTitle.Render("Issue 自动化"))
	lines = append(lines, fmt.Sprintf("当前: %s · 修库 PR: %s",
		m.cfg.IssueAutomation.ModeSummary(), m.cfg.IssueAutomation.RefactorPR.Summary()))
	if m.menuNotice != "" {
		lines = append(lines, styleModeMenuDesc.Render("↳ "+truncateMenuNotice(m.menuNotice, menuNoticeWidth(m.width))))
	}
	lines = append(lines, "")

	for i, it := range items {
		marker := "  "
		style := styleModeMenuItem
		if i == m.modeMenuSel {
			marker = "> "
			style = styleModeMenuSelected
		}
		line := fmt.Sprintf("%s[%s] %s — %s", marker, it.key, it.title, it.value)
		lines = append(lines, style.Render(line))
		if it.desc != "" && i == m.modeMenuSel {
			lines = append(lines, styleModeMenuDesc.Render("     "+truncateMenuNotice(it.desc, menuNoticeWidth(m.width)-5)))
		}
	}

	lines = append(lines, "", styleModeMenuHint.Render("1-5 或 Enter 切换 · j/k 移动 · Esc 关闭"))
	return m.renderMenuBox(lines)
}

func issueModeLabel(cfg *config.Config, mode string) string {
	label := config.ModeTitle(mode)
	if cfg.IssueAutomation.Mode == mode {
		return label + " · 当前"
	}
	return label
}
