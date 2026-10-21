package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/juniorcorzo/dev-tui/internal/config"
	"github.com/juniorcorzo/dev-tui/internal/service"
)

func TestModel_KeyNavigation(t *testing.T) {
	cfg := &config.Config{
		Title: "TestApp",
		Services: []config.ServiceConfig{
			{Name: "Backend", Port: 3001},
			{Name: "Frontend", Port: 5173},
		},
	}
	mgr := service.NewManager(cfg, "/tmp")
	m := NewModel(cfg, mgr)

	mModel, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = mModel.(Model)

	if m.activeTab != 0 {
		t.Fatalf("expected initial activeTab 0, got %d", m.activeTab)
	}

	tests := []struct {
		name    string
		key     string
		wantTab int
	}{
		{"tab advances to backend", "tab", 1},
		{"tab advances to frontend", "tab", 2},
		{"tab wraps to all", "tab", 0},
		{"shift+tab wraps backwards to frontend", "shift+tab", 2},
		{"key 1 jumps to backend", "1", 1},
		{"key 0 jumps to all", "0", 0},
		{"key 2 jumps to frontend", "2", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)})
			if tt.key == "tab" {
				newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
			} else if tt.key == "shift+tab" {
				newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
			}
			m = newModel.(Model)
			if m.activeTab != tt.wantTab {
				t.Errorf("activeTab = %d, want %d", m.activeTab, tt.wantTab)
			}
		})
	}
}

func TestModel_AutoScrollToggle(t *testing.T) {
	cfg := &config.Config{Title: "Test"}
	mgr := service.NewManager(cfg, "/tmp")
	m := NewModel(cfg, mgr)

	if !m.autoScroll {
		t.Errorf("expected autoScroll true initially")
	}

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = newM.(Model)
	if m.autoScroll {
		t.Errorf("expected autoScroll false after toggle")
	}

	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = newM.(Model)
	if !m.autoScroll {
		t.Errorf("expected autoScroll true after second toggle")
	}
}
