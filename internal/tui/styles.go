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

	styleHelp = lipgloss.NewStyle().
			Foreground(colorMuted)
)

const bannerASCII = `   ___     ___   _                    _
  / _ \   / _ \ | |__    ___    __ _  | |_    ___
 | | | | | | | || '_ \  / _ \  / _` + "`" + ` | | __|  / _ \
 | |_| | | |_| || |_) || (_) || (_| | | |_  |  __/
  \___/   \___/ |_.__/  \___/  \__,_|  \__|  \___|`
