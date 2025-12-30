package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/mcp"
)

var (
	appStyle = lipgloss.NewStyle().Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#A49FA5", Dark: "#777777"}).
			Padding(0, 1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#A49FA5", Dark: "#777777"}).
			Padding(1, 0)
)

// Item represents a list item in the TUI
type Item struct {
	name        string
	description string
	status      string
	server      *mcp.Server
}

func (i Item) Title() string       { return i.name }
func (i Item) Description() string { return i.description }
func (i Item) FilterValue() string { return i.name }

// Model is the Bubble Tea model for the TUI
type Model struct {
	list     list.Model
	cfg      *config.Config
	keys     keyMap
	quitting bool
	err      error
}

type keyMap struct {
	Up     key.Binding
	Down   key.Binding
	Enter  key.Binding
	Delete key.Binding
	Sync   key.Binding
	Test   key.Binding
	Quit   key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "details"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
		Sync: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "sync"),
		),
		Test: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "test"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "esc", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

// New creates a new TUI model
func New() (*Model, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	items := []list.Item{}
	for name, server := range cfg.Servers {
		status := "enabled"
		if server.Disabled {
			status = "disabled"
		}

		desc := fmt.Sprintf("%s (%s)", server.Source.Type, server.Command)
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}

		items = append(items, Item{
			name:        name,
			description: desc,
			status:      status,
			server:      server,
		})
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true

	l := list.New(items, delegate, 0, 0)
	l.Title = "agentctl - MCP Servers"
	l.Styles.Title = titleStyle
	l.SetShowHelp(true)

	return &Model{
		list: l,
		cfg:  cfg,
		keys: newKeyMap(),
	}, nil
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := appStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.quitting = true
			return m, tea.Quit

		case key.Matches(msg, m.keys.Delete):
			if item, ok := m.list.SelectedItem().(Item); ok {
				return m, m.deleteServer(item.name)
			}

		case key.Matches(msg, m.keys.Sync):
			return m, m.syncAll()

		case key.Matches(msg, m.keys.Test):
			if item, ok := m.list.SelectedItem().(Item); ok {
				return m, m.testServer(item.name)
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View renders the UI
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder
	b.WriteString(m.list.View())

	helpText := "↑/↓: navigate • d: delete • s: sync all • t: test • q: quit"
	b.WriteString(helpStyle.Render(helpText))

	return appStyle.Render(b.String())
}

// Commands

type serverDeletedMsg struct {
	name string
}

type serverTestedMsg struct {
	name    string
	healthy bool
	err     error
}

type syncCompletedMsg struct {
	errors map[string]error
}

func (m *Model) deleteServer(name string) tea.Cmd {
	return func() tea.Msg {
		delete(m.cfg.Servers, name)
		if err := m.cfg.Save(); err != nil {
			return nil
		}
		return serverDeletedMsg{name: name}
	}
}

func (m *Model) testServer(name string) tea.Cmd {
	return func() tea.Msg {
		// Simplified test - just return success for now
		return serverTestedMsg{name: name, healthy: true}
	}
}

func (m *Model) syncAll() tea.Cmd {
	return func() tea.Msg {
		// Simplified sync - would actually call sync.All()
		return syncCompletedMsg{errors: make(map[string]error)}
	}
}

// Run starts the TUI application
func Run() error {
	m, err := New()
	if err != nil {
		return err
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}
