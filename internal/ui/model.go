package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/juniorcorzo/dev-tui/internal/config"
	"github.com/juniorcorzo/dev-tui/internal/git"
	"github.com/juniorcorzo/dev-tui/internal/service"
)

type tickMsg time.Time
type logBatchMsg []service.LogEntry
type statusUpdateMsg struct{}

type Model struct {
	cfg                 *config.Config
	mgr                 *service.Manager
	rootDir             string
	gitInfo             *git.GitInfo
	showWorktreeModal   bool
	selectedWorktreeIdx int
	activeTab           int
	viewport            viewport.Model
	autoScroll          bool
	width               int
	height              int
	ready               bool
	quitting            bool
}

func NewModel(cfg *config.Config, mgr *service.Manager) Model {
	rootDir := mgr.GetRootDir()
	gitInfo, _ := git.GetInfo(rootDir)

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

		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			m.mgr.StopAll()
			return m, tea.Quit

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
				m.mgr.RestartAll()
			} else {
				svcIdx := m.activeTab - 1
				if svcIdx < len(m.mgr.Services) {
					go func(id int) {
						_ = m.mgr.RestartService(id)
					}(svcIdx)
				}
			}

		case "R":
			m.mgr.RestartAll()

		case "s":
			if m.activeTab == 0 {
				anyRunning := false
				for _, s := range m.mgr.Services {
					if s.GetStatus() == service.StatusRunning {
						anyRunning = true
						break
					}
				}
				if anyRunning {
					m.mgr.StopAll()
				} else {
					m.mgr.StartAll()
				}
			} else {
				svcIdx := m.activeTab - 1
				if svcIdx < len(m.mgr.Services) {
					s := m.mgr.Services[svcIdx]
					if s.GetStatus() == service.StatusRunning || s.GetStatus() == service.StatusStarting {
						_ = m.mgr.StopService(svcIdx)
					} else {
						_ = m.mgr.StartService(svcIdx)
					}
				}
			}

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
		headerHeight := 6
		footerHeight := 2
		viewportHeight := m.height - headerHeight - footerHeight
		if viewportHeight < 5 {
			viewportHeight = 5
		}

		if !m.ready {
			m.viewport = viewport.New(msg.Width, viewportHeight)
			m.viewport.YPosition = headerHeight
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = viewportHeight
		}
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
	}

	return m, tea.Batch(cmds...)
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

	m.mgr.AppendAllLog(service.LogEntry{
		Timestamp:   time.Now(),
		ServiceID:   -1,
		ServiceName: "git",
		Text:        "Configuration and services reloaded for workspace",
	})

	m.refreshViewportContent()
}

func (m *Model) refreshViewportContent() {
	var logs []service.LogEntry
	if m.activeTab == 0 {
		logs = m.mgr.GetAllLogs()
	} else {
		svcIdx := m.activeTab - 1
		if svcIdx < len(m.mgr.Services) {
			logs = m.mgr.Services[svcIdx].GetLogs()
		}
	}

	var sb strings.Builder
	for _, entry := range logs {
		timeStr := entry.Timestamp.Format("15:04:05")
		timePrefix := lipgloss.NewStyle().Foreground(ColorMuted).Render(timeStr)

		tagColor := ServiceColor(service.CleanServiceName(entry.ServiceName))
		tag := lipgloss.NewStyle().
			Foreground(tagColor).
			Bold(true).
			Render(fmt.Sprintf("[%s]", strings.ToLower(entry.ServiceName)))

		sb.WriteString(fmt.Sprintf("%s %s %s\n", timePrefix, tag, entry.Text))
	}

	m.viewport.SetContent(sb.String())
	if m.autoScroll {
		m.viewport.GotoBottom()
	}
}

func (m Model) View() string {
	if !m.ready {
		return "\n  Initializing dev-tui..."
	}
	if m.quitting {
		return "\n  Stopping all services and quitting...\n"
	}

	header := m.renderHeader()
	cards := m.renderCards()

	var mainView string
	if m.showWorktreeModal {
		mainView = m.renderWorktreeModal()
	} else {
		mainView = m.viewport.View()
	}

	footer := m.renderFooter()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		cards,
		mainView,
		footer,
	)
}

func (m Model) renderHeader() string {
	title := TitleStyle.Render(fmt.Sprintf(" ⚡ %s DEV TUI ", strings.ToUpper(m.cfg.Title)))
	now := ClockStyle.Render(time.Now().Format("2006-01-02 15:04:05"))

	var gitBadge string
	if m.gitInfo != nil && m.gitInfo.IsGit {
		var parts []string
		if m.gitInfo.IsLinked {
			parts = append(parts, GitWorktreeBadge.Render(fmt.Sprintf("⎇ wt:%s", m.gitInfo.WorktreeName)))
			parts = append(parts, GitBranchBadge.Render(fmt.Sprintf("[🌿 %s]", m.gitInfo.Branch)))
		} else {
			parts = append(parts, GitBranchBadge.Render(fmt.Sprintf("🌿 %s", m.gitInfo.Branch)))
		}

		if m.gitInfo.Commit != "" {
			parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("#888899")).Render("("+m.gitInfo.Commit+")"))
		}

		if m.gitInfo.Dirty {
			parts = append(parts, GitDirtyBadge.Render("*"))
		} else {
			parts = append(parts, GitCleanBadge.Render("✔"))
		}

		if len(m.gitInfo.Worktrees) > 1 {
			parts = append(parts, lipgloss.NewStyle().Foreground(ColorCyan).Render(fmt.Sprintf("[%d wt]", len(m.gitInfo.Worktrees))))
		}

		gitBadge = strings.Join(parts, " ")
	}

	left := title
	if gitBadge != "" {
		left = lipgloss.JoinHorizontal(lipgloss.Top, title, "  ", gitBadge)
	}

	space := m.width - lipgloss.Width(left) - lipgloss.Width(now) - 2
	if space < 1 {
		space = 1
	}

	bar := lipgloss.JoinHorizontal(
		lipgloss.Top,
		left,
		strings.Repeat(" ", space),
		now,
	)

	return HeaderStyle.Width(m.width).Render(bar)
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
	if len(m.gitInfo.Worktrees) == 0 {
		sb.WriteString(ModalDim.Render("  (No additional worktrees found. Create one with: git worktree add <dir> -b <branch>)\n"))
	} else {
		for i, wt := range m.gitInfo.Worktrees {
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
	}

	sb.WriteString("\n" + ModalDim.Render("Keys: [↑/↓/k/j] Navigate  •  [Enter] Switch Worktree Workspace  •  [Esc/w] Close"))

	return ModalContainer.Width(m.width - 4).Height(m.viewport.Height).Render(sb.String())
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
			icon := "🌐"
			if s.Type == config.TypeCompose {
				icon = "🐳"
			}
			portStr = fmt.Sprintf("%s :%d", icon, s.Port)
		}

		cardContent := fmt.Sprintf("[%d] %s\n%s  %s\nUp: %s",
			i+1,
			strings.ToUpper(s.Name),
			statusStr,
			portStr,
			s.GetUptime(),
		)

		cardViews = append(cardViews, style.Render(cardContent))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, cardViews...)
}

func (m Model) renderFooter() string {
	autoScrollStr := AutoScrollActive.Render("[AUTO-SCROLL ON]")
	if !m.autoScroll {
		autoScrollStr = AutoScrollPaused.Render("[SCROLL PAUSED]")
	}

	wtKey := ""
	if m.gitInfo != nil && m.gitInfo.IsGit {
		wtKey = fmt.Sprintf(" %s Worktrees ", KeyStyle.Render("[w]"))
	}

	help := fmt.Sprintf(
		"%s Switch  %s Select  %s Restart  %s Restart All  %s Start/Stop %s %s Clear  %s Auto-scroll  %s Quit  |  %s",
		KeyStyle.Render("[Tab]"),
		KeyStyle.Render("[0-9]"),
		KeyStyle.Render("[r]"),
		KeyStyle.Render("[R]"),
		KeyStyle.Render("[s]"),
		wtKey,
		KeyStyle.Render("[c]"),
		KeyStyle.Render("[a]"),
		KeyStyle.Render("[q]"),
		autoScrollStr,
	)

	return FooterStyle.Width(m.width).Render(help)
}
