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

	styleCompleteBar = lipgloss.NewStyle().
			Foreground(colorMuted)

	styleCompleteActive = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	styleCompleteGhost = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4B5563"))

	styleCompleteHint = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))

	styleHelp = lipgloss.NewStyle().
			Foreground(colorMuted)
)

// bannerASCII: figlet "Ops-Agent" (standard) — left "Ops", right "Agent".
const bannerASCII = `   ___                    _                    _   
  / _ \ _ __  ___        / \   __ _  ___ _ __ | |_ 
 | | | | '_ \/ __|_____ / _ \ / _` + "`" + ` |/ _ \ '_ \| __|
 | |_| | |_) \__ \_____/ ___ \ (_| |  __/ | | | |_ 
  \___/| .__/|___/    /_/   \_\__, |\___|_| |_|\__|
       |_|                    |___/`

const outputPlaceholder = "输出区域 — 命令结果将显示在这里"

// headerLineCount is the fixed header height (banner + welcome + status + spacing).
const headerLineCount = 12
// footerLineCount is the fixed footer height (spacing + input + help).
const footerLineCount = 3
