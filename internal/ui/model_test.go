package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/juniorcorzo/dev-tui/internal/config"
	"github.com/juniorcorzo/dev-tui/internal/git"
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

func TestModel_WorktreeModal(t *testing.T) {
	cfg := &config.Config{Title: "Test"}
	mgr := service.NewManager(cfg, "/tmp")
	m := NewModel(cfg, mgr)

	m.gitInfo = &git.GitInfo{
		IsGit:      true,
		IsWorktree: true,
		Branch:     "main",
		Worktrees: []git.WorktreeInfo{
			{Path: "/tmp/repo", Branch: "main", IsCurrent: true},
			{Path: "/tmp/repo-feat", Branch: "feat-auth", IsCurrent: false},
		},
	}

	// Press 'w' to open worktree modal
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	m = newM.(Model)
	if !m.showWorktreeModal {
		t.Errorf("expected showWorktreeModal to be true after pressing 'w'")
	}

	// Press 'down' or 'j'
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newM.(Model)
	if m.selectedWorktreeIdx != 1 {
		t.Errorf("expected selectedWorktreeIdx 1 after Down key, got %d", m.selectedWorktreeIdx)
	}

	// Press 'up' or 'k'
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = newM.(Model)
	if m.selectedWorktreeIdx != 0 {
		t.Errorf("expected selectedWorktreeIdx 0 after Up key, got %d", m.selectedWorktreeIdx)
	}

	// Press 'esc' to close modal
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newM.(Model)
	if m.showWorktreeModal {
		t.Errorf("expected showWorktreeModal to be false after pressing 'esc'")
	}
}

func TestModel_GitLiveUpdate(t *testing.T) {
	dir := t.TempDir()

	// Initialize git repo in dir
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = dir
	_ = cmd.Run()

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = dir
	_ = cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Tester")
	cmd.Dir = dir
	_ = cmd.Run()

	_ = os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0644)
	cmd = exec.Command("git", "add", "file.txt")
	cmd.Dir = dir
	_ = cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "init")
	cmd.Dir = dir
	_ = cmd.Run()

	cfg := &config.Config{Title: "LiveApp"}
	mgr := service.NewManager(cfg, dir)
	m := NewModel(cfg, mgr)

	if m.gitInfo == nil || m.gitInfo.Branch != "main" {
		t.Fatalf("expected initial branch main, got %v", m.gitInfo)
	}

	// Switch branch externally
	cmd = exec.Command("git", "checkout", "-b", "feature/live-branch")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	// Send tickMsg to simulate live timer
	newM, _ := m.Update(tickMsg(time.Now()))
	m = newM.(Model)

	if m.gitInfo.Branch != "feature/live-branch" {
		t.Errorf("expected updated branch feature/live-branch, got %s", m.gitInfo.Branch)
	}

	// Verify log entry was recorded for branch change
	logs := mgr.GetAllLogs()
	found := false
	for _, l := range logs {
		if l.ServiceName == "git" && strings.Contains(l.Text, "Branch switched: main -> feature/live-branch") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected git log notification for branch change, got logs: %+v", logs)
	}
}

