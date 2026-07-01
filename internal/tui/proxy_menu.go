package tui

// proxy_menu.go — /proxy 网络代理配置菜单与连通性测试。

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/netproxy"
)

const (
	proxyMenuLevelRoot       = 1
	proxyMenuLevelConnection = 2
)

const proxyEditNone = -1

type proxyRootItem int

const (
	proxyRootConnection proxyRootItem = iota
	proxyRootTestGitHub
)

type proxyConnectionItem int

const (
	proxyConnEnabled proxyConnectionItem = iota
	proxyConnHTTPS
	proxyConnHTTP
	proxyConnNoProxy
)

func isProxyMenuCommand(line string) bool {
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "/proxy" || line == "/vpn" || line == "/网络"
}

func (m *Model) openProxyMenu() {
	m.proxyMenuOpen = true
	m.proxyMenuLevel = proxyMenuLevelRoot
	m.proxyMenuSel = 0
	m.proxyEditField = proxyEditNone
	m.menuNotice = ""
	m.input.Blur()
	m.layout()
}

func (m *Model) closeProxyMenu() {
	m.proxyMenuOpen = false
	m.proxyMenuLevel = proxyMenuLevelRoot
	m.proxyMenuSel = 0
	m.proxyEditField = proxyEditNone
	m.proxyInput.Blur()
	m.flushMenuNotice()
	m.input.Focus()
	m.layout()
}

func (m *Model) proxyMenuItemCount() int {
	switch m.proxyMenuLevel {
	case proxyMenuLevelConnection:
		return 4
	default:
		return 2
	}
}

func (m *Model) proxyMenuGoBack() {
	if m.proxyEditField >= 0 {
		m.cancelProxyConnEdit()
		return
	}
	switch m.proxyMenuLevel {
	case proxyMenuLevelConnection:
		m.proxyMenuLevel = proxyMenuLevelRoot
		m.proxyMenuSel = int(proxyRootConnection)
	default:
		m.closeProxyMenu()
		return
	}
	m.layout()
}

func (m *Model) handleProxyMenuKey(msg string) bool {
	if m.proxyEditField >= 0 {
		return false
	}
	switch msg {
	case "esc", "b":
		if m.proxyMenuLevel != proxyMenuLevelRoot {
			m.proxyMenuGoBack()
			return true
		}
		m.closeProxyMenu()
		return true
	case "j", "down":
		n := m.proxyMenuItemCount()
		m.proxyMenuSel = (m.proxyMenuSel + 1) % n
		return true
	case "k", "up":
		n := m.proxyMenuItemCount()
		m.proxyMenuSel = (m.proxyMenuSel - 1 + n) % n
		return true
	case "enter":
		return m.proxyMenuActivate()
	default:
		if len(msg) == 1 && msg[0] >= '1' && msg[0] <= '9' {
			idx := int(msg[0] - '1')
			if idx < m.proxyMenuItemCount() {
				m.proxyMenuSel = idx
				return m.proxyMenuActivate()
			}
		}
	}
	return false
}

func (m *Model) proxyMenuActivate() bool {
	switch m.proxyMenuLevel {
	case proxyMenuLevelRoot:
		switch proxyRootItem(m.proxyMenuSel) {
		case proxyRootConnection:
			m.proxyMenuLevel = proxyMenuLevelConnection
			m.proxyMenuSel = 0
			m.layout()
		case proxyRootTestGitHub:
			m.testGitHubProxy()
		}
	case proxyMenuLevelConnection:
		switch proxyConnectionItem(m.proxyMenuSel) {
		case proxyConnEnabled:
			m.cfg.Proxy.Enabled = !m.cfg.Proxy.Enabled
			m.applyProxyConfig("启用代理")
		default:
			m.startProxyConnEdit(proxyConnectionItem(m.proxyMenuSel))
		}
	}
	return true
}

func (m *Model) applyProxyConfig(label string) {
	m.cfg.Proxy.Normalize()
	m.gh.SetProxy(m.cfg.Proxy)
	msg := persistConfig(m.cfg) + " · " + label + " → " + m.cfg.Proxy.Summary()
	if err := m.cfg.Proxy.Validate(); err != nil {
		msg += " · ⚠ " + err.Error()
	}
	m.setMenuNotice(msg)
}

func (m *Model) testGitHubProxy() {
	health := netproxy.CheckGitHubWithGH(context.Background(), m.cfg.Proxy)
	status := "不可达"
	if health.Reachable {
		status = "可达"
	}
	m.setMenuNotice(persistConfig(m.cfg) + " · GitHub 检测: " + status + " — " + health.Message)
}

func (m *Model) renderProxyMenu() string {
	switch m.proxyMenuLevel {
	case proxyMenuLevelConnection:
		return m.renderProxyConnectionMenu()
	default:
		return m.renderProxyRootMenu()
	}
}

func (m *Model) renderProxyRootMenu() string {
	items := []menuItem{
		{"1", "代理配置", m.cfg.Proxy.Summary(), "HTTP(S) 代理 · 供 gh clone / git pull"},
		{"2", "测试 GitHub", "HEAD github.com + gh api", "检测当前代理能否访问 GitHub"},
	}
	return m.renderProxyMenuBox("网络 / 代理", items, "Enter 操作 · 1 进入代理配置 · j/k 移动 · Esc 关闭", config.ProxyConnectionIntro())
}

func (m *Model) renderProxyConnectionMenu() string {
	fields := config.ProxyConnFields()
	enabled := "关闭"
	if m.cfg.Proxy.Enabled {
		enabled = "开启"
	}
	items := []menuItem{
		{"1", "启用代理", enabled, "开启后 clone/pull 使用下方代理地址（仅 Ops-Agent 子进程）"},
		{"2", fields[0].Title, config.FormatProxyDisplay(m.cfg.Proxy.HTTPSProxy), fields[0].Description},
		{"3", fields[1].Title, config.FormatProxyDisplay(m.cfg.Proxy.HTTPProxy), fields[1].Description},
		{"4", fields[2].Title, config.FormatProxyDisplay(m.cfg.Proxy.NoProxy), fields[2].Description},
	}
	return m.renderProxyMenuBox("代理配置", items, "Enter 编辑/切换 · Esc/B 返回", config.ProxyConnectionIntro())
}

func (m *Model) renderProxyMenuBox(title string, items []menuItem, hint string, intro string) string {
	lines := []string{
		styleModeMenuTitle.Render(title),
	}
	if intro != "" {
		lines = append(lines, styleModeMenuDesc.Render(intro))
	}
	lines = append(lines, "当前: "+m.cfg.Proxy.Summary())
	if m.menuNotice != "" {
		lines = append(lines, styleModeMenuDesc.Render("↳ "+truncateMenuNotice(m.menuNotice, menuNoticeWidth(m.width))))
	}
	lines = append(lines, "")

	for i, it := range items {
		marker := "  "
		style := styleModeMenuItem
		if i == m.proxyMenuSel {
			marker = "> "
			style = styleModeMenuSelected
		}
		line := fmt.Sprintf("%s[%s] %s — %s", marker, it.key, it.title, it.value)
		lines = append(lines, style.Render(line))
		if it.desc != "" && i == m.proxyMenuSel {
			lines = append(lines, styleModeMenuDesc.Render("     "+truncateMenuNotice(it.desc, menuNoticeWidth(m.width)-5)))
		}
	}

	lines = append(lines, "", styleModeMenuHint.Render(hint))
	return m.renderMenuBox(lines)
}

func (m *Model) initProxyInput() {
	if m.proxyInput.Width > 0 {
		return
	}
	ti := textinput.New()
	ti.CharLimit = 512
	ti.Width = 60
	m.proxyInput = ti
}

func (m *Model) proxyConnFieldLabel(field proxyConnectionItem) string {
	switch field {
	case proxyConnHTTPS:
		return config.ProxyConnFields()[0].Title
	case proxyConnHTTP:
		return config.ProxyConnFields()[1].Title
	case proxyConnNoProxy:
		return config.ProxyConnFields()[2].Title
	default:
		return ""
	}
}

func (m *Model) proxyConnPlaceholder(field proxyConnectionItem) string {
	fields := config.ProxyConnFields()
	switch field {
	case proxyConnHTTPS:
		return fields[0].Placeholder
	case proxyConnHTTP:
		return fields[1].Placeholder
	case proxyConnNoProxy:
		return fields[2].Placeholder
	default:
		return ""
	}
}

func (m *Model) proxyConnDescription(field proxyConnectionItem) string {
	fields := config.ProxyConnFields()
	switch field {
	case proxyConnHTTPS:
		return fields[0].Description
	case proxyConnHTTP:
		return fields[1].Description
	case proxyConnNoProxy:
		return fields[2].Description
	default:
		return ""
	}
}

func (m *Model) proxyConnCurrentValue(field proxyConnectionItem) string {
	p := m.cfg.Proxy
	switch field {
	case proxyConnHTTPS:
		return p.HTTPSProxy
	case proxyConnHTTP:
		return p.HTTPProxy
	case proxyConnNoProxy:
		return p.NoProxy
	default:
		return ""
	}
}

func (m *Model) startProxyConnEdit(field proxyConnectionItem) {
	m.initProxyInput()
	m.proxyEditField = int(field)
	m.proxyInput.SetValue(m.proxyConnCurrentValue(field))
	m.proxyInput.Placeholder = m.proxyConnPlaceholder(field)
	m.proxyInput.Focus()
	if m.width > 8 {
		m.proxyInput.Width = max(20, m.width-8)
	}
	m.layout()
}

func (m *Model) cancelProxyConnEdit() {
	m.proxyEditField = proxyEditNone
	m.proxyInput.Blur()
	m.layout()
}

func (m *Model) commitProxyConnEdit() {
	field := proxyConnectionItem(m.proxyEditField)
	val := strings.TrimSpace(m.proxyInput.Value())
	label := m.proxyConnFieldLabel(field)

	switch field {
	case proxyConnHTTPS:
		if err := config.ValidateProxyURL(val); err != nil {
			m.setMenuNotice("HTTPS Proxy 无效: " + err.Error())
			m.cancelProxyConnEdit()
			return
		}
		m.cfg.Proxy.HTTPSProxy = val
	case proxyConnHTTP:
		if err := config.ValidateProxyURL(val); err != nil {
			m.setMenuNotice("HTTP Proxy 无效: " + err.Error())
			m.cancelProxyConnEdit()
			return
		}
		m.cfg.Proxy.HTTPProxy = val
	case proxyConnNoProxy:
		m.cfg.Proxy.NoProxy = val
	default:
		m.cancelProxyConnEdit()
		return
	}

	m.cancelProxyConnEdit()
	m.applyProxyConfig(label + " 已更新")
}

func (m *Model) handleProxyConnEdit(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.cancelProxyConnEdit()
			return m, nil
		case "enter":
			m.commitProxyConnEdit()
			return m, textinput.Blink
		}
	}
	var cmd tea.Cmd
	m.proxyInput, cmd = m.proxyInput.Update(msg)
	return m, cmd
}

func (m *Model) renderProxyConnEditBar() string {
	if m.proxyEditField < 0 {
		return ""
	}
	field := proxyConnectionItem(m.proxyEditField)
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(styleModeMenuTitle.Render("编辑 " + m.proxyConnFieldLabel(field)))
	b.WriteString("\n")
	b.WriteString(styleModeMenuDesc.Render(m.proxyConnDescription(field)))
	b.WriteString("\n")
	b.WriteString(m.proxyInput.View())
	b.WriteString("\n")
	b.WriteString(styleModeMenuHint.Render("Enter 保存并生效 · Esc 取消编辑"))
	return b.String()
}
