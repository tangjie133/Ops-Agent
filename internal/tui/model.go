package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ZzedJay/Ops-Agent/internal/ai"
	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
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
	cfg    *config.Config
	gh     *github.Client
	input  textinput.Model
	output strings.Builder
	width  int
	height int

	ghOK   bool
	ghWarn string
	aiOK   bool
	aiWarn string
	repo   string

	ready bool
}

func NewModel(cfg *config.Config) Model {
	ti := textinput.New()
	ti.Placeholder = "ask a question, or describe a task  (/help)"
	ti.Focus()
	ti.CharLimit = 2048
	ti.Width = 60

	return Model{
		cfg:   cfg,
		gh:    github.NewClient(),
		input: ti,
	}
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
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "m", "M":
			m.cycleMode()
			m.appendOutput(fmt.Sprintf("模式已切换为: %s", m.cfg.IssueAutomation.ModeLabel()))
			return m, nil
		case "enter":
			line := m.input.Value()
			if line == "" {
				return m, nil
			}
			m.input.SetValue("")
			if isOutputClearCommand(line) {
				m.clearOutput()
				return m, nil
			}
			m.appendOutput("> " + line)
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
		m.appendOutput("\n输入 /help 查看命令。待办列表将在 M2.5 启用。")
		return m, nil

	case commandDoneMsg:
		if msg.output != "" {
			m.appendOutput(msg.output)
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = max(20, msg.Width-6)
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *Model) cycleMode() {
	switch m.cfg.IssueAutomation.Mode {
	case config.ModeManual:
		m.cfg.IssueAutomation.SetMode(config.ModeSemi)
	case config.ModeSemi:
		m.cfg.IssueAutomation.SetMode(config.ModeFull)
	default:
		m.cfg.IssueAutomation.SetMode(config.ModeManual)
	}
}

func (m *Model) runCommand(line string) tea.Cmd {
	return func() tea.Msg {
		out := runCommand(context.Background(), m.cfg, m.gh, line)
		return commandDoneMsg{output: out}
	}
}

func (m *Model) clearOutput() {
	m.output.Reset()
}

func (m *Model) appendOutput(s string) {
	if m.output.Len() > 0 {
		m.output.WriteString("\n")
	}
	m.output.WriteString(s)
}

func (m *Model) View() string {
	if m.width == 0 {
		return "Initializing...\n"
	}

	var b strings.Builder

	b.WriteString(styleBanner.Render(bannerASCII))
	b.WriteString("\n")
	b.WriteString(styleWelcome.Render("Welcome to Ops-Agent!  /help  ·  /clean  ·  M: cycle mode  ·  Ctrl+C: quit"))
	b.WriteString("\n\n")

	status := m.renderStatusBar()
	b.WriteString(status)
	b.WriteString("\n\n")

	todoW := min(28, m.width/3)
	if todoW < 20 {
		todoW = 20
	}
	todoPanel := styleTodoHeader.Render("待办") + "\n" +
		styleTodoItem.Render("  (M2.5)")

	outW := m.width - todoW - 4
	if outW < 20 {
		outW = m.width - 2
		todoPanel = ""
	}

	outText := m.output.String()
	if outText == "" {
		outText = styleOutput.Render("输出区域 — 命令结果将显示在这里")
	} else {
		outText = styleOutput.Render(outText)
	}

	if todoPanel != "" {
		left := lipgloss.NewStyle().Width(todoW).Render(todoPanel)
		right := lipgloss.NewStyle().Width(outW).Render(outText)
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right))
	} else {
		b.WriteString(outText)
	}

	b.WriteString("\n\n")
	b.WriteString(m.input.View())
	b.WriteString("\n")
	b.WriteString(styleHelp.Render("ctrl+g: agent monitor (M4)"))

	return b.String()
}

func (m *Model) renderStatusBar() string {
	mode := m.cfg.IssueAutomation.ModeLabel()
	model := m.cfg.AI.Model
	repo := m.repo
	if repo == "" {
		repo = "—"
	}

	cwd, _ := os.Getwd()
	if len(cwd) > 36 {
		cwd = "…" + cwd[len(cwd)-33:]
	}

	line := fmt.Sprintf("%s · %s · %s · 待办 0", model, mode, repo)
	if m.width > 0 {
		pad := m.width - lipgloss.Width(line) - lipgloss.Width(cwd) - 2
		if pad > 0 {
			line += strings.Repeat(" ", pad)
		}
	}
	line += cwd
	return styleStatusBar.Width(m.width).Render(line)
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
