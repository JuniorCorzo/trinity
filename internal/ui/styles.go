package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/juniorcorzo/dev-tui/internal/theme"
)

var CurrentTheme *theme.Theme

func init() {
	if os.Getenv("NO_COLOR") != "" {
		lipgloss.SetColorProfile(termenv.Ascii)
	} else {
		lipgloss.SetColorProfile(termenv.TrueColor)
	}
	ApplyTheme(&theme.TerminalTheme)
}

// HexToBgCode converts a 6-character hex code (e.g. "#09090d") to an ANSI 24-bit background sequence.
func HexToBgCode(hex string) string {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) == 6 {
		var r, g, b int
		fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
		return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
	}
	return ""
}

// PaintLine ensures that every character across targetWidth has the specified background color
// when running a non-transparent theme. In transparent mode (e.g. terminal default), it only pads with spaces.
func PaintLine(line string, bgHex string, targetWidth int) string {
	if CurrentTheme != nil && CurrentTheme.Transparent {
		w := ansi.StringWidth(line)
		if w < targetWidth {
			line += strings.Repeat(" ", targetWidth-w)
		}
		return line
	}
	if os.Getenv("NO_COLOR") != "" {
		return line
	}
	bgCode := HexToBgCode(bgHex)
	if bgCode == "" {
		return line
	}
	w := ansi.StringWidth(line)
	if w < targetWidth {
		line += strings.Repeat(" ", targetWidth-w)
	}
	line = strings.ReplaceAll(line, "\x1b[0m", "\x1b[0m"+bgCode)
	line = strings.ReplaceAll(line, "\x1b[m", "\x1b[m"+bgCode)
	return bgCode + line + "\x1b[0m"
}

var (
	ColorBg        lipgloss.TerminalColor
	ColorSurface   lipgloss.TerminalColor
	ColorCardBg    lipgloss.TerminalColor
	ColorBorder    lipgloss.TerminalColor
	ColorPrimary   lipgloss.TerminalColor
	ColorSecondary lipgloss.TerminalColor
	ColorSuccess   lipgloss.TerminalColor
	ColorWarning   lipgloss.TerminalColor
	ColorDanger    lipgloss.TerminalColor
	ColorMuted     lipgloss.TerminalColor
	ColorDim       lipgloss.TerminalColor
	ColorWhite     lipgloss.TerminalColor
	ColorCyan      lipgloss.TerminalColor
	ColorMagenta   lipgloss.TerminalColor

	ColorBackend  lipgloss.TerminalColor
	ColorFrontend lipgloss.TerminalColor
	ColorCompose  lipgloss.TerminalColor
	ColorSpring   lipgloss.TerminalColor
	ColorDefault  lipgloss.TerminalColor

	ColorGitBranch   lipgloss.TerminalColor
	ColorGitWorktree lipgloss.TerminalColor
	ColorGitDirty    lipgloss.TerminalColor

	HeaderStyle            lipgloss.Style
	TitleStyle             lipgloss.Style
	ClockStyle             lipgloss.Style
	SidebarStyle           lipgloss.Style
	CardNormalStyle        lipgloss.Style
	CardActiveStyle        lipgloss.Style
	CardCompactNormalStyle lipgloss.Style
	CardCompactActiveStyle lipgloss.Style

	BadgeRunning  lipgloss.Style
	BadgeStarting lipgloss.Style
	BadgeStopped  lipgloss.Style
	BadgeFailed   lipgloss.Style

	FooterStyle   lipgloss.Style
	ModeTagNormal lipgloss.Style
	ModeTagVisual lipgloss.Style
	KeyHintKey    lipgloss.Style
	KeyHintLabel  lipgloss.Style
	KeyStyle      lipgloss.Style

	AutoScrollActive lipgloss.Style
	AutoScrollPaused lipgloss.Style

	GitBranchBadge   lipgloss.Style
	GitWorktreeBadge lipgloss.Style
	GitDirtyBadge    lipgloss.Style
	GitCleanBadge    lipgloss.Style

	ModalContainer lipgloss.Style
	ModalTitle     lipgloss.Style
	ModalSelected  lipgloss.Style
	ModalNormal    lipgloss.Style
	ModalDim       lipgloss.Style

	VisualModeIndicator  lipgloss.Style
	VisualSelectionStyle lipgloss.Style
)

// ApplyTheme configures all active UI styles from the given theme definition.
func ApplyTheme(th *theme.Theme) {
	if th == nil {
		th = &theme.TerminalTheme
	}
	CurrentTheme = th

	ColorBg = theme.Color(th.Bg)
	ColorSurface = theme.Color(th.Surface)
	ColorCardBg = theme.Color(th.CardBg)
	ColorBorder = theme.Color(th.Border)
	ColorPrimary = theme.Color(th.Primary)
	ColorSecondary = theme.Color(th.Secondary)
	ColorSuccess = theme.Color(th.Success)
	ColorWarning = theme.Color(th.Warning)
	ColorDanger = theme.Color(th.Danger)
	ColorMuted = theme.Color(th.Muted)
	ColorDim = theme.Color(th.Dim)
	ColorWhite = theme.Color(th.White)
	ColorCyan = theme.Color(th.Secondary)
	ColorMagenta = theme.Color(th.Primary)

	ColorBackend = ColorPrimary
	ColorFrontend = ColorSecondary
	ColorCompose = ColorSuccess
	ColorSpring = ColorSuccess
	ColorDefault = ColorWhite

	ColorGitBranch = ColorSuccess
	ColorGitWorktree = ColorPrimary
	ColorGitDirty = ColorWarning

	if th.Transparent {
		HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorWhite).
			Padding(0, 1)

		SidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(ColorBorder)

		CardNormalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)

		CardActiveStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Bold(true).
			Padding(0, 1)

		CardCompactNormalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 0)

		CardCompactActiveStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Bold(true).
			Padding(0, 0)

		FooterStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Padding(0, 1)

		ModeTagNormal = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			Padding(0, 1)

		ModeTagVisual = lipgloss.NewStyle().
			Reverse(true).
			Bold(true).
			Padding(0, 1)

		ModalContainer = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 2)
	} else {
		HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorWhite).
			Background(ColorSurface).
			Padding(0, 1)

		SidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(ColorBorder).
			Background(ColorSurface)

		CardNormalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Background(ColorCardBg).
			Padding(0, 1)

		CardActiveStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Background(ColorCardBg).
			Bold(true).
			Padding(0, 1)

		CardCompactNormalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Background(ColorCardBg).
			Padding(0, 0)

		CardCompactActiveStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Background(ColorCardBg).
			Bold(true).
			Padding(0, 0)

		FooterStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Background(ColorSurface).
			Padding(0, 1)

		ModeTagNormal = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Background(ColorCardBg).
			Bold(true).
			Padding(0, 1)

		ModeTagVisual = lipgloss.NewStyle().
			Foreground(ColorBg).
			Background(ColorWarning).
			Bold(true).
			Padding(0, 1)

		ModalContainer = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Background(ColorSurface).
			Padding(1, 2)
	}

	TitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorWhite)

	ClockStyle = lipgloss.NewStyle().
		Foreground(ColorDim)

	BadgeRunning = lipgloss.NewStyle().
		Foreground(ColorSuccess).
		Bold(true).
		SetString("◎ RUNNING")

	BadgeStarting = lipgloss.NewStyle().
		Foreground(ColorWarning).
		Bold(true).
		SetString("◌ STARTING")

	BadgeStopped = lipgloss.NewStyle().
		Foreground(ColorMuted).
		SetString("○ STOPPED")

	BadgeFailed = lipgloss.NewStyle().
		Foreground(ColorDanger).
		Bold(true).
		SetString("✕ FAILED")

	KeyHintKey = lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)

	KeyHintLabel = lipgloss.NewStyle().
		Foreground(ColorMuted)

	KeyStyle = lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)

	AutoScrollActive = lipgloss.NewStyle().
		Foreground(ColorSuccess).
		Bold(true)

	AutoScrollPaused = lipgloss.NewStyle().
		Foreground(ColorWarning).
		Bold(true)

	GitBranchBadge = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorGitBranch)

	GitWorktreeBadge = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorGitWorktree)

	GitDirtyBadge = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorGitDirty)

	GitCleanBadge = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorSuccess)

	ModalTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorWhite).
		Background(ColorPrimary).
		Padding(0, 1)

	ModalSelected = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorBg).
		Background(ColorPrimary)

	ModalNormal = lipgloss.NewStyle().
		Foreground(ColorDefault)

	ModalDim = lipgloss.NewStyle().
		Foreground(ColorMuted)

	VisualModeIndicator = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorBg).
		Background(ColorWarning).
		Padding(0, 1)

	VisualSelectionStyle = lipgloss.NewStyle().
		Reverse(true)
}

func ServiceColor(name string) lipgloss.TerminalColor {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "git"):
		return ColorGitBranch
	case strings.Contains(lower, "db") || strings.Contains(lower, "data") || strings.Contains(lower, "postgres") ||
		strings.Contains(lower, "sql") || strings.Contains(lower, "mongo") || strings.Contains(lower, "redis") ||
		strings.Contains(lower, "compose") || strings.Contains(lower, "docker"):
		return ColorSuccess
	case strings.Contains(lower, "api") || strings.Contains(lower, "back") || strings.Contains(lower, "server") ||
		strings.Contains(lower, "node") || strings.Contains(lower, "spring") || strings.Contains(lower, "auth") ||
		strings.Contains(lower, "gateway"):
		return ColorPrimary
	case strings.Contains(lower, "web") || strings.Contains(lower, "front") || strings.Contains(lower, "client") ||
		strings.Contains(lower, "ui") || strings.Contains(lower, "app") || strings.Contains(lower, "vite"):
		return ColorSecondary
	default:
		return ColorDefault
	}
}
