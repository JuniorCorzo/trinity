package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/juniorcorzo/dev-tui/internal/theme"
)

func isCatppuccinColor(c lipgloss.TerminalColor) bool {
	if c == nil {
		return false
	}
	if colorVal, ok := c.(lipgloss.Color); ok {
		s := string(colorVal)
		return strings.HasPrefix(s, "#")
	}
	return false
}

func TestStyles_ApplyTheme(t *testing.T) {
	// 1. Test Catppuccin theme
	ApplyTheme(&theme.CatppuccinNoirTheme)
	colors := []struct {
		name  string
		color lipgloss.TerminalColor
	}{
		{"ColorPrimary", ColorPrimary},
		{"ColorSecondary", ColorSecondary},
		{"ColorSuccess", ColorSuccess},
		{"ColorWarning", ColorWarning},
		{"ColorDanger", ColorDanger},
		{"ColorMuted", ColorMuted},
		{"ColorCyan", ColorCyan},
		{"ColorMagenta", ColorMagenta},
		{"ColorBackend", ColorBackend},
		{"ColorFrontend", ColorFrontend},
		{"ColorCompose", ColorCompose},
		{"ColorSpring", ColorSpring},
	}

	for _, c := range colors {
		t.Run("catppuccin_"+c.name, func(t *testing.T) {
			if !isCatppuccinColor(c.color) {
				t.Errorf("%s = %v is not a valid Catppuccin hex color", c.name, c.color)
			}
		})
	}

	// 2. Test Terminal theme (default transparent)
	ApplyTheme(&theme.TerminalTheme)
	if !CurrentTheme.Transparent {
		t.Errorf("expected TerminalTheme to be transparent")
	}
	if ColorPrimary == nil {
		t.Errorf("expected ColorPrimary to be defined")
	}

	// Verify PaintLine transparent does not inject background
	raw := "\x1b[32m[database]\x1b[0m test line"
	painted := PaintLine(raw, "#000000", 30)
	if strings.Contains(painted, "\x1b[48;2;") {
		t.Errorf("PaintLine in transparent mode should NOT inject 24bit background, got: %q", painted)
	}
}
