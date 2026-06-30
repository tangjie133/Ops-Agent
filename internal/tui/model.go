package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ZzedJay/Ops-Agent/internal/ai"
	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
	"github.com/ZzedJay/Ops-Agent/internal/webhook"
)

type startupDoneMsg struct {
	ghOK   bool
	ghWarn string
	aiOK   bool
	aiWarn string
	repo   string
}

type commandDoneMsg struct {
	output string
}

type Model struct {
	cfg            *config.Config
	gh             *github.Client
	store          *todo.FileStore
	whRuntime      *WebhookRuntime
	input          textinput.Model
	outputViewport viewport.Model
	outputContent  string
	width          int
	height         int

	ghOK   bool
	ghWarn string
	aiOK   bool
	aiWarn string
	repo   string

	todoSel int
	ready   bool

	modeMenuOpen bool
	modeMenuSel  int

	webhookMenuOpen  bool
	webhookMenuLevel int
	webhookMenuSel   int
	webhookEditField int
	menuNotice       string
	connInput        textinput.Model

	completions []Completion
	completeIdx int
}

func NewModel(cfg *config.Config, store *todo.FileStore, wh *WebhookRuntime) Model {
	ti := textinput.New()
	ti.Placeholder = "ask a question, or describe a task  (/help)"
	ti.Focus()
	ti.CharLimit = 2048
	ti.Width = 60

	m := Model{
		cfg:            cfg,
		gh:             github.NewClient(),
		store:          store,
		whRuntime:      wh,
		input:          ti,
		outputViewport: viewport.New(60, 8),
	}
	m.syncOutputViewport(true)
	m.ensureTodoSelection()
	return m
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.runStartup(),
	)
}

func (m Model) runStartup() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		msg := startupDoneMsg{}

		if !m.gh.Available() {
			msg.ghOK = false
			msg.ghWarn = "gh 未安装或不在 PATH"
			return msg
		}

		auth, _ := m.gh.AuthStatus(ctx)
		if !auth.LoggedIn {
			msg.ghOK = false
			msg.ghWarn = "gh 未登录 — 运行 gh auth login"
		} else {
			msg.ghOK = true
			repo, err := m.gh.RepoFromCwd(ctx)
			if err != nil {
				msg.repo = "—"
				msg.ghWarn = fmt.Sprintf("无法解析当前仓库: %v", err)
			} else {
				msg.repo = repo
			}
		}

		health := ai.CheckHealth(ctx, m.cfg.AI)
		msg.aiOK = health.Reachable
		if !health.Reachable {
			msg.aiWarn = health.Message
		}

		return msg
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.webhookMenuOpen && m.webhookEditField >= 0 {
			return m.handleWebhookConnEdit(msg)
		}
		if m.webhookMenuOpen {
			if m.handleWebhookMenuKey(msg.String()) {
				return m, textinput.Blink
			}
			return m, nil
		}
		if m.modeMenuOpen {
			if m.handleModeMenuKey(msg.String()) {
				return m, textinput.Blink
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			return m, tea.Quit
		case "j":
			if m.input.Value() == "" {
				m.todoDown()
				return m, nil
			}
		case "k":
			if m.input.Value() == "" {
				m.todoUp()
				return m, nil
			}
		case "d":
			if m.input.Value() == "" {
				m.dismissSelectedTodo()
				return m, nil
			}
		case "i":
			if m.input.Value() == "" {
				return m, m.focusSelectedTodo()
			}
		case "tab":
			if m.applyCompletionTab() {
				return m, nil
			}
		case "right":
			if m.applyCompletionGhost() {
				return m, nil
			}
		case "enter":
			line := strings.TrimSpace(m.input.Value())
			if line == "" {
				return m, nil
			}
			m.input.SetValue("")
			m.resetCompletions()
			if isOutputClearCommand(line) {
				m.clearOutput()
				return m, nil
			}
			m.appendOutput("> " + line)
			if isModeMenuCommand(line) {
				m.openModeMenu()
				return m, textinput.Blink
			}
			if isWebhookMenuCommand(line) {
				m.openWebhookMenu()
				return m, textinput.Blink
			}
			return m, m.runCommand(line)
		}

	case startupDoneMsg:
		m.ghOK = msg.ghOK
		m.ghWarn = msg.ghWarn
		m.aiOK = msg.aiOK
		m.aiWarn = msg.aiWarn
		m.repo = msg.repo
		m.ready = true

		var lines []string
		if !m.ghOK {
			lines = append(lines, styleStatusErr.Render("✗ "+m.ghWarn))
		} else {
			lines = append(lines, styleStatusOK.Render("✓ GitHub CLI 就绪"))
		}
		if !m.aiOK {
			lines = append(lines, styleStatusWarn.Render("⚠ "+m.aiWarn+"（AI 功能 M3 前不可用）"))
		} else {
			lines = append(lines, styleStatusOK.Render("✓ llama-server 可达"))
		}
		m.appendOutput(strings.Join(lines, "\n"))
		if m.cfg.Webhook.Enabled && m.whRuntime != nil {
			var wh []string
			wh = append(wh, "Webhook: "+m.whRuntime.ListenURL())
			if m.cfg.Webhook.Tunnel.Smee.Enabled {
				wh = append(wh, "Smee: "+m.whRuntime.SmeeSummary())
			}
			if payload := m.whRuntime.PayloadURL(); payload != "" {
				wh = append(wh, "Payload: "+payload)
			}
			m.appendOutput(strings.Join(wh, " · "))
		}
		m.appendOutput("输入 /help 查看命令 · /webhook /mode 配置")
		return m, nil

	case WebhookEventMsg:
		m.appendOutput(msg.Event.Message())
		switch msg.Event.Kind {
		case webhook.EventAdded, webhook.EventCommentAdded, webhook.EventClosed, webhook.EventReopened:
			m.ensureTodoSelection()
		}
		return m, nil

	case commandDoneMsg:
		if msg.output != "" {
			m.appendOutput(msg.output)
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout()
		return m, nil

	case tea.MouseMsg:
		if m.handleMouseScroll(msg) {
			return m, nil
		}
	}

	var cmd tea.Cmd
	if m.modeMenuOpen || m.webhookMenuOpen {
		return m, nil
	}
	prev := m.input.Value()
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != prev {
		m.resetCompletions()
	}
	m.refreshCompletions()
	return m, cmd
}

func (m *Model) runCommand(line string) tea.Cmd {
	return func() tea.Msg {
		out := runCommand(context.Background(), m.cfg, m.gh, m.store, line)
		return commandDoneMsg{output: out}
	}
}

func (m *Model) clearOutput() {
	m.outputContent = ""
	m.syncOutputViewport(true)
}

func (m *Model) appendOutput(s string) {
	atBottom := m.outputContent == "" || m.outputViewport.AtBottom()
	if m.outputContent != "" {
		m.outputContent += "\n"
	}
	m.outputContent += s
	m.syncOutputViewport(atBottom)
}

func (m *Model) syncOutputViewport(stickBottom bool) {
	content := m.outputContent
	if content == "" {
		content = outputPlaceholder
	}
	m.outputViewport.SetContent(content)
	if stickBottom {
		m.outputViewport.GotoBottom()
	}
}

func (m *Model) handleMouseScroll(msg tea.MouseMsg) bool {
	if !m.isInOutputArea(msg.Y) {
		return false
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.outputViewport.LineUp(3)
		return true
	case tea.MouseButtonWheelDown:
		m.outputViewport.LineDown(3)
		return true
	}
	return false
}

func (m *Model) isInOutputArea(y int) bool {
	if m.height == 0 {
		return true
	}
	top := headerLineCount
	bottom := m.height - m.activeFooterLines()
	return y >= top && y < bottom
}

func (m *Model) layout() {
	if m.width == 0 || m.height == 0 {
		return
	}

	bodyH := m.height - headerLineCount - m.activeFooterLines()
	if bodyH < 3 {
		bodyH = 3
	}

	outW := m.outputWidth()
	if outW < 10 {
		outW = 10
	}

	m.outputViewport.Width = outW
	m.outputViewport.Height = bodyH
	m.input.Width = max(20, m.width-6)
	m.syncOutputViewport(m.outputViewport.AtBottom())
}

func (m *Model) outputWidth() int {
	todoW := m.todoPanelWidth()
	outW := m.width - todoW - 4
	if outW < 20 {
		return m.width - 2
	}
	return outW
}

func (m *Model) renderHeader() string {
	var b strings.Builder
	b.WriteString(styleBanner.Render(bannerASCII))
	b.WriteString("\n")
	b.WriteString(styleWelcome.Render("Welcome to Ops-Agent!  /help  ·  /mode  ·  Ctrl+C: quit"))
	b.WriteString("\n\n")
	b.WriteString(m.renderStatusBar())
	b.WriteString("\n\n")
	return b.String()
}

func (m *Model) todoPanelWidth() int {
	w := min(44, m.width*2/5)
	if w < 32 {
		w = 32
	}
	return w
}

func (m *Model) renderTodoPanel() string {
	active := m.activeTodos()
	if len(active) == 0 {
		return styleTodoItem.Render("  (无)")
	}
	m.ensureTodoSelection()

	maxLines := m.outputViewport.Height - 1
	if maxLines < 1 {
		maxLines = 5
	}
	lineWidth := m.todoPanelWidth() - 2
	var lines []string
	for i, it := range active {
		entry := formatTodoEntry(it, lineWidth, i == m.todoSel)
		if len(lines)+len(entry) > maxLines {
			remaining := len(active) - i
			if remaining > 0 {
				lines = append(lines, styleTodoItem.Render(fmt.Sprintf("  …+%d", remaining)))
			}
			break
		}
		for j, line := range entry {
			style := styleTodoItem
			if i == m.todoSel && j == 0 {
				style = styleTodoSelected
			}
			lines = append(lines, style.Render(line))
		}
	}
	return strings.Join(lines, "\n")
}

func (m *Model) renderBody() string {
	todoW := m.todoPanelWidth()
	todoPanel := styleTodoHeader.Render("待办") + "\n" + m.renderTodoPanel()

	outW := m.outputWidth()
	outView := m.outputViewport.View()

	if outW >= 20 && m.width > todoW+4 {
		left := lipgloss.NewStyle().Width(todoW).Height(m.outputViewport.Height).Render(todoPanel)
		right := lipgloss.NewStyle().Width(outW).Render(outView)
		return lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
	}
	return outView
}

func (m *Model) renderFooter() string {
	var b strings.Builder
	b.WriteString("\n")

	if m.webhookMenuOpen {
		b.WriteString(m.renderWebhookMenu())
		b.WriteString(m.renderWebhookConnEditBar())
		b.WriteString("\n")
		return b.String()
	}

	if m.modeMenuOpen {
		b.WriteString(m.renderModeMenu())
		b.WriteString("\n")
		return b.String()
	}

	line := m.input.View()
	if ghost := ghostSuffix(m.input.Value(), m.completions); ghost != "" {
		line += styleCompleteGhost.Render(ghost)
	}
	b.WriteString(line)
	b.WriteString("\n")

	if bar := m.renderCompletionBar(); bar != "" {
		b.WriteString(bar)
		b.WriteString("\n")
	}

	b.WriteString(styleHelp.Render("Tab/→ 补全 · j/k 待办 · /webhook /mode 配置菜单"))
	return b.String()
}

func (m *Model) renderCompletionBar() string {
	if len(m.completions) == 0 {
		return ""
	}
	maxShow := min(5, len(m.completions))
	var parts []string
	for i := 0; i < maxShow; i++ {
		c := m.completions[i]
		label := c.Text
		if c.Hint != "" {
			label += " " + styleCompleteHint.Render("· "+c.Hint)
		}
		if i == m.completeIdx%len(m.completions) {
			parts = append(parts, styleCompleteActive.Render(label))
		} else {
			parts = append(parts, styleCompleteBar.Render(label))
		}
	}
	if len(m.completions) > maxShow {
		parts = append(parts, styleCompleteBar.Render("…"))
	}
	return strings.Join(parts, "  ")
}

func (m *Model) View() string {
	if m.width == 0 {
		return "Initializing...\n"
	}
	view := m.renderHeader() + m.renderBody() + m.renderFooter()
	return lipgloss.NewStyle().Width(m.width).Render(view)
}

func (m *Model) renderStatusBar() string {
	mode := m.cfg.IssueAutomation.ModeLabel()
	model := m.cfg.AI.Model
	repo := m.repo
	if repo == "" {
		repo = "—"
	}

	wh := "wh:off"
	if m.cfg.Webhook.Enabled {
		wh = "wh:on"
	}

	cwd, _ := os.Getwd()
	if len(cwd) > 36 {
		cwd = "…" + cwd[len(cwd)-33:]
	}

	line := fmt.Sprintf("%s · %s · %s · %s · 待办 %d", model, mode, wh, repo, m.store.ActiveCount())
	if m.width > 0 {
		pad := m.width - lipgloss.Width(line) - lipgloss.Width(cwd) - 2
		if pad > 0 {
			line += strings.Repeat(" ", pad)
		}
	}
	line += cwd
	return styleStatusBar.Width(m.width).Render(line)
}

func (m *Model) refreshCompletions() {
	m.completions = computeCompletions(m.input.Value(), m.activeTodos())
}

func (m *Model) resetCompletions() {
	m.completeIdx = 0
	m.completions = nil
}

func (m *Model) applyCompletionTab() bool {
	m.refreshCompletions()
	if len(m.completions) == 0 {
		return false
	}
	idx := m.completeIdx % len(m.completions)
	m.input.SetValue(m.completions[idx].Text)
	m.completeIdx++
	return true
}

func (m *Model) applyCompletionGhost() bool {
	m.refreshCompletions()
	suffix := ghostSuffix(m.input.Value(), m.completions)
	if suffix == "" {
		return false
	}
	m.input.SetValue(m.input.Value() + suffix)
	m.resetCompletions()
	m.refreshCompletions()
	return true
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
