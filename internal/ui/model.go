package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/aymanbagabas/go-osc52/v2"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/juniorcorzo/dev-tui/internal/config"
	"github.com/juniorcorzo/dev-tui/internal/git"
	"github.com/juniorcorzo/dev-tui/internal/service"
	"github.com/juniorcorzo/dev-tui/internal/theme"
)

var (
	oscRegex       = regexp.MustCompile(`\x1b\][^\x07\x1b]*(\x07|\x1b\\)?`)
	csiNonSgrRegex = regexp.MustCompile(`\x1b\[[0-9;?]*[A-LN-Za-ln-z]`)
)

func sanitizeLogText(text string) string {
	text = oscRegex.ReplaceAllString(text, "")
	text = csiNonSgrRegex.ReplaceAllString(text, "")
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		if (r < 32 && r != '\t' && r != '\n' && r != 0x1b) || r == 127 {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

type tickMsg time.Time
type logBatchMsg []service.LogEntry
type statusUpdateMsg struct{}

type serviceOpResultMsg struct {
	op  string
	id  int
	err error
}

type SidebarMode int

const (
	SidebarExpanded SidebarMode = 0 // Full cards, ~30-34 cols
	SidebarCompact  SidebarMode = 1 // Compact icon badges, ~12 cols
	SidebarHidden   SidebarMode = 2 // Hidden, 100% viewport width
)

type Model struct {
	cfg                 *config.Config
	mgr                 *service.Manager
	rootDir             string
	gitInfo             *git.GitInfo
	showWorktreeModal   bool
	selectedWorktreeIdx int
	activeTab           int
	sidebarMode         SidebarMode
	viewport            viewport.Model
	autoScroll          bool
	width               int
	height              int
	ready               bool
	quitting            bool

	// Neovim visual copy mode
	visualMode   bool
	visualLine   bool
	cursorLine   int
	cursorCol    int
	anchorLine   int
	anchorCol    int
	pendingKey   string
	yankFeedback string
	logLines     []string
}

func NewModel(cfg *config.Config, mgr *service.Manager) Model {
	rootDir := mgr.GetRootDir()
	gitInfo, _ := git.GetInfo(rootDir)

	if cfg != nil && cfg.Theme != "" {
		if th, err := theme.LoadTheme(cfg.Theme, rootDir); err == nil {
			ApplyTheme(th)
		}
	} else {
		ApplyTheme(&theme.TerminalTheme)
	}

	selectedWT := 0
	if gitInfo != nil {
		for i, wt := range gitInfo.Worktrees {
			if wt.IsCurrent {
				selectedWT = i
				break
			}
		}
	}

	return Model{
		cfg:                 cfg,
		mgr:                 mgr,
		rootDir:             rootDir,
		gitInfo:             gitInfo,
		selectedWorktreeIdx: selectedWT,
		activeTab:           0,
		sidebarMode:         SidebarExpanded,
		autoScroll:          true,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		waitForLogs(m.mgr.LogStream),
		waitForStatus(m.mgr.StatusStream),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func waitForLogs(ch chan service.LogEntry) tea.Cmd {
	return func() tea.Msg {
		entry, ok := <-ch
		if !ok {
			return nil
		}
		batch := []service.LogEntry{entry}
		for {
			select {
			case next, open := <-ch:
				if !open {
					return logBatchMsg(batch)
				}
				batch = append(batch, next)
				if len(batch) >= 100 {
					return logBatchMsg(batch)
				}
			default:
				return logBatchMsg(batch)
			}
		}
	}
}

func waitForStatus(ch chan struct{}) tea.Cmd {
	return func() tea.Msg {
		_, ok := <-ch
		if !ok {
			return nil
		}
		return statusUpdateMsg{}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.showWorktreeModal {
			switch msg.String() {
			case "ctrl+c", "q":
				m.quitting = true
				m.mgr.StopAll()
				return m, tea.Quit
			case "esc", "w":
				m.showWorktreeModal = false
				return m, nil
			case "up", "k":
				if m.selectedWorktreeIdx > 0 {
					m.selectedWorktreeIdx--
				}
				return m, nil
			case "down", "j":
				if m.gitInfo != nil && m.selectedWorktreeIdx < len(m.gitInfo.Worktrees)-1 {
					m.selectedWorktreeIdx++
				}
				return m, nil
			case "enter":
				if m.gitInfo != nil && m.selectedWorktreeIdx >= 0 && m.selectedWorktreeIdx < len(m.gitInfo.Worktrees) {
					target := m.gitInfo.Worktrees[m.selectedWorktreeIdx]
					if !target.IsCurrent {
						m.switchWorktree(target.Path)
					}
				}
				m.showWorktreeModal = false
				return m, nil
			default:
				return m, nil
			}
		}

		if m.visualMode {
			switch msg.String() {
			case "ctrl+c":
				m.quitting = true
				m.mgr.StopAll()
				return m, tea.Quit

			case "esc", "q":
				m.visualMode = false
				m.pendingKey = ""
				m.refreshViewportContent()
				return m, nil

			case "v":
				if m.visualLine {
					m.visualLine = false
				} else {
					m.visualMode = false
				}
				m.refreshViewportContent()
				return m, nil

			case "V":
				if !m.visualLine {
					m.visualLine = true
				} else {
					m.visualMode = false
				}
				m.refreshViewportContent()
				return m, nil

			case "h", "left":
				m.moveCursorLeft()
				m.refreshViewportContent()
				return m, nil

			case "l", "right":
				m.moveCursorRight()
				m.refreshViewportContent()
				return m, nil

			case "j", "down":
				m.moveCursorDown()
				m.refreshViewportContent()
				return m, nil

			case "k", "up":
				m.moveCursorUp()
				m.refreshViewportContent()
				return m, nil

			case "0", "^":
				m.cursorCol = 0
				m.pendingKey = ""
				m.refreshViewportContent()
				return m, nil

			case "$":
				m.moveCursorLineEnd()
				m.refreshViewportContent()
				return m, nil

			case "w":
				m.moveCursorWordForward()
				m.refreshViewportContent()
				return m, nil

			case "b":
				m.moveCursorWordBackward()
				m.refreshViewportContent()
				return m, nil

			case "g":
				if m.pendingKey == "g" {
					m.cursorLine = 0
					m.cursorCol = 0
					m.pendingKey = ""
					m.viewport.GotoTop()
					m.refreshViewportContent()
				} else {
					m.pendingKey = "g"
				}
				return m, nil

			case "G":
				m.pendingKey = ""
				if len(m.logLines) > 0 {
					m.cursorLine = len(m.logLines) - 1
					m.cursorCol = 0
				}
				m.viewport.GotoBottom()
				m.refreshViewportContent()
				return m, nil

			case "ctrl+u":
				m.pendingKey = ""
				half := m.viewport.Height / 2
				if half < 1 {
					half = 1
				}
				m.cursorLine -= half
				if m.cursorLine < 0 {
					m.cursorLine = 0
				}
				m.viewport.LineUp(half)
				m.refreshViewportContent()
				return m, nil

			case "ctrl+d":
				m.pendingKey = ""
				half := m.viewport.Height / 2
				if half < 1 {
					half = 1
				}
				m.cursorLine += half
				if m.cursorLine >= len(m.logLines) {
					m.cursorLine = len(m.logLines) - 1
				}
				m.viewport.LineDown(half)
				m.refreshViewportContent()
				return m, nil

			case "y":
				m.pendingKey = ""
				textToYank := m.extractSelectionText()
				m.visualMode = false
				lineCount := m.countSelectedLines()
				if lineCount <= 1 {
					m.yankFeedback = "[Yanked 1 line to clipboard]"
				} else {
					m.yankFeedback = fmt.Sprintf("[Yanked %d lines to clipboard]", lineCount)
				}
				m.refreshViewportContent()
				return m, copyToClipboardCmd(textToYank)

			default:
				m.pendingKey = ""
				return m, nil
			}
		}

		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			m.mgr.StopAll()
			return m, tea.Quit

		case "v":
			m.enterVisualMode(false)
			return m, nil

		case "V":
			m.enterVisualMode(true)
			return m, nil

		case "b", "B":
			m.sidebarMode = (m.sidebarMode + 1) % 3
			m.updateViewportDimensions()
			m.refreshViewportContent()
			return m, nil

		case "w":
			if m.gitInfo != nil && m.gitInfo.IsGit {
				m.showWorktreeModal = true
				for i, wt := range m.gitInfo.Worktrees {
					if wt.IsCurrent {
						m.selectedWorktreeIdx = i
						break
					}
				}
			}
			return m, nil

		case "ctrl+r":
			m.reloadCurrentWorkspace()
			return m, nil

		case "tab", "l", "right":
			totalTabs := len(m.mgr.Services) + 1
			m.activeTab = (m.activeTab + 1) % totalTabs
			m.refreshViewportContent()

		case "shift+tab", "h", "left":
			totalTabs := len(m.mgr.Services) + 1
			m.activeTab = (m.activeTab - 1 + totalTabs) % totalTabs
			m.refreshViewportContent()

		case "0":
			m.activeTab = 0
			m.refreshViewportContent()

		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			idx := int(msg.String()[0] - '0')
			if idx <= len(m.mgr.Services) {
				m.activeTab = idx
				m.refreshViewportContent()
			}

		case "r":
			if m.activeTab == 0 {
				return m, m.restartAllCmd()
			}
			svcIdx := m.activeTab - 1
			if svcIdx < len(m.mgr.Services) {
				return m, m.restartServiceCmd(svcIdx)
			}
			return m, nil

		case "R":
			return m, m.restartAllCmd()

		case "s":
			if m.activeTab == 0 {
				anyRunning := false
				for _, s := range m.mgr.Services {
					if s.GetStatus() == service.StatusRunning || s.GetStatus() == service.StatusStarting {
						anyRunning = true
						break
					}
				}
				if anyRunning {
					return m, m.stopAllCmd()
				}
				return m, m.startAllCmd()
			}
			svcIdx := m.activeTab - 1
			if svcIdx < len(m.mgr.Services) {
				s := m.mgr.Services[svcIdx]
				if s.GetStatus() == service.StatusRunning || s.GetStatus() == service.StatusStarting {
					return m, m.stopServiceCmd(svcIdx)
				}
				return m, m.startServiceCmd(svcIdx)
			}
			return m, nil

		case "c":
			if m.activeTab == 0 {
				m.mgr.ClearAllLogs()
			} else {
				svcIdx := m.activeTab - 1
				if svcIdx < len(m.mgr.Services) {
					m.mgr.Services[svcIdx].ClearLogs()
				}
			}
			m.refreshViewportContent()

		case "k", "up":
			m.autoScroll = false
			m.moveCursorUp()
			m.refreshViewportContent()
			return m, nil

		case "j", "down":
			m.autoScroll = false
			m.moveCursorDown()
			m.refreshViewportContent()
			return m, nil

		case "g":
			if m.pendingKey == "g" {
				m.autoScroll = false
				m.cursorLine = 0
				m.cursorCol = 0
				m.pendingKey = ""
				m.viewport.GotoTop()
				m.refreshViewportContent()
			} else {
				m.pendingKey = "g"
			}
			return m, nil

		case "G":
			m.pendingKey = ""
			if len(m.logLines) > 0 {
				m.cursorLine = len(m.logLines) - 1
				m.cursorCol = 0
			}
			m.autoScroll = true
			m.viewport.GotoBottom()
			m.refreshViewportContent()
			return m, nil

		case "ctrl+u":
			m.pendingKey = ""
			m.autoScroll = false
			half := m.viewport.Height / 2
			if half < 1 {
				half = 1
			}
			m.cursorLine -= half
			if m.cursorLine < 0 {
				m.cursorLine = 0
			}
			m.viewport.LineUp(half)
			m.refreshViewportContent()
			return m, nil

		case "ctrl+d":
			m.pendingKey = ""
			m.autoScroll = false
			half := m.viewport.Height / 2
			if half < 1 {
				half = 1
			}
			m.cursorLine += half
			if m.cursorLine >= len(m.logLines) {
				m.cursorLine = len(m.logLines) - 1
			}
			m.viewport.LineDown(half)
			m.refreshViewportContent()
			return m, nil

		case "a":
			m.autoScroll = !m.autoScroll
			if m.autoScroll {
				m.viewport.GotoBottom()
			}

		default:
			var cmd tea.Cmd
			prevYOffset := m.viewport.YOffset
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)

			if m.viewport.YOffset < prevYOffset {
				m.autoScroll = false
			} else if m.viewport.AtBottom() {
				m.autoScroll = true
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateViewportDimensions()
		m.refreshViewportContent()

	case tickMsg:
		cmds = append(cmds, tickCmd())
		if m.rootDir != "" {
			newGit, err := git.GetInfo(m.rootDir)
			if err == nil && newGit != nil {
				if git.HasChanged(m.gitInfo, newGit) {
					if m.gitInfo != nil && m.gitInfo.IsGit && newGit.IsGit {
						if newGit.Branch != m.gitInfo.Branch {
							m.mgr.AppendAllLog(service.LogEntry{
								Timestamp:   time.Now(),
								ServiceID:   -1,
								ServiceName: "git",
								Text:        fmt.Sprintf("Branch switched: %s -> %s (%s)", m.gitInfo.Branch, newGit.Branch, newGit.Commit),
							})
							m.refreshViewportContent()
						} else if newGit.Commit != m.gitInfo.Commit {
							m.mgr.AppendAllLog(service.LogEntry{
								Timestamp:   time.Now(),
								ServiceID:   -1,
								ServiceName: "git",
								Text:        fmt.Sprintf("Commit updated on %s: %s (%s)", newGit.Branch, newGit.Commit, newGit.CommitMsg),
							})
							m.refreshViewportContent()
						} else if newGit.IsWorktree != m.gitInfo.IsWorktree || newGit.WorktreeName != m.gitInfo.WorktreeName {
							m.mgr.AppendAllLog(service.LogEntry{
								Timestamp:   time.Now(),
								ServiceID:   -1,
								ServiceName: "git",
								Text:        fmt.Sprintf("Worktree state updated: %s", newGit.WorktreeName),
							})
							m.refreshViewportContent()
						}
					}
					m.gitInfo = newGit
				}
			}
		}

	case logBatchMsg:
		shouldUpdate := false
		if m.activeTab == 0 {
			shouldUpdate = true
		} else {
			activeID := m.activeTab - 1
			for _, entry := range msg {
				if entry.ServiceID == activeID {
					shouldUpdate = true
					break
				}
			}
		}

		if shouldUpdate {
			m.refreshViewportContent()
		}
		cmds = append(cmds, waitForLogs(m.mgr.LogStream))

	case statusUpdateMsg:
		cmds = append(cmds, waitForStatus(m.mgr.StatusStream))

	case serviceOpResultMsg:
		if msg.err != nil {
			m.mgr.AppendAllLog(service.LogEntry{
				Timestamp:   time.Now(),
				ServiceID:   msg.id,
				ServiceName: "dev-tui",
				Text:        fmt.Sprintf("Service %s failed: %v", msg.op, msg.err),
			})
			m.refreshViewportContent()
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) restartServiceCmd(id int) tea.Cmd {
	return func() tea.Msg {
		err := m.mgr.RestartService(id)
		return serviceOpResultMsg{op: "restart", id: id, err: err}
	}
}

func (m *Model) restartAllCmd() tea.Cmd {
	return func() tea.Msg {
		m.mgr.RestartAll()
		return serviceOpResultMsg{op: "restartAll", id: -1}
	}
}

func (m *Model) startServiceCmd(id int) tea.Cmd {
	return func() tea.Msg {
		err := m.mgr.StartService(id)
		return serviceOpResultMsg{op: "start", id: id, err: err}
	}
}

func (m *Model) stopServiceCmd(id int) tea.Cmd {
	return func() tea.Msg {
		err := m.mgr.StopService(id)
		return serviceOpResultMsg{op: "stop", id: id, err: err}
	}
}

func (m *Model) startAllCmd() tea.Cmd {
	return func() tea.Msg {
		m.mgr.StartAll()
		return serviceOpResultMsg{op: "startAll", id: -1}
	}
}

func (m *Model) stopAllCmd() tea.Cmd {
	return func() tea.Msg {
		m.mgr.StopAll()
		return serviceOpResultMsg{op: "stopAll", id: -1}
	}
}

func (m Model) getSidebarWidth() int {
	switch m.sidebarMode {
	case SidebarCompact:
		return 12
	case SidebarHidden:
		return 0
	default: // SidebarExpanded
		if m.width < 70 {
			return 12
		}
		w := 30
		if m.width > 120 {
			w = 34
		}
		return w
	}
}

func (m *Model) updateViewportDimensions() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	headerHeight := lipgloss.Height(m.renderHeader())
	footerHeight := 1
	bodyHeight := m.height - headerHeight - footerHeight
	if bodyHeight < 5 {
		bodyHeight = 5
	}

	sidebarWidth := m.getSidebarWidth()
	viewportWidth := m.width - sidebarWidth
	if viewportWidth < 10 {
		viewportWidth = 10
	}

	if !m.ready {
		m.viewport = viewport.New(viewportWidth, bodyHeight)
		m.viewport.YPosition = headerHeight
		m.ready = true
	} else {
		m.viewport.Width = viewportWidth
		m.viewport.Height = bodyHeight
		m.viewport.YPosition = headerHeight
	}
}

func (m *Model) switchWorktree(targetPath string) {
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return
	}

	newCfg, err := config.LoadOrDetect(absTarget, "")
	if err != nil {
		m.mgr.AppendAllLog(service.LogEntry{
			Timestamp:   time.Now(),
			ServiceID:   -1,
			ServiceName: "git",
			Text:        fmt.Sprintf("Failed to load config for worktree %s: %v", absTarget, err),
		})
		return
	}

	oldDir := m.rootDir
	m.rootDir = absTarget
	m.cfg = newCfg
	m.mgr.ResetServices(newCfg, absTarget)

	for i, sc := range newCfg.Services {
		if sc.AutoStart {
			_ = m.mgr.StartService(i)
		}
	}

	m.gitInfo, _ = git.GetInfo(absTarget)
	m.activeTab = 0
	m.updateViewportDimensions()

	wtName := filepath.Base(absTarget)
	branchName := "unknown"
	if m.gitInfo != nil {
		branchName = m.gitInfo.Branch
	}

	m.mgr.AppendAllLog(service.LogEntry{
		Timestamp:   time.Now(),
		ServiceID:   -1,
		ServiceName: "git",
		Text:        fmt.Sprintf("Switched workspace from '%s' to worktree '%s' [branch: %s]", filepath.Base(oldDir), wtName, branchName),
	})

	m.refreshViewportContent()
}

func (m *Model) reloadCurrentWorkspace() {
	newCfg, err := config.LoadOrDetect(m.rootDir, "")
	if err != nil {
		m.mgr.AppendAllLog(service.LogEntry{
			Timestamp:   time.Now(),
			ServiceID:   -1,
			ServiceName: "git",
			Text:        fmt.Sprintf("Reload failed: %v", err),
		})
		return
	}

	m.cfg = newCfg
	m.mgr.ResetServices(newCfg, m.rootDir)
	for i, sc := range newCfg.Services {
		if sc.AutoStart {
			_ = m.mgr.StartService(i)
		}
	}
	m.gitInfo, _ = git.GetInfo(m.rootDir)
	m.activeTab = 0
	m.updateViewportDimensions()

	m.mgr.AppendAllLog(service.LogEntry{
		Timestamp:   time.Now(),
		ServiceID:   -1,
		ServiceName: "git",
		Text:        "Configuration and services reloaded for workspace",
	})

	m.refreshViewportContent()
}

func copyToClipboardCmd(text string) tea.Cmd {
	return func() tea.Msg {
		if text == "" {
			return nil
		}
		seq := osc52.New(text)
		if os.Getenv("TMUX") != "" {
			_, _ = fmt.Fprint(os.Stderr, seq.Tmux())
		} else {
			_, _ = fmt.Fprint(os.Stderr, seq)
		}
		return nil
	}
}

func (m *Model) updateLogLines() {
	var logs []service.LogEntry
	if m.activeTab == 0 {
		logs = m.mgr.GetAllLogs()
	} else {
		svcIdx := m.activeTab - 1
		if svcIdx < len(m.mgr.Services) {
			logs = m.mgr.Services[svcIdx].GetLogs()
		}
	}

	var lines []string
	for _, entry := range logs {
		timeStr := entry.Timestamp.Format("15:04:05")
		timePrefix := lipgloss.NewStyle().Foreground(ColorDim).Render(timeStr)

		svcColor := ServiceColor(entry.ServiceName)
		tag := lipgloss.NewStyle().
			Foreground(svcColor).
			Bold(true).
			Render(fmt.Sprintf("[%s]", strings.ToLower(entry.ServiceName)))

		cleanText := sanitizeLogText(entry.Text)
		parts := strings.Split(cleanText, "\n")
		for _, part := range parts {
			if part == "" && len(parts) > 1 {
				continue
			}
			lines = append(lines, fmt.Sprintf("%s %s %s", timePrefix, tag, part))
		}
	}
	m.logLines = lines
}

func (m *Model) enterVisualMode(lineMode bool) {
	m.updateLogLines()
	m.visualMode = true
	m.visualLine = lineMode
	m.autoScroll = false
	m.pendingKey = ""
	m.yankFeedback = ""

	if len(m.logLines) > 0 {
		if m.cursorLine >= len(m.logLines) {
			m.cursorLine = len(m.logLines) - 1
		}
		if m.cursorLine < 0 {
			m.cursorLine = 0
		}
	} else {
		m.cursorLine = 0
	}
	m.anchorLine = m.cursorLine
	m.anchorCol = m.cursorCol

	m.ensureCursorVisible()
	m.refreshViewportContent()
}

func (m *Model) moveCursorLeft() {
	m.pendingKey = ""
	if m.cursorCol > 0 {
		m.cursorCol--
	}
}

func (m *Model) moveCursorRight() {
	m.pendingKey = ""
	if m.cursorLine >= 0 && m.cursorLine < len(m.logLines) {
		lineLen := len([]rune(ansi.Strip(m.logLines[m.cursorLine])))
		if m.cursorCol < lineLen-1 {
			m.cursorCol++
		}
	}
}

func (m *Model) moveCursorDown() {
	m.pendingKey = ""
	if m.cursorLine < len(m.logLines)-1 {
		m.cursorLine++
		m.ensureCursorVisible()
	}
}

func (m *Model) moveCursorUp() {
	m.pendingKey = ""
	if m.cursorLine > 0 {
		m.cursorLine--
		m.ensureCursorVisible()
	}
}

func (m *Model) moveCursorLineEnd() {
	m.pendingKey = ""
	if m.cursorLine >= 0 && m.cursorLine < len(m.logLines) {
		lineLen := len([]rune(ansi.Strip(m.logLines[m.cursorLine])))
		if lineLen > 0 {
			m.cursorCol = lineLen - 1
		} else {
			m.cursorCol = 0
		}
	}
}

func (m *Model) moveCursorWordForward() {
	m.pendingKey = ""
	if m.cursorLine < 0 || m.cursorLine >= len(m.logLines) {
		return
	}
	line := ansi.Strip(m.logLines[m.cursorLine])
	runes := []rune(line)
	if m.cursorCol >= len(runes) {
		if m.cursorLine < len(m.logLines)-1 {
			m.cursorLine++
			m.cursorCol = 0
			m.ensureCursorVisible()
		}
		return
	}

	i := m.cursorCol
	for i < len(runes) && !unicode.IsSpace(runes[i]) {
		i++
	}
	for i < len(runes) && unicode.IsSpace(runes[i]) {
		i++
	}
	if i < len(runes) {
		m.cursorCol = i
	} else if m.cursorLine < len(m.logLines)-1 {
		m.cursorLine++
		m.cursorCol = 0
		m.ensureCursorVisible()
	}
}

func (m *Model) moveCursorWordBackward() {
	m.pendingKey = ""
	if m.cursorLine < 0 || m.cursorLine >= len(m.logLines) {
		return
	}
	line := ansi.Strip(m.logLines[m.cursorLine])
	runes := []rune(line)
	if m.cursorCol <= 0 {
		if m.cursorLine > 0 {
			m.cursorLine--
			m.ensureCursorVisible()
			prevLine := ansi.Strip(m.logLines[m.cursorLine])
			prevRunes := []rune(prevLine)
			if len(prevRunes) > 0 {
				m.cursorCol = len(prevRunes) - 1
			} else {
				m.cursorCol = 0
			}
		}
		return
	}

	i := m.cursorCol
	if i >= len(runes) {
		i = len(runes) - 1
	}
	for i > 0 && unicode.IsSpace(runes[i-1]) {
		i--
	}
	for i > 0 && !unicode.IsSpace(runes[i-1]) {
		i--
	}
	m.cursorCol = i
}

func (m *Model) ensureCursorVisible() {
	if m.cursorLine < m.viewport.YOffset {
		m.viewport.SetYOffset(m.cursorLine)
	} else if m.cursorLine >= m.viewport.YOffset+m.viewport.Height {
		m.viewport.SetYOffset(m.cursorLine - m.viewport.Height + 1)
	}
}

func (m *Model) countSelectedLines() int {
	start := m.anchorLine
	end := m.cursorLine
	if start > end {
		start, end = end, start
	}
	return end - start + 1
}

func (m *Model) extractSelectionText() string {
	if len(m.logLines) == 0 {
		return ""
	}
	startLine := m.anchorLine
	endLine := m.cursorLine
	if startLine > endLine {
		startLine, endLine = endLine, startLine
	}
	if startLine < 0 {
		startLine = 0
	}
	if endLine >= len(m.logLines) {
		endLine = len(m.logLines) - 1
	}

	if m.visualLine {
		var selected []string
		for i := startLine; i <= endLine; i++ {
			selected = append(selected, ansi.Strip(m.logLines[i]))
		}
		return strings.Join(selected, "\n")
	}

	// Character mode
	if startLine == endLine {
		line := ansi.Strip(m.logLines[startLine])
		runes := []rune(line)
		if len(runes) == 0 {
			return ""
		}
		startCol := m.anchorCol
		endCol := m.cursorCol
		if startCol > endCol {
			startCol, endCol = endCol, startCol
		}
		if startCol < 0 {
			startCol = 0
		}
		if startCol >= len(runes) {
			startCol = len(runes) - 1
		}
		if endCol >= len(runes) {
			endCol = len(runes) - 1
		}
		return string(runes[startCol : endCol+1])
	}

	var sb strings.Builder
	for i := startLine; i <= endLine; i++ {
		line := ansi.Strip(m.logLines[i])
		runes := []rune(line)
		if i == startLine {
			from := m.anchorCol
			if m.anchorLine > m.cursorLine {
				from = m.cursorCol
			}
			if from < 0 {
				from = 0
			}
			if from < len(runes) {
				sb.WriteString(string(runes[from:]))
			}
			sb.WriteString("\n")
		} else if i == endLine {
			to := m.cursorCol
			if m.anchorLine > m.cursorLine {
				to = m.anchorCol
			}
			if to >= len(runes) {
				to = len(runes) - 1
			}
			if to >= 0 && len(runes) > 0 {
				sb.WriteString(string(runes[:to+1]))
			}
		} else {
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func (m *Model) refreshViewportContent() {
	m.updateLogLines()

	if m.autoScroll && len(m.logLines) > 0 {
		m.cursorLine = len(m.logLines) - 1
		m.cursorCol = 0
	} else if len(m.logLines) > 0 {
		if m.cursorLine >= len(m.logLines) {
			m.cursorLine = len(m.logLines) - 1
		}
		if m.cursorLine < 0 {
			m.cursorLine = 0
		}
	}

	if !m.visualMode {
		var sb strings.Builder
		for i, line := range m.logLines {
			if i == m.cursorLine && len(m.logLines) > 0 {
				plain := ansi.Strip(line)
				runes := []rune(plain)
				col := m.cursorCol
				if col < 0 {
					col = 0
				}
				if len(runes) == 0 {
					sb.WriteString(VisualSelectionStyle.Render(" "))
				} else if col >= len(runes) {
					sb.WriteString(line + VisualSelectionStyle.Render(" "))
				} else {
					before := string(runes[:col])
					cursorChar := VisualSelectionStyle.Render(string(runes[col]))
					after := ""
					if col+1 < len(runes) {
						after = string(runes[col+1:])
					}
					sb.WriteString(before + cursorChar + after)
				}
			} else {
				sb.WriteString(line)
			}
			sb.WriteString("\n")
		}
		m.viewport.SetContent(sb.String())
		if m.autoScroll {
			m.viewport.GotoBottom()
		}
		return
	}

	startLine := m.anchorLine
	endLine := m.cursorLine
	if startLine > endLine {
		startLine, endLine = endLine, startLine
	}

	var sb strings.Builder
	for i, line := range m.logLines {
		if m.visualLine {
			if i >= startLine && i <= endLine {
				sb.WriteString(VisualSelectionStyle.Render(ansi.Strip(line)))
			} else {
				sb.WriteString(line)
			}
		} else {
			if i < startLine || i > endLine {
				sb.WriteString(line)
			} else if startLine == endLine && i == startLine {
				cStart := m.anchorCol
				cEnd := m.cursorCol
				if cStart > cEnd {
					cStart, cEnd = cEnd, cStart
				}
				plain := ansi.Strip(line)
				runes := []rune(plain)
				if cStart < 0 {
					cStart = 0
				}
				if cStart > len(runes) {
					cStart = len(runes)
				}
				if cEnd >= len(runes) {
					cEnd = len(runes) - 1
				}
				if cEnd < cStart {
					sb.WriteString(line)
				} else {
					before := string(runes[:cStart])
					selected := VisualSelectionStyle.Render(string(runes[cStart : cEnd+1]))
					after := ""
					if cEnd+1 < len(runes) {
						after = string(runes[cEnd+1:])
					}
					sb.WriteString(before + selected + after)
				}
			} else {
				plain := ansi.Strip(line)
				runes := []rune(plain)
				if i == startLine {
					from := m.anchorCol
					if m.anchorLine > m.cursorLine {
						from = m.cursorCol
					}
					if from < 0 {
						from = 0
					}
					if from >= len(runes) {
						sb.WriteString(line)
					} else {
						sb.WriteString(string(runes[:from]) + VisualSelectionStyle.Render(string(runes[from:])))
					}
				} else if i == endLine {
					to := m.cursorCol
					if m.anchorLine > m.cursorLine {
						to = m.anchorCol
					}
					if to >= len(runes) {
						to = len(runes) - 1
					}
					if to >= 0 {
						sb.WriteString(VisualSelectionStyle.Render(string(runes[:to+1])) + string(runes[to+1:]))
					} else {
						sb.WriteString(line)
					}
				} else {
					sb.WriteString(VisualSelectionStyle.Render(plain))
				}
			}
		}
		sb.WriteString("\n")
	}

	m.viewport.SetContent(sb.String())
}

func (m Model) View() string {
	if !m.ready {
		return "\n  Initializing trinity..."
	}
	if m.quitting {
		return "\n  Stopping all services and quitting...\n"
	}

	header := m.renderHeader()
	footer := m.renderFooter()

	bodyHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer)
	if bodyHeight < 5 {
		bodyHeight = 5
	}

	sidebarWidth := m.getSidebarWidth()
	viewportWidth := m.width - sidebarWidth
	if viewportWidth < 10 {
		viewportWidth = 10
	}

	var mainView string
	if m.showWorktreeModal {
		mainView = m.renderWorktreeModal()
	} else {
		m.viewport.Width = viewportWidth
		m.viewport.Height = bodyHeight
		rawVp := m.viewport.View()
		vpLines := strings.Split(rawVp, "\n")
		var paintedVp []string
		for _, l := range vpLines {
			paintedVp = append(paintedVp, PaintLine(l, "#000000", viewportWidth))
		}
		mainView = strings.Join(paintedVp, "\n")
	}

	var body string
	if sidebarWidth > 0 && !m.showWorktreeModal {
		sidebar := m.renderSidebar(sidebarWidth, bodyHeight)
		body = lipgloss.JoinHorizontal(lipgloss.Top, sidebar, mainView)
	} else {
		body = mainView
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		body,
		footer,
	)
}

func (m Model) renderHeader() string {
	appTitle := "TRINITY"
	if m.cfg != nil && m.cfg.Title != "" {
		appTitle = fmt.Sprintf("TRINITY · %s", strings.ToUpper(m.cfg.Title))
	}
	title := TitleStyle.Render(fmt.Sprintf(" ▲ %s ", appTitle))
	now := ClockStyle.Render(time.Now().Format("2006-01-02 15:04:05"))

	var gitBadge string
	if m.gitInfo != nil && m.gitInfo.IsGit {
		gitIcon := ""
		dirty := ""
		if m.gitInfo.Dirty {
			dirty = GitDirtyBadge.Render("*")
		}
		commit := ""
		if m.gitInfo.Commit != "" {
			commit = lipgloss.NewStyle().Foreground(ColorDim).Render(fmt.Sprintf("(%s)", m.gitInfo.Commit))
		}

		// Mutually exclusive: if linked worktree, show ONLY worktree. Else show ONLY branch.
		if m.gitInfo.IsLinked {
			gitBadge = fmt.Sprintf("%s%s %s",
				GitWorktreeBadge.Render(fmt.Sprintf("%s wt:%s", gitIcon, m.gitInfo.WorktreeName)),
				dirty,
				commit,
			)
		} else {
			gitBadge = fmt.Sprintf("%s%s %s",
				GitBranchBadge.Render(fmt.Sprintf("%s %s", gitIcon, m.gitInfo.Branch)),
				dirty,
				commit,
			)
		}
	}

	left := title
	if gitBadge != "" {
		left = lipgloss.JoinHorizontal(lipgloss.Top, title, "  ", gitBadge)
	}

	contentWidth := m.width - HeaderStyle.GetHorizontalFrameSize()
	if contentWidth < 10 {
		contentWidth = 10
	}

	space := contentWidth - lipgloss.Width(left) - lipgloss.Width(now)
	if space < 1 {
		space = 1
	}

	bar := lipgloss.JoinHorizontal(
		lipgloss.Top,
		left,
		strings.Repeat(" ", space),
		now,
	)

	rawHeader := HeaderStyle.Width(m.width).Render(bar)
	return PaintLine(rawHeader, "#09090d", m.width)
}

func (m Model) renderWorktreeModal() string {
	if m.gitInfo == nil || !m.gitInfo.IsGit {
		return ModalContainer.Width(m.width - 4).Render("Not a git repository.")
	}

	var sb strings.Builder
	sb.WriteString(ModalTitle.Render(" 🌿 GIT WORKTREES & REPOSITORY STATUS ") + "\n\n")

	repoType := "Standard Git Repository"
	if m.gitInfo.IsLinked {
		repoType = fmt.Sprintf("Linked Worktree (name: %s)", m.gitInfo.WorktreeName)
	} else if len(m.gitInfo.Worktrees) > 1 {
		repoType = fmt.Sprintf("Main Worktree (%d total worktrees)", len(m.gitInfo.Worktrees))
	}

	sb.WriteString(fmt.Sprintf("%s %s\n", lipgloss.NewStyle().Bold(true).Render("Type:"), repoType))
	sb.WriteString(fmt.Sprintf("%s %s\n", lipgloss.NewStyle().Bold(true).Render("Repo Root:"), m.gitInfo.RepoRoot))
	if m.gitInfo.CommonDir != "" {
		sb.WriteString(fmt.Sprintf("%s %s\n", lipgloss.NewStyle().Bold(true).Render("Common Dir:"), m.gitInfo.CommonDir))
	}

	statusMarker := GitCleanBadge.Render("✔ clean")
	if m.gitInfo.Dirty {
		statusMarker = GitDirtyBadge.Render("* uncommitted changes")
	}
	sb.WriteString(fmt.Sprintf("%s %s (%s) • %s\n\n",
		lipgloss.NewStyle().Bold(true).Render("Active Branch:"),
		GitBranchBadge.Render("🌿 "+m.gitInfo.Branch),
		m.gitInfo.Commit,
		statusMarker,
	))

	sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ColorCyan).Render("Available Worktrees:") + "\n")
	total := len(m.gitInfo.Worktrees)
	if total == 0 {
		sb.WriteString(ModalDim.Render("  (No additional worktrees found. Create one with: git worktree add <dir> -b <branch>)\n"))
	} else {
		maxVisible := m.viewport.Height - 10
		if maxVisible < 3 {
			maxVisible = 3
		}

		start := 0
		end := total
		if total > maxVisible {
			start = m.selectedWorktreeIdx - maxVisible/2
			if start < 0 {
				start = 0
			}
			end = start + maxVisible
			if end > total {
				end = total
				start = end - maxVisible
				if start < 0 {
					start = 0
				}
			}
		}

		if start > 0 {
			sb.WriteString(ModalDim.Render(fmt.Sprintf("  ↑ ... (%d worktrees above)\n", start)))
		}

		for i := start; i < end; i++ {
			wt := m.gitInfo.Worktrees[i]
			cursor := "  "
			itemStyle := ModalNormal
			if i == m.selectedWorktreeIdx {
				cursor = "> "
				itemStyle = ModalSelected
			}

			badges := ""
			if wt.IsCurrent {
				badges += " " + lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true).Render("[CURRENT]")
			}
			if wt.IsMain {
				badges += " " + lipgloss.NewStyle().Foreground(ColorSecondary).Render("(main repo)")
			}
			if wt.Locked {
				badges += " " + lipgloss.NewStyle().Foreground(ColorWarning).Render("[locked]")
			}

			line := fmt.Sprintf("%s[%d] %s  🌿 %s (%s)%s",
				cursor,
				i+1,
				wt.Path,
				wt.Branch,
				wt.Head,
				badges,
			)
			sb.WriteString(itemStyle.Render(line) + "\n")
		}

		if end < total {
			sb.WriteString(ModalDim.Render(fmt.Sprintf("  ↓ ... (%d worktrees below)\n", total-end)))
		}
	}

	sb.WriteString("\n" + ModalDim.Render("Keys: [↑/↓/k/j] Navigate  •  [Enter] Switch Worktree Workspace  •  [Esc/w] Close"))

	return ModalContainer.Width(m.width - 4).Height(m.viewport.Height).Render(sb.String())
}

func (m Model) renderSidebar(width, height int) string {
	if width <= 0 {
		return ""
	}

	frameW := SidebarStyle.GetHorizontalFrameSize()
	sidebarContentWidth := width - frameW
	if sidebarContentWidth < 2 {
		sidebarContentWidth = 2
	}

	cardOuterWidth := sidebarContentWidth - 2
	if cardOuterWidth < 4 {
		cardOuterWidth = sidebarContentWidth
	}

	var items []string

	if m.sidebarMode == SidebarExpanded {
		titleStyle := lipgloss.NewStyle().
			Foreground(ColorMuted).
			Bold(true).
			Padding(0, 1)
		items = append(items, titleStyle.Render("SERVICIOS [Tab]"))
	}

	// [0] ALL SERVICES card
	allActive := m.activeTab == 0
	allRunning := true
	for _, s := range m.mgr.Services {
		if s.GetStatus() != service.StatusRunning {
			allRunning = false
			break
		}
	}

	if m.sidebarMode == SidebarCompact {
		st := CardCompactNormalStyle
		if allActive {
			st = CardCompactActiveStyle
		}
		ring := "◎"
		if !allRunning {
			ring = "◌"
		}
		cardInner := cardOuterWidth - st.GetHorizontalFrameSize()
		if cardInner < 1 {
			cardInner = 1
		}
		items = append(items, st.Width(cardInner).Render(fmt.Sprintf("[0] %s", ring)))
	} else {
		st := CardNormalStyle
		if allActive {
			st = CardActiveStyle
		}
		statusText := BadgeRunning.Render()
		if !allRunning {
			statusText = BadgeStarting.Render()
		}
		content := fmt.Sprintf("[0] ALL SERVICES\n%s · %d svc", statusText, len(m.mgr.Services))
		cardInner := cardOuterWidth - st.GetHorizontalFrameSize()
		if cardInner < 1 {
			cardInner = 1
		}
		items = append(items, st.Width(cardInner).Render(content))
	}

	// Each service
	for i, s := range m.mgr.Services {
		isActive := m.activeTab == i+1

		var statusStr string
		var ring string
		switch s.GetStatus() {
		case service.StatusRunning:
			statusStr = BadgeRunning.Render()
			ring = lipgloss.NewStyle().Foreground(ColorSuccess).Render("◎")
		case service.StatusStarting:
			statusStr = BadgeStarting.Render()
			ring = lipgloss.NewStyle().Foreground(ColorWarning).Render("◌")
		case service.StatusFailed:
			statusStr = BadgeFailed.Render()
			ring = lipgloss.NewStyle().Foreground(ColorDanger).Render("✕")
		default:
			statusStr = BadgeStopped.Render()
			ring = lipgloss.NewStyle().Foreground(ColorMuted).Render("○")
		}

		portStr := "--"
		if s.Port > 0 {
			portStr = fmt.Sprintf(":%d", s.Port)
		}

		svcColor := ServiceColor(s.Name)
		idxStyled := lipgloss.NewStyle().Foreground(svcColor).Bold(true).Render(fmt.Sprintf("[%d]", i+1))

		if m.sidebarMode == SidebarCompact {
			st := CardCompactNormalStyle
			if isActive {
				st = CardCompactActiveStyle
			}
			cardInner := cardOuterWidth - st.GetHorizontalFrameSize()
			if cardInner < 1 {
				cardInner = 1
			}
			items = append(items, st.Width(cardInner).Render(fmt.Sprintf("%s %s", idxStyled, ring)))
		} else {
			st := CardNormalStyle
			if isActive {
				st = CardActiveStyle
			}
			nameStyled := lipgloss.NewStyle().Foreground(ColorWhite).Bold(true).Render(strings.ToUpper(s.Name))
			portStyled := lipgloss.NewStyle().Foreground(ColorDim).Render(portStr)
			if s.Port > 0 {
				portStyled = lipgloss.NewStyle().Foreground(svcColor).Render(portStr)
			}
			uptimeStyled := lipgloss.NewStyle().Foreground(ColorMuted).Render("Up: " + s.GetUptime())

			cardContent := fmt.Sprintf("%s %s\n%s  %s\n%s",
				idxStyled,
				nameStyled,
				statusStr,
				portStyled,
				uptimeStyled,
			)
			cardInner := cardOuterWidth - st.GetHorizontalFrameSize()
			if cardInner < 1 {
				cardInner = 1
			}
			items = append(items, st.Width(cardInner).Render(cardContent))
		}
	}

	if m.sidebarMode == SidebarExpanded && height > len(items)*4+4 {
		footerHint := lipgloss.NewStyle().
			Foreground(ColorDim).
			Padding(1, 1).
			Render("[b] Toggle\n[w] Worktrees")
		items = append(items, footerHint)
	}

	sidebarContent := lipgloss.JoinVertical(lipgloss.Left, items...)
	rawSidebar := SidebarStyle.
		Width(sidebarContentWidth).
		Height(height).
		MaxHeight(height).
		Render(sidebarContent)
	sbLines := strings.Split(rawSidebar, "\n")
	var paintedSb []string
	for _, l := range sbLines {
		paintedSb = append(paintedSb, PaintLine(l, "#09090d", width))
	}
	return strings.Join(paintedSb, "\n")
}

func (m Model) renderCards() string {
	var cardViews []string

	allStyle := CardNormalStyle
	if m.activeTab == 0 {
		allStyle = CardActiveStyle
	}
	allRunning := true
	for _, s := range m.mgr.Services {
		if s.GetStatus() != service.StatusRunning {
			allRunning = false
			break
		}
	}
	allStatus := BadgeRunning.Render()
	if !allRunning {
		allStatus = BadgeStarting.Render()
	}
	allCard := allStyle.Render(fmt.Sprintf("[0] ALL SERVICES\n%s\n%d Services", allStatus, len(m.mgr.Services)))
	cardViews = append(cardViews, allCard)

	for i, s := range m.mgr.Services {
		style := CardNormalStyle
		if m.activeTab == i+1 {
			style = CardActiveStyle
		}

		var statusStr string
		switch s.GetStatus() {
		case service.StatusRunning:
			statusStr = BadgeRunning.Render()
		case service.StatusStarting:
			statusStr = BadgeStarting.Render()
		case service.StatusFailed:
			statusStr = BadgeFailed.Render()
		default:
			statusStr = BadgeStopped.Render()
		}

		portStr := "--"
		if s.Port > 0 {
			portStr = fmt.Sprintf(":%d", s.Port)
		}

		svcColor := ServiceColor(s.Name)
		idxStyled := lipgloss.NewStyle().Foreground(svcColor).Bold(true).Render(fmt.Sprintf("[%d]", i+1))
		nameStyled := lipgloss.NewStyle().Foreground(ColorWhite).Bold(true).Render(strings.ToUpper(s.Name))
		portStyled := lipgloss.NewStyle().Foreground(ColorDim).Render(portStr)
		if s.Port > 0 {
			portStyled = lipgloss.NewStyle().Foreground(svcColor).Render(portStr)
		}
		uptimeStyled := lipgloss.NewStyle().Foreground(ColorMuted).Render("Up: " + s.GetUptime())

		cardContent := fmt.Sprintf("%s %s\n%s  %s\n%s",
			idxStyled,
			nameStyled,
			statusStr,
			portStyled,
			uptimeStyled,
		)

		cardViews = append(cardViews, style.Render(cardContent))
	}

	if len(cardViews) == 0 {
		return ""
	}

	maxWidth := m.width
	if maxWidth <= 0 {
		return lipgloss.JoinHorizontal(lipgloss.Top, cardViews...)
	}

	var rows []string
	var currentRow []string
	currentWidth := 0

	for _, card := range cardViews {
		w := lipgloss.Width(card)
		if len(currentRow) > 0 && currentWidth+w > maxWidth {
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, currentRow...))
			currentRow = []string{card}
			currentWidth = w
		} else {
			currentRow = append(currentRow, card)
			currentWidth += w
		}
	}
	if len(currentRow) > 0 {
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, currentRow...))
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m Model) renderFooter() string {
	contentWidth := m.width - FooterStyle.GetHorizontalFrameSize()
	if contentWidth < 10 {
		contentWidth = 10
	}

	if m.visualMode {
		modeLabel := "VISUAL"
		if m.visualLine {
			modeLabel = "VISUAL LINE"
		}
		modeTag := ModeTagVisual.Render(modeLabel)
		hints := []string{
			KeyHintKey.Render("y") + " " + KeyHintLabel.Render("Yank"),
			KeyHintKey.Render("Esc") + " " + KeyHintLabel.Render("Cancelar"),
			KeyHintKey.Render("h/j/k/l") + " " + KeyHintLabel.Render("Mover"),
			KeyHintKey.Render("w/b") + " " + KeyHintLabel.Render("Palabras"),
			KeyHintKey.Render("gg/G") + " " + KeyHintLabel.Render("Inicio/Fin"),
		}
		leftSide := lipgloss.JoinHorizontal(lipgloss.Top, modeTag, "  ", strings.Join(hints, "  "))
		quitHint := KeyHintKey.Render("q") + " " + KeyHintLabel.Render("Salir")

		space := contentWidth - lipgloss.Width(leftSide) - lipgloss.Width(quitHint)
		var bar string
		if space >= 1 {
			bar = lipgloss.JoinHorizontal(lipgloss.Top, leftSide, strings.Repeat(" ", space), quitHint)
		} else {
			bar = leftSide
		}
		rawFooter := FooterStyle.Width(m.width).Render(bar)
		return PaintLine(rawFooter, "#09090d", m.width)
	}

	modeTag := ModeTagNormal.Render("NORMAL")

	var hints []string
	hints = append(hints, KeyHintKey.Render("b")+" "+KeyHintLabel.Render("Sidebar"))

	svcLabel := "Servicios"
	if len(m.mgr.Services) > 0 {
		svcKey := fmt.Sprintf("Tab / 1..%d", len(m.mgr.Services))
		hints = append(hints, KeyHintKey.Render(svcKey)+" "+KeyHintLabel.Render(svcLabel))
	} else {
		hints = append(hints, KeyHintKey.Render("Tab")+" "+KeyHintLabel.Render(svcLabel))
	}

	if m.gitInfo != nil && m.gitInfo.IsGit {
		hints = append(hints, KeyHintKey.Render("w")+" "+KeyHintLabel.Render("Git Worktrees"))
	}
	hints = append(hints, KeyHintKey.Render("r")+" "+KeyHintLabel.Render("Restart"))
	hints = append(hints, KeyHintKey.Render("s")+" "+KeyHintLabel.Render("Stop/Start"))
	hints = append(hints, KeyHintKey.Render("v")+" "+KeyHintLabel.Render("Copy mode"))

	quitHint := KeyHintKey.Render("q") + " " + KeyHintLabel.Render("Salir")

	var rightParts []string
	if m.yankFeedback != "" {
		rightParts = append(rightParts, lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true).Render(m.yankFeedback))
	}
	if !m.autoScroll {
		rightParts = append(rightParts, AutoScrollPaused.Render("[PAUSED]"))
	}
	rightParts = append(rightParts, quitHint)
	rightSide := strings.Join(rightParts, "  ")

	// Progressive trimming if screen is narrow so footer NEVER wraps
	for len(hints) > 2 {
		leftSide := lipgloss.JoinHorizontal(lipgloss.Top, modeTag, "  ", strings.Join(hints, "  "))
		needed := lipgloss.Width(leftSide) + lipgloss.Width(rightSide) + 1
		if needed <= contentWidth {
			break
		}
		hints = hints[:len(hints)-1]
	}

	leftSide := lipgloss.JoinHorizontal(lipgloss.Top, modeTag, "  ", strings.Join(hints, "  "))
	space := contentWidth - lipgloss.Width(leftSide) - lipgloss.Width(rightSide)
	if space < 1 {
		space = 1
	}

	bar := lipgloss.JoinHorizontal(lipgloss.Top, leftSide, strings.Repeat(" ", space), rightSide)
	rawFooter := FooterStyle.Width(m.width).Render(bar)
	return PaintLine(rawFooter, "#09090d", m.width)
}
