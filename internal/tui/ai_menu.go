package tui

// ai_menu.go — /model、/ai 模型连接配置菜单。

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ZzedJay/Ops-Agent/internal/ai"
	"github.com/ZzedJay/Ops-Agent/internal/config"
)

const (
	aiMenuLevelRoot       = 1
	aiMenuLevelConnection = 2
)

const aiEditNone = -1

type aiRootItem int

const (
	aiRootConnection aiRootItem = iota
	aiRootTestHealth
)

type aiConnectionItem int

const (
	aiConnBaseURL aiConnectionItem = iota
	aiConnModel
	aiConnAPIKey
)

func isAIMenuCommand(line string) bool {
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "/model" || line == "/ai"
}

func (m *Model) openAIMenu() {
	m.aiMenuOpen = true
	m.aiMenuLevel = aiMenuLevelRoot
	m.aiMenuSel = 0
	m.aiEditField = aiEditNone
	m.menuNotice = ""
	m.input.Blur()
	m.layout()
}

func (m *Model) closeAIMenu() {
	m.aiMenuOpen = false
	m.aiMenuLevel = aiMenuLevelRoot
	m.aiMenuSel = 0
	m.aiEditField = aiEditNone
	m.aiInput.Blur()
	m.flushMenuNotice()
	m.input.Focus()
	m.layout()
}

func (m *Model) aiMenuItemCount() int {
	switch m.aiMenuLevel {
	case aiMenuLevelConnection:
		return 3
	default:
		return 2
	}
}

func (m *Model) aiMenuGoBack() {
	if m.aiEditField >= 0 {
		m.cancelAIConnEdit()
		return
	}
	switch m.aiMenuLevel {
	case aiMenuLevelConnection:
		m.aiMenuLevel = aiMenuLevelRoot
		m.aiMenuSel = int(aiRootConnection)
	default:
		m.closeAIMenu()
		return
	}
	m.layout()
}

func (m *Model) handleAIMenuKey(msg string) bool {
	if m.aiEditField >= 0 {
		return false
	}
	switch msg {
	case "esc", "b":
		if m.aiMenuLevel != aiMenuLevelRoot {
			m.aiMenuGoBack()
			return true
		}
		m.closeAIMenu()
		return true
	case "j", "down":
		n := m.aiMenuItemCount()
		m.aiMenuSel = (m.aiMenuSel + 1) % n
		m.markDirty()
		return true
	case "k", "up":
		n := m.aiMenuItemCount()
		m.aiMenuSel = (m.aiMenuSel - 1 + n) % n
		m.markDirty()
		return true
	case "enter":
		return m.aiMenuActivate()
	default:
		if len(msg) == 1 && msg[0] >= '1' && msg[0] <= '9' {
			idx := int(msg[0] - '1')
			if idx < m.aiMenuItemCount() {
				m.aiMenuSel = idx
				return m.aiMenuActivate()
			}
		}
	}
	return false
}

func (m *Model) aiMenuActivate() bool {
	switch m.aiMenuLevel {
	case aiMenuLevelRoot:
		switch aiRootItem(m.aiMenuSel) {
		case aiRootConnection:
			m.aiMenuLevel = aiMenuLevelConnection
			m.aiMenuSel = 0
			m.layout()
		case aiRootTestHealth:
			m.testAIHealth()
		}
	case aiMenuLevelConnection:
		m.startAIConnEdit(aiConnectionItem(m.aiMenuSel))
	}
	return true
}

func (m *Model) testAIHealth() {
	health := ai.CheckHealth(context.Background(), m.cfg.AI)
	m.aiOK = health.Reachable
	m.aiWarn = health.Message
	status := "不可达"
	if health.Reachable {
		status = "可达"
	}
	m.setMenuNotice(persistConfig(m.cfg) + " · 检测: " + status + " — " + health.Message)
}

func (m *Model) saveAISetting(label string) {
	msg := persistConfig(m.cfg) + " · " + label + " → " + m.cfg.AISummary()
	health := ai.CheckHealth(context.Background(), m.cfg.AI)
	m.aiOK = health.Reachable
	m.aiWarn = health.Message
	if health.Reachable {
		msg += " · llama-server 可达"
	} else {
		msg += " · ⚠ " + health.Message
	}
	m.setMenuNotice(msg)
}

func (m *Model) renderAIMenu() string {
	switch m.aiMenuLevel {
	case aiMenuLevelConnection:
		return m.renderAIConnectionMenu()
	default:
		return m.renderAIRootMenu()
	}
}

func (m *Model) renderAIRootMenu() string {
	health := "未检测"
	if m.aiOK {
		health = "可达"
	} else if m.aiWarn != "" {
		health = "不可达"
	}
	items := []menuItem{
		{"1", "连接配置", m.cfg.AISummary(), "base_url / model / api_key"},
		{"2", "测试连通性", health, "请求 GET /v1/models 检测 llama-server"},
	}
	return m.renderAIMenuBox("模型配置", items, "Enter 操作 · 1 进入连接配置 · j/k 移动 · Esc 关闭", config.AIConnectionIntro())
}

func (m *Model) renderAIConnectionMenu() string {
	fields := config.AIConnFields()
	items := []menuItem{
		{"1", fields[0].Title, config.FormatAIBaseURLDisplay(m.cfg.AI.BaseURL), fields[0].Description},
		{"2", fields[1].Title, m.cfg.AI.Model, fields[1].Description},
		{"3", fields[2].Title, config.FormatAIAPIKeyDisplay(m.cfg.AI.APIKey), fields[2].Description},
	}
	return m.renderAIMenuBox("连接配置", items, "Enter 编辑 → Enter 保存 · Esc/B 返回 · Provider: "+m.cfg.AI.Provider, config.AIConnectionIntro())
}

func (m *Model) renderAIMenuBox(title string, items []menuItem, hint string, intro string) string {
	lines := []string{
		styleModeMenuTitle.Render(title),
	}
	if intro != "" {
		lines = append(lines, styleModeMenuDesc.Render(intro))
	}
	lines = append(lines, "当前: "+m.cfg.AISummary())
	if !m.aiOK && m.aiWarn != "" {
		lines = append(lines, styleStatusWarn.Render("⚠ "+truncateMenuNotice(m.aiWarn, menuNoticeWidth(m.width))))
	}
	if m.menuNotice != "" {
		lines = append(lines, styleModeMenuDesc.Render("↳ "+truncateMenuNotice(m.menuNotice, menuNoticeWidth(m.width))))
	}
	lines = append(lines, "")

	for i, it := range items {
		marker := "  "
		style := styleModeMenuItem
		if i == m.aiMenuSel {
			marker = "> "
			style = styleModeMenuSelected
		}
		line := fmt.Sprintf("%s[%s] %s — %s", marker, it.key, it.title, it.value)
		lines = append(lines, style.Render(line))
		if it.desc != "" && i == m.aiMenuSel {
			lines = append(lines, styleModeMenuDesc.Render("     "+truncateMenuNotice(it.desc, menuNoticeWidth(m.width)-5)))
		}
	}

	lines = append(lines, "", styleModeMenuHint.Render(hint))
	return m.renderMenuBox(lines)
}

func (m *Model) initAIInput() {
	if m.aiInput.Width > 0 {
		return
	}
	ti := textinput.New()
	ti.CharLimit = 512
	ti.Width = 60
	m.aiInput = ti
}

func (m *Model) aiConnFieldLabel(field aiConnectionItem) string {
	return config.AIConnFields()[int(field)].Title
}

func (m *Model) aiConnPlaceholder(field aiConnectionItem) string {
	return config.AIConnFields()[int(field)].Placeholder
}

func (m *Model) aiConnDescription(field aiConnectionItem) string {
	return config.AIConnFields()[int(field)].Description
}

func (m *Model) aiConnCurrentValue(field aiConnectionItem) string {
	a := m.cfg.AI
	switch field {
	case aiConnBaseURL:
		return a.BaseURL
	case aiConnModel:
		return a.Model
	case aiConnAPIKey:
		return a.APIKey
	default:
		return ""
	}
}

func (m *Model) startAIConnEdit(field aiConnectionItem) {
	m.initAIInput()
	m.aiEditField = int(field)
	m.aiInput.SetValue(m.aiConnCurrentValue(field))
	m.aiInput.Placeholder = m.aiConnPlaceholder(field)
	m.aiInput.Focus()
	if m.width > 8 {
		m.aiInput.Width = max(20, m.width-8)
	}
	m.layout()
}

func (m *Model) cancelAIConnEdit() {
	m.aiEditField = aiEditNone
	m.aiInput.Blur()
	m.layout()
}

func (m *Model) commitAIConnEdit() {
	field := aiConnectionItem(m.aiEditField)
	val := strings.TrimSpace(m.aiInput.Value())
	label := m.aiConnFieldLabel(field)

	switch field {
	case aiConnBaseURL:
		val = config.NormalizeAIBaseURL(val)
		if err := config.ValidateAIBaseURL(val); err != nil {
			m.setMenuNotice("Base URL 无效: " + err.Error())
			m.cancelAIConnEdit()
			return
		}
		m.cfg.AI.BaseURL = val
	case aiConnModel:
		if err := config.ValidateAIModel(val); err != nil {
			m.setMenuNotice("Model 无效: " + err.Error())
			m.cancelAIConnEdit()
			return
		}
		m.cfg.AI.Model = val
	case aiConnAPIKey:
		if val == "" {
			val = "local"
		}
		m.cfg.AI.APIKey = val
	default:
		m.cancelAIConnEdit()
		return
	}

	m.cancelAIConnEdit()
	m.saveAISetting(label + " 已更新")
}

func (m *Model) handleAIConnEdit(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.cancelAIConnEdit()
			return m, nil
		case "enter":
			m.commitAIConnEdit()
			return m, textinput.Blink
		}
	}
	var cmd tea.Cmd
	m.aiInput, cmd = m.aiInput.Update(msg)
	m.markDirty()
	return m, cmd
}

func (m *Model) renderAIConnEditBar() string {
	if m.aiEditField < 0 {
		return ""
	}
	field := aiConnectionItem(m.aiEditField)
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(styleModeMenuTitle.Render("编辑 " + m.aiConnFieldLabel(field)))
	b.WriteString("\n")
	b.WriteString(styleModeMenuDesc.Render(m.aiConnDescription(field)))
	b.WriteString("\n")
	b.WriteString(m.aiInput.View())
	b.WriteString("\n")
	b.WriteString(styleModeMenuHint.Render("Enter 保存并生效 · Esc 取消编辑"))
	return b.String()
}
