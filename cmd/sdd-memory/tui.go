package main

import (
	"fmt"
	"strings"

	"github.com/KevG1t/sdd-memory/internal/store"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Claude Style Colors
var (
	claudeOrange = lipgloss.Color("#FF6B35")
	claudeBlue   = lipgloss.Color("#4A90E2")
	claudeGray   = lipgloss.Color("#8E8E93")
	claudeDark   = lipgloss.Color("#2C2C2E")
)

// Claude-styled components
var (
	activeTabStyle    = lipgloss.NewStyle().Foreground(claudeOrange).Bold(true).Border(lipgloss.NormalBorder(), false, false, true, false).BorderForeground(claudeOrange).Padding(0, 1)
	inactiveTabStyle  = lipgloss.NewStyle().Foreground(claudeGray).Padding(0, 1)
	titleStyle        = lipgloss.NewStyle().Foreground(claudeBlue).Bold(true).MarginBottom(1)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(2).Foreground(claudeDark)
	selectedItemStyle = lipgloss.NewStyle().Foreground(claudeOrange).PaddingLeft(0).Bold(true)
	helpStyle         = lipgloss.NewStyle().Foreground(claudeGray).MarginTop(1)
	headerStyle       = lipgloss.NewStyle().Foreground(claudeBlue).Bold(true).MarginBottom(1).MarginTop(1)
	logoStyle         = lipgloss.NewStyle().Foreground(claudeOrange).Bold(true)
	subtitleStyle     = lipgloss.NewStyle().Foreground(claudeGray)

	// Observation rendering (Engram-aligned formatting)
	idStyle             = lipgloss.NewStyle().Foreground(claudeGray)
	typeBadgeStyle      = lipgloss.NewStyle().Foreground(claudeBlue).Bold(true)
	projectStyle        = lipgloss.NewStyle().Foreground(claudeGray).Italic(true)
	timestampStyle      = lipgloss.NewStyle().Foreground(claudeGray)
	contentPreviewStyle = lipgloss.NewStyle().PaddingLeft(4).Foreground(claudeGray)
	detailLabelStyle    = lipgloss.NewStyle().Foreground(claudeBlue).Bold(true)
	detailValueStyle    = lipgloss.NewStyle().Foreground(claudeDark)
	sectionHeadingStyle = lipgloss.NewStyle().Foreground(claudeOrange).Bold(true).MarginTop(1)
)

type viewState int

const (
	viewTabs viewState = iota
	viewSessionObservations
	viewObservationDetail
)

type tuiModel struct {
	store        *store.LocalStore
	activeTab    int
	cursor       int
	stats        *store.Stats
	observations []store.Observation
	sessions     []store.SessionSummary
	tabs         []string

	searchInput   textinput.Model
	searchResults []store.Observation
	searchCursor  int
	searchError   error
	searchDone    bool

	state               viewState
	sessionCursor       int
	sessionObservations []store.Observation
	selectedSession     *store.SessionSummary

	selectedObservation *store.Observation
	viewport            viewport.Model
	ready               bool
}

func initialModel(s *store.LocalStore) tuiModel {
	st, _ := s.Stats()
	obs, _ := s.RecentObservations(15)
	sess, _ := s.RecentSessions(15)

	ti := textinput.New()
	ti.Placeholder = "Type to search memories..."
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 40

	vp := viewport.New(80, 20)

	return tuiModel{
		store:        s,
		activeTab:    0,
		tabs:         []string{"Dashboard", "Search", "Observations", "Sessions", "Setup"},
		stats:        st,
		observations: obs,
		sessions:     sess,
		searchInput:  ti,
		state:        viewTabs,
		viewport:     vp,
	}
}

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(tea.EnterAltScreen, textinput.Blink)
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		headerHeight := 4
		footerHeight := 4
		verticalMarginHeight := headerHeight + footerHeight

		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - verticalMarginHeight
		m.ready = true

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc:
			if m.state == viewObservationDetail {
				if m.selectedSession != nil {
					m.state = viewSessionObservations
				} else {
					m.state = viewTabs
				}
				return m, nil
			} else if m.state == viewSessionObservations {
				m.state = viewTabs
				m.selectedSession = nil
				return m, nil
			}
			return m, tea.Quit
		case tea.KeyTab:
			if m.state == viewTabs {
				m.activeTab = (m.activeTab + 1) % len(m.tabs)
				m.cursor = 0
				m.searchCursor = 0
			}
			return m, nil
		case tea.KeyShiftTab:
			if m.state == viewTabs {
				m.activeTab = (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
				m.cursor = 0
				m.searchCursor = 0
			}
			return m, nil
		case tea.KeyEnter:
			if m.state == viewTabs {
				if m.activeTab == 1 { // Search
					if m.searchInput.Focused() && m.searchInput.Value() != "" {
						res, err := m.store.SearchObservations(m.searchInput.Value(), "")
						m.searchResults = res
						m.searchError = err
						m.searchDone = true
						m.searchCursor = 0
						m.searchInput.Blur()
					} else if len(m.searchResults) > 0 {
						if m.searchCursor < len(m.searchResults) {
							m.selectedObservation = &m.searchResults[m.searchCursor]
							m.state = viewObservationDetail
							m.selectedSession = nil
							m.viewport.SetContent(formatObservation(*m.selectedObservation))
							m.viewport.GotoTop()
						}
					}
				} else if m.activeTab == 2 { // Observations
					if len(m.observations) > 0 {
						m.selectedObservation = &m.observations[m.cursor]
						m.state = viewObservationDetail
						m.selectedSession = nil
						m.viewport.SetContent(formatObservation(*m.selectedObservation))
						m.viewport.GotoTop()
					}
				} else if m.activeTab == 3 { // Sessions
					if len(m.sessions) > 0 {
						m.selectedSession = &m.sessions[m.cursor]
						obs, _ := m.store.ObservationsBySession(m.selectedSession.ID)
						m.sessionObservations = obs
						m.sessionCursor = 0
						m.state = viewSessionObservations
					}
				}
			} else if m.state == viewSessionObservations {
				if len(m.sessionObservations) > 0 {
					m.selectedObservation = &m.sessionObservations[m.sessionCursor]
					m.state = viewObservationDetail
					m.viewport.SetContent(formatObservation(*m.selectedObservation))
					m.viewport.GotoTop()
				}
			}
		}

		// Handle q to quit or back
		if msg.String() == "q" && msg.Type != tea.KeyEsc && msg.Type != tea.KeyCtrlC && msg.Type != tea.KeyEnter {
			if m.state == viewTabs && (m.activeTab != 1 || !m.searchInput.Focused()) {
				return m, tea.Quit
			} else if m.state != viewTabs {
				if m.state == viewObservationDetail {
					if m.selectedSession != nil {
						m.state = viewSessionObservations
					} else {
						m.state = viewTabs
					}
					return m, nil
				} else if m.state == viewSessionObservations {
					m.state = viewTabs
					m.selectedSession = nil
					return m, nil
				}
			}
		}

		// Focus search input when typing in search tab and not focused
		if m.state == viewTabs && m.activeTab == 1 && !m.searchInput.Focused() {
			if msg.Type == tea.KeyRunes || msg.Type == tea.KeyBackspace {
				m.searchInput.Focus()
				// Reset results on new search
				if msg.Type == tea.KeyRunes || msg.Type == tea.KeyBackspace {
					m.searchDone = false
				}
			}
		}

		// Handle j/k up/down/left/right only if we are NOT focusing text input
		if m.state == viewTabs && m.activeTab != 1 {
			switch msg.String() {
			case "right", "l":
				m.activeTab = (m.activeTab + 1) % len(m.tabs)
				m.cursor = 0
			case "left", "h":
				m.activeTab = (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
				m.cursor = 0
			case "down", "j":
				max := 0
				if m.activeTab == 2 {
					max = len(m.observations) - 1
				} else if m.activeTab == 3 {
					max = len(m.sessions) - 1
				}
				if m.cursor < max {
					m.cursor++
				}
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			}
		} else if m.state == viewTabs && m.activeTab == 1 && !m.searchInput.Focused() {
			switch msg.String() {
			case "right", "l":
				m.activeTab = (m.activeTab + 1) % len(m.tabs)
				m.cursor = 0
			case "left", "h":
				m.activeTab = (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
				m.cursor = 0
			case "down", "j":
				if m.searchCursor < len(m.searchResults)-1 {
					m.searchCursor++
				}
			case "up", "k":
				if m.searchCursor > 0 {
					m.searchCursor--
				}
			}
		} else if m.state == viewSessionObservations {
			switch msg.String() {
			case "down", "j":
				if m.sessionCursor < len(m.sessionObservations)-1 {
					m.sessionCursor++
				}
			case "up", "k":
				if m.sessionCursor > 0 {
					m.sessionCursor--
				}
			}
		}
	}

	// Always update textinput if we are on search tab and in viewTabs state
	if m.state == viewTabs && m.activeTab == 1 {
		oldVal := m.searchInput.Value()
		m.searchInput, cmd = m.searchInput.Update(msg)
		if m.searchInput.Value() != oldVal {
			m.searchDone = false
		}
		cmds = append(cmds, cmd)
	}

	if m.state == viewObservationDetail {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m tuiModel) View() string {
	if !m.ready {
		// Only show initializing if we don't have dimensions for viewport, but we default to 80x20
		// so it should be fine. We just check if state is viewObservationDetail
	}

	var b strings.Builder

	if m.state == viewTabs {
		// Render tabs
		var tabs []string
		for i, t := range m.tabs {
			if i == m.activeTab {
				tabs = append(tabs, activeTabStyle.Render(t))
			} else {
				tabs = append(tabs, inactiveTabStyle.Render(t))
			}
		}
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, tabs...))
		b.WriteString("\n\n")

		// Render content based on active tab
		switch m.activeTab {
		case 0:
			b.WriteString(m.viewDashboard())
		case 1:
			b.WriteString(m.viewSearch())
		case 2:
			b.WriteString(m.viewObservations())
		case 3:
			b.WriteString(m.viewSessions())
		case 4:
			b.WriteString(m.viewSetup())
		}
		b.WriteString(helpStyle.Render("\n\n(tab/shift+tab) change tab • (enter) select • (q/esc) quit"))
	} else if m.state == viewSessionObservations {
		b.WriteString(titleStyle.Render(fmt.Sprintf("Observations for Session: %s", m.selectedSession.Project)))
		b.WriteString("\n\n")

		if len(m.sessionObservations) == 0 {
			b.WriteString("No observations found for this session.")
		} else {
			for i, o := range m.sessionObservations {
				b.WriteString(renderObservationListItem(o, m.sessionCursor == i, false))
			}
		}
		b.WriteString(helpStyle.Render("\n\n(enter) view • (q/esc) back"))
	} else if m.state == viewObservationDetail {
		b.WriteString(titleStyle.Render(fmt.Sprintf("Observation Detail")))
		b.WriteString("\n")
		b.WriteString(m.viewport.View())
		b.WriteString(helpStyle.Render("\n\n(up/down/j/k) scroll • (q/esc) back"))
	}

	return b.String()
}

func formatObservation(o store.Observation) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("%s %s\n", detailLabelStyle.Render("ID:     "), idStyle.Render(o.ID)))
	b.WriteString(fmt.Sprintf("%s %s\n", detailLabelStyle.Render("Type:   "), typeBadgeStyle.Render(o.Type)))
	b.WriteString(fmt.Sprintf("%s %s\n", detailLabelStyle.Render("Title:  "), detailValueStyle.Bold(true).Render(displayTitle(o))))
	b.WriteString(fmt.Sprintf("%s %s\n", detailLabelStyle.Render("Topic:  "), detailValueStyle.Render(o.TopicKey)))
	b.WriteString(fmt.Sprintf("%s %s\n", detailLabelStyle.Render("Project:"), projectStyle.Render(o.Project)))
	b.WriteString(fmt.Sprintf("%s %s\n", detailLabelStyle.Render("Scope:  "), detailValueStyle.Render(o.Scope)))
	b.WriteString(fmt.Sprintf("%s %s\n", detailLabelStyle.Render("Updated:"), timestampStyle.Render(o.UpdatedAt.Local().Format("2006-01-02 15:04:05"))))

	b.WriteString(sectionHeadingStyle.Render("Content"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(claudeDark).Render(o.Content))
	return b.String()
}

// renderObservationListItem renders one observation as a two-line entry,
// mirroring Engram's list format:
//
//	▸ obs-1a2b      [type        ] Title  project  2006-01-02 15:04:05
//	    content preview...
func renderObservationListItem(o store.Observation, selected, showProject bool) string {
	cursor := "  "
	style := itemStyle
	if selected {
		cursor = "▸ "
		style = selectedItemStyle
	}

	proj := ""
	if showProject && o.Project != "" {
		proj = "  " + projectStyle.Render(o.Project)
	}

	line := fmt.Sprintf("%s%s %s %s%s  %s\n",
		cursor,
		idStyle.Render(fmt.Sprintf("%-12s", o.ID)),
		typeBadgeStyle.Render(fmt.Sprintf("[%-12s]", o.Type)),
		style.Render(truncate(displayTitle(o), 50)),
		proj,
		timestampStyle.Render(o.CreatedAt.Local().Format("2006-01-02 15:04:05")))

	preview := truncate(strings.ReplaceAll(o.Content, "\n", " "), 80)
	if preview != "" {
		line += contentPreviewStyle.Render(preview) + "\n"
	}
	return line
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}

func (m tuiModel) viewDashboard() string {
	if m.stats == nil {
		return "Loading..."
	}
	var b strings.Builder

	// Claude-style logo
	logo := `
   ╭─────────────────────────────────────────╮
   │  ███████ ██████  ██████      ███    ███ │
   │  ██      ██   ██ ██   ██     ████  ████ │
   │  ███████ ██   ██ ██   ██     ██ ████ ██ │
   │       ██ ██   ██ ██   ██     ██  ██  ██ │
   │  ███████ ██████  ██████      ██      ██ │
   ╰─────────────────────────────────────────╯
   `
	b.WriteString(logoStyle.Render(logo))
	b.WriteString("\n")
	b.WriteString(titleStyle.Render("🧠 Smart Development Documentation Memory"))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("Powered by Claude AI"))

	b.WriteString(fmt.Sprintf("\n\n  Sessions: %d", m.stats.TotalSessions))
	b.WriteString(fmt.Sprintf("\n  Observations: %d", m.stats.TotalObservations))
	b.WriteString(fmt.Sprintf("\n  Total User Prompts: %d", m.stats.TotalPrompts))

	b.WriteString("\n" + headerStyle.Render("Projects"))
	if len(m.stats.Projects) == 0 {
		b.WriteString("\n  No projects found.")
	} else {
		for _, p := range m.stats.Projects {
			b.WriteString(fmt.Sprintf("\n  • %s", p))
		}
	}

	return b.String()
}

func (m tuiModel) viewSearch() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("🔍 Search Memories\n"))
	b.WriteString(m.searchInput.View() + "\n\n")

	if m.searchError != nil {
		b.WriteString("Error: " + m.searchError.Error())
		return b.String()
	}

	if len(m.searchResults) > 0 {
		b.WriteString(fmt.Sprintf("Found %d results:\n\n", len(m.searchResults)))
		for i, o := range m.searchResults {
			selected := m.searchCursor == i && !m.searchInput.Focused()
			b.WriteString(renderObservationListItem(o, selected, true))
		}
	} else if m.searchDone {
		b.WriteString("No results found for your query.")
	} else if m.searchInput.Value() != "" {
		b.WriteString("Press Enter to search.")
	}
	return b.String()
}

func (m tuiModel) viewObservations() string {
	if len(m.observations) == 0 {
		return "No observations yet."
	}
	var s strings.Builder
	s.WriteString(titleStyle.Render("📝 Recent Observations") + "\n")

	for i, o := range m.observations {
		s.WriteString(renderObservationListItem(o, m.cursor == i, true))
	}
	return s.String()
}

func (m tuiModel) viewSessions() string {
	if len(m.sessions) == 0 {
		return "No sessions yet."
	}
	var s strings.Builder
	s.WriteString(titleStyle.Render("🔄 Recent Sessions") + "\n")

	for i, sess := range m.sessions {
		cursor := "  "
		style := itemStyle
		if m.cursor == i {
			cursor = "▸ "
			style = selectedItemStyle
		}

		summary := ""
		if sess.Summary != nil {
			summary = truncate(strings.ReplaceAll(*sess.Summary, "\n", " "), 50)
		}

		s.WriteString(fmt.Sprintf("%s%s  %s  %s obs  %s\n",
			cursor,
			projectStyle.Render(fmt.Sprintf("%-20s", sess.Project)),
			timestampStyle.Render(sess.StartedAt.Local().Format("2006-01-02 15:04:05")),
			typeBadgeStyle.Render(fmt.Sprintf("%d", sess.ObservationCount)),
			style.Render(summary)))
	}
	return s.String()
}

func (m tuiModel) viewSetup() string {
	return titleStyle.Render("⚙️  Configuración de Servidor MCP") + "\n\n" +
		"Para conectar sdd-memory a tu IDE o Agentes (Cursor, Antigravity, etc.),\n" +
		"agregá esta configuración en tu mcp_config.json:\n\n" +
		"\"sdd-memory\": {\n" +
		"  \"command\": \"sdd-memory\",\n" +
		"  \"args\": [\"mcp\"]\n" +
		"}\n\n" +
		subtitleStyle.Render("Nota: Gracias a 'go install', el comando ya está en tu PATH global.")
}

// displayTitle resolves the best human-readable label for an observation:
// the explicit Title field, then a "What:" line in the content, then the
// topic_key as a last resort.
func displayTitle(o store.Observation) string {
	if strings.TrimSpace(o.Title) != "" {
		return o.Title
	}
	lines := strings.Split(o.Content, "\n")
	for _, l := range lines {
		if strings.HasPrefix(l, "**What**:") || strings.HasPrefix(l, "What:") {
			return strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(l, "**What**:"), "What:"))
		}
	}
	return o.TopicKey
}
