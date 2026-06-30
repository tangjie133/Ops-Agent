package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorPrimary = lipgloss.Color("#7C3AED")
	colorMuted   = lipgloss.Color("#6B7280")
	colorError   = lipgloss.Color("#EF4444")
	colorWarn    = lipgloss.Color("#F59E0B")
	colorOK      = lipgloss.Color("#10B981")

	styleBanner = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	styleWelcome = lipgloss.NewStyle().
			Foreground(colorMuted)

	styleStatusBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB")).
			Background(lipgloss.Color("#1F2937")).
			Padding(0, 1)

	styleStatusOK = lipgloss.NewStyle().Foreground(colorOK)
	styleStatusWarn = lipgloss.NewStyle().Foreground(colorWarn)
	styleStatusErr = lipgloss.NewStyle().Foreground(colorError)

	styleOutput = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D1D5DB"))

	styleTodoHeader = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	styleTodoItem = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF"))

	styleTodoSelected = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	styleTodoAnalyzing = lipgloss.NewStyle().
			Foreground(colorWarn).
			Bold(true)

	styleLogHeader = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Bold(true)

	styleCompleteBar = lipgloss.NewStyle().
			Foreground(colorMuted)

	styleCompleteActive = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	styleCompleteGhost = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4B5563"))

	styleCompleteHint = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))

	styleWebhookLog = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF"))

	styleWebhookEvent = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A5B4FC"))

	styleWorkerEvent = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#34D399"))

	styleInvestigatorLog = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FBBF24"))

	styleTestHeader = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#34D399")).
			Bold(true)

	styleTestSelected = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#34D399")).
			Bold(true)

	styleTestPass = lipgloss.NewStyle().
			Foreground(colorOK)

	styleTestFail = lipgloss.NewStyle().
			Foreground(colorError).
			Bold(true)

	styleHelp = lipgloss.NewStyle().
			Foreground(colorMuted)

	styleModeMenuTitle = lipgloss.NewStyle().
				Foreground(colorPrimary).
				Bold(true)

	styleModeMenuItem = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#D1D5DB"))

	styleModeMenuSelected = lipgloss.NewStyle().
				Foreground(colorPrimary).
				Bold(true)

	styleModeMenuDesc = lipgloss.NewStyle().
				Foreground(colorMuted)

	styleModeMenuHint = lipgloss.NewStyle().
				Foreground(colorMuted).
				Italic(true)
)

// bannerASCII: figlet "Ops-Agent" (standard) — left "Ops", right "Agent".
const bannerASCII = `   ___                    _                    _   
  / _ \ _ __  ___        / \   __ _  ___ _ __ | |_ 
 | | | | '_ \/ __|_____ / _ \ / _` + "`" + ` |/ _ \ '_ \| __|
 | |_| | |_) \__ \_____/ ___ \ (_| |  __/ | | | |_ 
  \___/| .__/|___/    /_/   \_\__, |\___|_| |_|\__|
       |_|                    |___/`

const outputPlaceholder = "对话 — 命令与 Agent 回复将显示在这里"

// headerLineCount is the fixed header height (banner + welcome + status + spacing).
const headerLineCount = 12
// footerLineCount is the default footer height (spacing + input + help).
const footerLineCount = 3

// menuFooterLines is the reserved footer height when a config menu is open.
const menuFooterLines = 15

func (m *Model) activeFooterLines() int {
	if m.confirmOpen {
		return 18
	}
	if m.aiMenuOpen {
		lines := 20
		if m.aiMenuLevel == aiMenuLevelConnection {
			lines = 22
		}
		if m.aiEditField >= 0 {
			lines += 4
		}
		return lines
	}
	if m.proxyMenuOpen {
		lines := 20
		if m.proxyMenuLevel == proxyMenuLevelConnection {
			lines = 22
		}
		if m.proxyEditField >= 0 {
			lines += 4
		}
		return lines
	}
	if m.webhookMenuOpen || m.modeMenuOpen || m.acceptMenuOpen {
		lines := menuFooterLines
		if m.webhookMenuOpen {
			switch m.webhookMenuLevel {
			case webhookMenuLevelConnection:
				lines = 24
			case webhookMenuLevelRoot:
				lines = 20
			case webhookMenuLevelIssue:
				lines = 16
			}
		}
		if m.acceptMenuOpen {
			lines = 16
		}
		if m.webhookMenuOpen && m.webhookEditField >= 0 {
			lines += 4
		}
		return lines
	}
	return footerLineCount
}
