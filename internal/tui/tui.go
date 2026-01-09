package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/iheanyi/agentctl/pkg/aliases"
	"github.com/iheanyi/agentctl/pkg/command"
	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/hook"
	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/iheanyi/agentctl/pkg/mcpclient"
	"github.com/iheanyi/agentctl/pkg/profile"
	"github.com/iheanyi/agentctl/pkg/prompt"
	"github.com/iheanyi/agentctl/pkg/rule"
	"github.com/iheanyi/agentctl/pkg/secrets"
	"github.com/iheanyi/agentctl/pkg/skill"
	"github.com/iheanyi/agentctl/pkg/sync"
)

// ResourceTab represents the current tab/resource type being viewed
type ResourceTab int

const (
	TabServers ResourceTab = iota
	TabCommands
	TabRules
	TabSkills
	TabPrompts
	TabHooks
)

// TabNames returns the display names for tabs
var TabNames = []string{"Servers", "Commands", "Rules", "Skills", "Prompts", "Hooks"}

// FilterMode represents the current filter for the server list
type FilterMode int

const (
	FilterAll FilterMode = iota
	FilterInstalled
	FilterAvailable
	FilterDisabled
)

// LogEntry represents a single log entry in the log panel
type LogEntry struct {
	Time    time.Time
	Level   string // info, warn, error, success
	Message string
}

// Model is the Bubble Tea model for the TUI
type Model struct {
	// Data
	cfg           *config.Config
	allServers    []Server // All servers (installed + available)
	filteredItems []Server // Currently visible items after filtering
	selected      map[string]bool

	// Other resource types
	commands []*command.Command
	rules    []*rule.Rule
	skills   []*skill.Skill
	prompts  []*prompt.Prompt
	hooks    []*hook.Hook

	// Resource CRUD handler
	resourceCRUD *ResourceCRUD

	// Tab state
	activeTab ResourceTab

	// State
	cursor      int
	filterMode  FilterMode
	searchQuery string
	searching   bool
	profile     string // Current profile name

	// Log panel
	logs        []LogEntry
	logExpanded bool
	logViewport viewport.Model

	// UI state
	showHelp  bool
	quitting  bool
	width     int
	height    int
	spinner   spinner.Model
	statusMsg string

	// Profile picker
	showProfilePicker bool
	profiles          []*profile.Profile
	profileCursor     int

	// Tool testing modal
	showToolModal    bool
	toolModalServer  *Server           // Server being tested
	toolCursor       int               // Selected tool index
	toolArgInput     string            // Current argument input (JSON)
	toolResult       *mcpclient.ToolCallResult
	toolExecuting    bool

	// Keys
	keys keyMap
}

// New creates a new TUI model
func New() (*Model, error) {
	cfg, err := config.LoadWithProject()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = SpinnerStyle

	m := &Model{
		cfg:          cfg,
		selected:     make(map[string]bool),
		filterMode:   FilterAll,
		profile:      "default",
		logs:         []LogEntry{},
		keys:         newKeyMap(),
		spinner:      s,
		resourceCRUD: NewResourceCRUD(cfg),
	}

	// Load profiles
	profilesDir := filepath.Join(cfg.ConfigDir, "profiles")
	if profiles, err := profile.LoadAll(profilesDir); err == nil {
		m.profiles = profiles
	}

	// Use default profile from settings if available
	if cfg.Settings.DefaultProfile != "" {
		m.profile = cfg.Settings.DefaultProfile
	}

	// Load all resource types
	m.loadAllResources()

	m.buildServerList()
	m.applyFilter()
	m.addLog("info", "agentctl UI ready")

	return m, nil
}

// loadAllResources loads commands, rules, skills, and prompts from config directory
func (m *Model) loadAllResources() {
	configDir := m.cfg.ConfigDir

	// Load commands
	commandsDir := filepath.Join(configDir, "commands")
	if cmds, err := command.LoadAll(commandsDir); err == nil {
		m.commands = cmds
	}

	// Load rules
	rulesDir := filepath.Join(configDir, "rules")
	if rules, err := rule.LoadAll(rulesDir); err == nil {
		m.rules = rules
	}

	// Load skills
	skillsDir := filepath.Join(configDir, "skills")
	if skills, err := skill.LoadAll(skillsDir); err == nil {
		m.skills = skills
	}

	// Load prompts
	promptsDir := filepath.Join(configDir, "prompts")
	if prompts, err := prompt.LoadAll(promptsDir); err == nil {
		m.prompts = prompts
	}

	// Load hooks (from all supported tools)
	if hooks, err := hook.LoadAll(); err == nil {
		m.hooks = hooks
	}
}

// buildServerList constructs the unified server list from config and aliases
func (m *Model) buildServerList() {
	m.allServers = []Server{}
	installedNames := make(map[string]bool)

	// Add installed servers
	for name, server := range m.cfg.Servers {
		installedNames[name] = true

		status := ServerStatusInstalled
		if server.Disabled {
			status = ServerStatusDisabled
		}

		transport := string(server.Transport)
		if transport == "" {
			transport = "stdio"
		}

		m.allServers = append(m.allServers, Server{
			Name:         name,
			Desc:         "", // Generated by FormatServerDescription from transport/command
			Status:       status,
			Health:       HealthStatusUnknown,
			Transport:    transport,
			Command:      server.Command,
			ServerConfig: server,
		})
	}

	// Add available servers from aliases (not yet installed)
	allAliases := aliases.List()
	for name, alias := range allAliases {
		if installedNames[name] {
			continue
		}

		transport := alias.Transport
		if transport == "" {
			transport = "stdio"
		}

		m.allServers = append(m.allServers, Server{
			Name:        name,
			Desc:        alias.Description,
			Status:      ServerStatusAvailable,
			Health:      HealthStatusUnknown,
			Transport:   transport,
			AliasConfig: &alias,
		})
	}

	// Sort: installed first, then alphabetically within each group
	sort.Slice(m.allServers, func(i, j int) bool {
		// Installed/disabled before available
		if m.allServers[i].Status != ServerStatusAvailable && m.allServers[j].Status == ServerStatusAvailable {
			return true
		}
		if m.allServers[i].Status == ServerStatusAvailable && m.allServers[j].Status != ServerStatusAvailable {
			return false
		}
		// Alphabetically within group
		return m.allServers[i].Name < m.allServers[j].Name
	})
}

// applyFilter updates filteredItems based on current filter mode and search
func (m *Model) applyFilter() {
	m.filteredItems = []Server{}

	for _, s := range m.allServers {
		// Apply filter mode
		switch m.filterMode {
		case FilterInstalled:
			if s.Status != ServerStatusInstalled {
				continue
			}
		case FilterAvailable:
			if s.Status != ServerStatusAvailable {
				continue
			}
		case FilterDisabled:
			if s.Status != ServerStatusDisabled {
				continue
			}
		}

		// Apply search query
		if m.searchQuery != "" {
			if !strings.Contains(strings.ToLower(s.Name), strings.ToLower(m.searchQuery)) &&
				!strings.Contains(strings.ToLower(s.Desc), strings.ToLower(m.searchQuery)) {
				continue
			}
		}

		m.filteredItems = append(m.filteredItems, s)
	}

	// Adjust cursor if needed
	if m.cursor >= len(m.filteredItems) {
		m.cursor = max(0, len(m.filteredItems)-1)
	}
}

// addLog adds a log entry to the log panel
func (m *Model) addLog(level, message string) {
	m.logs = append(m.logs, LogEntry{
		Time:    time.Now(),
		Level:   level,
		Message: message,
	})
	// Keep only last 100 entries
	if len(m.logs) > 100 {
		m.logs = m.logs[1:]
	}
}

// counts returns the number of installed, available, and disabled servers
func (m *Model) counts() (installed, available, disabled int) {
	for _, s := range m.allServers {
		switch s.Status {
		case ServerStatusInstalled:
			installed++
		case ServerStatusAvailable:
			available++
		case ServerStatusDisabled:
			disabled++
		}
	}
	return
}

// selectedServer returns the currently highlighted server, or nil if none
func (m *Model) selectedServer() *Server {
	if m.cursor >= 0 && m.cursor < len(m.filteredItems) {
		return &m.filteredItems[m.cursor]
	}
	return nil
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.logViewport.Width = msg.Width - 4
		m.logViewport.Height = 3
		if m.logExpanded {
			m.logViewport.Height = msg.Height / 3
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case serverDeletedMsg:
		m.addLog("success", fmt.Sprintf("Deleted %s", msg.name))
		m.buildServerList()
		m.applyFilter()

	case serverTestedMsg:
		if msg.healthy {
			toolCount := len(msg.tools)
			latencyStr := msg.latency.Round(time.Millisecond).String()
			m.addLog("success", fmt.Sprintf("%s is healthy (%d tools, %s)", msg.name, toolCount, latencyStr))
		} else {
			m.addLog("error", fmt.Sprintf("%s: %v", msg.name, msg.err))
		}
		// Update health status in the list
		for i := range m.allServers {
			if m.allServers[i].Name == msg.name {
				if msg.healthy {
					m.allServers[i].Health = HealthStatusHealthy
					m.allServers[i].Tools = msg.tools
					m.allServers[i].HealthLatency = msg.latency.Round(time.Millisecond).String()
				} else {
					m.allServers[i].Health = HealthStatusUnhealthy
					m.allServers[i].HealthError = msg.err
				}
				break
			}
		}
		m.applyFilter()

	case syncCompletedMsg:
		if len(msg.errors) == 0 {
			m.addLog("success", "Synced to all tools")
		} else {
			m.addLog("warn", fmt.Sprintf("Sync completed with %d errors", len(msg.errors)))
		}

	case serverAddedMsg:
		if msg.err != nil {
			m.addLog("error", fmt.Sprintf("Failed to add %s: %v", msg.name, msg.err))
		} else {
			m.addLog("success", fmt.Sprintf("Installed %s", msg.name))
			m.buildServerList()
			m.applyFilter()
		}

	case serverToggledMsg:
		if msg.err != nil {
			m.addLog("error", fmt.Sprintf("Failed to toggle %s: %v", msg.name, msg.err))
		} else {
			action := "Enabled"
			if msg.disabled {
				action = "Disabled"
			}
			m.addLog("success", fmt.Sprintf("%s %s", action, msg.name))
			m.buildServerList()
			m.applyFilter()
		}

	case editorFinishedMsg:
		if msg.err != nil {
			m.addLog("error", fmt.Sprintf("Editor error: %v", msg.err))
		} else {
			m.addLog("success", "Config updated")
			// Reload config after editing
			if cfg, err := config.LoadWithProject(); err == nil {
				m.cfg = cfg
				m.buildServerList()
				m.applyFilter()
			}
		}

	case toolExecutedMsg:
		m.toolExecuting = false
		m.toolResult = &msg.result
		if msg.result.Error != nil {
			m.addLog("error", fmt.Sprintf("Tool %s failed: %v", msg.toolName, msg.result.Error))
		} else if msg.result.IsError {
			m.addLog("warn", fmt.Sprintf("Tool %s returned error", msg.toolName))
		} else {
			m.addLog("success", fmt.Sprintf("Tool %s executed (%s)", msg.toolName, msg.result.Latency.Round(time.Millisecond)))
		}

	case resourceCreatedMsg:
		if msg.err != nil {
			m.addLog("error", fmt.Sprintf("Failed to create %s: %v", msg.resourceType, msg.err))
		} else {
			m.addLog("success", fmt.Sprintf("Created %s: %s", msg.resourceType, msg.name))
			m.loadAllResources()
		}

	case resourceEditedMsg:
		if msg.err != nil {
			m.addLog("error", fmt.Sprintf("Failed to edit %s: %v", msg.resourceType, msg.err))
		} else {
			m.addLog("success", fmt.Sprintf("Edited %s", msg.resourceType))
			m.loadAllResources()
		}

	case resourceDeletedMsg:
		if msg.err != nil {
			m.addLog("error", fmt.Sprintf("Failed to delete %s: %v", msg.resourceType, msg.err))
		} else {
			m.addLog("success", fmt.Sprintf("Deleted %s: %s", msg.resourceType, msg.name))
			m.loadAllResources()
			// Adjust cursor if needed
			m.adjustCursorForCurrentTab()
		}

	case tea.KeyMsg:
		// Handle search input mode
		if m.searching {
			switch msg.String() {
			case "esc":
				m.searching = false
				m.searchQuery = ""
				m.applyFilter()
			case "enter":
				m.searching = false
			case "backspace":
				if len(m.searchQuery) > 0 {
					m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
					m.applyFilter()
				}
			default:
				if len(msg.String()) == 1 {
					m.searchQuery += msg.String()
					m.applyFilter()
				}
			}
			return m, nil
		}

		// Handle help overlay
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}

		// Handle profile picker
		if m.showProfilePicker {
			switch msg.String() {
			case "esc", "q":
				m.showProfilePicker = false
			case "j", "down":
				// +1 for "default" option at index 0
				if m.profileCursor < len(m.profiles) {
					m.profileCursor++
				}
			case "k", "up":
				if m.profileCursor > 0 {
					m.profileCursor--
				}
			case "enter":
				if m.profileCursor == 0 {
					m.profile = "default"
					m.addLog("info", "Switched to default profile")
				} else if m.profileCursor <= len(m.profiles) {
					selectedProfile := m.profiles[m.profileCursor-1]
					m.profile = selectedProfile.Name
					m.addLog("info", fmt.Sprintf("Switched to profile: %s", selectedProfile.Name))
				}
				m.showProfilePicker = false
				m.buildServerList()
				m.applyFilter()
			}
			return m, nil
		}

		// Handle tool modal
		if m.showToolModal {
			if m.toolExecuting {
				// Don't accept input while executing
				return m, nil
			}

			switch msg.String() {
			case "esc", "q":
				m.showToolModal = false
				m.toolModalServer = nil
				m.toolResult = nil
				m.toolArgInput = ""
			case "j", "down":
				if m.toolModalServer != nil && m.toolCursor < len(m.toolModalServer.Tools)-1 {
					m.toolCursor++
					m.toolResult = nil // Clear previous result when changing selection
				}
			case "k", "up":
				if m.toolCursor > 0 {
					m.toolCursor--
					m.toolResult = nil
				}
			case "enter":
				// Execute the selected tool
				if m.toolModalServer != nil && len(m.toolModalServer.Tools) > 0 {
					tool := m.toolModalServer.Tools[m.toolCursor]
					m.toolExecuting = true
					m.toolResult = nil
					m.addLog("info", fmt.Sprintf("Executing %s...", tool.Name))
					// Parse JSON args if provided, otherwise use empty map
					var args map[string]any
					if m.toolArgInput != "" {
						// Try to parse as JSON
						if err := json.Unmarshal([]byte(m.toolArgInput), &args); err != nil {
							m.addLog("error", fmt.Sprintf("Invalid JSON args: %v", err))
							m.toolExecuting = false
							return m, nil
						}
					} else {
						args = make(map[string]any)
					}
					return m, m.executeTool(m.toolModalServer.Name, tool.Name, args)
				}
			case "backspace":
				if len(m.toolArgInput) > 0 {
					m.toolArgInput = m.toolArgInput[:len(m.toolArgInput)-1]
				}
			default:
				// Add to argument input
				if len(msg.String()) == 1 {
					m.toolArgInput += msg.String()
				}
			}
			return m, nil
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			m.quitting = true
			return m, tea.Quit

		case key.Matches(msg, m.keys.Help):
			m.showHelp = !m.showHelp

		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}

		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.filteredItems)-1 {
				m.cursor++
			}

		case key.Matches(msg, m.keys.Top):
			m.cursor = 0

		case key.Matches(msg, m.keys.Bottom):
			m.cursor = max(0, len(m.filteredItems)-1)

		case key.Matches(msg, m.keys.PageDown):
			m.cursor = min(m.cursor+10, len(m.filteredItems)-1)

		case key.Matches(msg, m.keys.PageUp):
			m.cursor = max(m.cursor-10, 0)

		case key.Matches(msg, m.keys.Search):
			m.searching = true
			m.searchQuery = ""

		case key.Matches(msg, m.keys.Escape):
			m.searchQuery = ""
			m.selected = make(map[string]bool)
			m.applyFilter()

		case key.Matches(msg, m.keys.Select):
			if s := m.selectedServer(); s != nil {
				m.selected[s.Name] = !m.selected[s.Name]
				if !m.selected[s.Name] {
					delete(m.selected, s.Name)
				}
			}

		case key.Matches(msg, m.keys.SelectAll):
			allSelected := true
			for _, s := range m.filteredItems {
				if !m.selected[s.Name] {
					allSelected = false
					break
				}
			}
			if allSelected {
				m.selected = make(map[string]bool)
			} else {
				for _, s := range m.filteredItems {
					m.selected[s.Name] = true
				}
			}

		case key.Matches(msg, m.keys.Install):
			if m.activeTab == TabServers {
				if s := m.selectedServer(); s != nil && s.Status == ServerStatusAvailable {
					m.addLog("info", fmt.Sprintf("Installing %s...", s.Name))
					return m, m.addServer(s.Name)
				}
			}

		case key.Matches(msg, m.keys.Add):
			// Add new resource based on current tab
			switch m.activeTab {
			case TabServers:
				return m, m.createServer()
			case TabCommands:
				return m, m.createCommand()
			case TabRules:
				return m, m.createRule()
			case TabSkills:
				return m, m.createSkill()
			case TabPrompts:
				return m, m.createPrompt()
			}

		case key.Matches(msg, m.keys.Delete):
			switch m.activeTab {
			case TabServers:
				if s := m.selectedServer(); s != nil && s.Status != ServerStatusAvailable {
					return m, m.deleteServer(s.Name)
				}
			case TabCommands:
				if m.cursor >= 0 && m.cursor < len(m.commands) {
					cmd := m.commands[m.cursor]
					return m, m.deleteCommand(cmd)
				}
			case TabRules:
				if m.cursor >= 0 && m.cursor < len(m.rules) {
					r := m.rules[m.cursor]
					return m, m.deleteRule(r)
				}
			case TabSkills:
				if m.cursor >= 0 && m.cursor < len(m.skills) {
					s := m.skills[m.cursor]
					return m, m.deleteSkill(s)
				}
			case TabPrompts:
				if m.cursor >= 0 && m.cursor < len(m.prompts) {
					p := m.prompts[m.cursor]
					return m, m.deletePrompt(p)
				}
			}

		case key.Matches(msg, m.keys.Edit):
			switch m.activeTab {
			case TabServers:
				if s := m.selectedServer(); s != nil && s.Status != ServerStatusAvailable {
					m.addLog("info", fmt.Sprintf("Opening editor for %s...", s.Name))
					return m, m.editServer(s.Name)
				}
			case TabCommands:
				if m.cursor >= 0 && m.cursor < len(m.commands) {
					cmd := m.commands[m.cursor]
					return m, m.editCommand(cmd)
				}
			case TabRules:
				if m.cursor >= 0 && m.cursor < len(m.rules) {
					r := m.rules[m.cursor]
					return m, m.editRule(r)
				}
			case TabSkills:
				if m.cursor >= 0 && m.cursor < len(m.skills) {
					s := m.skills[m.cursor]
					return m, m.editSkill(s)
				}
			case TabPrompts:
				if m.cursor >= 0 && m.cursor < len(m.prompts) {
					p := m.prompts[m.cursor]
					return m, m.editPrompt(p)
				}
			}

		case key.Matches(msg, m.keys.Toggle):
			if s := m.selectedServer(); s != nil && s.Status != ServerStatusAvailable {
				return m, m.toggleServer(s.Name)
			}

		case key.Matches(msg, m.keys.Sync):
			if s := m.selectedServer(); s != nil {
				m.addLog("info", fmt.Sprintf("Syncing %s...", s.Name))
			}
			return m, m.syncAll()

		case key.Matches(msg, m.keys.SyncAll):
			m.addLog("info", "Syncing all servers...")
			return m, m.syncAll()

		case key.Matches(msg, m.keys.Test):
			if s := m.selectedServer(); s != nil && s.Status != ServerStatusAvailable {
				m.addLog("info", fmt.Sprintf("Testing %s...", s.Name))
				// Mark as checking
				for i := range m.allServers {
					if m.allServers[i].Name == s.Name {
						m.allServers[i].Health = HealthStatusChecking
						break
					}
				}
				m.applyFilter()
				return m, m.testServer(s.Name)
			}

		case key.Matches(msg, m.keys.TestAll):
			m.addLog("info", "Testing all installed servers...")
			return m, m.testAllServers()

		case key.Matches(msg, m.keys.ExecTool):
			if s := m.selectedServer(); s != nil && s.Status != ServerStatusAvailable {
				if len(s.Tools) == 0 {
					m.addLog("warn", fmt.Sprintf("%s has no discovered tools - run test (t) first", s.Name))
				} else {
					m.showToolModal = true
					m.toolModalServer = s
					m.toolCursor = 0
					m.toolArgInput = ""
					m.toolResult = nil
				}
			}

		case key.Matches(msg, m.keys.Refresh):
			m.buildServerList()
			m.applyFilter()
			m.addLog("info", "Refreshed server list")

		case key.Matches(msg, m.keys.CycleFilter):
			m.filterMode = (m.filterMode + 1) % 4
			m.applyFilter()

		case key.Matches(msg, m.keys.FilterAll):
			m.filterMode = FilterAll
			m.applyFilter()

		case key.Matches(msg, m.keys.FilterInstalled):
			m.filterMode = FilterInstalled
			m.applyFilter()

		case key.Matches(msg, m.keys.FilterAvailable):
			m.filterMode = FilterAvailable
			m.applyFilter()

		case key.Matches(msg, m.keys.FilterDisabled):
			m.filterMode = FilterDisabled
			m.applyFilter()

		case key.Matches(msg, m.keys.ToggleLogs):
			m.logExpanded = !m.logExpanded
			if m.logExpanded {
				m.logViewport.Height = m.height / 3
			} else {
				m.logViewport.Height = 3
			}

		case key.Matches(msg, m.keys.ProfileSwitch):
			m.showProfilePicker = true
			m.profileCursor = 0
			// Find current profile index
			for i, p := range m.profiles {
				if p.Name == m.profile {
					m.profileCursor = i + 1 // +1 for default at index 0
					break
				}
			}

		// Tab switching
		case key.Matches(msg, m.keys.NextTab):
			m.activeTab = (m.activeTab + 1) % 5
			m.cursor = 0

		case key.Matches(msg, m.keys.PrevTab):
			if m.activeTab == 0 {
				m.activeTab = 4
			} else {
				m.activeTab--
			}
			m.cursor = 0

		case key.Matches(msg, m.keys.Tab1):
			m.activeTab = TabServers
			m.cursor = 0

		case key.Matches(msg, m.keys.Tab2):
			m.activeTab = TabCommands
			m.cursor = 0

		case key.Matches(msg, m.keys.Tab3):
			m.activeTab = TabRules
			m.cursor = 0

		case key.Matches(msg, m.keys.Tab4):
			m.activeTab = TabSkills
			m.cursor = 0

		case key.Matches(msg, m.keys.Tab5):
			m.activeTab = TabPrompts
			m.cursor = 0

		case key.Matches(msg, m.keys.Tab6):
			m.activeTab = TabHooks
			m.cursor = 0
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the UI
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	if m.showHelp {
		return m.renderHelpOverlay()
	}

	if m.showProfilePicker {
		return m.renderProfilePicker()
	}

	if m.showToolModal {
		return m.renderToolModal()
	}

	var sections []string

	// Header
	sections = append(sections, m.renderHeader())

	// Divider
	sections = append(sections, m.renderDivider())

	// Main content based on active tab
	switch m.activeTab {
	case TabServers:
		sections = append(sections, m.renderServerList())
	case TabCommands:
		sections = append(sections, m.renderCommandsList())
	case TabRules:
		sections = append(sections, m.renderRulesList())
	case TabSkills:
		sections = append(sections, m.renderSkillsList())
	case TabPrompts:
		sections = append(sections, m.renderPromptsList())
	case TabHooks:
		sections = append(sections, m.renderHooksList())
	}

	// Divider
	sections = append(sections, m.renderDivider())

	// Log panel
	sections = append(sections, m.renderLogPanel())

	// Divider
	sections = append(sections, m.renderDivider())

	// Key hints
	sections = append(sections, m.renderKeyHints())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderHeader renders the header bar with tabs
func (m *Model) renderHeader() string {
	// Left: App name
	title := HeaderTitleStyle.Render("agentctl")

	// Tab bar
	var tabs []string
	for i, name := range TabNames {
		if ResourceTab(i) == m.activeTab {
			tabs = append(tabs, TabActiveStyle.Render("["+name+"]"))
		} else {
			tabs = append(tabs, TabStyle.Render(" "+name+" "))
		}
	}
	tabBar := strings.Join(tabs, " ")

	// Right: Resource counts based on active tab
	var countStr string
	switch m.activeTab {
	case TabServers:
		installed, available, disabled := m.counts()
		countStr = fmt.Sprintf("%s %d  %s %d  %s %d",
			StatusInstalledStyle.Render(StatusInstalled), installed,
			StatusAvailableStyle.Render(StatusAvailable), available,
			StatusDisabledStyle.Render(StatusDisabled), disabled,
		)
	case TabCommands:
		countStr = fmt.Sprintf("%d commands", len(m.commands))
	case TabRules:
		countStr = fmt.Sprintf("%d rules", len(m.rules))
	case TabSkills:
		countStr = fmt.Sprintf("%d skills", len(m.skills))
	case TabPrompts:
		countStr = fmt.Sprintf("%d prompts", len(m.prompts))
	case TabHooks:
		countStr = fmt.Sprintf("%d hooks", len(m.hooks))
	}
	counts := HeaderSubtitleStyle.Render(countStr)

	// Calculate spacing
	leftWidth := lipgloss.Width(title)
	centerWidth := lipgloss.Width(tabBar)
	rightWidth := lipgloss.Width(counts)
	totalContent := leftWidth + centerWidth + rightWidth

	availableSpace := m.width - totalContent - 4
	if availableSpace < 0 {
		availableSpace = 0
	}
	leftPad := availableSpace / 2
	rightPad := availableSpace - leftPad

	header := title + strings.Repeat(" ", leftPad) + tabBar + strings.Repeat(" ", rightPad) + counts

	return HeaderStyle.Width(m.width).Render(header)
}

// renderDivider renders a horizontal divider
func (m *Model) renderDivider() string {
	return lipgloss.NewStyle().
		Foreground(colorFgSubtle).
		Width(m.width).
		Render(strings.Repeat("─", m.width))
}

// renderServerList renders the main server list
func (m *Model) renderServerList() string {
	var rows []string

	// Calculate available height for list
	listHeight := m.height - 12 // Account for header, log panel, hints, dividers
	if m.logExpanded {
		listHeight = m.height - 8 - m.height/3
	}
	if listHeight < 5 {
		listHeight = 5
	}

	// Search bar if searching
	if m.searching {
		searchBar := SearchPromptStyle.Render("/") + SearchInputStyle.Render(m.searchQuery+"█")
		rows = append(rows, SearchStyle.Width(m.width-4).Render(searchBar))
		listHeight--
	}

	// Filter indicator
	filterLabel := ""
	switch m.filterMode {
	case FilterAll:
		filterLabel = "All"
	case FilterInstalled:
		filterLabel = "Installed"
	case FilterAvailable:
		filterLabel = "Available"
	case FilterDisabled:
		filterLabel = "Disabled"
	}
	if filterLabel != "" && m.filterMode != FilterAll {
		filterText := lipgloss.NewStyle().Foreground(colorCyan).Render("Filter: " + filterLabel)
		rows = append(rows, "  "+filterText)
		listHeight--
	}

	if len(m.filteredItems) == 0 {
		emptyMsg := lipgloss.NewStyle().
			Foreground(colorFgSubtle).
			Italic(true).
			Render("No servers match the current filter")
		rows = append(rows, "  "+emptyMsg)
	} else {
		// Calculate visible range
		startIdx := 0
		if m.cursor >= listHeight {
			startIdx = m.cursor - listHeight + 1
		}
		endIdx := min(startIdx+listHeight, len(m.filteredItems))

		for i := startIdx; i < endIdx; i++ {
			server := m.filteredItems[i]
			row := m.renderServerRow(server, i == m.cursor)
			rows = append(rows, row)
		}
	}

	// Pad to fill height
	for len(rows) < listHeight {
		rows = append(rows, "")
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// renderServerRow renders a single server row
func (m *Model) renderServerRow(s Server, selected bool) string {
	// Selection indicator
	selectIndicator := "  "
	if m.selected[s.Name] {
		selectIndicator = ListCursorStyle.Render("▶ ")
	}

	// Status badge (styled)
	var statusBadge string
	switch s.Status {
	case ServerStatusInstalled:
		statusBadge = StatusInstalledStyle.Render(StatusInstalled)
	case ServerStatusAvailable:
		statusBadge = StatusAvailableStyle.Render(StatusAvailable)
	case ServerStatusDisabled:
		statusBadge = StatusDisabledStyle.Render(StatusDisabled)
	}

	// Server name
	nameStyle := ListItemNameStyle
	if selected {
		nameStyle = ListItemNameSelectedStyle
	}
	name := nameStyle.Render(s.Name)

	// Health indicator with tool count
	var healthBadge string
	switch s.Health {
	case HealthStatusHealthy:
		toolInfo := ""
		if len(s.Tools) > 0 {
			toolInfo = fmt.Sprintf(" (%d tools)", len(s.Tools))
		}
		healthBadge = HealthHealthyStyle.Render(" "+HealthHealthy) + lipgloss.NewStyle().Foreground(colorCyan).Render(toolInfo)
	case HealthStatusUnhealthy:
		healthBadge = HealthUnhealthyStyle.Render(" " + HealthUnhealthy)
	case HealthStatusChecking:
		healthBadge = HealthCheckingStyle.Render(" " + m.spinner.View())
	}

	// Description
	descStyle := ListItemDescStyle
	if selected {
		descStyle = ListItemDescSelectedStyle
	}
	desc := descStyle.Render(s.Transport + " · " + truncate(s.Description(), 50))

	// Build the row
	leftPart := selectIndicator + statusBadge + " " + name + healthBadge

	// Calculate padding for right-aligned description
	leftWidth := lipgloss.Width(leftPart)
	descWidth := lipgloss.Width(desc)
	padding := m.width - leftWidth - descWidth - 4
	if padding < 2 {
		padding = 2
	}

	row := leftPart + strings.Repeat(" ", padding) + desc

	// Apply selection styling
	if selected {
		row = ListItemSelectedStyle.Width(m.width).Render(row)
	} else {
		row = ListItemNormalStyle.Width(m.width).Render(row)
	}

	return row
}

// renderCommandsList renders the commands list
func (m *Model) renderCommandsList() string {
	var rows []string

	// Calculate available height for list
	listHeight := m.height - 12
	if m.logExpanded {
		listHeight = m.height - 8 - m.height/3
	}
	if listHeight < 5 {
		listHeight = 5
	}

	if len(m.commands) == 0 {
		emptyMsg := lipgloss.NewStyle().
			Foreground(colorFgSubtle).
			Italic(true).
			Render("No commands defined")
		rows = append(rows, "  "+emptyMsg)
		rows = append(rows, "")
		rows = append(rows, lipgloss.NewStyle().
			Foreground(colorFgSubtle).
			Render("  Create commands in ~/.config/agentctl/commands/"))
	} else {
		// Calculate visible range
		startIdx := 0
		if m.cursor >= listHeight {
			startIdx = m.cursor - listHeight + 1
		}
		endIdx := min(startIdx+listHeight, len(m.commands))

		for i := startIdx; i < endIdx; i++ {
			cmd := m.commands[i]
			row := m.renderCommandRow(cmd, i == m.cursor)
			rows = append(rows, row)
		}
	}

	// Pad to fill height
	for len(rows) < listHeight {
		rows = append(rows, "")
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// renderCommandRow renders a single command row
func (m *Model) renderCommandRow(cmd *command.Command, selected bool) string {
	// Icon
	icon := StatusInstalledStyle.Render("⌘")

	// Command name with / prefix
	nameStyle := ListItemNameStyle
	if selected {
		nameStyle = ListItemNameSelectedStyle
	}
	name := nameStyle.Render("/" + cmd.Name)

	// Description
	descStyle := ListItemDescStyle
	if selected {
		descStyle = ListItemDescSelectedStyle
	}
	desc := descStyle.Render(truncate(cmd.Description, 50))

	// Build the row
	leftPart := "  " + icon + " " + name

	// Calculate padding for right-aligned description
	leftWidth := lipgloss.Width(leftPart)
	descWidth := lipgloss.Width(desc)
	padding := m.width - leftWidth - descWidth - 4
	if padding < 2 {
		padding = 2
	}

	row := leftPart + strings.Repeat(" ", padding) + desc

	// Apply selection styling
	if selected {
		row = ListItemSelectedStyle.Width(m.width).Render(row)
	} else {
		row = ListItemNormalStyle.Width(m.width).Render(row)
	}

	return row
}

// renderRulesList renders the rules list
func (m *Model) renderRulesList() string {
	var rows []string

	// Calculate available height for list
	listHeight := m.height - 12
	if m.logExpanded {
		listHeight = m.height - 8 - m.height/3
	}
	if listHeight < 5 {
		listHeight = 5
	}

	if len(m.rules) == 0 {
		emptyMsg := lipgloss.NewStyle().
			Foreground(colorFgSubtle).
			Italic(true).
			Render("No rules defined")
		rows = append(rows, "  "+emptyMsg)
		rows = append(rows, "")
		rows = append(rows, lipgloss.NewStyle().
			Foreground(colorFgSubtle).
			Render("  Create rules in ~/.config/agentctl/rules/"))
	} else {
		// Calculate visible range
		startIdx := 0
		if m.cursor >= listHeight {
			startIdx = m.cursor - listHeight + 1
		}
		endIdx := min(startIdx+listHeight, len(m.rules))

		for i := startIdx; i < endIdx; i++ {
			r := m.rules[i]
			row := m.renderRuleRow(r, i == m.cursor)
			rows = append(rows, row)
		}
	}

	// Pad to fill height
	for len(rows) < listHeight {
		rows = append(rows, "")
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// renderRuleRow renders a single rule row
func (m *Model) renderRuleRow(r *rule.Rule, selected bool) string {
	// Icon
	icon := StatusInstalledStyle.Render("📜")

	// Rule name
	nameStyle := ListItemNameStyle
	if selected {
		nameStyle = ListItemNameSelectedStyle
	}
	name := nameStyle.Render(r.Name)

	// Description - show first line of content or applies pattern
	descStyle := ListItemDescStyle
	if selected {
		descStyle = ListItemDescSelectedStyle
	}
	var descText string
	if r.Frontmatter != nil && r.Frontmatter.Applies != "" {
		descText = "applies: " + r.Frontmatter.Applies
	} else {
		// Use first line of content as description
		lines := strings.Split(r.Content, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				descText = line
				break
			}
		}
	}
	desc := descStyle.Render(truncate(descText, 50))

	// Build the row
	leftPart := "  " + icon + " " + name

	// Calculate padding for right-aligned description
	leftWidth := lipgloss.Width(leftPart)
	descWidth := lipgloss.Width(desc)
	padding := m.width - leftWidth - descWidth - 4
	if padding < 2 {
		padding = 2
	}

	row := leftPart + strings.Repeat(" ", padding) + desc

	// Apply selection styling
	if selected {
		row = ListItemSelectedStyle.Width(m.width).Render(row)
	} else {
		row = ListItemNormalStyle.Width(m.width).Render(row)
	}

	return row
}

// renderSkillsList renders the skills list
func (m *Model) renderSkillsList() string {
	var rows []string

	// Calculate available height for list
	listHeight := m.height - 12
	if m.logExpanded {
		listHeight = m.height - 8 - m.height/3
	}
	if listHeight < 5 {
		listHeight = 5
	}

	if len(m.skills) == 0 {
		emptyMsg := lipgloss.NewStyle().
			Foreground(colorFgSubtle).
			Italic(true).
			Render("No skills installed")
		rows = append(rows, "  "+emptyMsg)
		rows = append(rows, "")
		rows = append(rows, lipgloss.NewStyle().
			Foreground(colorFgSubtle).
			Render("  Create skills in ~/.config/agentctl/skills/"))
	} else {
		// Calculate visible range
		startIdx := 0
		if m.cursor >= listHeight {
			startIdx = m.cursor - listHeight + 1
		}
		endIdx := min(startIdx+listHeight, len(m.skills))

		for i := startIdx; i < endIdx; i++ {
			s := m.skills[i]
			row := m.renderSkillRow(s, i == m.cursor)
			rows = append(rows, row)
		}
	}

	// Pad to fill height
	for len(rows) < listHeight {
		rows = append(rows, "")
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// renderSkillRow renders a single skill row
func (m *Model) renderSkillRow(s *skill.Skill, selected bool) string {
	// Icon
	icon := StatusInstalledStyle.Render("⚡")

	// Skill name
	nameStyle := ListItemNameStyle
	if selected {
		nameStyle = ListItemNameSelectedStyle
	}
	name := nameStyle.Render(s.Name)

	// Version badge if available
	versionBadge := ""
	if s.Version != "" {
		versionBadge = lipgloss.NewStyle().Foreground(colorCyan).Render(" v" + s.Version)
	}

	// Description
	descStyle := ListItemDescStyle
	if selected {
		descStyle = ListItemDescSelectedStyle
	}
	descText := s.Description
	if descText == "" && len(s.Prompts) > 0 {
		descText = fmt.Sprintf("%d prompts", len(s.Prompts))
	}
	desc := descStyle.Render(truncate(descText, 40))

	// Build the row
	leftPart := "  " + icon + " " + name + versionBadge

	// Calculate padding for right-aligned description
	leftWidth := lipgloss.Width(leftPart)
	descWidth := lipgloss.Width(desc)
	padding := m.width - leftWidth - descWidth - 4
	if padding < 2 {
		padding = 2
	}

	row := leftPart + strings.Repeat(" ", padding) + desc

	// Apply selection styling
	if selected {
		row = ListItemSelectedStyle.Width(m.width).Render(row)
	} else {
		row = ListItemNormalStyle.Width(m.width).Render(row)
	}

	return row
}

// renderPromptsList renders the prompts list
func (m *Model) renderPromptsList() string {
	var rows []string

	// Calculate available height for list
	listHeight := m.height - 12
	if m.logExpanded {
		listHeight = m.height - 8 - m.height/3
	}
	if listHeight < 5 {
		listHeight = 5
	}

	if len(m.prompts) == 0 {
		emptyMsg := lipgloss.NewStyle().
			Foreground(colorFgSubtle).
			Italic(true).
			Render("No prompts defined")
		rows = append(rows, "  "+emptyMsg)
		rows = append(rows, "")
		rows = append(rows, lipgloss.NewStyle().
			Foreground(colorFgSubtle).
			Render("  Create prompts in ~/.config/agentctl/prompts/"))
	} else {
		// Calculate visible range
		startIdx := 0
		if m.cursor >= listHeight {
			startIdx = m.cursor - listHeight + 1
		}
		endIdx := min(startIdx+listHeight, len(m.prompts))

		for i := startIdx; i < endIdx; i++ {
			p := m.prompts[i]
			row := m.renderPromptRow(p, i == m.cursor)
			rows = append(rows, row)
		}
	}

	// Pad to fill height
	for len(rows) < listHeight {
		rows = append(rows, "")
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// renderPromptRow renders a single prompt row
func (m *Model) renderPromptRow(p *prompt.Prompt, selected bool) string {
	// Icon
	icon := StatusInstalledStyle.Render("💬")

	// Prompt name
	nameStyle := ListItemNameStyle
	if selected {
		nameStyle = ListItemNameSelectedStyle
	}
	name := nameStyle.Render(p.Name)

	// Variables badge if available
	varsBadge := ""
	if len(p.Variables) > 0 {
		varsBadge = lipgloss.NewStyle().Foreground(colorCyan).Render(fmt.Sprintf(" (%d vars)", len(p.Variables)))
	}

	// Description
	descStyle := ListItemDescStyle
	if selected {
		descStyle = ListItemDescSelectedStyle
	}
	descText := p.Description
	if descText == "" {
		// Use truncated template as description
		descText = truncate(strings.ReplaceAll(p.Template, "\n", " "), 40)
	}
	desc := descStyle.Render(truncate(descText, 40))

	// Build the row
	leftPart := "  " + icon + " " + name + varsBadge

	// Calculate padding for right-aligned description
	leftWidth := lipgloss.Width(leftPart)
	descWidth := lipgloss.Width(desc)
	padding := m.width - leftWidth - descWidth - 4
	if padding < 2 {
		padding = 2
	}

	row := leftPart + strings.Repeat(" ", padding) + desc

	// Apply selection styling
	if selected {
		row = ListItemSelectedStyle.Width(m.width).Render(row)
	} else {
		row = ListItemNormalStyle.Width(m.width).Render(row)
	}

	return row
}

// renderHooksList renders the hooks list (read-only)
func (m *Model) renderHooksList() string {
	var rows []string

	// Calculate available height for list
	listHeight := m.height - 12
	if m.logExpanded {
		listHeight = m.height - 8 - m.height/3
	}
	if listHeight < 5 {
		listHeight = 5
	}

	if len(m.hooks) == 0 {
		emptyMsg := lipgloss.NewStyle().
			Foreground(colorFgSubtle).
			Italic(true).
			Render("No hooks configured")
		rows = append(rows, "  "+emptyMsg)
		rows = append(rows, "")
		rows = append(rows, lipgloss.NewStyle().
			Foreground(colorFgSubtle).
			Render("  Hooks are configured in ~/.claude/settings.json"))
		rows = append(rows, lipgloss.NewStyle().
			Foreground(colorFgSubtle).
			Render("  (Read-only view - edit settings.json directly)"))
	} else {
		// Calculate visible range
		startIdx := 0
		if m.cursor >= listHeight {
			startIdx = m.cursor - listHeight + 1
		}
		endIdx := min(startIdx+listHeight, len(m.hooks))

		for i := startIdx; i < endIdx; i++ {
			h := m.hooks[i]
			row := m.renderHookRow(h, i == m.cursor)
			rows = append(rows, row)
		}
	}

	// Pad to fill height
	for len(rows) < listHeight {
		rows = append(rows, "")
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// renderHookRow renders a single hook row
func (m *Model) renderHookRow(h *hook.Hook, selected bool) string {
	// Icon based on hook type
	icon := "🪝"
	switch h.Type {
	case "PreToolUse":
		icon = "⏮"
	case "PostToolUse":
		icon = "⏭"
	case "Notification":
		icon = "🔔"
	case "Stop":
		icon = "⏹"
	case "UserPromptSubmit":
		icon = "📝"
	}
	iconStyled := StatusInstalledStyle.Render(icon)

	// Hook type
	nameStyle := ListItemNameStyle
	if selected {
		nameStyle = ListItemNameSelectedStyle
	}
	name := nameStyle.Render(h.Type)

	// Matcher badge
	matcherBadge := ""
	if h.Matcher != "" {
		matcherBadge = lipgloss.NewStyle().Foreground(colorCyan).Render(fmt.Sprintf(" [%s]", h.Matcher))
	}

	// Source badge
	sourceBadge := lipgloss.NewStyle().Foreground(colorFgSubtle).Render(fmt.Sprintf(" (%s)", h.Source))

	// Command (truncated)
	descStyle := ListItemDescStyle
	if selected {
		descStyle = ListItemDescSelectedStyle
	}
	cmdText := truncate(h.Command, 40)
	desc := descStyle.Render(cmdText)

	// Build the row
	leftPart := "  " + iconStyled + " " + name + matcherBadge + sourceBadge

	// Calculate padding for right-aligned command
	leftWidth := lipgloss.Width(leftPart)
	descWidth := lipgloss.Width(desc)
	padding := m.width - leftWidth - descWidth - 4
	if padding < 2 {
		padding = 2
	}

	row := leftPart + strings.Repeat(" ", padding) + desc

	// Apply selection styling
	if selected {
		row = ListItemSelectedStyle.Width(m.width).Render(row)
	} else {
		row = ListItemNormalStyle.Width(m.width).Render(row)
	}

	return row
}

// renderLogPanel renders the log panel
func (m *Model) renderLogPanel() string {
	var logLines []string

	// Determine how many log entries to show
	numLogs := 3
	if m.logExpanded {
		numLogs = m.height / 3
	}

	startIdx := max(0, len(m.logs)-numLogs)
	for i := startIdx; i < len(m.logs); i++ {
		entry := m.logs[i]
		timeStr := LogTimestampStyle.Render(entry.Time.Format("15:04:05"))

		var levelStyle lipgloss.Style
		var symbol string
		switch entry.Level {
		case "success":
			levelStyle = LogEntryInfoStyle.Foreground(colorTeal)
			symbol = "✓"
		case "error":
			levelStyle = LogEntryErrorStyle
			symbol = "✗"
		case "warn":
			levelStyle = LogEntryWarnStyle
			symbol = "⚠"
		default:
			levelStyle = LogEntryInfoStyle
			symbol = "↻"
		}

		logLine := timeStr + "  " + levelStyle.Render(symbol+" "+entry.Message)
		logLines = append(logLines, logLine)
	}

	// Pad if needed
	for len(logLines) < numLogs {
		logLines = append([]string{""}, logLines...)
	}

	return lipgloss.JoinVertical(lipgloss.Left, logLines...)
}

// renderKeyHints renders the bottom key hints bar
func (m *Model) renderKeyHints() string {
	hints := []struct{ Key, Desc string }{
		{"j/k", "navigate"},
		{"Tab", "switch tab"},
	}

	// Add context-sensitive hints based on active tab
	switch m.activeTab {
	case TabServers:
		hints = append(hints,
			struct{ Key, Desc string }{"i", "install"},
			struct{ Key, Desc string }{"d", "delete"},
			struct{ Key, Desc string }{"t", "test"},
			struct{ Key, Desc string }{"x", "tools"},
			struct{ Key, Desc string }{"s", "sync"},
		)
	case TabCommands, TabRules, TabSkills, TabPrompts:
		hints = append(hints,
			struct{ Key, Desc string }{"a", "add"},
			struct{ Key, Desc string }{"e", "edit"},
			struct{ Key, Desc string }{"d", "delete"},
		)
	}

	hints = append(hints,
		struct{ Key, Desc string }{"/", "search"},
		struct{ Key, Desc string }{"?", "help"},
	)

	return RenderKeyHintsBar(hints)
}

// renderHelpOverlay renders the full help overlay
func (m *Model) renderHelpOverlay() string {
	content := `
  Navigation           Operations           Filters
  ──────────           ──────────           ───────
  j/k     up/down      i      install       f    cycle filter
  g/G     top/bottom   a      add new       1    all
  /       search       d      delete        2    installed
  Esc     clear        e      edit          3    available
                       Enter  toggle        4    disabled
  Selection            s      sync
  ─────────            t      test          UI
  Space   toggle       x      tools (exec)  ──
  V       select all                        L    toggle logs
                       Profiles             ?    this help
  Tabs                 ────────             q    quit
  ────                 P      switch
  Tab/Shift+Tab  next/prev tab
  F1-F6          jump to tab (Servers/Commands/Rules/Skills/Prompts/Hooks)

                       Press any key to close
`

	return ModalStyle.
		Width(60).
		Render(ModalTitleStyle.Render("Keyboard Shortcuts") + content)
}

// renderProfilePicker renders the profile quick-switcher modal
func (m *Model) renderProfilePicker() string {
	var rows []string

	// Add "default" option at index 0
	cursor := "  "
	if m.profileCursor == 0 {
		cursor = "> "
	}
	defaultLabel := "default"
	if m.profile == "default" {
		defaultLabel += " (current)"
	}
	if m.profileCursor == 0 {
		rows = append(rows, ListItemSelectedStyle.Render(cursor+defaultLabel+" (all servers)"))
	} else {
		rows = append(rows, ListItemNormalStyle.Render(cursor+defaultLabel+" (all servers)"))
	}

	// Add user profiles
	for i, p := range m.profiles {
		cursor := "  "
		idx := i + 1 // +1 for default at index 0
		if m.profileCursor == idx {
			cursor = "> "
		}

		label := p.Name
		if m.profile == p.Name {
			label += " (current)"
		}
		desc := ""
		if len(p.Servers) > 0 {
			desc = fmt.Sprintf(" (%d servers)", len(p.Servers))
		}

		row := cursor + label + desc
		if m.profileCursor == idx {
			rows = append(rows, ListItemSelectedStyle.Render(row))
		} else {
			rows = append(rows, ListItemNormalStyle.Render(row))
		}
	}

	if len(m.profiles) == 0 {
		rows = append(rows, "")
		rows = append(rows, ListItemDimmedStyle.Render("  No profiles found"))
		rows = append(rows, ListItemDimmedStyle.Render("  Use 'agentctl profile create' to add one"))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)

	hints := "\n\n" + KeyDescStyle.Render("j/k:navigate  Enter:select  Esc:cancel")

	return ModalStyle.
		Width(40).
		Render(ModalTitleStyle.Render("Switch Profile") + "\n\n" + content + hints)
}

// renderToolModal renders the tool testing modal
func (m *Model) renderToolModal() string {
	if m.toolModalServer == nil {
		return ""
	}

	var sections []string

	// Title
	title := ModalTitleStyle.Render(fmt.Sprintf("Tools - %s", m.toolModalServer.Name))
	sections = append(sections, title)
	sections = append(sections, "")

	// Tool list
	if len(m.toolModalServer.Tools) == 0 {
		sections = append(sections, ListItemDimmedStyle.Render("  No tools available"))
	} else {
		for i, tool := range m.toolModalServer.Tools {
			cursor := "  "
			if i == m.toolCursor {
				cursor = "> "
			}

			name := tool.Name
			desc := truncate(tool.Description, 40)

			row := cursor + name
			if desc != "" {
				row += " - " + lipgloss.NewStyle().Foreground(colorFgSubtle).Render(desc)
			}

			if i == m.toolCursor {
				sections = append(sections, ListItemSelectedStyle.Render(row))
			} else {
				sections = append(sections, ListItemNormalStyle.Render(row))
			}
		}
	}

	sections = append(sections, "")

	// Argument input
	argLabel := lipgloss.NewStyle().Foreground(colorCyan).Render("Args (JSON): ")
	argInput := m.toolArgInput
	if m.toolExecuting {
		argInput += m.spinner.View()
	} else {
		argInput += "█"
	}
	sections = append(sections, argLabel+argInput)

	// Result display
	if m.toolResult != nil {
		sections = append(sections, "")
		if m.toolResult.Error != nil {
			errStyle := lipgloss.NewStyle().Foreground(colorError)
			sections = append(sections, errStyle.Render("Error: "+m.toolResult.Error.Error()))
		} else if m.toolResult.IsError {
			warnStyle := lipgloss.NewStyle().Foreground(colorYellow)
			sections = append(sections, warnStyle.Render("Tool returned error"))
			for _, content := range m.toolResult.Content {
				sections = append(sections, "  "+truncate(content, 60))
			}
		} else {
			successStyle := lipgloss.NewStyle().Foreground(colorTeal)
			sections = append(sections, successStyle.Render(fmt.Sprintf("Success (%s):", m.toolResult.Latency.Round(time.Millisecond))))
			for _, content := range m.toolResult.Content {
				// Wrap long content
				lines := strings.Split(content, "\n")
				for _, line := range lines {
					if len(line) > 60 {
						sections = append(sections, "  "+line[:60]+"...")
					} else {
						sections = append(sections, "  "+line)
					}
					if len(sections) > 20 {
						sections = append(sections, "  ...")
						break
					}
				}
			}
		}
	}

	sections = append(sections, "")
	hints := KeyDescStyle.Render("j/k:select  Enter:execute  Esc:close")
	sections = append(sections, hints)

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	return ModalStyle.
		Width(70).
		Render(content)
}

// Helper functions

// adjustCursorForCurrentTab adjusts the cursor after deletion based on the current tab
func (m *Model) adjustCursorForCurrentTab() {
	var maxIndex int
	switch m.activeTab {
	case TabServers:
		maxIndex = len(m.filteredItems) - 1
	case TabCommands:
		maxIndex = len(m.commands) - 1
	case TabRules:
		maxIndex = len(m.rules) - 1
	case TabSkills:
		maxIndex = len(m.skills) - 1
	case TabPrompts:
		maxIndex = len(m.prompts) - 1
	case TabHooks:
		maxIndex = len(m.hooks) - 1
	}
	if m.cursor > maxIndex {
		m.cursor = max(0, maxIndex)
	}
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen < 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Messages

type serverDeletedMsg struct {
	name string
}

type serverTestedMsg struct {
	name    string
	healthy bool
	err     error
	tools   []mcpclient.Tool
	latency time.Duration
}

type syncCompletedMsg struct {
	errors map[string]error
}

type serverAddedMsg struct {
	name string
	err  error
}

type serverToggledMsg struct {
	name     string
	disabled bool
	err      error
}

type editorFinishedMsg struct {
	err error
}

type toolExecutedMsg struct {
	toolName string
	result   mcpclient.ToolCallResult
}

type resourceCreatedMsg struct {
	resourceType string
	name         string
	err          error
}

type resourceEditedMsg struct {
	resourceType string
	err          error
}

type resourceDeletedMsg struct {
	resourceType string
	name         string
	err          error
}

// Commands

func (m *Model) deleteServer(name string) tea.Cmd {
	return func() tea.Msg {
		delete(m.cfg.Servers, name)
		if err := m.cfg.Save(); err != nil {
			return serverDeletedMsg{name: name}
		}
		return serverDeletedMsg{name: name}
	}
}

func (m *Model) testServer(name string) tea.Cmd {
	return func() tea.Msg {
		server, ok := m.cfg.Servers[name]
		if !ok {
			return serverTestedMsg{name: name, healthy: false, err: fmt.Errorf("server not found")}
		}

		if server.Disabled {
			return serverTestedMsg{name: name, healthy: false, err: fmt.Errorf("server is disabled")}
		}

		// Resolve environment variables and create a copy of the server config
		serverCopy := *server
		if server.Env != nil {
			resolvedEnv, err := secrets.ResolveEnv(server.Env)
			if err != nil {
				return serverTestedMsg{name: name, healthy: false, err: err}
			}
			serverCopy.Env = resolvedEnv
		}

		// Use real MCP client for health check
		client := mcpclient.NewClient().WithTimeout(10 * time.Second)
		result := client.CheckHealth(context.Background(), &serverCopy)

		return serverTestedMsg{
			name:    name,
			healthy: result.Healthy,
			err:     result.Error,
			tools:   result.Tools,
			latency: result.Latency,
		}
	}
}

func (m *Model) testAllServers() tea.Cmd {
	return func() tea.Msg {
		// This just starts the batch - individual results will come via testServer
		return syncCompletedMsg{errors: make(map[string]error)}
	}
}

func (m *Model) syncAll() tea.Cmd {
	return func() tea.Msg {
		// Build server list from config
		var servers []*mcp.Server
		for _, server := range m.cfg.Servers {
			if !server.Disabled {
				servers = append(servers, server)
			}
		}

		// Sync to all detected tools
		results := sync.SyncAll(servers, nil, nil)

		errors := make(map[string]error)
		for _, result := range results {
			if result.Error != nil {
				errors[result.Tool] = result.Error
			}
		}

		return syncCompletedMsg{errors: errors}
	}
}

func (m *Model) toggleServer(name string) tea.Cmd {
	return func() tea.Msg {
		server, ok := m.cfg.Servers[name]
		if !ok {
			return serverToggledMsg{name: name, err: fmt.Errorf("server not found")}
		}

		server.Disabled = !server.Disabled
		if err := m.cfg.Save(); err != nil {
			return serverToggledMsg{name: name, err: err}
		}

		return serverToggledMsg{name: name, disabled: server.Disabled}
	}
}

func (m *Model) editServer(name string) tea.Cmd {
	// Use project config if available, otherwise global config
	configPath := m.cfg.Path
	if m.cfg.ProjectPath != "" {
		configPath = m.cfg.ProjectPath
	}

	// Determine editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi" // fallback
	}

	// Use tea.ExecProcess to run the editor
	c := exec.Command(editor, configPath)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err: err}
	})
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

func (m *Model) executeTool(serverName, toolName string, args map[string]any) tea.Cmd {
	return func() tea.Msg {
		server, ok := m.cfg.Servers[serverName]
		if !ok {
			return toolExecutedMsg{
				toolName: toolName,
				result: mcpclient.ToolCallResult{
					Error: fmt.Errorf("server not found"),
				},
			}
		}

		// Resolve environment variables
		serverCopy := *server
		if server.Env != nil {
			resolvedEnv, err := secrets.ResolveEnv(server.Env)
			if err != nil {
				return toolExecutedMsg{
					toolName: toolName,
					result: mcpclient.ToolCallResult{
						Error: err,
					},
				}
			}
			serverCopy.Env = resolvedEnv
		}

		client := mcpclient.NewClient().WithTimeout(30 * time.Second)
		result := client.CallTool(context.Background(), &serverCopy, toolName, args)

		return toolExecutedMsg{
			toolName: toolName,
			result:   result,
		}
	}
}

// Resource CRUD commands
// These use tea.Exec to properly suspend the TUI while running interactive forms

// formExec wraps a function to implement tea.ExecCommand interface
type formExec struct {
	run func() error
}

func (f formExec) Run() error              { return f.run() }
func (f formExec) SetStdin(r io.Reader)    {}
func (f formExec) SetStdout(w io.Writer)   {}
func (f formExec) SetStderr(w io.Writer)   {}

func (m *Model) createServer() tea.Cmd {
	crud := m.resourceCRUD
	var createdName string
	return tea.Exec(formExec{run: func() error {
		s, err := crud.CreateServer()
		if err != nil {
			return err
		}
		createdName = s.Name
		return nil
	}}, func(err error) tea.Msg {
		if err != nil {
			return serverAddedMsg{name: "new server", err: err}
		}
		return serverAddedMsg{name: createdName, err: nil}
	})
}

func (m *Model) createCommand() tea.Cmd {
	crud := m.resourceCRUD
	var createdName string
	return tea.Exec(formExec{run: func() error {
		cmd, err := crud.CreateCommand()
		if err != nil {
			return err
		}
		createdName = cmd.Name
		return nil
	}}, func(err error) tea.Msg {
		if err != nil {
			return resourceCreatedMsg{resourceType: "command", err: err}
		}
		return resourceCreatedMsg{resourceType: "command", name: createdName}
	})
}

func (m *Model) editCommand(cmd *command.Command) tea.Cmd {
	crud := m.resourceCRUD
	c := exec.Command(getEditor(), filepath.Join(crud.cfg.ConfigDir, "commands", cmd.Name+".json"))
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return resourceEditedMsg{resourceType: "command", err: err}
	})
}

func (m *Model) deleteCommand(cmd *command.Command) tea.Cmd {
	crud := m.resourceCRUD
	cmdName := cmd.Name
	var deleted bool
	return tea.Exec(formExec{run: func() error {
		confirmed, err := ConfirmDelete("command", cmdName)
		if err != nil {
			return err
		}
		if !confirmed {
			return nil
		}
		deleted = true
		return crud.DeleteCommand(cmd)
	}}, func(err error) tea.Msg {
		if err != nil {
			return resourceDeletedMsg{resourceType: "command", err: err}
		}
		if !deleted {
			return resourceDeletedMsg{resourceType: "command", err: nil} // cancelled
		}
		return resourceDeletedMsg{resourceType: "command", name: cmdName}
	})
}

func (m *Model) createRule() tea.Cmd {
	crud := m.resourceCRUD
	var createdName string
	return tea.Exec(formExec{run: func() error {
		r, err := crud.CreateRule()
		if err != nil {
			return err
		}
		createdName = r.Name
		return nil
	}}, func(err error) tea.Msg {
		if err != nil {
			return resourceCreatedMsg{resourceType: "rule", err: err}
		}
		return resourceCreatedMsg{resourceType: "rule", name: createdName}
	})
}

func (m *Model) editRule(r *rule.Rule) tea.Cmd {
	c := exec.Command(getEditor(), r.Path)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return resourceEditedMsg{resourceType: "rule", err: err}
	})
}

func (m *Model) deleteRule(r *rule.Rule) tea.Cmd {
	crud := m.resourceCRUD
	ruleName := r.Name
	ruleRef := r
	var deleted bool
	return tea.Exec(formExec{run: func() error {
		confirmed, err := ConfirmDelete("rule", ruleName)
		if err != nil {
			return err
		}
		if !confirmed {
			return nil
		}
		deleted = true
		return crud.DeleteRule(ruleRef)
	}}, func(err error) tea.Msg {
		if err != nil {
			return resourceDeletedMsg{resourceType: "rule", err: err}
		}
		if !deleted {
			return resourceDeletedMsg{resourceType: "rule", err: nil}
		}
		return resourceDeletedMsg{resourceType: "rule", name: ruleName}
	})
}

func (m *Model) createSkill() tea.Cmd {
	crud := m.resourceCRUD
	var createdName string
	return tea.Exec(formExec{run: func() error {
		s, err := crud.CreateSkill()
		if err != nil {
			return err
		}
		createdName = s.Name
		return nil
	}}, func(err error) tea.Msg {
		if err != nil {
			return resourceCreatedMsg{resourceType: "skill", err: err}
		}
		return resourceCreatedMsg{resourceType: "skill", name: createdName}
	})
}

func (m *Model) editSkill(s *skill.Skill) tea.Cmd {
	mainPromptPath := filepath.Join(s.Path, "prompts", "main.md")
	c := exec.Command(getEditor(), mainPromptPath)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return resourceEditedMsg{resourceType: "skill", err: err}
	})
}

func (m *Model) deleteSkill(s *skill.Skill) tea.Cmd {
	crud := m.resourceCRUD
	skillName := s.Name
	skillRef := s
	var deleted bool
	return tea.Exec(formExec{run: func() error {
		confirmed, err := ConfirmDelete("skill", skillName)
		if err != nil {
			return err
		}
		if !confirmed {
			return nil
		}
		deleted = true
		return crud.DeleteSkill(skillRef)
	}}, func(err error) tea.Msg {
		if err != nil {
			return resourceDeletedMsg{resourceType: "skill", err: err}
		}
		if !deleted {
			return resourceDeletedMsg{resourceType: "skill", err: nil}
		}
		return resourceDeletedMsg{resourceType: "skill", name: skillName}
	})
}

func (m *Model) createPrompt() tea.Cmd {
	crud := m.resourceCRUD
	var createdName string
	return tea.Exec(formExec{run: func() error {
		p, err := crud.CreatePrompt()
		if err != nil {
			return err
		}
		createdName = p.Name
		return nil
	}}, func(err error) tea.Msg {
		if err != nil {
			return resourceCreatedMsg{resourceType: "prompt", err: err}
		}
		return resourceCreatedMsg{resourceType: "prompt", name: createdName}
	})
}

func (m *Model) editPrompt(p *prompt.Prompt) tea.Cmd {
	crud := m.resourceCRUD
	c := exec.Command(getEditor(), filepath.Join(crud.cfg.ConfigDir, "prompts", p.Name+".json"))
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return resourceEditedMsg{resourceType: "prompt", err: err}
	})
}

func (m *Model) deletePrompt(p *prompt.Prompt) tea.Cmd {
	crud := m.resourceCRUD
	promptName := p.Name
	promptRef := p
	var deleted bool
	return tea.Exec(formExec{run: func() error {
		confirmed, err := ConfirmDelete("prompt", promptName)
		if err != nil {
			return err
		}
		if !confirmed {
			return nil
		}
		deleted = true
		return crud.DeletePrompt(promptRef)
	}}, func(err error) tea.Msg {
		if err != nil {
			return resourceDeletedMsg{resourceType: "prompt", err: err}
		}
		if !deleted {
			return resourceDeletedMsg{resourceType: "prompt", err: nil}
		}
		return resourceDeletedMsg{resourceType: "prompt", name: promptName}
	})
}

// getEditor returns the user's preferred editor
func getEditor() string {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		// Try common editors
		for _, e := range []string{"code", "vim", "nano", "vi"} {
			if _, err := exec.LookPath(e); err == nil {
				editor = e
				break
			}
		}
	}
	if editor == "" {
		editor = "vi"
	}
	return editor
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
