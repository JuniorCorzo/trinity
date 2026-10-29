package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

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

func TestSanitizeLogText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain text",
			input:    "Hello world!",
			expected: "Hello world!",
		},
		{
			name:     "strips clear screen and cursor moves",
			input:    "\x1b[2J\x1b[HServer started on :8080",
			expected: "Server started on :8080",
		},
		{
			name:     "preserves SGR color codes",
			input:    "\x1b[32mSUCCESS\x1b[0m: connected",
			expected: "\x1b[32mSUCCESS\x1b[0m: connected",
		},
		{
			name:     "strips OSC title changes",
			input:    "\x1b]0;Title\x07Running",
			expected: "Running",
		},
		{
			name:     "strips carriage return without newline",
			input:    "Loading...\rDone!\n",
			expected: "Loading...Done!\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeLogText(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeLogText(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestModel_CardsWrap(t *testing.T) {
	cfg := &config.Config{
		Title: "WrapApp",
		Services: []config.ServiceConfig{
			{Name: "S1", Port: 1001},
			{Name: "S2", Port: 1002},
			{Name: "S3", Port: 1003},
			{Name: "S4", Port: 1004},
			{Name: "S5", Port: 1005},
		},
	}
	mgr := service.NewManager(cfg, "/tmp")
	m := NewModel(cfg, mgr)

	// Set a narrow width that forces wrapping into multiple rows
	mModel, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 30})
	m = mModel.(Model)

	cardsView := m.renderCards()
	lines := strings.Split(cardsView, "\n")
	if len(lines) <= 5 {
		t.Errorf("expected cards to wrap into multiple rows for narrow width, got %d lines", len(lines))
	}
}

func TestModel_VisualMode_Toggle(t *testing.T) {
	cfg := &config.Config{Title: "TestVisual"}
	mgr := service.NewManager(cfg, "/tmp")
	m := NewModel(cfg, mgr)

	// Add some logs
	mgr.AppendAllLog(service.LogEntry{Timestamp: time.Now(), ServiceName: "app", Text: "Line 1"})
	mgr.AppendAllLog(service.LogEntry{Timestamp: time.Now(), ServiceName: "app", Text: "Line 2"})
	mgr.AppendAllLog(service.LogEntry{Timestamp: time.Now(), ServiceName: "app", Text: "Line 3"})

	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newM.(Model)

	// Press 'v' to activate character visual mode
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	m = newM.(Model)
	if !m.visualMode {
		t.Fatalf("expected visualMode true after pressing 'v'")
	}
	if m.visualLine {
		t.Fatalf("expected visualLine false for 'v' mode")
	}

	// Press 'V' to switch to line visual mode
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("V")})
	m = newM.(Model)
	if !m.visualMode || !m.visualLine {
		t.Fatalf("expected visualMode true and visualLine true after pressing 'V'")
	}

	// Press 'esc' to exit
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newM.(Model)
	if m.visualMode {
		t.Fatalf("expected visualMode false after pressing 'esc'")
	}

	// Press 'V' to activate line mode directly
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("V")})
	m = newM.(Model)
	if !m.visualMode || !m.visualLine {
		t.Fatalf("expected visualMode and visualLine true after pressing 'V'")
	}

	// Press 'q' to exit
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	m = newM.(Model)
	if m.visualMode {
		t.Fatalf("expected visualMode false after pressing 'q'")
	}
}

func TestModel_VisualMode_MotionsAndYank(t *testing.T) {
	cfg := &config.Config{Title: "TestMotions"}
	mgr := service.NewManager(cfg, "/tmp")
	m := NewModel(cfg, mgr)

	for i := 1; i <= 5; i++ {
		mgr.AppendAllLog(service.LogEntry{Timestamp: time.Now(), ServiceName: "svc", Text: strings.Repeat("x", i*5)})
	}

	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newM.(Model)

	// Enter visual line mode
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("V")})
	m = newM.(Model)

	// Go to top using gg
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	m = newM.(Model)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	m = newM.(Model)

	if m.cursorLine != 0 {
		t.Errorf("expected cursorLine 0 after gg, got %d", m.cursorLine)
	}

	// Move down 2 lines using j
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = newM.(Model)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = newM.(Model)

	if m.cursorLine != 2 {
		t.Errorf("expected cursorLine 2 after 2x j, got %d", m.cursorLine)
	}

	// Move up 1 line using k
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = newM.(Model)

	if m.cursorLine != 1 {
		t.Errorf("expected cursorLine 1 after k, got %d", m.cursorLine)
	}

	// Jump to bottom using G
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	m = newM.(Model)

	if m.cursorLine != 4 {
		t.Errorf("expected cursorLine 4 (last line) after G, got %d", m.cursorLine)
	}

	// Yank with 'y'
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = newM.(Model)

	if m.visualMode {
		t.Errorf("expected visualMode false after yank 'y'")
	}
	if !strings.Contains(m.yankFeedback, "Yanked") {
		t.Errorf("expected yankFeedback to contain 'Yanked', got %q", m.yankFeedback)
	}
}

func TestModel_VisualCharMode_MotionsAndExtraction(t *testing.T) {
	cfg := &config.Config{Title: "TestCharVisual"}
	mgr := service.NewManager(cfg, "/tmp")
	m := NewModel(cfg, mgr)

	mgr.AppendAllLog(service.LogEntry{
		Timestamp:   time.Now(),
		ServiceName: "web",
		Text:        "hello awesome world of tui",
	})

	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m = newM.(Model)

	// Enter visual character mode
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	m = newM.(Model)

	if !m.visualMode || m.visualLine {
		t.Fatalf("expected visualMode true and visualLine false")
	}

	// Move cursor to start of line using '0'
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("0")})
	m = newM.(Model)
	if m.cursorCol != 0 {
		t.Errorf("expected cursorCol 0 after '0', got %d", m.cursorCol)
	}

	// Move word forward using 'w'
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	m = newM.(Model)
	if m.cursorCol == 0 {
		t.Errorf("expected cursorCol > 0 after 'w'")
	}

	// Move word backward using 'b'
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	m = newM.(Model)
	if m.cursorCol != 0 {
		t.Errorf("expected cursorCol 0 after 'b', got %d", m.cursorCol)
	}

	// Move to end of line using '$'
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("$")})
	m = newM.(Model)
	if m.cursorCol <= 0 {
		t.Errorf("expected cursorCol > 0 after '$'")
	}

	// Extract selection text
	extracted := m.extractSelectionText()
	if !strings.Contains(extracted, "hello awesome world") {
		t.Errorf("expected extracted text to contain 'hello awesome world', got %q", extracted)
	}
	if strings.Contains(extracted, "\x1b[") {
		t.Errorf("extracted text should not contain ANSI escape codes: %q", extracted)
	}

	// Yank
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = newM.(Model)
	if m.visualMode {
		t.Errorf("expected visualMode false after 'y'")
	}
	if !strings.Contains(m.yankFeedback, "Yanked") {
		t.Errorf("expected yankFeedback to contain 'Yanked', got %q", m.yankFeedback)
	}
}

func TestModel_NormalMode_PersistentCursor(t *testing.T) {
	cfg := &config.Config{Title: "TestCursor"}
	mgr := service.NewManager(cfg, "/tmp")
	m := NewModel(cfg, mgr)

	mgr.AppendAllLog(service.LogEntry{Timestamp: time.Now(), ServiceName: "app", Text: "Line 0"})
	mgr.AppendAllLog(service.LogEntry{Timestamp: time.Now(), ServiceName: "app", Text: "Line 1"})
	mgr.AppendAllLog(service.LogEntry{Timestamp: time.Now(), ServiceName: "app", Text: "Line 2"})
	mgr.AppendAllLog(service.LogEntry{Timestamp: time.Now(), ServiceName: "app", Text: "Line 3"})

	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newM.(Model)

	// In normal mode, navigate with 'k' to move cursor up
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = newM.(Model)

	if m.cursorLine != 2 {
		t.Errorf("expected cursorLine 2 after 'k' from bottom (index 3), got %d", m.cursorLine)
	}

	// Move cursor up again with 'k'
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = newM.(Model)

	if m.cursorLine != 1 {
		t.Errorf("expected cursorLine 1 after 2x 'k', got %d", m.cursorLine)
	}

	// Press 'V' to start visual line mode from cursorLine 1
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("V")})
	m = newM.(Model)

	if !m.visualMode || !m.visualLine {
		t.Fatalf("expected visual line mode")
	}
	if m.anchorLine != 1 || m.cursorLine != 1 {
		t.Errorf("expected anchorLine 1 and cursorLine 1, got anchor=%d cursor=%d", m.anchorLine, m.cursorLine)
	}

	// Move selection down to line 2
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = newM.(Model)

	text := m.extractSelectionText()
	if !strings.Contains(text, "Line 1") || !strings.Contains(text, "Line 2") {
		t.Errorf("expected selection to contain Line 1 and Line 2, got %q", text)
	}
}

func TestModel_SidebarMode_Toggle(t *testing.T) {
	cfg := &config.Config{Title: "TestSidebar"}
	mgr := service.NewManager(cfg, "/tmp")
	m := NewModel(cfg, mgr)

	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = newM.(Model)

	if m.sidebarMode != SidebarExpanded {
		t.Fatalf("expected initial sidebarMode SidebarExpanded, got %v", m.sidebarMode)
	}

	// Press 'b' to cycle to Compact
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	m = newM.(Model)
	if m.sidebarMode != SidebarCompact {
		t.Errorf("expected sidebarMode SidebarCompact, got %v", m.sidebarMode)
	}
	if m.getSidebarWidth() != 12 {
		t.Errorf("expected compact sidebar width 12, got %d", m.getSidebarWidth())
	}

	// Press 'b' to cycle to Hidden
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	m = newM.(Model)
	if m.sidebarMode != SidebarHidden {
		t.Errorf("expected sidebarMode SidebarHidden, got %v", m.sidebarMode)
	}
	if m.getSidebarWidth() != 0 {
		t.Errorf("expected hidden sidebar width 0, got %d", m.getSidebarWidth())
	}

	// Press 'b' to cycle back to Expanded
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	m = newM.(Model)
	if m.sidebarMode != SidebarExpanded {
		t.Errorf("expected sidebarMode SidebarExpanded after full cycle, got %v", m.sidebarMode)
	}
}

func TestModel_GitBadge_MutuallyExclusive(t *testing.T) {
	cfg := &config.Config{Title: "GitTest"}
	mgr := service.NewManager(cfg, "/tmp")
	m := NewModel(cfg, mgr)

	// Case 1: Standard repo (not linked worktree) -> shows ONLY branch
	m.gitInfo = &git.GitInfo{
		IsGit:    true,
		IsLinked: false,
		Branch:   "main",
		Commit:   "a1b2c3d",
		Dirty:    true,
	}
	header := m.renderHeader()
	cleanHeader := ansi.Strip(header)
	if !strings.Contains(cleanHeader, "main*") {
		t.Errorf("expected header to contain 'main*', got %q", cleanHeader)
	}
	if strings.Contains(cleanHeader, "wt:") {
		t.Errorf("expected standard repo header NOT to contain 'wt:', got %q", cleanHeader)
	}

	// Case 2: Linked worktree -> shows ONLY worktree
	m.gitInfo = &git.GitInfo{
		IsGit:        true,
		IsLinked:     true,
		WorktreeName: "feature-login",
		Branch:       "feature/login",
		Commit:       "e5f6g7h",
		Dirty:        false,
	}
	header = m.renderHeader()
	cleanHeader = ansi.Strip(header)
	if !strings.Contains(cleanHeader, "wt:feature-login") {
		t.Errorf("expected header to contain 'wt:feature-login', got %q", cleanHeader)
	}
	if strings.Contains(cleanHeader, "feature/login") {
		t.Errorf("expected linked worktree header NOT to contain branch 'feature/login', got %q", cleanHeader)
	}
}

func TestModel_Dimensions(t *testing.T) {
	cfg := &config.Config{
		Title: "DimTest",
		Services: []config.ServiceConfig{
			{Name: "Postgres", Port: 5432},
			{Name: "API", Port: 3001},
			{Name: "Web", Port: 5173},
		},
	}
	mgr := service.NewManager(cfg, "/tmp")
	m := NewModel(cfg, mgr)

	targetWidth := 100
	targetHeight := 30
	newM, _ := m.Update(tea.WindowSizeMsg{Width: targetWidth, Height: targetHeight})
	m = newM.(Model)

	header := m.renderHeader()
	footer := m.renderFooter()
	sidebarWidth := m.getSidebarWidth()
	sidebar := m.renderSidebar(sidebarWidth, targetHeight-lipgloss.Height(header)-lipgloss.Height(footer))
	view := m.View()

	t.Logf("targetWidth: %d, targetHeight: %d", targetWidth, targetHeight)
	t.Logf("header: width=%d, height=%d, raw=%q", lipgloss.Width(header), lipgloss.Height(header), header)
	t.Logf("footer: width=%d, height=%d, raw=%q", lipgloss.Width(footer), lipgloss.Height(footer), footer)
	t.Logf("sidebar: width=%d, height=%d (requested width=%d)", lipgloss.Width(sidebar), lipgloss.Height(sidebar), sidebarWidth)
	t.Logf("viewport: width=%d, height=%d", m.viewport.Width, m.viewport.Height)
	t.Logf("viewport.View(): width=%d, height=%d", lipgloss.Width(m.viewport.View()), lipgloss.Height(m.viewport.View()))
	t.Logf("view: width=%d, height=%d", lipgloss.Width(view), lipgloss.Height(view))

	lines := strings.Split(view, "\n")
	for i, line := range lines {
		w := lipgloss.Width(line)
		if w > targetWidth {
			t.Errorf("line %d width %d exceeds target width %d: %q", i, w, targetWidth, line)
		}
	}
	if len(lines) > targetHeight {
		t.Errorf("total lines %d exceeds target height %d", len(lines), targetHeight)
	}
}



