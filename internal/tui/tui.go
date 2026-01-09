package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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

// FilterModeNames returns the display names for filter modes
var FilterModeNames = []string{"All", "Installed", "Available", "Disabled"}

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
	cursor     int
	filterMode FilterMode
	searchInput textinput.Model
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
	showToolModal   bool
	toolModalServer *Server // Server being tested
	toolCursor      int     // Selected tool index
	toolArgInput    textinput.Model
	toolResult      *mcpclient.ToolCallResult
	toolExecuting   bool

	// Rule editor modal
	showRuleEditor    bool
	ruleEditorIsNew   bool            // true if creating new, false if editing
	ruleEditorRule    *rule.Rule      // nil if new, existing rule if editing
	ruleEditorName    textinput.Model // Rule name (filename)
	ruleEditorApplies textinput.Model // Applies pattern (e.g., "*.go")
	ruleEditorContent textarea.Model  // Markdown content
	ruleEditorPriority int            // Priority (1-10)
	ruleEditorScope   int             // 0=global, 1=local (only shown when in project)
	ruleEditorFocus   int             // Which field is focused (0=name, 1=priority, 2=applies, 3=content, 4=scope when in project)

	// Confirm delete modal
	showConfirmDelete      bool
	confirmDeleteType      string // "server", "command", "rule", "skill", "prompt", "skill_command"
	confirmDeleteName      string
	confirmDeleteConfirmed bool
	confirmDeleteSkill     *skill.Skill   // Parent skill when deleting a skill command
	confirmDeleteCmd       *skill.Command // Command to delete (for skill_command type)

	// Prompt editor modal
	showPromptEditor     bool
	promptEditorIsNew    bool
	promptEditorPrompt   *prompt.Prompt
	promptEditorName     textinput.Model
	promptEditorDesc     textinput.Model
	promptEditorContent  textarea.Model
	promptEditorScope    int // 0=global, 1=local (only shown when in project)
	promptEditorFocus    int // 0=name, 1=desc, 2=content, 3=scope (when in project)

	// Skill editor modal
	showSkillEditor    bool
	skillEditorIsNew   bool
	skillEditorSkill   *skill.Skill
	skillEditorName    textinput.Model
	skillEditorDesc    textinput.Model
	skillEditorAuthor  textinput.Model
	skillEditorVersion textinput.Model
	skillEditorScope   int // 0=global, 1=local (only shown when in project)
	skillEditorFocus   int // 0=name, 1=desc, 2=author, 3=version, 4=scope (when in project)

	// Skill detail modal (shows commands/invocations)
	showSkillDetail    bool
	skillDetailSkill   *skill.Skill
	skillDetailCursor  int // For selecting commands within the skill

	// Skill command editor modal (for editing commands within a skill)
	showSkillCmdEditor    bool
	skillCmdEditorIsNew   bool
	skillCmdEditorSkill   *skill.Skill   // Parent skill
	skillCmdEditorCmd     *skill.Command // Command being edited (nil if new)
	skillCmdEditorName    textinput.Model
	skillCmdEditorDesc    textinput.Model
	skillCmdEditorContent textarea.Model
	skillCmdEditorFocus   int // 0=name, 1=desc, 2=content

	// Command editor modal
	showCommandEditor      bool
	commandEditorIsNew     bool
	commandEditorCommand   *command.Command
	commandEditorName      textinput.Model
	commandEditorDesc      textinput.Model
	commandEditorArgHint   textinput.Model
	commandEditorModel     int    // 0=default, 1=opus, 2=sonnet, 3=haiku
	commandEditorContent   textarea.Model
	commandEditorScope     int // 0=global, 1=local (only shown when in project)
	commandEditorFocus     int // 0=name, 1=desc, 2=argHint, 3=model, 4=content, 5=scope (when in project)

	// Server editor modal
	showServerEditor     bool
	serverEditorIsNew    bool
	serverEditorServer   *mcp.Server
	serverEditorName     textinput.Model
	serverEditorSource   textinput.Model // alias, URL, or path
	serverEditorCommand  textinput.Model
	serverEditorArgs     textinput.Model
	serverEditorTransport int // 0=stdio, 1=http, 2=sse
	serverEditorScope     int // 0=global, 1=local (only shown when in project)
	serverEditorFocus    int // 0=name, 1=source, 2=command, 3=args, 4=transport, 5=scope (when in project)

	// Alias wizard modal (multi-step)
	showAliasWizard      bool
	aliasWizardIsNew     bool
	aliasWizardStep      int // 0=basic, 1=type, 2=simple/variants config, 3=git url
	aliasWizardName      textinput.Model
	aliasWizardDesc      textinput.Model
	aliasWizardConfigType int // 0=simple, 1=variants
	aliasWizardTransport  int // 0=stdio, 1=http, 2=sse
	aliasWizardRuntime    int // 0=node, 1=python, 2=go, 3=docker
	aliasWizardPackage    textinput.Model
	aliasWizardURL        textinput.Model
	aliasWizardHasLocal   bool
	aliasWizardHasRemote  bool
	aliasWizardLocalRuntime int // 0=node, 1=python
	aliasWizardLocalPackage textinput.Model
	aliasWizardRemoteTransport int // 0=http, 1=sse
	aliasWizardRemoteURL  textinput.Model
	aliasWizardDefaultVariant int // 0=local, 1=remote
	aliasWizardWantGitURL bool
	aliasWizardGitURL     textinput.Model
	aliasWizardFocus      int // current focused field within step
	aliasWizardExisting   *aliases.Alias // for editing

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

	// Search input
	searchInput := textinput.New()
	searchInput.Placeholder = "Search..."
	searchInput.Prompt = "/ "
	searchInput.PromptStyle = SearchPromptStyle
	searchInput.TextStyle = SearchInputStyle
	searchInput.PlaceholderStyle = SearchPlaceholderStyle
	searchInput.CharLimit = 100

	// Tool argument input
	toolArgInput := textinput.New()
	toolArgInput.Placeholder = `{}`
	toolArgInput.Prompt = "Args (JSON): "
	toolArgInput.PromptStyle = lipgloss.NewStyle().Foreground(colorCyan)
	toolArgInput.CharLimit = 500

	// Rule editor inputs
	ruleEditorName := textinput.New()
	ruleEditorName.Placeholder = "my-rule"
	ruleEditorName.Prompt = ""
	ruleEditorName.CharLimit = 50

	ruleEditorApplies := textinput.New()
	ruleEditorApplies.Placeholder = "*.go, src/**/*.ts"
	ruleEditorApplies.Prompt = ""
	ruleEditorApplies.CharLimit = 100

	ruleEditorContent := textarea.New()
	ruleEditorContent.Placeholder = "Enter rule content in markdown..."
	ruleEditorContent.ShowLineNumbers = false
	ruleEditorContent.SetHeight(10)
	ruleEditorContent.SetWidth(60)
	ruleEditorContent.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ruleEditorContent.BlurredStyle.CursorLine = lipgloss.NewStyle()

	// Prompt editor inputs
	promptEditorName := textinput.New()
	promptEditorName.Placeholder = "my-prompt"
	promptEditorName.Prompt = ""
	promptEditorName.CharLimit = 50

	promptEditorDesc := textinput.New()
	promptEditorDesc.Placeholder = "Description of this prompt"
	promptEditorDesc.Prompt = ""
	promptEditorDesc.CharLimit = 200

	promptEditorContent := textarea.New()
	promptEditorContent.Placeholder = "You are a {{role}} expert.\n\nAnalyze: {{input}}"
	promptEditorContent.ShowLineNumbers = false
	promptEditorContent.SetHeight(10)
	promptEditorContent.SetWidth(60)
	promptEditorContent.FocusedStyle.CursorLine = lipgloss.NewStyle()
	promptEditorContent.BlurredStyle.CursorLine = lipgloss.NewStyle()

	// Skill editor inputs
	skillEditorName := textinput.New()
	skillEditorName.Placeholder = "my-skill"
	skillEditorName.Prompt = ""
	skillEditorName.CharLimit = 50

	skillEditorDesc := textinput.New()
	skillEditorDesc.Placeholder = "What this skill does"
	skillEditorDesc.Prompt = ""
	skillEditorDesc.CharLimit = 200

	skillEditorAuthor := textinput.New()
	skillEditorAuthor.Placeholder = "Your name"
	skillEditorAuthor.Prompt = ""
	skillEditorAuthor.CharLimit = 100

	skillEditorVersion := textinput.New()
	skillEditorVersion.Placeholder = "1.0.0"
	skillEditorVersion.Prompt = ""
	skillEditorVersion.CharLimit = 20

	// Skill command editor inputs (for commands within a skill)
	skillCmdEditorName := textinput.New()
	skillCmdEditorName.Placeholder = "review"
	skillCmdEditorName.Prompt = ""
	skillCmdEditorName.CharLimit = 50

	skillCmdEditorDesc := textinput.New()
	skillCmdEditorDesc.Placeholder = "What this command does"
	skillCmdEditorDesc.Prompt = ""
	skillCmdEditorDesc.CharLimit = 200

	skillCmdEditorContent := textarea.New()
	skillCmdEditorContent.Placeholder = "Write the prompt for this command...\n\nUse $ARGUMENTS for user input."
	skillCmdEditorContent.ShowLineNumbers = false
	skillCmdEditorContent.SetHeight(12)
	skillCmdEditorContent.SetWidth(60)
	skillCmdEditorContent.FocusedStyle.CursorLine = lipgloss.NewStyle()
	skillCmdEditorContent.BlurredStyle.CursorLine = lipgloss.NewStyle()

	// Command editor inputs
	commandEditorName := textinput.New()
	commandEditorName.Placeholder = "my-command"
	commandEditorName.Prompt = ""
	commandEditorName.CharLimit = 50

	commandEditorDesc := textinput.New()
	commandEditorDesc.Placeholder = "What this command does"
	commandEditorDesc.Prompt = ""
	commandEditorDesc.CharLimit = 200

	commandEditorArgHint := textinput.New()
	commandEditorArgHint.Placeholder = "[file or description]"
	commandEditorArgHint.Prompt = ""
	commandEditorArgHint.CharLimit = 100

	commandEditorContent := textarea.New()
	commandEditorContent.Placeholder = "Review this code for...\n\n$ARGUMENTS"
	commandEditorContent.ShowLineNumbers = false
	commandEditorContent.SetHeight(10)
	commandEditorContent.SetWidth(60)
	commandEditorContent.FocusedStyle.CursorLine = lipgloss.NewStyle()
	commandEditorContent.BlurredStyle.CursorLine = lipgloss.NewStyle()

	// Server editor inputs
	serverEditorName := textinput.New()
	serverEditorName.Placeholder = "my-server"
	serverEditorName.Prompt = ""
	serverEditorName.CharLimit = 50

	serverEditorSource := textinput.New()
	serverEditorSource.Placeholder = "alias, URL, or ./path"
	serverEditorSource.Prompt = ""
	serverEditorSource.CharLimit = 200

	serverEditorCommand := textinput.New()
	serverEditorCommand.Placeholder = "npx"
	serverEditorCommand.Prompt = ""
	serverEditorCommand.CharLimit = 100

	serverEditorArgs := textinput.New()
	serverEditorArgs.Placeholder = "-y @modelcontextprotocol/server-filesystem"
	serverEditorArgs.Prompt = ""
	serverEditorArgs.CharLimit = 500

	// Alias wizard inputs
	aliasWizardName := textinput.New()
	aliasWizardName.Placeholder = "my-alias"
	aliasWizardName.Prompt = ""
	aliasWizardName.CharLimit = 50

	aliasWizardDesc := textinput.New()
	aliasWizardDesc.Placeholder = "Description of the MCP server"
	aliasWizardDesc.Prompt = ""
	aliasWizardDesc.CharLimit = 200

	aliasWizardPackage := textinput.New()
	aliasWizardPackage.Placeholder = "@org/mcp-server"
	aliasWizardPackage.Prompt = ""
	aliasWizardPackage.CharLimit = 200

	aliasWizardURL := textinput.New()
	aliasWizardURL.Placeholder = "https://mcp.example.com/mcp"
	aliasWizardURL.Prompt = ""
	aliasWizardURL.CharLimit = 300

	aliasWizardLocalPackage := textinput.New()
	aliasWizardLocalPackage.Placeholder = "@org/mcp-server"
	aliasWizardLocalPackage.Prompt = ""
	aliasWizardLocalPackage.CharLimit = 200

	aliasWizardRemoteURL := textinput.New()
	aliasWizardRemoteURL.Placeholder = "https://mcp.example.com/mcp"
	aliasWizardRemoteURL.Prompt = ""
	aliasWizardRemoteURL.CharLimit = 300

	aliasWizardGitURL := textinput.New()
	aliasWizardGitURL.Placeholder = "github.com/org/repo"
	aliasWizardGitURL.Prompt = ""
	aliasWizardGitURL.CharLimit = 200

	m := &Model{
		cfg:               cfg,
		selected:          make(map[string]bool),
		filterMode:        FilterAll,
		profile:           "default",
		logs:              []LogEntry{},
		keys:              newKeyMap(),
		spinner:           s,
		searchInput:       searchInput,
		toolArgInput:      toolArgInput,
		// Rule editor
		ruleEditorName:    ruleEditorName,
		ruleEditorApplies: ruleEditorApplies,
		ruleEditorContent: ruleEditorContent,
		ruleEditorPriority: 3,
		// Prompt editor
		promptEditorName:    promptEditorName,
		promptEditorDesc:    promptEditorDesc,
		promptEditorContent: promptEditorContent,
		// Skill editor
		skillEditorName:    skillEditorName,
		skillEditorDesc:    skillEditorDesc,
		skillEditorAuthor:  skillEditorAuthor,
		skillEditorVersion: skillEditorVersion,
		// Skill command editor
		skillCmdEditorName:    skillCmdEditorName,
		skillCmdEditorDesc:    skillCmdEditorDesc,
		skillCmdEditorContent: skillCmdEditorContent,
		// Command editor
		commandEditorName:    commandEditorName,
		commandEditorDesc:    commandEditorDesc,
		commandEditorArgHint: commandEditorArgHint,
		commandEditorContent: commandEditorContent,
		// Server editor
		serverEditorName:    serverEditorName,
		serverEditorSource:  serverEditorSource,
		serverEditorCommand: serverEditorCommand,
		serverEditorArgs:    serverEditorArgs,
		// Alias wizard
		aliasWizardName:         aliasWizardName,
		aliasWizardDesc:         aliasWizardDesc,
		aliasWizardPackage:      aliasWizardPackage,
		aliasWizardURL:          aliasWizardURL,
		aliasWizardLocalPackage: aliasWizardLocalPackage,
		aliasWizardRemoteURL:    aliasWizardRemoteURL,
		aliasWizardGitURL:       aliasWizardGitURL,
		resourceCRUD:            NewResourceCRUD(cfg),
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

// loadAllResources loads commands, rules, skills, and prompts from config
// This uses the config's already-loaded resources which include both global and local scopes
func (m *Model) loadAllResources() {
	// Load commands from config (includes both global and local)
	m.commands = m.cfg.CommandsForScope(config.ScopeAll)

	// Load rules from config (includes both global and local)
	m.rules = m.cfg.RulesForScope(config.ScopeAll)

	// Load skills from config (includes both global and local)
	m.skills = m.cfg.SkillsForScope(config.ScopeAll)

	// Load prompts from config (includes both global and local)
	m.prompts = m.cfg.PromptsForScope(config.ScopeAll)

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
		if m.searchInput.Value() != "" {
			if !strings.Contains(strings.ToLower(s.Name), strings.ToLower(m.searchInput.Value())) &&
				!strings.Contains(strings.ToLower(s.Desc), strings.ToLower(m.searchInput.Value())) {
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
			m.addLog("error", fmt.Sprintf("Failed to add server: %v", msg.err))
		} else {
			scopeLabel := ""
			if msg.scope != "" {
				scopeLabel = fmt.Sprintf(" (%s)", msg.scope)
			}
			m.addLog("success", fmt.Sprintf("Added server: %s%s", msg.name, scopeLabel))
			m.buildServerList()
			m.applyFilter()
			m.showServerEditor = false
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
			scopeLabel := ""
			if msg.scope != "" {
				scopeLabel = fmt.Sprintf(" (%s)", msg.scope)
			}
			m.addLog("success", fmt.Sprintf("Created %s: %s%s", msg.resourceType, msg.name, scopeLabel))
			m.loadAllResources()
			// Close editor modal on successful save
			switch msg.resourceType {
			case "rule":
				m.showRuleEditor = false
			case "prompt":
				m.showPromptEditor = false
			case "skill":
				m.showSkillEditor = false
			case "command":
				m.showCommandEditor = false
			}
		}

	case resourceEditedMsg:
		if msg.err != nil {
			m.addLog("error", fmt.Sprintf("Failed to edit %s: %v", msg.resourceType, msg.err))
		} else {
			m.addLog("success", fmt.Sprintf("Edited %s", msg.resourceType))
			m.loadAllResources()
			// Close editor modal on successful save
			switch msg.resourceType {
			case "rule":
				m.showRuleEditor = false
			case "prompt":
				m.showPromptEditor = false
			case "skill":
				m.showSkillEditor = false
			case "command":
				m.showCommandEditor = false
			}
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

	case skillCmdSavedMsg:
		action := "Created"
		if !msg.isNew {
			action = "Updated"
		}
		m.addLog("success", fmt.Sprintf("%s command: %s:%s", action, msg.skill.Name, msg.cmdName))
		m.showSkillCmdEditor = false
		// Re-open skill detail to show the updated commands
		m.openSkillDetail(msg.skill)

	case skillCmdDeletedMsg:
		m.addLog("success", fmt.Sprintf("Deleted command: %s:%s", msg.skill.Name, msg.cmdName))
		// Adjust cursor if needed
		if m.skillDetailCursor > 0 {
			m.skillDetailCursor--
		}
		// Re-open skill detail to show updated commands
		m.openSkillDetail(msg.skill)

	case tea.KeyMsg:
		// Handle search input mode
		if m.searching {
			switch msg.String() {
			case "esc":
				m.searching = false
				m.searchInput.SetValue("")
				m.searchInput.Blur()
				m.applyFilter()
			case "enter":
				m.searching = false
				m.searchInput.Blur()
			default:
				// Delegate to textinput
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				m.applyFilter()
				return m, cmd
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
				m.toolArgInput.SetValue("")
				m.toolArgInput.Blur()
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
			case "tab":
				// Toggle focus between tool list and arg input
				if m.toolArgInput.Focused() {
					m.toolArgInput.Blur()
				} else {
					m.toolArgInput.Focus()
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
					argValue := m.toolArgInput.Value()
					if argValue != "" {
						// Try to parse as JSON
						if err := json.Unmarshal([]byte(argValue), &args); err != nil {
							m.addLog("error", fmt.Sprintf("Invalid JSON args: %v", err))
							m.toolExecuting = false
							return m, nil
						}
					} else {
						args = make(map[string]any)
					}
					return m, m.executeTool(m.toolModalServer.Name, tool.Name, args)
				}
			default:
				// If arg input is focused, delegate to textinput
				if m.toolArgInput.Focused() {
					var cmd tea.Cmd
					m.toolArgInput, cmd = m.toolArgInput.Update(msg)
					return m, cmd
				}
			}
			return m, nil
		}

		// Handle rule editor modal
		if m.showRuleEditor {
			return m.handleRuleEditorInput(msg)
		}

		// Handle confirm delete modal
		if m.showConfirmDelete {
			return m.handleConfirmDeleteInput(msg)
		}

		// Handle prompt editor modal
		if m.showPromptEditor {
			return m.handlePromptEditorInput(msg)
		}

		// Handle skill editor modal
		if m.showSkillEditor {
			return m.handleSkillEditorInput(msg)
		}

		// Handle skill detail modal
		if m.showSkillDetail {
			return m.handleSkillDetailInput(msg)
		}

		// Handle skill command editor modal
		if m.showSkillCmdEditor {
			return m.handleSkillCmdEditorInput(msg)
		}

		// Handle command editor modal
		if m.showCommandEditor {
			return m.handleCommandEditorInput(msg)
		}

		// Handle server editor modal
		if m.showServerEditor {
			return m.handleServerEditorInput(msg)
		}

		// Handle alias wizard modal
		if m.showAliasWizard {
			return m.handleAliasWizardInput(msg)
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
			if m.cursor < m.currentTabLength()-1 {
				m.cursor++
			}

		case key.Matches(msg, m.keys.Top):
			m.cursor = 0

		case key.Matches(msg, m.keys.Bottom):
			m.cursor = max(0, m.currentTabLength()-1)

		case key.Matches(msg, m.keys.PageDown):
			m.cursor = min(m.cursor+10, m.currentTabLength()-1)

		case key.Matches(msg, m.keys.PageUp):
			m.cursor = max(m.cursor-10, 0)

		case key.Matches(msg, m.keys.Search):
			m.searching = true
			m.searchInput.SetValue("")
			m.searchInput.Focus()

		case key.Matches(msg, m.keys.Escape):
			m.searchInput.SetValue("")
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
				m.openServerEditor(nil)
				return m, nil
			case TabCommands:
				m.openCommandEditor(nil)
				return m, nil
			case TabRules:
				m.openRuleEditor(nil)
				return m, nil
			case TabSkills:
				m.openSkillEditor(nil)
				return m, nil
			case TabPrompts:
				m.openPromptEditor(nil)
				return m, nil
			}

		case key.Matches(msg, m.keys.Delete):
			switch m.activeTab {
			case TabServers:
				if s := m.selectedServer(); s != nil && s.Status != ServerStatusAvailable {
					m.openConfirmDelete("server", s.Name)
					return m, nil
				}
			case TabCommands:
				if m.cursor >= 0 && m.cursor < len(m.commands) {
					cmd := m.commands[m.cursor]
					m.openConfirmDelete("command", cmd.Name)
					return m, nil
				}
			case TabRules:
				if m.cursor >= 0 && m.cursor < len(m.rules) {
					r := m.rules[m.cursor]
					m.openConfirmDelete("rule", r.Name)
					return m, nil
				}
			case TabSkills:
				if m.cursor >= 0 && m.cursor < len(m.skills) {
					s := m.skills[m.cursor]
					m.openConfirmDelete("skill", s.Name)
					return m, nil
				}
			case TabPrompts:
				if m.cursor >= 0 && m.cursor < len(m.prompts) {
					p := m.prompts[m.cursor]
					m.openConfirmDelete("prompt", p.Name)
					return m, nil
				}
			}

		case key.Matches(msg, m.keys.Edit):
			switch m.activeTab {
			case TabServers:
				if s := m.selectedServer(); s != nil && s.Status != ServerStatusAvailable && s.ServerConfig != nil {
					m.openServerEditor(s.ServerConfig)
					return m, nil
				}
			case TabCommands:
				if m.cursor >= 0 && m.cursor < len(m.commands) {
					cmd := m.commands[m.cursor]
					m.openCommandEditor(cmd)
					return m, nil
				}
			case TabRules:
				if m.cursor >= 0 && m.cursor < len(m.rules) {
					r := m.rules[m.cursor]
					m.openRuleEditor(r)
					return m, nil
				}
			case TabSkills:
				if m.cursor >= 0 && m.cursor < len(m.skills) {
					s := m.skills[m.cursor]
					m.openSkillEditor(s)
					return m, nil
				}
			case TabPrompts:
				if m.cursor >= 0 && m.cursor < len(m.prompts) {
					p := m.prompts[m.cursor]
					m.openPromptEditor(p)
					return m, nil
				}
			}

		case key.Matches(msg, m.keys.Toggle):
			switch m.activeTab {
			case TabServers:
				if s := m.selectedServer(); s != nil && s.Status != ServerStatusAvailable {
					return m, m.toggleServer(s.Name)
				}
			case TabSkills:
				if m.cursor >= 0 && m.cursor < len(m.skills) {
					s := m.skills[m.cursor]
					m.openSkillDetail(s)
					return m, nil
				}
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
					m.toolArgInput.SetValue("")
					m.toolArgInput.Blur()
					m.toolResult = nil
				}
			}

		case key.Matches(msg, m.keys.Refresh):
			m.buildServerList()
			m.applyFilter()
			m.addLog("info", "Refreshed server list")

		case key.Matches(msg, m.keys.CycleFilter):
			m.filterMode = (m.filterMode + 1) % FilterMode(len(FilterModeNames))
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
			m.activeTab = (m.activeTab + 1) % ResourceTab(len(TabNames))
			m.cursor = 0

		case key.Matches(msg, m.keys.PrevTab):
			if m.activeTab == 0 {
				m.activeTab = ResourceTab(len(TabNames) - 1)
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

	if m.showRuleEditor {
		return m.renderRuleEditor()
	}

	if m.showConfirmDelete {
		return m.renderConfirmDelete()
	}

	if m.showPromptEditor {
		return m.renderPromptEditor()
	}

	if m.showSkillEditor {
		return m.renderSkillEditor()
	}

	if m.showSkillDetail {
		return m.renderSkillDetail()
	}

	if m.showSkillCmdEditor {
		return m.renderSkillCmdEditor()
	}

	if m.showCommandEditor {
		return m.renderCommandEditor()
	}

	if m.showServerEditor {
		return m.renderServerEditor()
	}

	if m.showAliasWizard {
		return m.renderAliasWizard()
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
		rows = append(rows, SearchStyle.Width(m.width-4).Render(m.searchInput.View()))
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

	// Scope indicator (only for installed servers)
	scopeIndicator := ""
	if s.ServerConfig != nil && s.ServerConfig.Scope != "" {
		scopeIndicator = " " + RenderScopeIndicator(s.ServerConfig.Scope)
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
	desc := descStyle.Render(s.Transport + " · " + ansi.Truncate(s.Description(), 50, "..."))

	// Build the row
	leftPart := selectIndicator + statusBadge + scopeIndicator + " " + name + healthBadge

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

	// Scope indicator
	scopeIndicator := RenderScopeIndicator(cmd.Scope)

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
	desc := descStyle.Render(ansi.Truncate(cmd.Description, 50, "..."))

	// Build the row
	leftPart := "  " + icon + " " + scopeIndicator + " " + name

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

	// Scope indicator
	scopeIndicator := RenderScopeIndicator(r.Scope)

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
	desc := descStyle.Render(ansi.Truncate(descText, 50, "..."))

	// Build the row
	leftPart := "  " + icon + " " + scopeIndicator + " " + name

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

	// Scope indicator
	scopeIndicator := RenderScopeIndicator(s.Scope)

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

	// Command count badge
	cmdBadge := ""
	cmdCount := len(s.Commands)
	if s.Content != "" {
		cmdCount++ // Include the default command
	}
	if cmdCount > 1 {
		cmdBadge = lipgloss.NewStyle().Foreground(colorPink).Render(fmt.Sprintf(" (%d cmds)", cmdCount))
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
	desc := descStyle.Render(ansi.Truncate(descText, 35, "..."))

	// Build the row
	leftPart := "  " + icon + " " + scopeIndicator + " " + name + versionBadge + cmdBadge

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

	// Scope indicator
	scopeIndicator := RenderScopeIndicator(p.Scope)

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
		descText = ansi.Truncate(strings.ReplaceAll(p.Template, "\n", " "), 40, "...")
	}
	desc := descStyle.Render(ansi.Truncate(descText, 40, "..."))

	// Build the row
	leftPart := "  " + icon + " " + scopeIndicator + " " + name + varsBadge

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
	cmdText := ansi.Truncate(h.Command, 40, "...")
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

	modal := ModalStyle.
		Width(60).
		Render(ModalTitleStyle.Render("Keyboard Shortcuts") + content)

	// Center the modal in the terminal
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
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

	modal := ModalStyle.
		Width(40).
		Render(ModalTitleStyle.Render("Switch Profile") + "\n\n" + content + hints)

	// Center the modal in the terminal
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
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
			desc := ansi.Truncate(tool.Description, 40, "...")

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
	argInputView := m.toolArgInput.View()
	if m.toolExecuting {
		argInputView += " " + m.spinner.View()
	}
	focusHint := ""
	if !m.toolArgInput.Focused() {
		focusHint = lipgloss.NewStyle().Foreground(colorFgSubtle).Render(" (Tab to edit)")
	}
	sections = append(sections, argInputView+focusHint)

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
				sections = append(sections, "  "+ansi.Truncate(content, 60, "..."))
			}
		} else {
			successStyle := lipgloss.NewStyle().Foreground(colorTeal)
			sections = append(sections, successStyle.Render(fmt.Sprintf("Success (%s):", m.toolResult.Latency.Round(time.Millisecond))))
			for _, content := range m.toolResult.Content {
				// Wrap long content
				lines := strings.Split(content, "\n")
				for _, line := range lines {
					sections = append(sections, "  "+ansi.Truncate(line, 60, "..."))
					if len(sections) > 20 {
						sections = append(sections, "  ...")
						break
					}
				}
			}
		}
	}

	sections = append(sections, "")
	hints := KeyDescStyle.Render("j/k:select  Tab:edit args  Enter:execute  Esc:close")
	sections = append(sections, hints)

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	modal := ModalStyle.
		Width(70).
		Render(content)

	// Center the modal in the terminal
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
}

// renderRuleEditor renders the rule editor modal
func (m *Model) renderRuleEditor() string {
	var sections []string

	// Title
	title := "Create New Rule"
	if !m.ruleEditorIsNew {
		title = "Edit Rule"
	}
	sections = append(sections, ModalTitleStyle.Render(title))
	sections = append(sections, "")

	// Field labels and inputs
	focusedStyle := lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(colorFgMuted)
	valueStyle := lipgloss.NewStyle().Foreground(colorFg)

	// Name field (only editable for new rules)
	nameLabel := labelStyle.Render("Name:")
	if m.ruleEditorFocus == 0 {
		nameLabel = focusedStyle.Render("Name:")
	}
	nameValue := m.ruleEditorName.View()
	if !m.ruleEditorIsNew {
		nameValue = valueStyle.Render(m.ruleEditorRule.Name + " (readonly)")
	}
	sections = append(sections, nameLabel+" "+nameValue)

	// Priority field
	priorityLabel := labelStyle.Render("Priority:")
	if m.ruleEditorFocus == 1 {
		priorityLabel = focusedStyle.Render("Priority:")
	}
	priorityNames := []string{"1-Low", "2", "3-Normal", "4", "5-Medium", "6", "7-High", "8", "9", "10-Critical"}
	priorityDisplay := priorityNames[m.ruleEditorPriority-1]
	priorityValue := valueStyle.Render(fmt.Sprintf("< %s >", priorityDisplay))
	sections = append(sections, priorityLabel+" "+priorityValue)

	// Applies pattern field
	appliesLabel := labelStyle.Render("Applies:")
	if m.ruleEditorFocus == 2 {
		appliesLabel = focusedStyle.Render("Applies:")
	}
	sections = append(sections, appliesLabel+" "+m.ruleEditorApplies.View())

	sections = append(sections, "")

	// Content field
	contentLabel := labelStyle.Render("Content (Markdown):")
	if m.ruleEditorFocus == 3 {
		contentLabel = focusedStyle.Render("Content (Markdown):")
	}
	sections = append(sections, contentLabel)

	// Style the textarea border based on focus
	contentStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorFgSubtle).
		Padding(0, 1)
	if m.ruleEditorFocus == 3 {
		contentStyle = contentStyle.BorderForeground(colorCyan)
	}
	sections = append(sections, contentStyle.Render(m.ruleEditorContent.View()))

	sections = append(sections, "")

	// Scope selector - always shown with icons
	scopeLabel := labelStyle.Render("Scope:")
	if m.ruleEditorFocus == 4 && m.hasProjectConfig() {
		scopeLabel = focusedStyle.Render("Scope:")
	}
	var scopeOpts strings.Builder
	scopeIcons := []string{"🌐", "📁"}
	scopeLabels := []string{"global", "local (project)"}

	if m.hasProjectConfig() {
		for i, label := range scopeLabels {
			icon := scopeIcons[i]
			if i == m.ruleEditorScope {
				scopeOpts.WriteString(lipgloss.NewStyle().Background(colorCyan).Foreground(colorBg).Bold(true).Render(" " + icon + " " + label + " "))
			} else {
				scopeOpts.WriteString(lipgloss.NewStyle().Foreground(colorFgMuted).Render(" " + icon + " " + label + " "))
			}
		}
	} else {
		scopeOpts.WriteString(lipgloss.NewStyle().Foreground(colorTeal).Render("🌐 global"))
		scopeOpts.WriteString(lipgloss.NewStyle().Foreground(colorFgSubtle).Italic(true).Render(" (open a project for local scope)"))
	}
	sections = append(sections, scopeLabel+" "+scopeOpts.String())
	sections = append(sections, "")

	// Hints
	hints := KeyDescStyle.Render("Tab:next field  Shift+Tab:prev  Ctrl+S:save  e:external editor  Esc:cancel")
	if m.ruleEditorFocus == 1 || m.ruleEditorFocus == 4 {
		hints = KeyDescStyle.Render("←/→:change selection  Tab:next  Ctrl+S:save  Esc:cancel")
	}
	sections = append(sections, hints)

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	modal := ModalStyle.
		Width(70).
		Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
}

// handleRuleEditorInput handles keyboard input for the rule editor modal
func (m *Model) handleRuleEditorInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.showRuleEditor = false
		m.ruleEditorRule = nil
		return m, nil

	case "ctrl+s":
		// Save the rule
		return m, m.saveRule()

	case "e":
		// Open in external editor (only if editing existing rule)
		if !m.ruleEditorIsNew && m.ruleEditorRule != nil {
			m.showRuleEditor = false
			return m, m.editRuleExternal(m.ruleEditorRule)
		}

	case "tab":
		// Move to next field
		m.cycleRuleEditorFocus(1)
		return m, nil

	case "shift+tab":
		// Move to previous field
		m.cycleRuleEditorFocus(-1)
		return m, nil

	case "left":
		// Decrease priority if on priority field
		if m.ruleEditorFocus == 1 && m.ruleEditorPriority > 1 {
			m.ruleEditorPriority--
		}
		// Cycle scope if on scope field
		if m.ruleEditorFocus == 4 && m.hasProjectConfig() {
			m.ruleEditorScope = (m.ruleEditorScope - 1 + len(scopeNames)) % len(scopeNames)
		}
		return m, nil

	case "right":
		// Increase priority if on priority field
		if m.ruleEditorFocus == 1 && m.ruleEditorPriority < 10 {
			m.ruleEditorPriority++
		}
		// Cycle scope if on scope field
		if m.ruleEditorFocus == 4 && m.hasProjectConfig() {
			m.ruleEditorScope = (m.ruleEditorScope + 1) % len(scopeNames)
		}
		return m, nil

	default:
		// Delegate to the focused input
		var cmd tea.Cmd
		switch m.ruleEditorFocus {
		case 0: // Name field (only for new rules)
			if m.ruleEditorIsNew {
				m.ruleEditorName, cmd = m.ruleEditorName.Update(msg)
			}
		case 2: // Applies field
			m.ruleEditorApplies, cmd = m.ruleEditorApplies.Update(msg)
		case 3: // Content field
			m.ruleEditorContent, cmd = m.ruleEditorContent.Update(msg)
		}
		return m, cmd
	}

	return m, nil
}

// cycleRuleEditorFocus moves focus between rule editor fields
func (m *Model) cycleRuleEditorFocus(delta int) {
	// Blur current field
	switch m.ruleEditorFocus {
	case 0:
		m.ruleEditorName.Blur()
	case 2:
		m.ruleEditorApplies.Blur()
	case 3:
		m.ruleEditorContent.Blur()
	}

	// Calculate new focus (skip name field if editing existing rule)
	maxFocus := 3
	if m.hasProjectConfig() {
		maxFocus = 4 // Include scope field when in project
	}
	minFocus := 0
	if !m.ruleEditorIsNew {
		minFocus = 1 // Skip name field
	}

	m.ruleEditorFocus += delta
	if m.ruleEditorFocus > maxFocus {
		m.ruleEditorFocus = minFocus
	} else if m.ruleEditorFocus < minFocus {
		m.ruleEditorFocus = maxFocus
	}

	// Focus new field
	switch m.ruleEditorFocus {
	case 0:
		m.ruleEditorName.Focus()
	case 2:
		m.ruleEditorApplies.Focus()
	case 3:
		m.ruleEditorContent.Focus()
	}
}

// openRuleEditor opens the rule editor for a new or existing rule
func (m *Model) openRuleEditor(r *rule.Rule) {
	m.showRuleEditor = true
	m.ruleEditorRule = r
	m.ruleEditorIsNew = (r == nil)

	if r == nil {
		// New rule - reset all fields
		m.ruleEditorName.SetValue("")
		m.ruleEditorApplies.SetValue("*")
		m.ruleEditorContent.SetValue("# Rule Title\n\nDescribe your guidelines here.\n\n## Guidelines\n\n- Guideline 1\n- Guideline 2\n")
		m.ruleEditorPriority = 3
		m.ruleEditorScope = m.defaultScopeIndex()
		m.ruleEditorFocus = 0
		m.ruleEditorName.Focus()
	} else {
		// Editing existing rule - populate fields
		m.ruleEditorName.SetValue(r.Name)
		if r.Frontmatter != nil {
			m.ruleEditorPriority = r.Frontmatter.Priority
			if m.ruleEditorPriority < 1 {
				m.ruleEditorPriority = 1
			}
			if m.ruleEditorPriority > 10 {
				m.ruleEditorPriority = 10
			}
			m.ruleEditorApplies.SetValue(r.Frontmatter.Applies)
		} else {
			m.ruleEditorPriority = 3
			m.ruleEditorApplies.SetValue("*")
		}
		m.ruleEditorContent.SetValue(r.Content)
		// Set scope based on existing rule
		m.ruleEditorScope = scopeIndexGlobal
		if r.Scope == "local" {
			m.ruleEditorScope = scopeIndexLocal
		}
		m.ruleEditorFocus = 1 // Start on priority since name is readonly
		m.ruleEditorApplies.Blur()
		m.ruleEditorContent.Blur()
	}
}

// saveRule saves the rule from the editor
func (m *Model) saveRule() tea.Cmd {
	return func() tea.Msg {
		// Determine scope and resource directory
		scope := config.ScopeGlobal
		resourceDir := m.cfg.ConfigDir
		if m.ruleEditorScope == scopeIndexLocal {
			scope = config.ScopeLocal
			// Use project's .agentctl directory for local scope
			if m.cfg.ProjectPath != "" {
				resourceDir = filepath.Join(filepath.Dir(m.cfg.ProjectPath), ".agentctl")
			}
		}

		rulesDir := filepath.Join(resourceDir, "rules")

		// Ensure directory exists
		if err := os.MkdirAll(rulesDir, 0755); err != nil {
			return resourceCreatedMsg{resourceType: "rule", err: fmt.Errorf("failed to create rules directory: %w", err)}
		}

		// Get values
		name := m.ruleEditorName.Value()
		if !m.ruleEditorIsNew && m.ruleEditorRule != nil {
			name = m.ruleEditorRule.Name
		}

		if name == "" {
			return resourceCreatedMsg{resourceType: "rule", err: fmt.Errorf("rule name is required")}
		}

		// Validate name for new rules
		if m.ruleEditorIsNew && strings.ContainsAny(name, " \t\n/\\") {
			return resourceCreatedMsg{resourceType: "rule", err: fmt.Errorf("name cannot contain spaces or path separators")}
		}

		applies := m.ruleEditorApplies.Value()
		if applies == "" {
			applies = "*"
		}

		content := m.ruleEditorContent.Value()

		// Build the rule file content with frontmatter
		var fileContent strings.Builder
		fileContent.WriteString("---\n")
		fileContent.WriteString(fmt.Sprintf("priority: %d\n", m.ruleEditorPriority))
		fileContent.WriteString(fmt.Sprintf("applies: \"%s\"\n", applies))
		fileContent.WriteString("---\n\n")
		fileContent.WriteString(content)

		// Determine path
		rulePath := filepath.Join(rulesDir, name+".md")
		if !m.ruleEditorIsNew && m.ruleEditorRule != nil {
			rulePath = m.ruleEditorRule.Path
		}

		// Check if file exists for new rules
		if m.ruleEditorIsNew {
			if _, err := os.Stat(rulePath); err == nil {
				return resourceCreatedMsg{resourceType: "rule", err: fmt.Errorf("rule %q already exists", name)}
			}
		}

		// Write the file
		if err := os.WriteFile(rulePath, []byte(fileContent.String()), 0644); err != nil {
			return resourceCreatedMsg{resourceType: "rule", err: fmt.Errorf("failed to save rule: %w", err)}
		}

		if m.ruleEditorIsNew {
			return resourceCreatedMsg{resourceType: "rule", name: name, scope: string(scope)}
		}
		return resourceEditedMsg{resourceType: "rule"}
	}
}

// editRuleExternal opens a rule in the external editor
func (m *Model) editRuleExternal(r *rule.Rule) tea.Cmd {
	c := exec.Command(getEditor(), r.Path)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return resourceEditedMsg{resourceType: "rule", err: err}
	})
}

// ============================================================================
// Confirm Delete Modal
// ============================================================================

func (m *Model) renderConfirmDelete() string {
	title := fmt.Sprintf("Delete %s?", m.confirmDeleteType)

	var content strings.Builder
	content.WriteString(ModalTitleStyle.Render(title))
	content.WriteString("\n\n")
	content.WriteString(fmt.Sprintf("Are you sure you want to delete %q?\n", m.confirmDeleteName))
	content.WriteString(lipgloss.NewStyle().Foreground(colorPink).Render("This action cannot be undone."))
	content.WriteString("\n\n")

	yesStyle := lipgloss.NewStyle().Foreground(colorPink).Bold(true)
	noStyle := lipgloss.NewStyle().Foreground(colorTeal).Bold(true)

	if m.confirmDeleteConfirmed {
		content.WriteString(yesStyle.Render("[Y] Delete"))
		content.WriteString("  ")
		content.WriteString(lipgloss.NewStyle().Faint(true).Render("[N] Cancel"))
	} else {
		content.WriteString(lipgloss.NewStyle().Faint(true).Render("[Y] Delete"))
		content.WriteString("  ")
		content.WriteString(noStyle.Render("[N] Cancel"))
	}
	content.WriteString("\n\n")
	content.WriteString(KeyDescStyle.Render("←/→ to select • Enter to confirm • Esc to cancel"))

	modal := ModalStyle.Width(50).Render(content.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
}

func (m *Model) handleConfirmDeleteInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.showConfirmDelete = false
		return m, nil
	case "left", "right", "h", "l", "tab":
		m.confirmDeleteConfirmed = !m.confirmDeleteConfirmed
		return m, nil
	case "y", "Y":
		m.confirmDeleteConfirmed = true
		return m, m.executeDelete()
	case "n", "N":
		m.showConfirmDelete = false
		return m, nil
	case "enter":
		if m.confirmDeleteConfirmed {
			return m, m.executeDelete()
		}
		m.showConfirmDelete = false
		return m, nil
	}
	return m, nil
}

func (m *Model) openConfirmDelete(resourceType, name string) {
	m.showConfirmDelete = true
	m.confirmDeleteType = resourceType
	m.confirmDeleteName = name
	m.confirmDeleteConfirmed = false
	m.confirmDeleteSkill = nil
	m.confirmDeleteCmd = nil
}

func (m *Model) openConfirmDeleteSkillCmd(s *skill.Skill, cmd *skill.Command) {
	m.showConfirmDelete = true
	m.confirmDeleteType = "skill_command"
	m.confirmDeleteName = fmt.Sprintf("%s:%s", s.Name, cmd.Name)
	m.confirmDeleteConfirmed = false
	m.confirmDeleteSkill = s
	m.confirmDeleteCmd = cmd
}

func (m *Model) executeDelete() tea.Cmd {
	m.showConfirmDelete = false

	return func() tea.Msg {
		var err error
		switch m.confirmDeleteType {
		case "server":
			delete(m.cfg.Servers, m.confirmDeleteName)
			err = m.cfg.Save()
			if err == nil {
				return serverDeletedMsg{name: m.confirmDeleteName}
			}
		case "command":
			for _, cmd := range m.commands {
				if cmd.Name == m.confirmDeleteName {
					err = m.resourceCRUD.DeleteCommand(cmd)
					break
				}
			}
		case "rule":
			for _, r := range m.rules {
				if r.Name == m.confirmDeleteName {
					err = m.resourceCRUD.DeleteRule(r)
					break
				}
			}
		case "skill":
			for _, s := range m.skills {
				if s.Name == m.confirmDeleteName {
					err = m.resourceCRUD.DeleteSkill(s)
					break
				}
			}
		case "prompt":
			for _, p := range m.prompts {
				if p.Name == m.confirmDeleteName {
					err = m.resourceCRUD.DeletePrompt(p)
					break
				}
			}
		case "skill_command":
			if m.confirmDeleteSkill != nil && m.confirmDeleteCmd != nil {
				err = m.confirmDeleteSkill.RemoveCommand(m.confirmDeleteCmd.Name)
				if err == nil {
					return skillCmdDeletedMsg{skill: m.confirmDeleteSkill, cmdName: m.confirmDeleteCmd.Name}
				}
			}
		}
		return resourceDeletedMsg{resourceType: m.confirmDeleteType, name: m.confirmDeleteName, err: err}
	}
}

// ============================================================================
// Prompt Editor Modal
// ============================================================================

func (m *Model) renderPromptEditor() string {
	title := "Create Prompt"
	if !m.promptEditorIsNew {
		title = "Edit Prompt"
	}

	var content strings.Builder
	content.WriteString(ModalTitleStyle.Render(title))
	content.WriteString("\n\n")

	// Name field
	nameLabel := "Name:"
	if m.promptEditorFocus == 0 {
		nameLabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Name:")
	}
	content.WriteString(fmt.Sprintf("%s ", nameLabel))
	if m.promptEditorIsNew {
		content.WriteString(m.promptEditorName.View())
	} else {
		content.WriteString(lipgloss.NewStyle().Faint(true).Render(m.promptEditorName.Value() + " (readonly)"))
	}
	content.WriteString("\n\n")

	// Description field
	descLabel := "Description:"
	if m.promptEditorFocus == 1 {
		descLabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Description:")
	}
	content.WriteString(fmt.Sprintf("%s ", descLabel))
	content.WriteString(m.promptEditorDesc.View())
	content.WriteString("\n\n")

	// Content field
	contentLabel := "Template (use {{var}} for placeholders):"
	if m.promptEditorFocus == 2 {
		contentLabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Template (use {{var}} for placeholders):")
	}
	content.WriteString(contentLabel)
	content.WriteString("\n")
	content.WriteString(m.promptEditorContent.View())
	content.WriteString("\n\n")

	// Scope selector - always shown with icons
	scopeLabel := "Scope:"
	if m.promptEditorFocus == 3 && m.hasProjectConfig() {
		scopeLabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Scope:")
	}
	content.WriteString(fmt.Sprintf("%s ", scopeLabel))

	scopeIcons := []string{"🌐", "📁"}
	scopeLabels := []string{"global", "local (project)"}

	if m.hasProjectConfig() {
		for i, label := range scopeLabels {
			icon := scopeIcons[i]
			if i == m.promptEditorScope {
				content.WriteString(lipgloss.NewStyle().Background(colorCyan).Foreground(colorBg).Bold(true).Render(" " + icon + " " + label + " "))
			} else {
				content.WriteString(lipgloss.NewStyle().Foreground(colorFgMuted).Render(" " + icon + " " + label + " "))
			}
		}
	} else {
		content.WriteString(lipgloss.NewStyle().Foreground(colorTeal).Render("🌐 global"))
		content.WriteString(lipgloss.NewStyle().Foreground(colorFgSubtle).Italic(true).Render(" (open a project for local scope)"))
	}
	content.WriteString("\n\n")

	helpText := "Tab: next field • ←/→: change scope • Ctrl+S: save • Esc: cancel"
	content.WriteString(KeyDescStyle.Render(helpText))

	modal := ModalStyle.Width(70).Render(content.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
}

func (m *Model) handlePromptEditorInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.showPromptEditor = false
		return m, nil
	case "tab":
		m.cyclePromptEditorFocus(1)
		return m, nil
	case "shift+tab":
		m.cyclePromptEditorFocus(-1)
		return m, nil
	case "ctrl+s":
		return m, m.savePrompt()
	case "left", "h":
		if m.promptEditorFocus == 3 && m.hasProjectConfig() {
			m.promptEditorScope = (m.promptEditorScope - 1 + len(scopeNames)) % len(scopeNames)
			return m, nil
		}
	case "right", "l":
		if m.promptEditorFocus == 3 && m.hasProjectConfig() {
			m.promptEditorScope = (m.promptEditorScope + 1) % len(scopeNames)
			return m, nil
		}
	}

	// Delegate to focused input
	var cmd tea.Cmd
	switch m.promptEditorFocus {
	case 0:
		if m.promptEditorIsNew {
			m.promptEditorName, cmd = m.promptEditorName.Update(msg)
		}
	case 1:
		m.promptEditorDesc, cmd = m.promptEditorDesc.Update(msg)
	case 2:
		m.promptEditorContent, cmd = m.promptEditorContent.Update(msg)
	}
	return m, cmd
}

func (m *Model) cyclePromptEditorFocus(delta int) {
	maxFocus := 2
	if m.hasProjectConfig() {
		maxFocus = 3 // Include scope field when in project
	}
	if !m.promptEditorIsNew {
		// Skip name field when editing
		if m.promptEditorFocus == 1 && delta < 0 {
			m.promptEditorFocus = maxFocus
			delta = 0
		}
	}
	m.promptEditorFocus = (m.promptEditorFocus + delta + maxFocus + 1) % (maxFocus + 1)
	if !m.promptEditorIsNew && m.promptEditorFocus == 0 {
		m.promptEditorFocus = 1
	}

	// Update focus state
	m.promptEditorName.Blur()
	m.promptEditorDesc.Blur()
	m.promptEditorContent.Blur()

	switch m.promptEditorFocus {
	case 0:
		m.promptEditorName.Focus()
	case 1:
		m.promptEditorDesc.Focus()
	case 2:
		m.promptEditorContent.Focus()
	}
}

func (m *Model) openPromptEditor(p *prompt.Prompt) {
	m.showPromptEditor = true
	m.promptEditorIsNew = (p == nil)
	m.promptEditorPrompt = p

	if p != nil {
		m.promptEditorName.SetValue(p.Name)
		m.promptEditorDesc.SetValue(p.Description)
		m.promptEditorContent.SetValue(p.Template)
		// Set scope based on existing prompt
		m.promptEditorScope = scopeIndexGlobal
		if p.Scope == "local" {
			m.promptEditorScope = scopeIndexLocal
		}
		m.promptEditorFocus = 1
		m.promptEditorDesc.Focus()
	} else {
		m.promptEditorName.SetValue("")
		m.promptEditorDesc.SetValue("")
		m.promptEditorContent.SetValue("")
		m.promptEditorScope = m.defaultScopeIndex()
		m.promptEditorFocus = 0
		m.promptEditorName.Focus()
	}
	m.promptEditorName.Blur()
	m.promptEditorDesc.Blur()
	m.promptEditorContent.Blur()

	if m.promptEditorIsNew {
		m.promptEditorName.Focus()
	} else {
		m.promptEditorDesc.Focus()
	}
}

func (m *Model) savePrompt() tea.Cmd {
	return func() tea.Msg {
		name := strings.TrimSpace(m.promptEditorName.Value())
		desc := strings.TrimSpace(m.promptEditorDesc.Value())
		template := m.promptEditorContent.Value()

		if name == "" {
			return resourceCreatedMsg{resourceType: "prompt", err: fmt.Errorf("name is required")}
		}
		if strings.ContainsAny(name, " \t\n/\\") {
			return resourceCreatedMsg{resourceType: "prompt", err: fmt.Errorf("name cannot contain spaces or path separators")}
		}

		// Determine scope and resource directory
		scope := config.ScopeGlobal
		resourceDir := m.cfg.ConfigDir
		if m.promptEditorScope == scopeIndexLocal {
			scope = config.ScopeLocal
			// Use project's .agentctl directory for local scope
			if m.cfg.ProjectPath != "" {
				resourceDir = filepath.Join(filepath.Dir(m.cfg.ProjectPath), ".agentctl")
			}
		}

		promptsDir := filepath.Join(resourceDir, "prompts")
		if err := os.MkdirAll(promptsDir, 0755); err != nil {
			return resourceCreatedMsg{resourceType: "prompt", err: err}
		}

		promptPath := filepath.Join(promptsDir, name+".json")

		if m.promptEditorIsNew {
			if _, err := os.Stat(promptPath); err == nil {
				return resourceCreatedMsg{resourceType: "prompt", err: fmt.Errorf("prompt %q already exists", name)}
			}
		}

		if desc == "" {
			desc = "No description"
		}
		if template == "" {
			template = "{{input}}"
		}

		// Extract variables from template
		variables := extractTemplateVariables(template)

		p := &prompt.Prompt{
			Name:        name,
			Description: desc,
			Template:    template,
			Variables:   variables,
			Scope:       string(scope),
		}

		data, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			return resourceCreatedMsg{resourceType: "prompt", err: err}
		}

		if err := os.WriteFile(promptPath, data, 0644); err != nil {
			return resourceCreatedMsg{resourceType: "prompt", err: err}
		}

		if m.promptEditorIsNew {
			return resourceCreatedMsg{resourceType: "prompt", name: name, scope: string(scope)}
		}
		return resourceEditedMsg{resourceType: "prompt"}
	}
}

// ============================================================================
// Skill Editor Modal
// ============================================================================

func (m *Model) renderSkillEditor() string {
	title := "Create Skill"
	if !m.skillEditorIsNew {
		title = "Edit Skill"
	}

	var content strings.Builder
	content.WriteString(ModalTitleStyle.Render(title))
	content.WriteString("\n\n")

	fields := []struct {
		label string
		input textinput.Model
		focus int
	}{
		{"Name:", m.skillEditorName, 0},
		{"Description:", m.skillEditorDesc, 1},
		{"Author:", m.skillEditorAuthor, 2},
		{"Version:", m.skillEditorVersion, 3},
	}

	for _, f := range fields {
		label := f.label
		if m.skillEditorFocus == f.focus {
			label = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render(f.label)
		}
		content.WriteString(fmt.Sprintf("%s ", label))
		if f.focus == 0 && !m.skillEditorIsNew {
			content.WriteString(lipgloss.NewStyle().Faint(true).Render(f.input.Value() + " (readonly)"))
		} else {
			content.WriteString(f.input.View())
		}
		content.WriteString("\n\n")
	}

	// Scope selector - always shown with icons
	scopeLabel := "Scope:"
	if m.skillEditorFocus == 4 && m.hasProjectConfig() {
		scopeLabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Scope:")
	}
	content.WriteString(fmt.Sprintf("%s ", scopeLabel))

	// Scope options with icons
	scopeIcons := []string{"🌐", "📁"}
	scopeLabels := []string{"global", "local (project)"}

	if m.hasProjectConfig() {
		// Show both options when in project
		for i, label := range scopeLabels {
			icon := scopeIcons[i]
			if i == m.skillEditorScope {
				content.WriteString(lipgloss.NewStyle().Background(colorCyan).Foreground(colorBg).Bold(true).Render(" " + icon + " " + label + " "))
			} else {
				content.WriteString(lipgloss.NewStyle().Foreground(colorFgMuted).Render(" " + icon + " " + label + " "))
			}
		}
	} else {
		// Not in project - show global only
		content.WriteString(lipgloss.NewStyle().Foreground(colorTeal).Render("🌐 global"))
		content.WriteString(lipgloss.NewStyle().Foreground(colorFgSubtle).Italic(true).Render(" (open a project for local scope)"))
	}
	content.WriteString("\n\n")

	helpText := "Tab: next field • ←/→: change scope • Ctrl+S: save • Esc: cancel"
	content.WriteString(KeyDescStyle.Render(helpText))

	modal := ModalStyle.Width(60).Render(content.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
}

func (m *Model) handleSkillEditorInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.showSkillEditor = false
		return m, nil
	case "tab":
		m.cycleSkillEditorFocus(1)
		return m, nil
	case "shift+tab":
		m.cycleSkillEditorFocus(-1)
		return m, nil
	case "ctrl+s":
		return m, m.saveSkill()
	case "left", "right":
		// Handle scope selection when on scope field (focus 4)
		if m.skillEditorFocus == 4 && m.hasProjectConfig() {
			m.skillEditorScope = (m.skillEditorScope + 1) % len(scopeNames)
			return m, nil
		}
	}

	// Delegate to focused input
	var cmd tea.Cmd
	switch m.skillEditorFocus {
	case 0:
		if m.skillEditorIsNew {
			m.skillEditorName, cmd = m.skillEditorName.Update(msg)
		}
	case 1:
		m.skillEditorDesc, cmd = m.skillEditorDesc.Update(msg)
	case 2:
		m.skillEditorAuthor, cmd = m.skillEditorAuthor.Update(msg)
	case 3:
		m.skillEditorVersion, cmd = m.skillEditorVersion.Update(msg)
	}
	return m, cmd
}

func (m *Model) cycleSkillEditorFocus(delta int) {
	maxFocus := 3
	// Include scope field when in project
	if m.hasProjectConfig() {
		maxFocus = 4
	}
	m.skillEditorFocus = (m.skillEditorFocus + delta + maxFocus + 1) % (maxFocus + 1)
	if !m.skillEditorIsNew && m.skillEditorFocus == 0 {
		if delta > 0 {
			m.skillEditorFocus = 1
		} else {
			m.skillEditorFocus = maxFocus
		}
	}

	m.skillEditorName.Blur()
	m.skillEditorDesc.Blur()
	m.skillEditorAuthor.Blur()
	m.skillEditorVersion.Blur()

	switch m.skillEditorFocus {
	case 0:
		m.skillEditorName.Focus()
	case 1:
		m.skillEditorDesc.Focus()
	case 2:
		m.skillEditorAuthor.Focus()
	case 3:
		m.skillEditorVersion.Focus()
	// case 4 is scope selector - no focus change needed (not a text input)
	}
}

func (m *Model) openSkillEditor(s *skill.Skill) {
	m.showSkillEditor = true
	m.skillEditorIsNew = (s == nil)
	m.skillEditorSkill = s

	if s != nil {
		m.skillEditorName.SetValue(s.Name)
		m.skillEditorDesc.SetValue(s.Description)
		m.skillEditorAuthor.SetValue(s.Author)
		m.skillEditorVersion.SetValue(s.Version)
		m.skillEditorFocus = 1
		// Set scope from existing skill
		if s.Scope == "local" {
			m.skillEditorScope = scopeIndexLocal
		} else {
			m.skillEditorScope = scopeIndexGlobal
		}
	} else {
		m.skillEditorName.SetValue("")
		m.skillEditorDesc.SetValue("")
		m.skillEditorAuthor.SetValue("")
		m.skillEditorVersion.SetValue("1.0.0")
		m.skillEditorFocus = 0
		m.skillEditorScope = m.defaultScopeIndex()
	}

	m.skillEditorName.Blur()
	m.skillEditorDesc.Blur()
	m.skillEditorAuthor.Blur()
	m.skillEditorVersion.Blur()

	if m.skillEditorIsNew {
		m.skillEditorName.Focus()
	} else {
		m.skillEditorDesc.Focus()
	}
}

func (m *Model) saveSkill() tea.Cmd {
	return func() tea.Msg {
		name := strings.TrimSpace(m.skillEditorName.Value())
		desc := strings.TrimSpace(m.skillEditorDesc.Value())
		author := strings.TrimSpace(m.skillEditorAuthor.Value())
		version := strings.TrimSpace(m.skillEditorVersion.Value())

		if name == "" {
			return resourceCreatedMsg{resourceType: "skill", err: fmt.Errorf("name is required")}
		}
		if strings.ContainsAny(name, " \t\n/\\") {
			return resourceCreatedMsg{resourceType: "skill", err: fmt.Errorf("name cannot contain spaces or path separators")}
		}

		// Determine scope and resource directory
		scope := config.ScopeGlobal
		resourceDir := m.cfg.ConfigDir
		if m.skillEditorScope == scopeIndexLocal {
			scope = config.ScopeLocal
			// Use project's .agentctl directory for local scope
			if m.cfg.ProjectPath != "" {
				resourceDir = filepath.Join(filepath.Dir(m.cfg.ProjectPath), ".agentctl")
			}
		}

		skillsDir := filepath.Join(resourceDir, "skills")
		skillDir := filepath.Join(skillsDir, name)

		if m.skillEditorIsNew {
			if _, err := os.Stat(skillDir); err == nil {
				return resourceCreatedMsg{resourceType: "skill", err: fmt.Errorf("skill %q already exists", name)}
			}
		}

		if desc == "" {
			desc = "No description"
		}
		if version == "" {
			version = "1.0.0"
		}

		// Create or update skill using SKILL.md format
		s := &skill.Skill{
			Name:        name,
			Description: desc,
			Author:      author,
			Version:     version,
			Scope:       string(scope),
		}

		// If editing existing skill, preserve the content
		if !m.skillEditorIsNew && m.skillEditorSkill != nil {
			s.Content = m.skillEditorSkill.Content
			s.Commands = m.skillEditorSkill.Commands
		}

		// Save using SKILL.md format
		if err := s.Save(skillDir); err != nil {
			return resourceCreatedMsg{resourceType: "skill", err: err}
		}

		if m.skillEditorIsNew {
			return resourceCreatedMsg{resourceType: "skill", name: name, scope: string(scope)}
		}
		return resourceEditedMsg{resourceType: "skill"}
	}
}

// ============================================================================
// Skill Detail Modal (shows commands/invocations)
// ============================================================================

func (m *Model) openSkillDetail(s *skill.Skill) {
	m.showSkillDetail = true
	m.skillDetailSkill = s
	m.skillDetailCursor = 0
}

func (m *Model) renderSkillDetail() string {
	s := m.skillDetailSkill
	if s == nil {
		return ""
	}

	var content strings.Builder
	content.WriteString(ModalTitleStyle.Render("Skill: " + s.Name))
	content.WriteString("\n\n")

	// Scope indicator
	scopeIcon := "🌐"
	scopeLabel := "global"
	if s.Scope == "local" {
		scopeIcon = "📁"
		scopeLabel = "local"
	}
	content.WriteString(lipgloss.NewStyle().Foreground(colorFgSubtle).Render(
		fmt.Sprintf("Scope: %s %s", scopeIcon, scopeLabel)))
	content.WriteString("\n")

	// Description
	if s.Description != "" {
		content.WriteString(lipgloss.NewStyle().Foreground(colorFgSubtle).Render(s.Description))
		content.WriteString("\n")
	}
	content.WriteString("\n")

	// Build list of invocations
	type invocation struct {
		name      string
		desc      string
		isDefault bool
	}
	var invocations []invocation

	// Add default command if skill has content
	if s.Content != "" {
		invocations = append(invocations, invocation{
			name:      "/" + s.Name,
			desc:      "(default)",
			isDefault: true,
		})
	}

	// Add subcommands
	for _, cmd := range s.Commands {
		inv := invocation{
			name: "/" + s.Name + ":" + cmd.Name,
			desc: cmd.Description,
		}
		invocations = append(invocations, inv)
	}

	// Render invocations list
	content.WriteString(lipgloss.NewStyle().Bold(true).Render("Invocations:"))
	content.WriteString("\n")

	if len(invocations) == 0 {
		content.WriteString(lipgloss.NewStyle().Foreground(colorFgSubtle).Italic(true).Render("  No commands defined"))
		content.WriteString("\n")
	} else {
		for i, inv := range invocations {
			prefix := "  "
			nameStyle := lipgloss.NewStyle().Foreground(colorCyan)
			descStyle := lipgloss.NewStyle().Foreground(colorFgSubtle)

			if i == m.skillDetailCursor {
				prefix = "▸ "
				nameStyle = nameStyle.Bold(true).Background(colorCyan).Foreground(colorBg)
				descStyle = descStyle.Bold(true)
			}

			line := prefix + nameStyle.Render(inv.name)
			if inv.desc != "" {
				line += " " + descStyle.Render(inv.desc)
			}
			content.WriteString(line)
			content.WriteString("\n")
		}
	}

	content.WriteString("\n")

	// Path info
	content.WriteString(lipgloss.NewStyle().Foreground(colorFgSubtle).Render(
		fmt.Sprintf("Path: %s", s.Path)))
	content.WriteString("\n\n")

	helpText := "↑/↓: navigate • Enter/e: edit • a: add command • d: delete • Esc: close"
	content.WriteString(KeyDescStyle.Render(helpText))

	modal := ModalStyle.Width(70).Render(content.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
}

func (m *Model) handleSkillDetailInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := m.skillDetailSkill
	if s == nil {
		m.showSkillDetail = false
		return m, nil
	}

	// Calculate number of items and determine if cursor is on default or a command
	hasDefault := s.Content != ""
	itemCount := len(s.Commands)
	if hasDefault {
		itemCount++
	}

	// Helper to get the selected command (nil if on default)
	getSelectedCommand := func() *skill.Command {
		if hasDefault {
			if m.skillDetailCursor == 0 {
				return nil // On default command
			}
			return s.Commands[m.skillDetailCursor-1]
		}
		if len(s.Commands) > 0 {
			return s.Commands[m.skillDetailCursor]
		}
		return nil
	}

	switch msg.String() {
	case "esc", "q":
		m.showSkillDetail = false
		return m, nil

	case "up", "k":
		if m.skillDetailCursor > 0 {
			m.skillDetailCursor--
		}
		return m, nil

	case "down", "j":
		if m.skillDetailCursor < itemCount-1 {
			m.skillDetailCursor++
		}
		return m, nil

	case "e", "enter":
		// Edit the selected item
		cmd := getSelectedCommand()
		if cmd != nil {
			// Edit the command
			m.openSkillCmdEditor(s, cmd, false)
		} else {
			// Edit the skill itself (default command content is in SKILL.md)
			m.showSkillDetail = false
			m.openSkillEditor(s)
		}
		return m, nil

	case "a":
		// Add a new command to the skill
		m.openSkillCmdEditor(s, nil, true)
		return m, nil

	case "d", "x":
		// Delete the selected command (not the default) - show confirmation first
		cmd := getSelectedCommand()
		if cmd != nil {
			m.openConfirmDeleteSkillCmd(s, cmd)
		}
		return m, nil
	}

	return m, nil
}

// ============================================================================
// Skill Command Editor
// ============================================================================

func (m *Model) openSkillCmdEditor(s *skill.Skill, cmd *skill.Command, isNew bool) {
	m.showSkillDetail = false
	m.showSkillCmdEditor = true
	m.skillCmdEditorIsNew = isNew
	m.skillCmdEditorSkill = s
	m.skillCmdEditorCmd = cmd
	m.skillCmdEditorFocus = 0

	// Reset inputs
	m.skillCmdEditorName.Reset()
	m.skillCmdEditorDesc.Reset()
	m.skillCmdEditorContent.Reset()

	if cmd != nil && !isNew {
		// Editing existing command
		m.skillCmdEditorName.SetValue(cmd.Name)
		m.skillCmdEditorDesc.SetValue(cmd.Description)
		m.skillCmdEditorContent.SetValue(cmd.Content)
	}

	m.skillCmdEditorName.Focus()
}

func (m *Model) renderSkillCmdEditor() string {
	var content strings.Builder

	title := "Add Command"
	if !m.skillCmdEditorIsNew {
		title = "Edit Command"
	}
	skillName := ""
	if m.skillCmdEditorSkill != nil {
		skillName = m.skillCmdEditorSkill.Name
	}
	content.WriteString(ModalTitleStyle.Render(fmt.Sprintf("%s - %s", title, skillName)))
	content.WriteString("\n\n")

	// Name field
	nameLabel := "Name:"
	if m.skillCmdEditorFocus == 0 {
		nameLabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Name:")
	}
	content.WriteString(fmt.Sprintf("%s ", nameLabel))
	content.WriteString(m.skillCmdEditorName.View())
	content.WriteString("\n")
	content.WriteString(lipgloss.NewStyle().Foreground(colorFgSubtle).Render("  alphanumeric and dashes only"))
	content.WriteString("\n\n")

	// Description field
	descLabel := "Description:"
	if m.skillCmdEditorFocus == 1 {
		descLabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Description:")
	}
	content.WriteString(fmt.Sprintf("%s ", descLabel))
	content.WriteString(m.skillCmdEditorDesc.View())
	content.WriteString("\n\n")

	// Content field
	contentLabel := "Prompt:"
	if m.skillCmdEditorFocus == 2 {
		contentLabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Prompt:")
	}
	content.WriteString(fmt.Sprintf("%s\n", contentLabel))
	content.WriteString(m.skillCmdEditorContent.View())
	content.WriteString("\n")
	content.WriteString(lipgloss.NewStyle().Foreground(colorFgSubtle).Render("  Use $ARGUMENTS for user input"))
	content.WriteString("\n\n")

	// Help
	helpText := "Tab: next field • Ctrl+S: save • Esc: cancel"
	content.WriteString(KeyDescStyle.Render(helpText))

	modal := ModalStyle.Width(70).Render(content.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
}

func (m *Model) handleSkillCmdEditorInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.showSkillCmdEditor = false
		// Re-open skill detail
		if m.skillCmdEditorSkill != nil {
			m.openSkillDetail(m.skillCmdEditorSkill)
		}
		return m, nil

	case "ctrl+s":
		return m, m.saveSkillCmd()

	case "tab", "shift+tab":
		// Cycle focus
		if msg.String() == "tab" {
			m.skillCmdEditorFocus = (m.skillCmdEditorFocus + 1) % 3
		} else {
			m.skillCmdEditorFocus = (m.skillCmdEditorFocus + 2) % 3
		}
		// Update focus state
		m.skillCmdEditorName.Blur()
		m.skillCmdEditorDesc.Blur()
		m.skillCmdEditorContent.Blur()
		switch m.skillCmdEditorFocus {
		case 0:
			m.skillCmdEditorName.Focus()
		case 1:
			m.skillCmdEditorDesc.Focus()
		case 2:
			m.skillCmdEditorContent.Focus()
		}
		return m, nil
	}

	// Update the focused input
	var cmd tea.Cmd
	switch m.skillCmdEditorFocus {
	case 0:
		m.skillCmdEditorName, cmd = m.skillCmdEditorName.Update(msg)
	case 1:
		m.skillCmdEditorDesc, cmd = m.skillCmdEditorDesc.Update(msg)
	case 2:
		m.skillCmdEditorContent, cmd = m.skillCmdEditorContent.Update(msg)
	}
	return m, cmd
}

func (m *Model) saveSkillCmd() tea.Cmd {
	return func() tea.Msg {
		s := m.skillCmdEditorSkill
		if s == nil {
			return resourceCreatedMsg{resourceType: "skill command", err: fmt.Errorf("no skill selected")}
		}

		name := strings.TrimSpace(m.skillCmdEditorName.Value())
		desc := strings.TrimSpace(m.skillCmdEditorDesc.Value())
		content := m.skillCmdEditorContent.Value()

		// Validate name
		if name == "" {
			return resourceCreatedMsg{resourceType: "skill command", err: fmt.Errorf("name is required")}
		}

		// Check alphanumeric and dashes only
		for _, c := range name {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-') {
				return resourceCreatedMsg{resourceType: "skill command", err: fmt.Errorf("name must be alphanumeric with dashes only")}
			}
		}

		if m.skillCmdEditorIsNew {
			// Check for duplicate
			if s.GetCommand(name) != nil {
				return resourceCreatedMsg{resourceType: "skill command", err: fmt.Errorf("command %q already exists", name)}
			}

			// Create new command
			newCmd := &skill.Command{
				Name:        name,
				Description: desc,
				Content:     content,
				FileName:    name + ".md",
			}

			if err := s.AddCommand(newCmd); err != nil {
				return resourceCreatedMsg{resourceType: "skill command", err: err}
			}

			if err := s.SaveCommand(newCmd); err != nil {
				return resourceCreatedMsg{resourceType: "skill command", err: err}
			}

			return skillCmdSavedMsg{skill: s, cmdName: name, isNew: true}
		} else {
			// Update existing command
			cmd := m.skillCmdEditorCmd
			if cmd == nil {
				return resourceCreatedMsg{resourceType: "skill command", err: fmt.Errorf("no command to edit")}
			}

			// If name changed, need to handle rename
			oldName := cmd.Name
			oldFileName := cmd.FileName

			cmd.Name = name
			cmd.Description = desc
			cmd.Content = content
			cmd.FileName = name + ".md"

			// Save the command
			if err := s.SaveCommand(cmd); err != nil {
				return resourceCreatedMsg{resourceType: "skill command", err: err}
			}

			// If name changed, remove old file
			if oldName != name && s.Path != "" {
				oldPath := filepath.Join(s.Path, oldFileName)
				os.Remove(oldPath) // Ignore error if file doesn't exist
			}

			return skillCmdSavedMsg{skill: s, cmdName: name, isNew: false}
		}
	}
}

// skillCmdSavedMsg is sent when a skill command is saved
type skillCmdSavedMsg struct {
	skill   *skill.Skill
	cmdName string
	isNew   bool
}

func (m *Model) deleteSkillCommand(s *skill.Skill, cmd *skill.Command) tea.Cmd {
	return func() tea.Msg {
		if err := s.RemoveCommand(cmd.Name); err != nil {
			return resourceCreatedMsg{resourceType: "skill command", err: fmt.Errorf("failed to delete command: %w", err)}
		}

		return skillCmdDeletedMsg{skill: s, cmdName: cmd.Name}
	}
}

// skillCmdDeletedMsg is sent when a skill command is deleted
type skillCmdDeletedMsg struct {
	skill   *skill.Skill
	cmdName string
}

// ============================================================================
// Command Editor Modal
// ============================================================================

var modelNames = []string{"default", "opus", "sonnet", "haiku"}

func (m *Model) renderCommandEditor() string {
	title := "Create Command"
	if !m.commandEditorIsNew {
		title = "Edit Command"
	}

	var content strings.Builder
	content.WriteString(ModalTitleStyle.Render(title))
	content.WriteString("\n\n")

	// Name field
	nameLabel := "Name:"
	if m.commandEditorFocus == 0 {
		nameLabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Name:")
	}
	content.WriteString(fmt.Sprintf("%s ", nameLabel))
	if m.commandEditorIsNew {
		content.WriteString(m.commandEditorName.View())
	} else {
		content.WriteString(lipgloss.NewStyle().Faint(true).Render(m.commandEditorName.Value() + " (readonly)"))
	}
	content.WriteString("\n\n")

	// Description field
	descLabel := "Description:"
	if m.commandEditorFocus == 1 {
		descLabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Description:")
	}
	content.WriteString(fmt.Sprintf("%s ", descLabel))
	content.WriteString(m.commandEditorDesc.View())
	content.WriteString("\n\n")

	// Argument hint field
	argLabel := "Argument Hint:"
	if m.commandEditorFocus == 2 {
		argLabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Argument Hint:")
	}
	content.WriteString(fmt.Sprintf("%s ", argLabel))
	content.WriteString(m.commandEditorArgHint.View())
	content.WriteString("\n\n")

	// Model selector
	modelLabel := "Model:"
	if m.commandEditorFocus == 3 {
		modelLabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Model:")
	}
	content.WriteString(fmt.Sprintf("%s ", modelLabel))
	for i, name := range modelNames {
		if i == m.commandEditorModel {
			content.WriteString(lipgloss.NewStyle().Background(colorCyan).Foreground(colorBg).Render(" " + name + " "))
		} else {
			content.WriteString(" " + name + " ")
		}
	}
	content.WriteString("\n\n")

	// Prompt content
	promptLabel := "Prompt (use $ARGUMENTS for input):"
	if m.commandEditorFocus == 4 {
		promptLabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Prompt (use $ARGUMENTS for input):")
	}
	content.WriteString(promptLabel)
	content.WriteString("\n")
	content.WriteString(m.commandEditorContent.View())
	content.WriteString("\n\n")

	// Scope selector - always shown with icons
	scopeLabel := "Scope:"
	if m.commandEditorFocus == 5 && m.hasProjectConfig() {
		scopeLabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Scope:")
	}
	content.WriteString(fmt.Sprintf("%s ", scopeLabel))

	scopeIcons := []string{"🌐", "📁"}
	scopeLabels := []string{"global", "local (project)"}

	if m.hasProjectConfig() {
		for i, label := range scopeLabels {
			icon := scopeIcons[i]
			if i == m.commandEditorScope {
				content.WriteString(lipgloss.NewStyle().Background(colorCyan).Foreground(colorBg).Bold(true).Render(" " + icon + " " + label + " "))
			} else {
				content.WriteString(lipgloss.NewStyle().Foreground(colorFgMuted).Render(" " + icon + " " + label + " "))
			}
		}
	} else {
		content.WriteString(lipgloss.NewStyle().Foreground(colorTeal).Render("🌐 global"))
		content.WriteString(lipgloss.NewStyle().Foreground(colorFgSubtle).Italic(true).Render(" (open a project for local scope)"))
	}
	content.WriteString("\n\n")

	helpText := "Tab: next • ←/→: change selection • Ctrl+S: save • Esc: cancel"
	content.WriteString(KeyDescStyle.Render(helpText))

	modal := ModalStyle.Width(70).Render(content.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
}

func (m *Model) handleCommandEditorInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.showCommandEditor = false
		return m, nil
	case "tab":
		m.cycleCommandEditorFocus(1)
		return m, nil
	case "shift+tab":
		m.cycleCommandEditorFocus(-1)
		return m, nil
	case "ctrl+s":
		return m, m.saveCommand()
	case "left", "h":
		if m.commandEditorFocus == 3 {
			m.commandEditorModel = (m.commandEditorModel - 1 + len(modelNames)) % len(modelNames)
			return m, nil
		}
		if m.commandEditorFocus == 5 && m.hasProjectConfig() {
			m.commandEditorScope = (m.commandEditorScope - 1 + len(scopeNames)) % len(scopeNames)
			return m, nil
		}
	case "right", "l":
		if m.commandEditorFocus == 3 {
			m.commandEditorModel = (m.commandEditorModel + 1) % len(modelNames)
			return m, nil
		}
		if m.commandEditorFocus == 5 && m.hasProjectConfig() {
			m.commandEditorScope = (m.commandEditorScope + 1) % len(scopeNames)
			return m, nil
		}
	}

	// Delegate to focused input
	var cmd tea.Cmd
	switch m.commandEditorFocus {
	case 0:
		if m.commandEditorIsNew {
			m.commandEditorName, cmd = m.commandEditorName.Update(msg)
		}
	case 1:
		m.commandEditorDesc, cmd = m.commandEditorDesc.Update(msg)
	case 2:
		m.commandEditorArgHint, cmd = m.commandEditorArgHint.Update(msg)
	case 4:
		m.commandEditorContent, cmd = m.commandEditorContent.Update(msg)
	}
	return m, cmd
}

func (m *Model) cycleCommandEditorFocus(delta int) {
	maxFocus := 4
	if m.hasProjectConfig() {
		maxFocus = 5 // Include scope field when in project
	}
	m.commandEditorFocus = (m.commandEditorFocus + delta + maxFocus + 1) % (maxFocus + 1)
	if !m.commandEditorIsNew && m.commandEditorFocus == 0 {
		if delta > 0 {
			m.commandEditorFocus = 1
		} else {
			m.commandEditorFocus = maxFocus
		}
	}

	m.commandEditorName.Blur()
	m.commandEditorDesc.Blur()
	m.commandEditorArgHint.Blur()
	m.commandEditorContent.Blur()

	switch m.commandEditorFocus {
	case 0:
		m.commandEditorName.Focus()
	case 1:
		m.commandEditorDesc.Focus()
	case 2:
		m.commandEditorArgHint.Focus()
	case 4:
		m.commandEditorContent.Focus()
	}
}

func (m *Model) openCommandEditor(c *command.Command) {
	m.showCommandEditor = true
	m.commandEditorIsNew = (c == nil)
	m.commandEditorCommand = c

	if c != nil {
		m.commandEditorName.SetValue(c.Name)
		m.commandEditorDesc.SetValue(c.Description)
		m.commandEditorArgHint.SetValue(c.ArgumentHint)
		m.commandEditorContent.SetValue(c.Prompt)
		m.commandEditorModel = 0
		for i, name := range modelNames {
			if name == c.Model || (c.Model == "" && name == "default") {
				m.commandEditorModel = i
				break
			}
		}
		// Set scope based on existing command
		m.commandEditorScope = scopeIndexGlobal
		if c.Scope == "local" {
			m.commandEditorScope = scopeIndexLocal
		}
		m.commandEditorFocus = 1
	} else {
		m.commandEditorName.SetValue("")
		m.commandEditorDesc.SetValue("")
		m.commandEditorArgHint.SetValue("")
		m.commandEditorContent.SetValue("")
		m.commandEditorModel = 0
		m.commandEditorScope = m.defaultScopeIndex()
		m.commandEditorFocus = 0
	}

	m.commandEditorName.Blur()
	m.commandEditorDesc.Blur()
	m.commandEditorArgHint.Blur()
	m.commandEditorContent.Blur()

	if m.commandEditorIsNew {
		m.commandEditorName.Focus()
	} else {
		m.commandEditorDesc.Focus()
	}
}

func (m *Model) saveCommand() tea.Cmd {
	return func() tea.Msg {
		name := strings.TrimSpace(m.commandEditorName.Value())
		desc := strings.TrimSpace(m.commandEditorDesc.Value())
		argHint := strings.TrimSpace(m.commandEditorArgHint.Value())
		promptText := m.commandEditorContent.Value()
		model := modelNames[m.commandEditorModel]
		if model == "default" {
			model = ""
		}

		if name == "" {
			return resourceCreatedMsg{resourceType: "command", err: fmt.Errorf("name is required")}
		}
		if strings.ContainsAny(name, " \t\n/") {
			return resourceCreatedMsg{resourceType: "command", err: fmt.Errorf("name cannot contain spaces or slashes")}
		}

		// Determine scope and resource directory
		scope := config.ScopeGlobal
		resourceDir := m.cfg.ConfigDir
		if m.commandEditorScope == scopeIndexLocal {
			scope = config.ScopeLocal
			// Use project's .agentctl directory for local scope
			if m.cfg.ProjectPath != "" {
				resourceDir = filepath.Join(filepath.Dir(m.cfg.ProjectPath), ".agentctl")
			}
		}

		commandsDir := filepath.Join(resourceDir, "commands")
		if err := os.MkdirAll(commandsDir, 0755); err != nil {
			return resourceCreatedMsg{resourceType: "command", err: err}
		}

		commandPath := filepath.Join(commandsDir, name+".json")

		if m.commandEditorIsNew {
			if _, err := os.Stat(commandPath); err == nil {
				return resourceCreatedMsg{resourceType: "command", err: fmt.Errorf("command %q already exists", name)}
			}
		}

		if desc == "" {
			desc = "No description"
		}
		if promptText == "" {
			promptText = "$ARGUMENTS"
		}

		cmd := &command.Command{
			Name:         name,
			Description:  desc,
			ArgumentHint: argHint,
			Model:        model,
			Prompt:       promptText,
			Scope:        string(scope),
		}

		data, err := json.MarshalIndent(cmd, "", "  ")
		if err != nil {
			return resourceCreatedMsg{resourceType: "command", err: err}
		}

		if err := os.WriteFile(commandPath, data, 0644); err != nil {
			return resourceCreatedMsg{resourceType: "command", err: err}
		}

		if m.commandEditorIsNew {
			return resourceCreatedMsg{resourceType: "command", name: name, scope: string(scope)}
		}
		return resourceEditedMsg{resourceType: "command"}
	}
}

// ============================================================================
// Server Editor Modal
// ============================================================================

var transportNames = []string{"stdio", "http", "sse"}
var scopeNames = []string{"global", "local"}

// Scope index constants
const (
	scopeIndexGlobal = 0
	scopeIndexLocal  = 1
)

// hasProjectConfig returns true if we're in a project with .agentctl.json
func (m *Model) hasProjectConfig() bool {
	return m.cfg.ProjectPath != ""
}

// defaultScopeIndex returns the default scope index (1=local if in project, 0=global otherwise)
func (m *Model) defaultScopeIndex() int {
	if m.hasProjectConfig() {
		return 1 // local
	}
	return 0 // global
}

func (m *Model) renderServerEditor() string {
	title := "Add Server"
	if !m.serverEditorIsNew {
		title = "Edit Server"
	}

	var content strings.Builder
	content.WriteString(ModalTitleStyle.Render(title))
	content.WriteString("\n\n")

	// Name field
	nameLabel := "Name:"
	if m.serverEditorFocus == 0 {
		nameLabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Name:")
	}
	content.WriteString(fmt.Sprintf("%s ", nameLabel))
	if m.serverEditorIsNew {
		content.WriteString(m.serverEditorName.View())
	} else {
		content.WriteString(lipgloss.NewStyle().Faint(true).Render(m.serverEditorName.Value() + " (readonly)"))
	}
	content.WriteString("\n\n")

	// Source field
	sourceLabel := "Source (alias, URL, or path):"
	if m.serverEditorFocus == 1 {
		sourceLabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Source (alias, URL, or path):")
	}
	content.WriteString(fmt.Sprintf("%s ", sourceLabel))
	content.WriteString(m.serverEditorSource.View())
	content.WriteString("\n\n")

	// Command field (for stdio)
	cmdLabel := "Command (for stdio):"
	if m.serverEditorFocus == 2 {
		cmdLabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Command (for stdio):")
	}
	content.WriteString(fmt.Sprintf("%s ", cmdLabel))
	content.WriteString(m.serverEditorCommand.View())
	content.WriteString("\n\n")

	// Args field
	argsLabel := "Arguments:"
	if m.serverEditorFocus == 3 {
		argsLabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Arguments:")
	}
	content.WriteString(fmt.Sprintf("%s ", argsLabel))
	content.WriteString(m.serverEditorArgs.View())
	content.WriteString("\n\n")

	// Transport selector
	transportLabel := "Transport:"
	if m.serverEditorFocus == 4 {
		transportLabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Transport:")
	}
	content.WriteString(fmt.Sprintf("%s ", transportLabel))
	for i, name := range transportNames {
		if i == m.serverEditorTransport {
			content.WriteString(lipgloss.NewStyle().Background(colorCyan).Foreground(colorBg).Render(" " + name + " "))
		} else {
			content.WriteString(" " + name + " ")
		}
	}
	content.WriteString("\n\n")

	// Scope selector (only shown when in project)
	if m.hasProjectConfig() {
		scopeLabel := "Scope:"
		if m.serverEditorFocus == 5 {
			scopeLabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Scope:")
		}
		content.WriteString(fmt.Sprintf("%s ", scopeLabel))
		for i, name := range scopeNames {
			if i == m.serverEditorScope {
				content.WriteString(lipgloss.NewStyle().Background(colorCyan).Foreground(colorBg).Render(" " + name + " "))
			} else {
				content.WriteString(" " + name + " ")
			}
		}
		content.WriteString("\n\n")
	}

	helpText := "Tab: next • ←/→: change selection • Ctrl+S: save • Esc: cancel"
	content.WriteString(KeyDescStyle.Render(helpText))

	modal := ModalStyle.Width(70).Render(content.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
}

func (m *Model) handleServerEditorInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.showServerEditor = false
		return m, nil
	case "tab":
		m.cycleServerEditorFocus(1)
		return m, nil
	case "shift+tab":
		m.cycleServerEditorFocus(-1)
		return m, nil
	case "ctrl+s":
		return m, m.saveServer()
	case "left", "h":
		if m.serverEditorFocus == 4 {
			m.serverEditorTransport = (m.serverEditorTransport - 1 + len(transportNames)) % len(transportNames)
			return m, nil
		}
		if m.serverEditorFocus == 5 && m.hasProjectConfig() {
			m.serverEditorScope = (m.serverEditorScope - 1 + len(scopeNames)) % len(scopeNames)
			return m, nil
		}
	case "right", "l":
		if m.serverEditorFocus == 4 {
			m.serverEditorTransport = (m.serverEditorTransport + 1) % len(transportNames)
			return m, nil
		}
		if m.serverEditorFocus == 5 && m.hasProjectConfig() {
			m.serverEditorScope = (m.serverEditorScope + 1) % len(scopeNames)
			return m, nil
		}
	}

	// Delegate to focused input
	var cmd tea.Cmd
	switch m.serverEditorFocus {
	case 0:
		if m.serverEditorIsNew {
			m.serverEditorName, cmd = m.serverEditorName.Update(msg)
		}
	case 1:
		m.serverEditorSource, cmd = m.serverEditorSource.Update(msg)
	case 2:
		m.serverEditorCommand, cmd = m.serverEditorCommand.Update(msg)
	case 3:
		m.serverEditorArgs, cmd = m.serverEditorArgs.Update(msg)
	}
	return m, cmd
}

func (m *Model) cycleServerEditorFocus(delta int) {
	maxFocus := 4
	if m.hasProjectConfig() {
		maxFocus = 5 // Include scope field when in project
	}
	m.serverEditorFocus = (m.serverEditorFocus + delta + maxFocus + 1) % (maxFocus + 1)
	if !m.serverEditorIsNew && m.serverEditorFocus == 0 {
		if delta > 0 {
			m.serverEditorFocus = 1
		} else {
			m.serverEditorFocus = maxFocus
		}
	}

	m.serverEditorName.Blur()
	m.serverEditorSource.Blur()
	m.serverEditorCommand.Blur()
	m.serverEditorArgs.Blur()

	switch m.serverEditorFocus {
	case 0:
		m.serverEditorName.Focus()
	case 1:
		m.serverEditorSource.Focus()
	case 2:
		m.serverEditorCommand.Focus()
	case 3:
		m.serverEditorArgs.Focus()
	}
}

func (m *Model) openServerEditor(s *mcp.Server) {
	m.showServerEditor = true
	m.serverEditorIsNew = (s == nil)
	m.serverEditorServer = s

	if s != nil {
		m.serverEditorName.SetValue(s.Name)
		if s.URL != "" {
			m.serverEditorSource.SetValue(s.URL)
		} else if s.Source.Alias != "" {
			m.serverEditorSource.SetValue(s.Source.Alias)
		} else {
			m.serverEditorSource.SetValue("")
		}
		m.serverEditorCommand.SetValue(s.Command)
		m.serverEditorArgs.SetValue(strings.Join(s.Args, " "))
		m.serverEditorTransport = 0
		for i, name := range transportNames {
			if name == string(s.Transport) {
				m.serverEditorTransport = i
				break
			}
		}
		// Set scope based on existing server
		m.serverEditorScope = scopeIndexGlobal
		if s.Scope == "local" {
			m.serverEditorScope = scopeIndexLocal
		}
		m.serverEditorFocus = 1
	} else {
		m.serverEditorName.SetValue("")
		m.serverEditorSource.SetValue("")
		m.serverEditorCommand.SetValue("")
		m.serverEditorArgs.SetValue("")
		m.serverEditorTransport = 0
		m.serverEditorScope = m.defaultScopeIndex()
		m.serverEditorFocus = 0
	}

	m.serverEditorName.Blur()
	m.serverEditorSource.Blur()
	m.serverEditorCommand.Blur()
	m.serverEditorArgs.Blur()

	if m.serverEditorIsNew {
		m.serverEditorName.Focus()
	} else {
		m.serverEditorSource.Focus()
	}
}

func (m *Model) saveServer() tea.Cmd {
	return func() tea.Msg {
		name := strings.TrimSpace(m.serverEditorName.Value())
		source := strings.TrimSpace(m.serverEditorSource.Value())
		command := strings.TrimSpace(m.serverEditorCommand.Value())
		args := strings.TrimSpace(m.serverEditorArgs.Value())
		transport := mcp.Transport(transportNames[m.serverEditorTransport])

		if name == "" {
			return serverAddedMsg{err: fmt.Errorf("name is required")}
		}

		if m.serverEditorIsNew {
			if _, exists := m.cfg.Servers[name]; exists {
				return serverAddedMsg{err: fmt.Errorf("server %q already exists", name)}
			}
		}

		server := &mcp.Server{
			Name:      name,
			Transport: transport,
		}

		// Determine source type and configure server
		if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
			server.URL = source
			server.Source = mcp.Source{Type: "remote", URL: source}
		} else if strings.HasPrefix(source, "./") || strings.HasPrefix(source, "/") {
			server.Source = mcp.Source{Type: "local", URL: source}
			server.Command = command
			if args != "" {
				server.Args = strings.Fields(args)
			}
		} else if source != "" {
			// Assume alias
			server.Source = mcp.Source{Type: "alias", Alias: source}
		} else {
			// Manual command
			server.Source = mcp.Source{Type: "manual"}
			server.Command = command
			if args != "" {
				server.Args = strings.Fields(args)
			}
		}

		// Determine scope and save to appropriate config
		scope := config.ScopeGlobal
		if m.serverEditorScope == scopeIndexLocal {
			scope = config.ScopeLocal
		}
		server.Scope = string(scope)

		// Load the scoped config, add server, and save
		scopedCfg, err := config.LoadScoped(scope)
		if err != nil {
			return serverAddedMsg{err: fmt.Errorf("failed to load %s config: %w", scope, err)}
		}
		if scopedCfg.Servers == nil {
			scopedCfg.Servers = make(map[string]*mcp.Server)
		}
		scopedCfg.Servers[name] = server
		if err := scopedCfg.Save(); err != nil {
			return serverAddedMsg{err: err}
		}

		// Also update the merged config in memory
		if m.cfg.Servers == nil {
			m.cfg.Servers = make(map[string]*mcp.Server)
		}
		m.cfg.Servers[name] = server

		return serverAddedMsg{name: name, scope: string(scope)}
	}
}

// Helper function to extract template variables
func extractTemplateVariables(template string) []string {
	re := regexp.MustCompile(`\{\{(\w+)\}\}`)
	matches := re.FindAllStringSubmatch(template, -1)

	seen := make(map[string]bool)
	var variables []string
	for _, match := range matches {
		if len(match) > 1 && !seen[match[1]] {
			seen[match[1]] = true
			variables = append(variables, match[1])
		}
	}
	return variables
}

// Helper functions

// currentTabLength returns the number of items in the current tab
func (m *Model) currentTabLength() int {
	switch m.activeTab {
	case TabServers:
		return len(m.filteredItems)
	case TabCommands:
		return len(m.commands)
	case TabRules:
		return len(m.rules)
	case TabSkills:
		return len(m.skills)
	case TabPrompts:
		return len(m.prompts)
	case TabHooks:
		return len(m.hooks)
	}
	return 0
}

// adjustCursorForCurrentTab adjusts the cursor after deletion based on the current tab
func (m *Model) adjustCursorForCurrentTab() {
	maxIndex := m.currentTabLength() - 1
	if m.cursor > maxIndex {
		m.cursor = max(0, maxIndex)
	}
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
	name  string
	scope string
	err   error
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
	scope        string
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
	// Collect all installed servers and mark them as checking
	var cmds []tea.Cmd
	for i := range m.allServers {
		if m.allServers[i].Status == ServerStatusInstalled {
			m.allServers[i].Health = HealthStatusChecking
			name := m.allServers[i].Name
			cmds = append(cmds, m.testServer(name))
		}
	}
	m.applyFilter()
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
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

// Resource CRUD helpers
// Note: Create/Edit operations now use native bubbles modals (openRuleEditor, openCommandEditor, etc.)
// The legacy huh-based form wrapper functions have been removed.

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

// handleAliasWizardInput handles input for the alias wizard modal
// TODO: Implement alias wizard functionality
func (m *Model) handleAliasWizardInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.showAliasWizard = false
	}
	return m, nil
}

// renderAliasWizard renders the alias wizard modal
// TODO: Implement alias wizard UI
func (m *Model) renderAliasWizard() string {
	content := ModalTitleStyle.Render("Alias Wizard (Coming Soon)") + "\n\n"
	content += "This feature is not yet implemented.\n\n"
	content += KeyDescStyle.Render("Press Esc to close")
	modal := ModalStyle.Width(50).Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
}
