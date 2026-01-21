package tui

import (
	"sort"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"

	"github.com/iheanyi/agentctl/internal/tui/testdata"
	"github.com/iheanyi/agentctl/pkg/command"
	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/hook"
	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/iheanyi/agentctl/pkg/rule"
	"github.com/iheanyi/agentctl/pkg/skill"
	"github.com/iheanyi/agentctl/pkg/sync"
)

// stubAdapter is a mock adapter for deterministic testing
type stubAdapter struct {
	name       string
	detected   bool
	configPath string
	resources  []sync.ResourceType
}

func (s *stubAdapter) Name() string                        { return s.name }
func (s *stubAdapter) Detect() (bool, error)               { return s.detected, nil }
func (s *stubAdapter) ConfigPath() string                  { return s.configPath }
func (s *stubAdapter) SupportedResources() []sync.ResourceType { return s.resources }

// testAdapters returns a deterministic set of mock adapters for testing
func testAdapters() []sync.Adapter {
	return []sync.Adapter{
		&stubAdapter{
			name:       "claude",
			detected:   true,
			configPath: "~/.claude/settings.json",
			resources:  []sync.ResourceType{sync.ResourceMCP, sync.ResourceCommands, sync.ResourceRules, sync.ResourceSkills},
		},
		&stubAdapter{
			name:       "cursor",
			detected:   true,
			configPath: "~/.cursor/mcp.json",
			resources:  []sync.ResourceType{sync.ResourceMCP, sync.ResourceRules, sync.ResourceCommands},
		},
		&stubAdapter{
			name:       "codex",
			detected:   false,
			configPath: "~/.codex/config.json",
			resources:  []sync.ResourceType{sync.ResourceMCP},
		},
	}
}

// testDetectedAdapters returns only the adapters marked as detected
func testDetectedAdapters() []sync.Adapter {
	var detected []sync.Adapter
	for _, a := range testAdapters() {
		if ok, _ := a.Detect(); ok {
			detected = append(detected, a)
		}
	}
	return detected
}

// newTestModel creates a Model for testing without loading from disk
func newTestModel() *Model {
	// Create minimal config
	cfg := &config.Config{
		Servers:   make(map[string]*mcp.Server),
		ConfigDir: "/tmp/agentctl-test",
	}

	// Create input components
	searchInput := textinput.New()
	searchInput.Placeholder = "Search..."
	searchInput.Prompt = "/ "

	toolArgInput := textinput.New()
	toolArgInput.Placeholder = `{}`
	toolArgInput.Prompt = "Args (JSON): "

	ruleEditorName := textinput.New()
	ruleEditorName.Placeholder = "my-rule"
	ruleEditorApplies := textinput.New()
	ruleEditorApplies.Placeholder = "*.go"
	ruleEditorContent := textarea.New()
	ruleEditorContent.Placeholder = "Rule content..."
	ruleEditorContent.SetHeight(10)

	skillEditorName := textinput.New()
	skillEditorDesc := textinput.New()
	skillEditorAuthor := textinput.New()
	skillEditorVersion := textinput.New()

	skillCmdEditorName := textinput.New()
	skillCmdEditorDesc := textinput.New()
	skillCmdEditorContent := textarea.New()
	skillCmdEditorContent.SetHeight(12)

	commandEditorName := textinput.New()
	commandEditorDesc := textinput.New()
	commandEditorArgHint := textinput.New()
	commandEditorContent := textarea.New()
	commandEditorContent.SetHeight(10)

	serverEditorName := textinput.New()
	serverEditorSource := textinput.New()
	serverEditorCommand := textinput.New()
	serverEditorArgs := textinput.New()

	aliasWizardName := textinput.New()
	aliasWizardDesc := textinput.New()
	aliasWizardPackage := textinput.New()
	aliasWizardURL := textinput.New()
	aliasWizardLocalPackage := textinput.New()
	aliasWizardRemoteURL := textinput.New()
	aliasWizardGitURL := textinput.New()

	s := spinner.New()
	s.Spinner = spinner.Dot

	return &Model{
		cfg:          cfg,
		selected:     make(map[string]bool),
		filterMode:   FilterAll,
		profile:      "default",
		logs:         []LogEntry{},
		keys:         newKeyMap(),
		spinner:      s,
		searchInput:  searchInput,
		toolArgInput: toolArgInput,
		width:        80,
		height:       24,
		// Rule editor
		ruleEditorName:     ruleEditorName,
		ruleEditorApplies:  ruleEditorApplies,
		ruleEditorContent:  ruleEditorContent,
		ruleEditorPriority: 3,
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
}

// TestGoldenServersTabEmpty tests the servers tab with no servers
func TestGoldenServersTabEmpty(t *testing.T) {
	m := newTestModel()
	m.activeTab = TabServers
	m.filteredItems = []Server{}
	m.allServers = []Server{}

	output := m.View()
	// Strip ANSI codes for golden comparison
	stripped := testdata.StripANSI(output)
	testdata.AssertGolden(t, "servers_tab_empty", []byte(stripped))
}

// TestGoldenServersTabPopulated tests the servers tab with servers
func TestGoldenServersTabPopulated(t *testing.T) {
	m := newTestModel()
	m.activeTab = TabServers

	// Add test servers
	m.allServers = []Server{
		{
			Name:      "filesystem",
			Desc:      "Access local files",
			Status:    ServerStatusInstalled,
			Health:    HealthStatusHealthy,
			Transport: "stdio",
			Command:   "npx",
			ServerConfig: &mcp.Server{
				Name:    "filesystem",
				Command: "npx",
				Args:    []string{"-y", "@modelcontextprotocol/server-filesystem"},
				Scope:   "global",
			},
		},
		{
			Name:      "github",
			Desc:      "GitHub integration",
			Status:    ServerStatusInstalled,
			Health:    HealthStatusUnknown,
			Transport: "stdio",
			Command:   "npx",
			ServerConfig: &mcp.Server{
				Name:    "github",
				Command: "npx",
				Args:    []string{"-y", "@modelcontextprotocol/server-github"},
				Scope:   "local",
			},
		},
		{
			Name:      "postgres",
			Desc:      "PostgreSQL database access",
			Status:    ServerStatusAvailable,
			Health:    HealthStatusUnknown,
			Transport: "stdio",
		},
		{
			Name:      "old-server",
			Desc:      "Legacy server",
			Status:    ServerStatusDisabled,
			Health:    HealthStatusUnknown,
			Transport: "stdio",
			ServerConfig: &mcp.Server{
				Name:     "old-server",
				Command:  "old-cmd",
				Disabled: true,
			},
		},
	}
	m.filteredItems = m.allServers

	output := m.View()
	stripped := testdata.StripANSI(output)
	testdata.AssertGolden(t, "servers_tab_populated", []byte(stripped))
}

// TestGoldenServersTabSelectedItem tests the servers tab with a selected item
func TestGoldenServersTabSelectedItem(t *testing.T) {
	m := newTestModel()
	m.activeTab = TabServers
	m.cursor = 1 // Select second item

	m.allServers = []Server{
		{
			Name:      "filesystem",
			Desc:      "Access local files",
			Status:    ServerStatusInstalled,
			Transport: "stdio",
		},
		{
			Name:      "github",
			Desc:      "GitHub integration",
			Status:    ServerStatusInstalled,
			Transport: "stdio",
		},
	}
	m.filteredItems = m.allServers

	output := m.View()
	stripped := testdata.StripANSI(output)
	testdata.AssertGolden(t, "servers_tab_selected", []byte(stripped))
}

// TestGoldenCommandsTabEmpty tests the commands tab with no commands
func TestGoldenCommandsTabEmpty(t *testing.T) {
	m := newTestModel()
	m.activeTab = TabCommands
	m.commands = []*command.Command{}

	output := m.View()
	stripped := testdata.StripANSI(output)
	testdata.AssertGolden(t, "commands_tab_empty", []byte(stripped))
}

// TestGoldenCommandsTabPopulated tests the commands tab with commands
func TestGoldenCommandsTabPopulated(t *testing.T) {
	m := newTestModel()
	m.activeTab = TabCommands

	m.commands = []*command.Command{
		{
			Name:        "review",
			Description: "Code review with best practices",
			Scope:       "global",
		},
		{
			Name:        "commit",
			Description: "Generate a commit message",
			Scope:       "global",
		},
		{
			Name:        "test",
			Description: "Generate tests for code",
			Scope:       "local",
		},
	}

	output := m.View()
	stripped := testdata.StripANSI(output)
	testdata.AssertGolden(t, "commands_tab_populated", []byte(stripped))
}

// TestGoldenRulesTabEmpty tests the rules tab with no rules
func TestGoldenRulesTabEmpty(t *testing.T) {
	m := newTestModel()
	m.activeTab = TabRules
	m.rules = []*rule.Rule{}

	output := m.View()
	stripped := testdata.StripANSI(output)
	testdata.AssertGolden(t, "rules_tab_empty", []byte(stripped))
}

// TestGoldenRulesTabPopulated tests the rules tab with rules
func TestGoldenRulesTabPopulated(t *testing.T) {
	m := newTestModel()
	m.activeTab = TabRules

	m.rules = []*rule.Rule{
		{
			Name:        "go-style",
			Content:     "Go coding style guidelines",
			Frontmatter: &rule.Frontmatter{Priority: 1},
			Scope:       "global",
		},
		{
			Name:        "typescript",
			Content:     "TypeScript best practices",
			Frontmatter: &rule.Frontmatter{Priority: 2},
			Scope:       "global",
		},
		{
			Name:        "project-rules",
			Content:     "Project-specific rules",
			Frontmatter: &rule.Frontmatter{Priority: 3},
			Scope:       "local",
		},
	}

	output := m.View()
	stripped := testdata.StripANSI(output)
	testdata.AssertGolden(t, "rules_tab_populated", []byte(stripped))
}

// TestGoldenSkillsTabEmpty tests the skills tab with no skills
func TestGoldenSkillsTabEmpty(t *testing.T) {
	m := newTestModel()
	m.activeTab = TabSkills
	m.skills = []*skill.Skill{}

	output := m.View()
	stripped := testdata.StripANSI(output)
	testdata.AssertGolden(t, "skills_tab_empty", []byte(stripped))
}

// TestGoldenSkillsTabPopulated tests the skills tab with skills
func TestGoldenSkillsTabPopulated(t *testing.T) {
	m := newTestModel()
	m.activeTab = TabSkills

	m.skills = []*skill.Skill{
		{
			Name:        "code-review",
			Description: "Multi-step code review workflow",
			Author:      "agentctl",
			Version:     "1.0.0",
			Scope:       "global",
			Commands: []*skill.Command{
				{Name: "review", Description: "Start a review"},
				{Name: "approve", Description: "Approve changes"},
			},
		},
		{
			Name:        "testing",
			Description: "Test generation and execution",
			Author:      "agentctl",
			Version:     "1.0.0",
			Scope:       "local",
			Commands: []*skill.Command{
				{Name: "generate", Description: "Generate tests"},
			},
		},
	}

	output := m.View()
	stripped := testdata.StripANSI(output)
	testdata.AssertGolden(t, "skills_tab_populated", []byte(stripped))
}

// TestGoldenHooksTabEmpty tests the hooks tab with no hooks
func TestGoldenHooksTabEmpty(t *testing.T) {
	m := newTestModel()
	m.activeTab = TabHooks
	m.hooks = []*hook.Hook{}

	output := m.View()
	stripped := testdata.StripANSI(output)
	testdata.AssertGolden(t, "hooks_tab_empty", []byte(stripped))
}

// TestGoldenHooksTabPopulated tests the hooks tab with hooks
func TestGoldenHooksTabPopulated(t *testing.T) {
	m := newTestModel()
	m.activeTab = TabHooks

	m.hooks = []*hook.Hook{
		{
			Type:    "PreToolUse",
			Matcher: "*",
			Command: "echo 'Session started'",
			Source:  "claude",
		},
		{
			Type:    "PostToolUse",
			Matcher: "Bash",
			Command: "npm test",
			Source:  "claude",
		},
	}

	output := m.View()
	stripped := testdata.StripANSI(output)
	testdata.AssertGolden(t, "hooks_tab_populated", []byte(stripped))
}

// TestGoldenConfirmDeleteModal tests the confirm delete modal
func TestGoldenConfirmDeleteModal(t *testing.T) {
	m := newTestModel()
	m.showConfirmDelete = true
	m.confirmDeleteType = "server"
	m.confirmDeleteName = "filesystem"

	output := m.View()
	stripped := testdata.StripANSI(output)
	testdata.AssertGolden(t, "modal_confirm_delete", []byte(stripped))
}

// TestGoldenRuleEditorNew tests the rule editor modal for a new rule
func TestGoldenRuleEditorNew(t *testing.T) {
	m := newTestModel()
	m.showRuleEditor = true
	m.ruleEditorIsNew = true
	m.ruleEditorFocus = 0 // Focus on name field

	output := m.View()
	stripped := testdata.StripANSI(output)
	testdata.AssertGolden(t, "modal_rule_editor_new", []byte(stripped))
}

// TestGoldenSkillEditorNew tests the skill editor modal for a new skill
func TestGoldenSkillEditorNew(t *testing.T) {
	m := newTestModel()
	m.showSkillEditor = true
	m.skillEditorIsNew = true
	m.skillEditorFocus = 0

	output := m.View()
	stripped := testdata.StripANSI(output)
	testdata.AssertGolden(t, "modal_skill_editor_new", []byte(stripped))
}

// TestGoldenCommandEditorNew tests the command editor modal for a new command
func TestGoldenCommandEditorNew(t *testing.T) {
	m := newTestModel()
	m.showCommandEditor = true
	m.commandEditorIsNew = true
	m.commandEditorFocus = 0

	output := m.View()
	stripped := testdata.StripANSI(output)
	testdata.AssertGolden(t, "modal_command_editor_new", []byte(stripped))
}

// TestGoldenServerEditorNew tests the server editor modal for a new server
func TestGoldenServerEditorNew(t *testing.T) {
	m := newTestModel()
	m.showServerEditor = true
	m.serverEditorIsNew = true
	m.serverEditorFocus = 0

	output := m.View()
	stripped := testdata.StripANSI(output)
	testdata.AssertGolden(t, "modal_server_editor_new", []byte(stripped))
}

// TestGoldenHelpOverlay tests the help overlay
func TestGoldenHelpOverlay(t *testing.T) {
	m := newTestModel()
	m.showHelp = true

	output := m.View()
	stripped := testdata.StripANSI(output)
	testdata.AssertGolden(t, "help_overlay", []byte(stripped))
}

// TestGoldenSearchActive tests the view with search active
func TestGoldenSearchActive(t *testing.T) {
	m := newTestModel()
	m.activeTab = TabServers
	m.searching = true
	m.searchInput.SetValue("file")
	m.searchInput.Focus()

	m.allServers = []Server{
		{Name: "filesystem", Desc: "Access files", Status: ServerStatusInstalled, Transport: "stdio"},
		{Name: "github", Desc: "GitHub", Status: ServerStatusInstalled, Transport: "stdio"},
	}
	// Only filesystem matches the search
	m.filteredItems = []Server{m.allServers[0]}

	output := m.View()
	stripped := testdata.StripANSI(output)
	testdata.AssertGolden(t, "search_active", []byte(stripped))
}

// TestGoldenFilterInstalled tests the view with installed filter active
func TestGoldenFilterInstalled(t *testing.T) {
	m := newTestModel()
	m.activeTab = TabServers
	m.filterMode = FilterInstalled

	m.allServers = []Server{
		{Name: "filesystem", Status: ServerStatusInstalled, Transport: "stdio"},
		{Name: "postgres", Status: ServerStatusAvailable, Transport: "stdio"},
	}
	m.filteredItems = []Server{m.allServers[0]}

	output := m.View()
	stripped := testdata.StripANSI(output)
	testdata.AssertGolden(t, "filter_installed", []byte(stripped))
}

// TestGoldenDifferentWidths tests responsive layout at different terminal widths
func TestGoldenDifferentWidths(t *testing.T) {
	widths := []struct {
		name  string
		width int
	}{
		{"narrow_60", 60},
		{"medium_100", 100},
		{"wide_120", 120},
	}

	for _, w := range widths {
		t.Run(w.name, func(t *testing.T) {
			m := newTestModel()
			m.width = w.width
			m.activeTab = TabServers

			m.allServers = []Server{
				{
					Name:      "filesystem",
					Desc:      "Access local files securely with path restrictions",
					Status:    ServerStatusInstalled,
					Transport: "stdio",
				},
			}
			m.filteredItems = m.allServers

			output := m.View()
			stripped := testdata.StripANSI(output)
			testdata.AssertGolden(t, "width_"+w.name, []byte(stripped))
		})
	}
}

// TestGoldenWithLogs tests the view with log entries
func TestGoldenWithLogs(t *testing.T) {
	m := newTestModel()
	m.activeTab = TabServers
	m.allServers = []Server{}
	m.filteredItems = []Server{}

	// Add some log entries
	m.addLog("info", "agentctl UI ready")
	m.addLog("success", "Added server: filesystem")
	m.addLog("warn", "Server github is slow")
	m.addLog("error", "Failed to connect to postgres")

	output := m.View()
	stripped := testdata.StripANSI(output)
	// Strip timestamps since they change on every run
	stripped = testdata.StripTimestamps(stripped)
	testdata.AssertGolden(t, "with_logs", []byte(stripped))
}

// TestGoldenProfilePicker tests the profile picker modal
func TestGoldenProfilePicker(t *testing.T) {
	m := newTestModel()
	m.showProfilePicker = true
	m.profileCursor = 0

	output := m.View()
	stripped := testdata.StripANSI(output)
	testdata.AssertGolden(t, "modal_profile_picker", []byte(stripped))
}

// TestGoldenBackupModalEmpty tests the backup modal with no adapters detected
func TestGoldenBackupModalEmpty(t *testing.T) {
	m := newTestModel()
	m.showBackupModal = true
	m.backupAdapters = []sync.Adapter{}
	m.backupCursor = 0
	m.backupAction = 0 // list

	output := m.View()
	stripped := testdata.StripANSI(output)
	testdata.AssertGolden(t, "modal_backup_empty", []byte(stripped))
}

// TestGoldenBackupModalPopulated tests the backup modal with adapters
func TestGoldenBackupModalPopulated(t *testing.T) {
	m := newTestModel()
	m.showBackupModal = true
	adapters := sync.All()
	sort.Slice(adapters, func(i, j int) bool {
		return adapters[i].Name() < adapters[j].Name()
	})
	m.backupAdapters = adapters[:3] // Use first 3 adapters (sorted)
	m.backupCursor = 0
	m.backupAction = 0 // list

	output := m.View()
	stripped := testdata.StripANSI(output)
	testdata.AssertGolden(t, "modal_backup_populated", []byte(stripped))
}

// TestGoldenBackupModalWithResult tests the backup modal showing a result
func TestGoldenBackupModalWithResult(t *testing.T) {
	m := newTestModel()
	m.showBackupModal = true
	adapters := sync.All()
	sort.Slice(adapters, func(i, j int) bool {
		return adapters[i].Name() < adapters[j].Name()
	})
	m.backupAdapters = adapters[:3]
	m.backupCursor = 0
	m.backupAction = 1 // create
	m.backupResult = "Backup created: claude_settings.json.bak.20260120-153000.json"
	m.backupResultIsError = false

	output := m.View()
	stripped := testdata.StripANSI(output)
	testdata.AssertGolden(t, "modal_backup_with_result", []byte(stripped))
}

// TestGoldenImportWizardStepTool tests the import wizard at tool selection step
func TestGoldenImportWizardStepTool(t *testing.T) {
	m := newTestModel()
	m.showImportWizard = true
	m.importWizardStep = 0 // Select tool
	tools := testDetectedAdapters() // Use mock adapters for deterministic testing
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name() < tools[j].Name()
	})
	m.importWizardTools = tools
	m.importWizardToolCursor = 0
	m.importWizardResources = map[string]bool{
		"servers":  true,
		"commands": true,
		"rules":    true,
		"skills":   true,
	}

	output := m.View()
	stripped := testdata.StripANSI(output)
	testdata.AssertGolden(t, "modal_import_wizard_tool", []byte(stripped))
}

// TestGoldenImportWizardStepResources tests the import wizard at resource selection step
func TestGoldenImportWizardStepResources(t *testing.T) {
	m := newTestModel()
	m.showImportWizard = true
	m.importWizardStep = 1 // Select resources
	tools := testDetectedAdapters() // Use mock adapters for deterministic testing
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name() < tools[j].Name()
	})
	m.importWizardTools = tools
	m.importWizardToolCursor = 0
	m.importWizardResources = map[string]bool{
		"servers":  true,
		"commands": false,
		"rules":    true,
		"skills":   false,
	}

	output := m.View()
	stripped := testdata.StripANSI(output)
	testdata.AssertGolden(t, "modal_import_wizard_resources", []byte(stripped))
}

// TestGoldenImportWizardEmpty tests the import wizard with no tools detected
func TestGoldenImportWizardEmpty(t *testing.T) {
	m := newTestModel()
	m.showImportWizard = true
	m.importWizardStep = 0
	m.importWizardTools = []sync.Adapter{}
	m.importWizardToolCursor = 0

	output := m.View()
	stripped := testdata.StripANSI(output)
	testdata.AssertGolden(t, "modal_import_wizard_empty", []byte(stripped))
}

// TestGoldenToolsTabEmpty tests the tools tab with no adapters registered
func TestGoldenToolsTabEmpty(t *testing.T) {
	m := newTestModel()
	m.activeTab = TabTools
	m.detectedTools = []sync.Adapter{}

	output := m.View()
	stripped := testdata.StripANSI(output)
	testdata.AssertGolden(t, "tools_tab_empty", []byte(stripped))
}

// TestGoldenToolsTabPopulated tests the tools tab with adapters
func TestGoldenToolsTabPopulated(t *testing.T) {
	m := newTestModel()
	m.activeTab = TabTools
	m.detectedTools = testAdapters() // Use mock adapters for deterministic testing
	// Sort by name for consistent ordering (matches real TUI behavior)
	sort.Slice(m.detectedTools, func(i, j int) bool {
		return m.detectedTools[i].Name() < m.detectedTools[j].Name()
	})

	output := m.View()
	stripped := testdata.StripANSI(output)
	testdata.AssertGolden(t, "tools_tab_populated", []byte(stripped))
}
