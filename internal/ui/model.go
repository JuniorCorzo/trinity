package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/juniorcorzo/dev-tui/internal/config"
	"github.com/juniorcorzo/dev-tui/internal/service"
)

type tickMsg time.Time
type logBatchMsg []service.LogEntry
type statusUpdateMsg struct{}

type Model struct {
	cfg        *config.Config
	mgr        *service.Manager
	activeTab  int
	viewport   viewport.Model
	autoScroll bool
	width      int
	height     int
	ready      bool
	quitting   bool
}

func NewModel(cfg *config.Config, mgr *service.Manager) Model {
	return Model{
		cfg:        cfg,
		mgr:        mgr,
		activeTab:  0,
		autoScroll: true,
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
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			m.mgr.StopAll()
			return m, tea.Quit

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
	viewportContent := m.viewport.View()
	footer := m.renderFooter()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		cards,
		viewportContent,
		footer,
	)
}

func (m Model) renderHeader() string {
	title := TitleStyle.Render(fmt.Sprintf(" ⚡ %s DEV TUI ", strings.ToUpper(m.cfg.Title)))
	now := ClockStyle.Render(time.Now().Format("2006-01-02 15:04:05"))

	space := m.width - lipgloss.Width(title) - lipgloss.Width(now) - 2
	if space < 1 {
		space = 1
	}

	bar := lipgloss.JoinHorizontal(
		lipgloss.Top,
		title,
		strings.Repeat(" ", space),
		now,
	)

	return HeaderStyle.Width(m.width).Render(bar)
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

	help := fmt.Sprintf(
		"%s Switch  %s Select  %s Restart  %s Restart All  %s Start/Stop  %s Clear  %s Auto-scroll  %s Quit  |  %s",
		KeyStyle.Render("[Tab]"),
		KeyStyle.Render("[0-9]"),
		KeyStyle.Render("[r]"),
		KeyStyle.Render("[R]"),
		KeyStyle.Render("[s]"),
		KeyStyle.Render("[c]"),
		KeyStyle.Render("[a]"),
		KeyStyle.Render("[q]"),
		autoScrollStr,
	)

	return FooterStyle.Width(m.width).Render(help)
}
