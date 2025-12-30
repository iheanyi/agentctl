package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/iheanyi/agentctl/pkg/mcp"
)

func TestAdapterRegistry(t *testing.T) {
	// All adapters should be auto-registered via init()
	adapters := All()

	expectedAdapters := []string{"claude", "cursor", "windsurf", "cline", "continue", "zed", "codex", "opencode"}

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

	// Verify MCP and Rules are supported
	hasMCP := false
	hasRules := false
	for _, r := range supported {
		if r == ResourceMCP {
			hasMCP = true
		}
		if r == ResourceRules {
			hasRules = true
		}
	}
	if !hasMCP {
		t.Error("Cursor adapter should support MCP")
	}
	if !hasRules {
		t.Error("Cursor adapter should support Rules")
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
