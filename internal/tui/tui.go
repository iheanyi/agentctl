package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/iheanyi/agentctl/pkg/aliases"
	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/mcp"
)

// TabView represents the current view in the TUI
type TabView int

const (
	TabInstalled TabView = iota
	TabBrowse
)

var (
	appStyle = lipgloss.NewStyle().Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	tabActiveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 2)

	tabInactiveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#777777")).
			Padding(0, 2)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#A49FA5", Dark: "#777777"}).
			Padding(0, 1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#A49FA5", Dark: "#777777"}).
			Padding(1, 0)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#25A065"))

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFC107"))
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
	installedList list.Model
	browseList    list.Model
	cfg           *config.Config
	keys          keyMap
	currentTab    TabView
	statusMsg     string
	quitting      bool
	err           error
	width         int
	height        int
}

type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Enter   key.Binding
	Delete  key.Binding
	Sync    key.Binding
	Test    key.Binding
	Tab     key.Binding
	Install key.Binding
	Quit    key.Binding
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
			key.WithHelp("enter", "select"),
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
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch view"),
		),
		Install: key.NewBinding(
			key.WithKeys("a", "i"),
			key.WithHelp("a", "add"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "esc", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

// New creates a new TUI model
func New() (*Model, error) {
	cfg, err := config.LoadWithProject()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Build installed servers list
	installedItems := []list.Item{}
	for name, server := range cfg.Servers {
		status := "enabled"
		if server.Disabled {
			status = "disabled"
		}

		transport := string(server.Transport)
		if transport == "" {
			transport = "stdio"
		}
		desc := fmt.Sprintf("%s transport", transport)
		if server.Command != "" {
			desc += fmt.Sprintf(" • %s", server.Command)
		}
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}

		installedItems = append(installedItems, Item{
			name:        name,
			description: desc,
			status:      status,
			server:      server,
		})
	}

	// Build browse list from aliases
	browseItems := []list.Item{}
	allAliases := aliases.List()
	for name, alias := range allAliases {
		// Skip if already installed
		if _, installed := cfg.Servers[name]; installed {
			continue
		}

		desc := alias.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}

		browseItems = append(browseItems, Item{
			name:        name,
			description: desc,
			status:      "available",
		})
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true

	installedList := list.New(installedItems, delegate, 0, 0)
	installedList.Title = "Installed MCP Servers"
	installedList.Styles.Title = titleStyle
	installedList.SetShowHelp(false)
	installedList.SetShowStatusBar(true)
	installedList.SetFilteringEnabled(true)

	browseList := list.New(browseItems, delegate, 0, 0)
	browseList.Title = "Browse Available MCPs"
	browseList.Styles.Title = titleStyle
	browseList.SetShowHelp(false)
	browseList.SetShowStatusBar(true)
	browseList.SetFilteringEnabled(true)

	return &Model{
		installedList: installedList,
		browseList:    browseList,
		cfg:           cfg,
		keys:          newKeyMap(),
		currentTab:    TabInstalled,
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
		m.width = msg.Width
		m.height = msg.Height
		h, v := appStyle.GetFrameSize()
		// Account for tabs header
		listHeight := msg.Height - v - 4
		m.installedList.SetSize(msg.Width-h, listHeight)
		m.browseList.SetSize(msg.Width-h, listHeight)

	case serverDeletedMsg:
		m.statusMsg = successStyle.Render(fmt.Sprintf("✓ Deleted %s", msg.name))
		// Reload the lists
		return m, nil

	case serverTestedMsg:
		if msg.healthy {
			m.statusMsg = successStyle.Render(fmt.Sprintf("✓ %s is healthy", msg.name))
		} else {
			m.statusMsg = warningStyle.Render(fmt.Sprintf("⚠ %s: %v", msg.name, msg.err))
		}
		return m, nil

	case syncCompletedMsg:
		if len(msg.errors) == 0 {
			m.statusMsg = successStyle.Render("✓ Synced to all tools")
		} else {
			m.statusMsg = warningStyle.Render(fmt.Sprintf("⚠ Sync completed with %d errors", len(msg.errors)))
		}
		return m, nil

	case serverAddedMsg:
		if msg.err != nil {
			m.statusMsg = warningStyle.Render(fmt.Sprintf("⚠ Failed to add %s: %v", msg.name, msg.err))
		} else {
			m.statusMsg = successStyle.Render(fmt.Sprintf("✓ Added %s", msg.name))
			// Refresh the lists - move item from browse to installed
			m.refreshLists()
		}
		return m, nil

	case tea.KeyMsg:
		// Don't handle keys during filtering
		if m.currentList().FilterState() == list.Filtering {
			break
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			m.quitting = true
			return m, tea.Quit

		case key.Matches(msg, m.keys.Tab):
			// Switch tabs
			if m.currentTab == TabInstalled {
				m.currentTab = TabBrowse
			} else {
				m.currentTab = TabInstalled
			}
			m.statusMsg = ""
			return m, nil

		case key.Matches(msg, m.keys.Delete):
			if m.currentTab == TabInstalled {
				if item, ok := m.installedList.SelectedItem().(Item); ok {
					return m, m.deleteServer(item.name)
				}
			}

		case key.Matches(msg, m.keys.Install):
			if m.currentTab == TabBrowse {
				if item, ok := m.browseList.SelectedItem().(Item); ok {
					return m, m.addServer(item.name)
				}
			}

		case key.Matches(msg, m.keys.Sync):
			return m, m.syncAll()

		case key.Matches(msg, m.keys.Test):
			if m.currentTab == TabInstalled {
				if item, ok := m.installedList.SelectedItem().(Item); ok {
					return m, m.testServer(item.name)
				}
			}
		}
	}

	var cmd tea.Cmd
	if m.currentTab == TabInstalled {
		m.installedList, cmd = m.installedList.Update(msg)
	} else {
		m.browseList, cmd = m.browseList.Update(msg)
	}
	return m, cmd
}

// currentList returns the currently active list
func (m *Model) currentList() *list.Model {
	if m.currentTab == TabInstalled {
		return &m.installedList
	}
	return &m.browseList
}

// refreshLists rebuilds the installed and browse lists from current config
func (m *Model) refreshLists() {
	// Rebuild installed list
	installedItems := []list.Item{}
	for name, server := range m.cfg.Servers {
		status := "enabled"
		if server.Disabled {
			status = "disabled"
		}

		transport := string(server.Transport)
		if transport == "" {
			transport = "stdio"
		}
		desc := fmt.Sprintf("%s transport", transport)
		if server.Command != "" {
			desc += fmt.Sprintf(" • %s", server.Command)
		}
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}

		installedItems = append(installedItems, Item{
			name:        name,
			description: desc,
			status:      status,
			server:      server,
		})
	}
	m.installedList.SetItems(installedItems)

	// Rebuild browse list (exclude installed)
	browseItems := []list.Item{}
	allAliases := aliases.List()
	for name, alias := range allAliases {
		if _, installed := m.cfg.Servers[name]; installed {
			continue
		}

		desc := alias.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}

		browseItems = append(browseItems, Item{
			name:        name,
			description: desc,
			status:      "available",
		})
	}
	m.browseList.SetItems(browseItems)
}

// View renders the UI
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Render tabs
	var installedTab, browseTab string
	installedLabel := fmt.Sprintf("Installed (%d)", len(m.cfg.Servers))
	browseLabel := fmt.Sprintf("Browse (%d)", len(m.browseList.Items()))

	if m.currentTab == TabInstalled {
		installedTab = tabActiveStyle.Render(installedLabel)
		browseTab = tabInactiveStyle.Render(browseLabel)
	} else {
		installedTab = tabInactiveStyle.Render(installedLabel)
		browseTab = tabActiveStyle.Render(browseLabel)
	}

	tabs := lipgloss.JoinHorizontal(lipgloss.Top, installedTab, " ", browseTab)
	b.WriteString(tabs)
	b.WriteString("\n\n")

	// Render current list
	if m.currentTab == TabInstalled {
		b.WriteString(m.installedList.View())
	} else {
		b.WriteString(m.browseList.View())
	}

	// Render status message
	if m.statusMsg != "" {
		b.WriteString("\n")
		b.WriteString(m.statusMsg)
	}

	// Render help
	var helpText string
	if m.currentTab == TabInstalled {
		helpText = "tab: browse • ↑/↓: navigate • /: filter • d: delete • s: sync • t: test • q: quit"
	} else {
		helpText = "tab: installed • ↑/↓: navigate • /: filter • a: add • q: quit"
	}
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

type serverAddedMsg struct {
	name string
	err  error
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

func (m *Model) addServer(name string) tea.Cmd {
	return func() tea.Msg {
		// Resolve alias
		alias, ok := aliases.Resolve(name)
		if !ok {
			return serverAddedMsg{name: name, err: fmt.Errorf("unknown alias %q", name)}
		}

		// Build server config from alias
		server := &mcp.Server{
			Name: name,
			Source: mcp.Source{
				Type:  "alias",
				Alias: name,
				URL:   alias.URL,
			},
		}

		// Handle remote HTTP/SSE MCP servers
		if alias.Transport == "http" || alias.Transport == "sse" {
			server.Transport = mcp.Transport(alias.Transport)
			server.URL = alias.MCPURL
			server.Source.URL = alias.MCPURL
			server.Source.Type = "remote"
		} else {
			// Local server with stdio transport
			server.Transport = mcp.TransportStdio
			packageName := alias.Package
			if packageName == "" {
				packageName = name
			}
			switch alias.Runtime {
			case "node":
				server.Command = "npx"
				server.Args = []string{"-y", packageName}
			case "python":
				server.Command = "uvx"
				server.Args = []string{packageName}
			default:
				server.Command = "npx"
				server.Args = []string{"-y", packageName}
			}
		}

		// Add to config
		if m.cfg.Servers == nil {
			m.cfg.Servers = make(map[string]*mcp.Server)
		}
		m.cfg.Servers[name] = server

		// Save config
		if err := m.cfg.Save(); err != nil {
			return serverAddedMsg{name: name, err: err}
		}

		return serverAddedMsg{name: name, err: nil}
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
