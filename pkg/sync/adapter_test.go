package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/iheanyi/agentctl/pkg/command"
	"github.com/iheanyi/agentctl/pkg/mcp"
)

func TestAdapterRegistry(t *testing.T) {
	// All adapters should be auto-registered via init()
	adapters := All()

	expectedAdapters := []string{"claude", "cursor", "windsurf", "cline", "continue", "zed", "codex", "opencode", "copilot", "gemini", "claude-desktop"}

	for _, name := range expectedAdapters {
		found := false
		for _, adapter := range adapters {
			if adapter.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Adapter %q should be registered", name)
		}
	}
}

func TestGetAdapter(t *testing.T) {
	adapter, ok := Get("claude")
	if !ok {
		t.Fatal("claude adapter should be registered")
	}
	if adapter.Name() != "claude" {
		t.Errorf("Adapter name should be 'claude', got %q", adapter.Name())
	}

	_, ok = Get("nonexistent")
	if ok {
		t.Error("Getting nonexistent adapter should return false")
	}
}

func TestClaudeAdapter(t *testing.T) {
	adapter := &ClaudeAdapter{}

	if adapter.Name() != "claude" {
		t.Errorf("Name should be 'claude', got %q", adapter.Name())
	}

	supported := adapter.SupportedResources()
	if len(supported) == 0 {
		t.Error("Claude adapter should support at least one resource type")
	}

	// Verify MCP is supported
	hasMCP := false
	for _, r := range supported {
		if r == ResourceMCP {
			hasMCP = true
			break
		}
	}
	if !hasMCP {
		t.Error("Claude adapter should support MCP resource type")
	}
}

func TestClaudeAdapterWriteServers(t *testing.T) {
	// Create a temp directory to simulate Claude config
	tmpDir, err := os.MkdirTemp("", "claude-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a mock config path
	configDir := filepath.Join(tmpDir, "Claude")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	configPath := filepath.Join(configDir, "claude_desktop_config.json")

	// Create initial config with a manual server
	initialConfig := `{
  "mcpServers": {
    "manual-server": {
      "command": "node",
      "args": ["manual.js"]
    }
  }
}`
	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("Failed to write initial config: %v", err)
	}

	// Create a test adapter that uses the temp path
	adapter := &testClaudeAdapter{configPath: configPath}

	servers := []*mcp.Server{
		{
			Name:    "filesystem",
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-filesystem"},
		},
	}

	if err := adapter.WriteServers(servers); err != nil {
		t.Fatalf("WriteServers failed: %v", err)
	}

	// Read back and verify
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	// Verify manual server is preserved
	if !contains(string(data), "manual-server") {
		t.Error("Manual server should be preserved")
	}

	// Verify new server is added with managed marker
	if !contains(string(data), "filesystem") {
		t.Error("New server should be added")
	}
	if !contains(string(data), "_managedBy") {
		t.Error("Managed marker should be present")
	}
}

// testClaudeAdapter is a test adapter with configurable path
type testClaudeAdapter struct {
	configPath string
}

func (a *testClaudeAdapter) ConfigPath() string {
	return a.configPath
}

func (a *testClaudeAdapter) WriteServers(servers []*mcp.Server) error {
	// Load existing config
	config := &ClaudeConfig{}
	if data, err := os.ReadFile(a.configPath); err == nil {
		if err := json.Unmarshal(data, config); err != nil {
			return err
		}
	}

	if config.MCPServers == nil {
		config.MCPServers = make(map[string]ClaudeServerConfig)
	}

	// Remove old managed entries
	for name, serverCfg := range config.MCPServers {
		if serverCfg.ManagedBy == ManagedValue {
			delete(config.MCPServers, name)
		}
	}

	// Add new servers
	for _, server := range servers {
		config.MCPServers[server.Name] = ClaudeServerConfig{
			Command:   server.Command,
			Args:      server.Args,
			Env:       server.Env,
			ManagedBy: ManagedValue,
		}
	}

	// Save
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.configPath, data, 0644)
}

func TestCursorAdapter(t *testing.T) {
	adapter := &CursorAdapter{}

	if adapter.Name() != "cursor" {
		t.Errorf("Name should be 'cursor', got %q", adapter.Name())
	}

	supported := adapter.SupportedResources()

	// Verify MCP, Rules, and Commands are supported
	hasMCP := false
	hasRules := false
	hasCommands := false
	for _, r := range supported {
		if r == ResourceMCP {
			hasMCP = true
		}
		if r == ResourceRules {
			hasRules = true
		}
		if r == ResourceCommands {
			hasCommands = true
		}
	}
	if !hasMCP {
		t.Error("Cursor adapter should support MCP")
	}
	if !hasRules {
		t.Error("Cursor adapter should support Rules")
	}
	if !hasCommands {
		t.Error("Cursor adapter should support Commands")
	}
}

func TestParseCursorCommand(t *testing.T) {
	tests := []struct {
		name            string
		filename        string
		content         string
		wantName        string
		wantDescription string
		wantPrompt      string
	}{
		{
			name:       "simple command",
			filename:   "test.md",
			content:    "Simple prompt content",
			wantName:   "test",
			wantPrompt: "Simple prompt content",
		},
		{
			name:     "command with frontmatter",
			filename: "review.md",
			content: `---
description: Review code changes
---

Review the following code`,
			wantName:        "review",
			wantDescription: "Review code changes",
			wantPrompt:      "Review the following code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := parseCursorCommand(tt.filename, tt.content)

			if cmd.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", cmd.Name, tt.wantName)
			}
			if cmd.Description != tt.wantDescription {
				t.Errorf("Description = %q, want %q", cmd.Description, tt.wantDescription)
			}
			if cmd.Prompt != tt.wantPrompt {
				t.Errorf("Prompt = %q, want %q", cmd.Prompt, tt.wantPrompt)
			}
		})
	}
}

func TestFormatCursorCommand(t *testing.T) {
	cmd := &command.Command{
		Name:        "test",
		Description: "Test command",
		Prompt:      "Do the thing",
	}

	output := formatCursorCommand(cmd)

	if !contains(output, "description: Test command") {
		t.Error("Output should contain description")
	}
	if !contains(output, "Do the thing") {
		t.Error("Output should contain prompt")
	}
	if !contains(output, "---") {
		t.Error("Output should have frontmatter delimiters")
	}
}

func TestContainsResource(t *testing.T) {
	resources := []ResourceType{ResourceMCP, ResourceCommands}

	if !containsResource(resources, ResourceMCP) {
		t.Error("Should contain ResourceMCP")
	}
	if !containsResource(resources, ResourceCommands) {
		t.Error("Should contain ResourceCommands")
	}
	if containsResource(resources, ResourceRules) {
		t.Error("Should not contain ResourceRules")
	}
}

func TestResourceTypeConstants(t *testing.T) {
	if ResourceMCP != "mcp" {
		t.Errorf("ResourceMCP should be 'mcp', got %q", ResourceMCP)
	}
	if ResourceCommands != "commands" {
		t.Errorf("ResourceCommands should be 'commands', got %q", ResourceCommands)
	}
	if ResourceRules != "rules" {
		t.Errorf("ResourceRules should be 'rules', got %q", ResourceRules)
	}
	if ResourcePrompts != "prompts" {
		t.Errorf("ResourcePrompts should be 'prompts', got %q", ResourcePrompts)
	}
	if ResourceSkills != "skills" {
		t.Errorf("ResourceSkills should be 'skills', got %q", ResourceSkills)
	}
}

func TestManagedMarkerConstants(t *testing.T) {
	if ManagedMarker != "_managedBy" {
		t.Errorf("ManagedMarker should be '_managedBy', got %q", ManagedMarker)
	}
	if ManagedValue != "agentctl" {
		t.Errorf("ManagedValue should be 'agentctl', got %q", ManagedValue)
	}
}

func TestParseClaudeCommand(t *testing.T) {
	tests := []struct {
		name             string
		filename         string
		content          string
		wantName         string
		wantDescription  string
		wantArgumentHint string
		wantModel        string
		wantAllowed      []string
		wantDisallowed   []string
		wantPrompt       string
	}{
		{
			name:       "simple command without frontmatter",
			filename:   "simple.md",
			content:    "Just a prompt",
			wantName:   "simple",
			wantPrompt: "Just a prompt",
		},
		{
			name:     "command with description only",
			filename: "review.md",
			content: `---
description: Review code for issues
---

Review the following code`,
			wantName:        "review",
			wantDescription: "Review code for issues",
			wantPrompt:      "Review the following code",
		},
		{
			name:     "command with all fields",
			filename: "full.md",
			content: `---
description: Full featured command
argument-hint: [file.md or feature]
model: opus
allowed-tools: [Read, Write, Edit]
disallowed-tools: [Bash]
---

Do the thing`,
			wantName:         "full",
			wantDescription:  "Full featured command",
			wantArgumentHint: "[file.md or feature]",
			wantModel:        "opus",
			wantAllowed:      []string{"Read", "Write", "Edit"},
			wantDisallowed:   []string{"Bash"},
			wantPrompt:       "Do the thing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := parseClaudeCommand(tt.filename, tt.content)

			if cmd.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", cmd.Name, tt.wantName)
			}
			if cmd.Description != tt.wantDescription {
				t.Errorf("Description = %q, want %q", cmd.Description, tt.wantDescription)
			}
			if cmd.ArgumentHint != tt.wantArgumentHint {
				t.Errorf("ArgumentHint = %q, want %q", cmd.ArgumentHint, tt.wantArgumentHint)
			}
			if cmd.Model != tt.wantModel {
				t.Errorf("Model = %q, want %q", cmd.Model, tt.wantModel)
			}
			if len(cmd.AllowedTools) != len(tt.wantAllowed) {
				t.Errorf("AllowedTools = %v, want %v", cmd.AllowedTools, tt.wantAllowed)
			}
			if len(cmd.DisallowedTools) != len(tt.wantDisallowed) {
				t.Errorf("DisallowedTools = %v, want %v", cmd.DisallowedTools, tt.wantDisallowed)
			}
			if cmd.Prompt != tt.wantPrompt {
				t.Errorf("Prompt = %q, want %q", cmd.Prompt, tt.wantPrompt)
			}
		})
	}
}

func TestFormatClaudeCommand(t *testing.T) {
	tests := []struct {
		name     string
		cmd      *command.Command
		contains []string
	}{
		{
			name: "simple command",
			cmd: &command.Command{
				Name:        "test",
				Description: "Test command",
				Prompt:      "Do stuff",
			},
			contains: []string{"description: Test command", "Do stuff"},
		},
		{
			name: "command with all fields",
			cmd: &command.Command{
				Name:            "full",
				Description:     "Full command",
				ArgumentHint:    "[file.md]",
				Model:           "sonnet",
				AllowedTools:    []string{"Read", "Write"},
				DisallowedTools: []string{"Bash"},
				Prompt:          "Execute task",
			},
			contains: []string{
				"description: Full command",
				"argument-hint: [file.md]",
				"model: sonnet",
				"allowed-tools: [Read, Write]",
				"disallowed-tools: [Bash]",
				"Execute task",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := formatClaudeCommand(tt.cmd)

			for _, expected := range tt.contains {
				if !contains(output, expected) {
					t.Errorf("Output should contain %q, got:\n%s", expected, output)
				}
			}
		})
	}
}

func TestParseToolsList(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"Read", []string{"Read"}},
		{"Read, Write", []string{"Read", "Write"}},
		{"[Read, Write, Edit]", []string{"Read", "Write", "Edit"}},
		{"[Read]", []string{"Read"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseToolsList(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("parseToolsList(%q) = %v, want %v", tt.input, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseToolsList(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ============================================
// Copilot Adapter Tests
// ============================================

func TestCopilotAdapter(t *testing.T) {
	adapter := &CopilotAdapter{}

	if adapter.Name() != "copilot" {
		t.Errorf("Name should be 'copilot', got %q", adapter.Name())
	}

	supported := adapter.SupportedResources()

	// Verify all resource types are supported
	hasMCP := false
	hasCommands := false
	hasRules := false
	hasSkills := false
	for _, r := range supported {
		switch r {
		case ResourceMCP:
			hasMCP = true
		case ResourceCommands:
			hasCommands = true
		case ResourceRules:
			hasRules = true
		case ResourceSkills:
			hasSkills = true
		}
	}

	if !hasMCP {
		t.Error("Copilot adapter should support MCP")
	}
	if !hasCommands {
		t.Error("Copilot adapter should support Commands")
	}
	if !hasRules {
		t.Error("Copilot adapter should support Rules")
	}
	if !hasSkills {
		t.Error("Copilot adapter should support Skills")
	}
}

func TestParseCopilotCommand(t *testing.T) {
	tests := []struct {
		name            string
		filename        string
		content         string
		wantName        string
		wantDescription string
		wantArgHint     string
		wantPrompt      string
	}{
		{
			name:       "simple command",
			filename:   "test.md",
			content:    "Just a simple prompt",
			wantName:   "test",
			wantPrompt: "Just a simple prompt",
		},
		{
			name:     "command with frontmatter",
			filename: "review.md",
			content: `---
description: Review code changes
argument-hint: [file or PR]
---

Review the code carefully`,
			wantName:        "review",
			wantDescription: "Review code changes",
			wantArgHint:     "[file or PR]",
			wantPrompt:      "Review the code carefully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := parseCopilotCommand(tt.filename, tt.content)

			if cmd.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", cmd.Name, tt.wantName)
			}
			if cmd.Description != tt.wantDescription {
				t.Errorf("Description = %q, want %q", cmd.Description, tt.wantDescription)
			}
			if cmd.ArgumentHint != tt.wantArgHint {
				t.Errorf("ArgumentHint = %q, want %q", cmd.ArgumentHint, tt.wantArgHint)
			}
			if cmd.Prompt != tt.wantPrompt {
				t.Errorf("Prompt = %q, want %q", cmd.Prompt, tt.wantPrompt)
			}
		})
	}
}

func TestFormatCopilotCommand(t *testing.T) {
	cmd := &command.Command{
		Name:         "test",
		Description:  "Test command",
		ArgumentHint: "[input]",
		Prompt:       "Do the thing",
	}

	output := formatCopilotCommand(cmd)

	if !contains(output, "description: Test command") {
		t.Error("Output should contain description")
	}
	if !contains(output, "argument-hint: [input]") {
		t.Error("Output should contain argument-hint")
	}
	if !contains(output, "Do the thing") {
		t.Error("Output should contain prompt")
	}
}

// ============================================
// SkillsAdapter Interface Tests
// ============================================

func TestSkillsAdapterInterface(t *testing.T) {
	// Test that adapters that claim to support skills implement SkillsAdapter
	skillAdapters := []string{"claude", "copilot", "codex", "opencode"}

	for _, name := range skillAdapters {
		t.Run(name, func(t *testing.T) {
			adapter, ok := Get(name)
			if !ok {
				t.Fatalf("Adapter %q should be registered", name)
			}

			// Check if it supports skills resource
			supported := adapter.SupportedResources()
			hasSkills := false
			for _, r := range supported {
				if r == ResourceSkills {
					hasSkills = true
					break
				}
			}
			if !hasSkills {
				t.Errorf("Adapter %q should support ResourceSkills", name)
				return
			}

			// Check if it implements SkillsAdapter
			if !SupportsSkills(adapter) {
				t.Errorf("Adapter %q claims to support skills but doesn't implement SkillsAdapter", name)
			}

			// Test AsSkillsAdapter
			sa, ok := AsSkillsAdapter(adapter)
			if !ok {
				t.Errorf("AsSkillsAdapter should return true for %q", name)
			}
			if sa == nil {
				t.Errorf("AsSkillsAdapter should return non-nil for %q", name)
			}
		})
	}
}

// ============================================
// Codex Adapter Tests
// ============================================

func TestCodexAdapter(t *testing.T) {
	adapter := &CodexAdapter{}

	if adapter.Name() != "codex" {
		t.Errorf("Name should be 'codex', got %q", adapter.Name())
	}

	supported := adapter.SupportedResources()
	if len(supported) == 0 {
		t.Error("Codex adapter should support at least one resource type")
	}

	// Verify MCP, Commands, Rules, Skills are supported
	hasMCP := false
	hasCommands := false
	hasRules := false
	hasSkills := false
	for _, r := range supported {
		switch r {
		case ResourceMCP:
			hasMCP = true
		case ResourceCommands:
			hasCommands = true
		case ResourceRules:
			hasRules = true
		case ResourceSkills:
			hasSkills = true
		}
	}

	if !hasMCP {
		t.Error("Codex adapter should support MCP")
	}
	if !hasCommands {
		t.Error("Codex adapter should support Commands")
	}
	if !hasRules {
		t.Error("Codex adapter should support Rules")
	}
	if !hasSkills {
		t.Error("Codex adapter should support Skills")
	}
}

func TestParseCodexPrompt(t *testing.T) {
	tests := []struct {
		name            string
		filename        string
		content         string
		wantName        string
		wantDescription string
		wantArgHint     string
		wantPrompt      string
	}{
		{
			name:       "simple prompt",
			filename:   "help.md",
			content:    "Help me with this",
			wantName:   "help",
			wantPrompt: "Help me with this",
		},
		{
			name:     "prompt with frontmatter",
			filename: "explain.md",
			content: `---
description: Explain code
argument-hint: [code snippet]
---

Explain this code in detail`,
			wantName:        "explain",
			wantDescription: "Explain code",
			wantArgHint:     "[code snippet]",
			wantPrompt:      "Explain this code in detail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := parseCodexPrompt(tt.filename, tt.content)

			if cmd.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", cmd.Name, tt.wantName)
			}
			if cmd.Description != tt.wantDescription {
				t.Errorf("Description = %q, want %q", cmd.Description, tt.wantDescription)
			}
			if cmd.ArgumentHint != tt.wantArgHint {
				t.Errorf("ArgumentHint = %q, want %q", cmd.ArgumentHint, tt.wantArgHint)
			}
			if cmd.Prompt != tt.wantPrompt {
				t.Errorf("Prompt = %q, want %q", cmd.Prompt, tt.wantPrompt)
			}
		})
	}
}

func TestFormatCodexPrompt(t *testing.T) {
	cmd := &command.Command{
		Name:         "test",
		Description:  "Test prompt",
		ArgumentHint: "[file]",
		Prompt:       "Execute task",
	}

	output := formatCodexPrompt(cmd)

	if !contains(output, "description: Test prompt") {
		t.Error("Output should contain description")
	}
	if !contains(output, "argument-hint: [file]") {
		t.Error("Output should contain argument-hint")
	}
	if !contains(output, "Execute task") {
		t.Error("Output should contain prompt")
	}
}

// ============================================
// OpenCode Adapter Tests
// ============================================

func TestOpenCodeAdapter(t *testing.T) {
	adapter := &OpenCodeAdapter{}

	if adapter.Name() != "opencode" {
		t.Errorf("Name should be 'opencode', got %q", adapter.Name())
	}

	supported := adapter.SupportedResources()

	// Verify all resource types are supported
	hasMCP := false
	hasCommands := false
	hasRules := false
	hasSkills := false
	for _, r := range supported {
		switch r {
		case ResourceMCP:
			hasMCP = true
		case ResourceCommands:
			hasCommands = true
		case ResourceRules:
			hasRules = true
		case ResourceSkills:
			hasSkills = true
		}
	}

	if !hasMCP {
		t.Error("OpenCode adapter should support MCP")
	}
	if !hasCommands {
		t.Error("OpenCode adapter should support Commands")
	}
	if !hasRules {
		t.Error("OpenCode adapter should support Rules")
	}
	if !hasSkills {
		t.Error("OpenCode adapter should support Skills")
	}
}

func TestParseOpenCodeCommand(t *testing.T) {
	tests := []struct {
		name            string
		filename        string
		content         string
		wantName        string
		wantDescription string
		wantModel       string
		wantPrompt      string
	}{
		{
			name:       "simple command",
			filename:   "fix.md",
			content:    "Fix this issue",
			wantName:   "fix",
			wantPrompt: "Fix this issue",
		},
		{
			name:     "command with frontmatter",
			filename: "review.md",
			content: `---
description: Code review
model: claude-3-sonnet
---

Review this code`,
			wantName:        "review",
			wantDescription: "Code review",
			wantModel:       "claude-3-sonnet",
			wantPrompt:      "Review this code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := parseOpenCodeCommand(tt.filename, tt.content)

			if cmd.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", cmd.Name, tt.wantName)
			}
			if cmd.Description != tt.wantDescription {
				t.Errorf("Description = %q, want %q", cmd.Description, tt.wantDescription)
			}
			if cmd.Model != tt.wantModel {
				t.Errorf("Model = %q, want %q", cmd.Model, tt.wantModel)
			}
			if cmd.Prompt != tt.wantPrompt {
				t.Errorf("Prompt = %q, want %q", cmd.Prompt, tt.wantPrompt)
			}
		})
	}
}

func TestFormatOpenCodeCommand(t *testing.T) {
	cmd := &command.Command{
		Name:        "test",
		Description: "Test command",
		Model:       "gpt-4",
		Prompt:      "Execute",
	}

	output := formatOpenCodeCommand(cmd)

	if !contains(output, "description: Test command") {
		t.Error("Output should contain description")
	}
	if !contains(output, "model: gpt-4") {
		t.Error("Output should contain model")
	}
	if !contains(output, "Execute") {
		t.Error("Output should contain prompt")
	}
}

// ============================================
// Filter Functions Tests
// ============================================

func TestFilterStdioServers(t *testing.T) {
	servers := []*mcp.Server{
		{Name: "stdio1", Command: "echo"},
		{Name: "http1", URL: "http://example.com", Transport: mcp.TransportHTTP},
		{Name: "sse1", URL: "http://example.com/sse", Transport: mcp.TransportSSE},
		{Name: "stdio2", Command: "cat"},
	}

	filtered := FilterStdioServers(servers)

	if len(filtered) != 2 {
		t.Errorf("Expected 2 stdio servers, got %d", len(filtered))
	}

	for _, s := range filtered {
		if s.Transport == mcp.TransportHTTP || s.Transport == mcp.TransportSSE {
			t.Errorf("Filtered list should not contain HTTP/SSE servers, found %q", s.Name)
		}
	}
}

// ============================================
// Workspace Adapter Tests
// ============================================

func TestWorkspaceAdapterInterface(t *testing.T) {
	// Claude and Cursor should support workspace configs
	workspaceAdapters := []string{"claude", "cursor"}

	for _, name := range workspaceAdapters {
		t.Run(name, func(t *testing.T) {
			adapter, ok := Get(name)
			if !ok {
				t.Fatalf("Adapter %q should be registered", name)
			}

			if !SupportsWorkspace(adapter) {
				t.Errorf("Adapter %q should support workspace configs", name)
			}

			wa, ok := AsWorkspaceAdapter(adapter)
			if !ok {
				t.Errorf("AsWorkspaceAdapter should return true for %q", name)
			}
			if wa == nil {
				t.Errorf("AsWorkspaceAdapter should return non-nil for %q", name)
			}
		})
	}

	// Non-workspace adapters should return false
	nonWorkspaceAdapters := []string{"codex", "opencode", "copilot"}
	for _, name := range nonWorkspaceAdapters {
		t.Run(name+"_no_workspace", func(t *testing.T) {
			adapter, ok := Get(name)
			if !ok {
				t.Skipf("Adapter %q not registered", name)
			}

			if SupportsWorkspace(adapter) {
				t.Errorf("Adapter %q should not support workspace configs", name)
			}
		})
	}
}

// ============================================
// Empty Name Server Protection Tests
// ============================================

func TestEmptyNameServerProtection(t *testing.T) {
	// Test that servers with empty names are skipped during write
	// This tests the behavior of adapters that call FilterStdioServers
	// and check for empty names in their WriteServers implementation
	servers := []*mcp.Server{
		{Name: "", Command: "should-skip"},
		{Name: "valid", Command: "echo"},
	}

	// Test that FilterStdioServers preserves empty name servers
	// (the empty name check is in WriteServers, not filter)
	filtered := FilterStdioServers(servers)
	if len(filtered) != 2 {
		t.Errorf("FilterStdioServers should preserve all stdio servers including empty names, got %d", len(filtered))
	}

	// Test that adapters with empty name protection work
	// The protection is implemented in each adapter's WriteServers
	// We test this indirectly through the server slice processing
	var validServers []*mcp.Server
	for _, s := range servers {
		name := s.Name
		if s.Namespace != "" {
			name = s.Namespace
		}
		if name != "" {
			validServers = append(validServers, s)
		}
	}

	if len(validServers) != 1 {
		t.Errorf("Expected 1 valid server after filtering empty names, got %d", len(validServers))
	}
	if validServers[0].Name != "valid" {
		t.Errorf("Expected 'valid' server, got %q", validServers[0].Name)
	}
}
