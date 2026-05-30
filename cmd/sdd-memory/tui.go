package main

import (
	"fmt"
	"strings"

	"sdd-memory/internal/store"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	activeTabStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("62")).Bold(true).Border(lipgloss.NormalBorder(), false, false, true, false).BorderForeground(lipgloss.Color("62")).Padding(0, 1)
	inactiveTabStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Padding(0, 1)
	titleStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("62")).Bold(true).MarginBottom(1)
	itemStyle        = lipgloss.NewStyle().PaddingLeft(2)
	selectedItemStyle= lipgloss.NewStyle().Foreground(lipgloss.Color("62")).PaddingLeft(0)
	helpStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginTop(1)
	headerStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).MarginBottom(1).MarginTop(1)
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
	searchError   error
	searchDone    bool
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

	return tuiModel{
		store:        s,
		activeTab:    0,
		tabs:         []string{"Dashboard", "Search", "Observations", "Sessions", "Setup"},
		stats:        st,
		observations: obs,
		sessions:     sess,
		searchInput:  ti,
	}
}

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(tea.EnterAltScreen, textinput.Blink)
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyTab:
			m.activeTab = (m.activeTab + 1) % len(m.tabs)
			m.cursor = 0
			return m, nil
		case tea.KeyShiftTab:
			m.activeTab = (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
			m.cursor = 0
			return m, nil
		case tea.KeyEnter:
			if m.activeTab == 1 && m.searchInput.Value() != "" {
				res, err := m.store.SearchObservations(m.searchInput.Value(), "")
				m.searchResults = res
				m.searchError = err
				m.searchDone = true
			}
		}

		// Only handle j/k up/down if we are NOT in the search input
		if m.activeTab != 1 {
			switch msg.String() {
			case "q":
				return m, tea.Quit
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
		}
	}

	// Always update textinput if we are on search tab
	if m.activeTab == 1 {
		// Only reset searchDone if typing changes the value, but let textinput handle msg first
		oldVal := m.searchInput.Value()
		m.searchInput, cmd = m.searchInput.Update(msg)
		if m.searchInput.Value() != oldVal {
			m.searchDone = false
		}
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m tuiModel) View() string {
	var b strings.Builder

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

	b.WriteString(helpStyle.Render("\n\n(tab/shift+tab) change tab • (q/esc) quit"))
	return b.String()
}

func (m tuiModel) viewDashboard() string {
	if m.stats == nil {
		return "Loading..."
	}
	var b strings.Builder
	
	b.WriteString(titleStyle.Render("🐘 SDD Memory Lite Dashboard\n"))
	
	b.WriteString(fmt.Sprintf("\n  Sessions: %d", m.stats.TotalSessions))
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
	b.WriteString(titleStyle.Render("Search Memories\n"))
	b.WriteString(m.searchInput.View() + "\n\n")
	
	if m.searchError != nil {
		b.WriteString("Error: " + m.searchError.Error())
		return b.String()
	}
	
	if len(m.searchResults) > 0 {
		b.WriteString(fmt.Sprintf("Found %d results:\n\n", len(m.searchResults)))
		for _, o := range m.searchResults {
			title := getTitle(o)
			b.WriteString(fmt.Sprintf("• [%s] %s (Proj: %s)\n", o.Topic, title, o.Project))
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
	s.WriteString(titleStyle.Render("Recent Observations") + "\n")
	
	for i, o := range m.observations {
		cursor := " "
		style := itemStyle
		if m.cursor == i {
			cursor = "▸"
			style = selectedItemStyle
		}
		
		title := getTitle(o)
		s.WriteString(style.Render(fmt.Sprintf("%s [%s] %s (Proj: %s)", cursor, o.Topic, title, o.Project)) + "\n")
	}
	return s.String()
}

func (m tuiModel) viewSessions() string {
	if len(m.sessions) == 0 {
		return "No sessions yet."
	}
	var s strings.Builder
	s.WriteString(titleStyle.Render("Recent Sessions") + "\n")

	for i, sess := range m.sessions {
		cursor := " "
		style := itemStyle
		if m.cursor == i {
			cursor = "▸"
			style = selectedItemStyle
		}
		
		summary := "No summary"
		if sess.Summary != nil {
			summary = *sess.Summary
			summary = strings.ReplaceAll(summary, "\n", " ")
			if len(summary) > 40 {
				summary = summary[:37] + "..."
			}
		}

		s.WriteString(style.Render(fmt.Sprintf("%s [%s] %s | Obs: %d | %s", cursor, sess.Project, sess.StartedAt.Local().Format("2006-01-02 15:04"), sess.ObservationCount, summary)) + "\n")
	}
	return s.String()
}

func (m tuiModel) viewSetup() string {
	return titleStyle.Render("Setup Agent Plugin") + "\n\n" +
		"To use SDD Memory Lite with Cursor, add this to your Cursor settings:\n\n" +
		"\"sdd-memory\": {\n" +
		"  \"command\": \"path/to/sdd-memory.exe\",\n" +
		"  \"args\": [\"mcp\"]\n" +
		"}"
}

// Helper to extract a title if available
func getTitle(o store.Observation) string {
	lines := strings.Split(o.Content, "\n")
	for _, l := range lines {
		if strings.HasPrefix(l, "**What**:") || strings.HasPrefix(l, "What:") {
			return strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(l, "**What**:"), "What:"))
		}
	}
	return o.Topic
}
