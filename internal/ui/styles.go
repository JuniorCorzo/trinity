package ui

import "github.com/charmbracelet/lipgloss"

var (
	ColorPrimary   = lipgloss.Color("#7D56F4")
	ColorSecondary = lipgloss.Color("#00D7D7")
	ColorSuccess   = lipgloss.Color("#00E676")
	ColorWarning   = lipgloss.Color("#FFD600")
	ColorDanger    = lipgloss.Color("#FF5252")
	ColorMuted     = lipgloss.Color("#626262")
	ColorBg        = lipgloss.Color("#181824")
	ColorCardBg    = lipgloss.Color("#1E1E2E")
	ColorWhite     = lipgloss.Color("#FAFAFA")
	ColorCyan      = lipgloss.Color("#00E5FF")
	ColorMagenta   = lipgloss.Color("#F50057")

	ColorBackend  = lipgloss.Color("#00D7D7")
	ColorFrontend = lipgloss.Color("#BD93F9")
	ColorCompose  = lipgloss.Color("#50FA7B")
	ColorDefault  = lipgloss.Color("#F8F8F2")

	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorWhite).
			Background(ColorPrimary).
			Padding(0, 1)

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorWhite)

	ClockStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A0A0A0"))

	CardNormalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#3A3A4C")).
			Padding(0, 1)

	CardActiveStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Bold(true).
			Padding(0, 1)

	BadgeRunning = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true).
			SetString("● RUNNING")

	BadgeStarting = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true).
			SetString("◐ STARTING")

	BadgeStopped = lipgloss.NewStyle().
			Foreground(ColorMuted).
			SetString("■ STOPPED")

	BadgeFailed = lipgloss.NewStyle().
			Foreground(ColorDanger).
			Bold(true).
			SetString("✖ FAILED")

	FooterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888899")).
			Padding(0, 1)

	KeyStyle = lipgloss.NewStyle().
			Foreground(ColorCyan).
			Bold(true)

	AutoScrollActive = lipgloss.NewStyle().
				Foreground(ColorSuccess).
				Bold(true)

	AutoScrollPaused = lipgloss.NewStyle().
				Foreground(ColorWarning).
				Bold(true)
)

func ServiceColor(name string) lipgloss.TerminalColor {
	switch name {
	case "backend":
		return ColorBackend
	case "frontend":
		return ColorFrontend
	case "compose":
		return ColorCompose
	default:
		return ColorDefault
	}
}
