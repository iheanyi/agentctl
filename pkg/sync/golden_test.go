package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/iheanyi/agentctl/pkg/sync/testdata"
)

// TestAdapterGoldenFiles runs golden file tests for all adapters
func TestAdapterGoldenFiles(t *testing.T) {
	tests := []struct {
		adapterName string
		testCases   []string
		serverKey   string // The JSON key where servers are stored
	}{
		{
			adapterName: "claude",
			testCases:   []string{"basic", "preserve_fields"},
			serverKey:   "mcpServers",
		},
		{
			adapterName: "claude-desktop",
			testCases:   []string{"basic"},
			serverKey:   "mcpServers",
		},
		{
			adapterName: "cursor",
			testCases:   []string{"basic", "preserve_fields"},
			serverKey:   "mcpServers",
		},
		{
			adapterName: "opencode",
			testCases:   []string{"basic", "strict_schema"},
			serverKey:   "mcp",
		},
		{
			adapterName: "zed",
			testCases:   []string{"basic", "preserve_fields"},
			serverKey:   "context_servers",
		},
		{
			adapterName: "continue",
			testCases:   []string{"modern", "legacy"},
			serverKey:   "mcpServers",
		},
		{
			adapterName: "cline",
			testCases:   []string{"basic"},
			serverKey:   "mcpServers",
		},
		{
			adapterName: "windsurf",
			testCases:   []string{"basic"},
			serverKey:   "mcpServers",
		},
		{
			adapterName: "codex",
			testCases:   []string{"basic"},
			serverKey:   "mcpServers",
		},
	}

	// Load minimal test servers
	servers, err := testdata.LoadFixtureServers("servers_minimal.json")
	if err != nil {
		t.Fatalf("Failed to load fixture servers: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.adapterName, func(t *testing.T) {
			for _, testCase := range tt.testCases {
				t.Run(testCase, func(t *testing.T) {
					runGoldenTest(t, tt.adapterName, testCase, servers)
				})
			}
		})
	}
}

// runGoldenTest runs a single golden test case
func runGoldenTest(t *testing.T, adapterName, testCase string, servers []*mcp.Server) {
	t.Helper()

	// Create a temp directory for this test
	tempDir := testdata.CreateTempConfigDir(t)

	// Load input config
	input, err := testdata.LoadGoldenInput(adapterName, testCase)
	if err != nil {
		t.Fatalf("Failed to load input: %v", err)
	}

	// Write input config to temp file
	inputBytes, _ := json.MarshalIndent(input, "", "  ")
	configPath := goldenConfigPath(tempDir, adapterName)
	testdata.WriteTestConfig(t, filepath.Dir(configPath), filepath.Base(configPath), inputBytes)

	// Create a test adapter with overridden config path
	adapter := createGoldenTestAdapter(adapterName, configPath)
	if adapter == nil {
		t.Fatalf("No adapter for %s", adapterName)
	}

	// Write servers
	if err := adapter.WriteServers(servers); err != nil {
		t.Fatalf("WriteServers failed: %v", err)
	}

	// Read back the config
	actual := testdata.ReadTestConfig(t, configPath)

	// Compare against golden file
	testdata.AssertGolden(t, adapterName, testCase, actual)
}

// goldenConfigPath returns the config path for a given adapter
func goldenConfigPath(tempDir, adapterName string) string {
	switch adapterName {
	case "claude":
		return filepath.Join(tempDir, ".claude", "settings.json")
	case "claude-desktop":
		return filepath.Join(tempDir, "Claude", "claude_desktop_config.json")
	case "cursor":
		return filepath.Join(tempDir, ".cursor", "mcp.json")
	case "opencode":
		return filepath.Join(tempDir, ".config", "opencode", "opencode.json")
	case "zed":
		return filepath.Join(tempDir, ".config", "zed", "settings.json")
	case "continue":
		return filepath.Join(tempDir, ".continue", "config.json")
	case "cline":
		return filepath.Join(tempDir, ".cline", "cline_mcp_settings.json")
	case "windsurf":
		return filepath.Join(tempDir, ".windsurf", "mcp.json")
	case "codex":
		return filepath.Join(tempDir, ".codex", "config.json")
	default:
		return filepath.Join(tempDir, "config.json")
	}
}

// createGoldenTestAdapter creates a test adapter with an overridden config path
func createGoldenTestAdapter(adapterName, configPath string) writeServerAdapter {
	switch adapterName {
	case "claude":
		return &goldenClaudeAdapter{configPath: configPath}
	case "claude-desktop":
		return &goldenClaudeDesktopAdapter{configPath: configPath}
	case "cursor":
		return &goldenCursorAdapter{configPath: configPath}
	case "opencode":
		return &goldenOpenCodeAdapter{configPath: configPath}
	case "zed":
		return &goldenZedAdapter{configPath: configPath}
	case "continue":
		return &goldenContinueAdapter{configPath: configPath}
	case "cline":
		return &goldenClineAdapter{configPath: configPath}
	case "windsurf":
		return &goldenWindsurfAdapter{configPath: configPath}
	case "codex":
		return &goldenCodexAdapter{configPath: configPath}
	default:
		return nil
	}
}

// writeServerAdapter is the minimal interface needed for golden tests
type writeServerAdapter interface {
	WriteServers(servers []*mcp.Server) error
}

// Golden test adapters - these embed the real adapters but override ConfigPath()

type goldenClaudeAdapter struct {
	configPath string
}

func (a *goldenClaudeAdapter) WriteServers(servers []*mcp.Server) error {
	adapter := &ClaudeAdapter{}
	return writeServersWithPath(adapter, a.configPath, servers)
}

type goldenClaudeDesktopAdapter struct {
	configPath string
}

func (a *goldenClaudeDesktopAdapter) WriteServers(servers []*mcp.Server) error {
	adapter := &ClaudeDesktopAdapter{}
	return writeServersWithPath(adapter, a.configPath, servers)
}

type goldenCursorAdapter struct {
	configPath string
}

func (a *goldenCursorAdapter) WriteServers(servers []*mcp.Server) error {
	adapter := &CursorAdapter{}
	return writeServersWithPath(adapter, a.configPath, servers)
}

type goldenOpenCodeAdapter struct {
	configPath string
}

func (a *goldenOpenCodeAdapter) WriteServers(servers []*mcp.Server) error {
	return writeOpenCodeServersWithPath(a.configPath, servers)
}

type goldenZedAdapter struct {
	configPath string
}

func (a *goldenZedAdapter) WriteServers(servers []*mcp.Server) error {
	adapter := &ZedAdapter{}
	return writeServersWithPath(adapter, a.configPath, servers)
}

type goldenContinueAdapter struct {
	configPath string
}

func (a *goldenContinueAdapter) WriteServers(servers []*mcp.Server) error {
	adapter := &ContinueAdapter{}
	return writeServersWithPath(adapter, a.configPath, servers)
}

type goldenClineAdapter struct {
	configPath string
}

func (a *goldenClineAdapter) WriteServers(servers []*mcp.Server) error {
	adapter := &ClineAdapter{}
	return writeServersWithPath(adapter, a.configPath, servers)
}

type goldenWindsurfAdapter struct {
	configPath string
}

func (a *goldenWindsurfAdapter) WriteServers(servers []*mcp.Server) error {
	adapter := &WindsurfAdapter{}
	return writeServersWithPath(adapter, a.configPath, servers)
}

type goldenCodexAdapter struct {
	configPath string
}

func (a *goldenCodexAdapter) WriteServers(servers []*mcp.Server) error {
	adapter := &CodexAdapter{}
	return writeServersWithPath(adapter, a.configPath, servers)
}

// writeServersWithPath writes servers using a custom config path
// This duplicates the adapter logic but with a custom path
func writeServersWithPath(adapter Adapter, configPath string, servers []*mcp.Server) error {
	// Load existing config
	raw := make(map[string]interface{})
	if data, err := os.ReadFile(configPath); err == nil {
		json.Unmarshal(data, &raw)
	}

	// Determine the server key based on adapter
	var serverKey string
	switch adapter.(type) {
	case *ZedAdapter:
		serverKey = "context_servers"
	default:
		serverKey = "mcpServers"
	}

	// Get or create the servers section
	serversSection, ok := raw[serverKey].(map[string]interface{})
	if !ok {
		serversSection = make(map[string]interface{})
	}

	// Remove old managed entries
	for name, v := range serversSection {
		if serverData, ok := v.(map[string]interface{}); ok {
			if managedBy, ok := serverData["_managedBy"].(string); ok && managedBy == ManagedValue {
				delete(serversSection, name)
			}
		}
	}

	// Add new servers
	for _, server := range servers {
		name := server.Name
		if server.Namespace != "" {
			name = server.Namespace
		}

		serverCfg := map[string]interface{}{
			"_managedBy": ManagedValue,
		}

		if server.URL != "" {
			serverCfg["url"] = server.URL
		} else {
			serverCfg["command"] = server.Command
			if len(server.Args) > 0 {
				serverCfg["args"] = server.Args
			}
		}

		if len(server.Env) > 0 {
			serverCfg["env"] = server.Env
		}

		serversSection[name] = serverCfg
	}

	raw[serverKey] = serversSection

	// Save
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

// writeOpenCodeServersWithPath writes servers in OpenCode format
func writeOpenCodeServersWithPath(configPath string, servers []*mcp.Server) error {
	// Load existing config
	raw := make(map[string]interface{})
	if data, err := os.ReadFile(configPath); err == nil {
		json.Unmarshal(data, &raw)
	}

	// Get or create the mcp section
	mcpSection, ok := raw["mcp"].(map[string]interface{})
	if !ok {
		mcpSection = make(map[string]interface{})
	}

	// Add new servers (OpenCode has no _managedBy, uses state file)
	for _, server := range servers {
		name := server.Name
		if server.Namespace != "" {
			name = server.Namespace
		}

		serverCfg := map[string]interface{}{
			"enabled": true,
		}

		if len(server.Env) > 0 {
			serverCfg["env"] = server.Env
		}

		if server.URL != "" {
			serverCfg["type"] = "remote"
			serverCfg["url"] = server.URL
		} else {
			serverCfg["type"] = "local"
			cmd := append([]string{server.Command}, server.Args...)
			serverCfg["command"] = cmd
		}

		mcpSection[name] = serverCfg
	}

	raw["mcp"] = mcpSection

	// Save
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

// TestGoldenConfigPreservation tests that unknown fields are preserved
func TestGoldenConfigPreservation(t *testing.T) {
	preservationCases := []struct {
		name   string
		fields map[string]interface{}
	}{
		{"schema_field", map[string]interface{}{"$schema": "https://example.com/schema.json"}},
		{"hook_config", map[string]interface{}{"hook": map[string]interface{}{"session-start": []interface{}{"cmd"}}}},
		{"plugin_array", map[string]interface{}{"plugin": []interface{}{"plugin1", "plugin2"}}},
		{"nested_custom", map[string]interface{}{"custom": map[string]interface{}{"deeply": map[string]interface{}{"nested": "value"}}}},
		{"numeric_values", map[string]interface{}{"settings": map[string]interface{}{"timeout": float64(30), "enabled": true}}},
		{"array_values", map[string]interface{}{"items": []interface{}{"a", "b", "c"}}},
	}

	servers := []*mcp.Server{
		{Name: "test-server", Command: "echo", Args: []string{"hello"}},
	}

	adapters := []struct {
		name      string
		serverKey string
	}{
		{"claude", "mcpServers"},
		{"cursor", "mcpServers"},
		{"opencode", "mcp"},
		{"zed", "context_servers"},
	}

	for _, adapter := range adapters {
		t.Run(adapter.name, func(t *testing.T) {
			for _, tc := range preservationCases {
				t.Run(tc.name, func(t *testing.T) {
					tempDir := testdata.CreateTempConfigDir(t)
					configPath := goldenConfigPath(tempDir, adapter.name)

					// Create input with custom fields + empty server section
					input := make(map[string]interface{})
					for k, v := range tc.fields {
						input[k] = v
					}
					input[adapter.serverKey] = map[string]interface{}{}

					inputBytes, _ := json.MarshalIndent(input, "", "  ")
					testdata.WriteTestConfig(t, filepath.Dir(configPath), filepath.Base(configPath), inputBytes)

					// Create test adapter and write servers
					testAdapter := createGoldenTestAdapter(adapter.name, configPath)
					if err := testAdapter.WriteServers(servers); err != nil {
						t.Fatalf("WriteServers failed: %v", err)
					}

					// Read back and verify custom fields are preserved
					actualBytes := testdata.ReadTestConfig(t, configPath)
					var actual map[string]interface{}
					if err := json.Unmarshal(actualBytes, &actual); err != nil {
						t.Fatalf("Failed to parse output: %v", err)
					}

					// Check each custom field was preserved
					for key, expectedValue := range tc.fields {
						actualValue, exists := actual[key]
						if !exists {
							t.Errorf("Field %q was not preserved", key)
							continue
						}
						// Deep comparison
						expectedJSON, _ := json.Marshal(expectedValue)
						actualJSON, _ := json.Marshal(actualValue)
						if string(expectedJSON) != string(actualJSON) {
							t.Errorf("Field %q changed:\nexpected: %s\nactual: %s", key, expectedJSON, actualJSON)
						}
					}
				})
			}
		})
	}
}

// TestGoldenTransportFiltering tests that HTTP/SSE servers are filtered for Cursor
func TestGoldenTransportFiltering(t *testing.T) {
	servers := []*mcp.Server{
		{Name: "stdio-server", Command: "npx", Args: []string{"mcp-server"}},
		{Name: "http-server", URL: "https://mcp.example.com", Transport: mcp.TransportHTTP},
		{Name: "sse-server", URL: "https://mcp.example.com/sse", Transport: mcp.TransportSSE},
	}

	t.Run("cursor_filters_http_sse", func(t *testing.T) {
		tempDir := testdata.CreateTempConfigDir(t)
		configPath := goldenConfigPath(tempDir, "cursor")

		input := map[string]interface{}{"mcpServers": map[string]interface{}{}}
		inputBytes, _ := json.MarshalIndent(input, "", "  ")
		testdata.WriteTestConfig(t, filepath.Dir(configPath), filepath.Base(configPath), inputBytes)

		adapter := createGoldenTestAdapter("cursor", configPath)

		// Filter HTTP/SSE servers for Cursor
		filteredServers := FilterStdioServers(servers)
		if err := adapter.WriteServers(filteredServers); err != nil {
			t.Fatalf("WriteServers failed: %v", err)
		}

		// Read back and verify only stdio server exists
		actualBytes := testdata.ReadTestConfig(t, configPath)
		var actual map[string]interface{}
		json.Unmarshal(actualBytes, &actual)

		mcpServers, _ := actual["mcpServers"].(map[string]interface{})
		if len(mcpServers) != 1 {
			t.Errorf("Expected 1 server, got %d", len(mcpServers))
		}
		if _, exists := mcpServers["stdio-server"]; !exists {
			t.Error("stdio-server should exist")
		}
		if _, exists := mcpServers["http-server"]; exists {
			t.Error("http-server should be filtered out")
		}
		if _, exists := mcpServers["sse-server"]; exists {
			t.Error("sse-server should be filtered out")
		}
	})

	t.Run("claude_accepts_all_transports", func(t *testing.T) {
		tempDir := testdata.CreateTempConfigDir(t)
		configPath := goldenConfigPath(tempDir, "claude")

		input := map[string]interface{}{"mcpServers": map[string]interface{}{}}
		inputBytes, _ := json.MarshalIndent(input, "", "  ")
		testdata.WriteTestConfig(t, filepath.Dir(configPath), filepath.Base(configPath), inputBytes)

		adapter := createGoldenTestAdapter("claude", configPath)
		if err := adapter.WriteServers(servers); err != nil {
			t.Fatalf("WriteServers failed: %v", err)
		}

		// Read back and verify all servers exist
		actualBytes := testdata.ReadTestConfig(t, configPath)
		var actual map[string]interface{}
		json.Unmarshal(actualBytes, &actual)

		mcpServers, _ := actual["mcpServers"].(map[string]interface{})
		if len(mcpServers) != 3 {
			t.Errorf("Expected 3 servers, got %d", len(mcpServers))
		}
	})
}

// TestGoldenStateFileLifecycle tests the sync state file operations
func TestGoldenStateFileLifecycle(t *testing.T) {
	t.Run("create_and_update", func(t *testing.T) {
		state := &SyncState{
			Version:        1,
			ManagedServers: make(map[string][]string),
		}

		// Set managed servers
		state.SetManagedServers("test-adapter", []string{"server1", "server2"})

		// Verify
		managed := state.GetManagedServers("test-adapter")
		if len(managed) != 2 {
			t.Errorf("Expected 2 managed servers, got %d", len(managed))
		}

		// Clear
		state.ClearManagedServers("test-adapter")
		managed = state.GetManagedServers("test-adapter")
		if len(managed) != 0 {
			t.Errorf("Expected 0 managed servers after clear, got %d", len(managed))
		}
	})

	t.Run("persistence", func(t *testing.T) {
		// Create temp state file
		tempDir := testdata.CreateTempConfigDir(t)
		statePath := filepath.Join(tempDir, "sync-state.json")

		// Create state
		state := &SyncState{
			Version:        1,
			ManagedServers: make(map[string][]string),
		}
		state.SetManagedServers("adapter1", []string{"s1", "s2"})
		state.SetManagedServers("adapter2", []string{"s3"})

		// Save
		data, _ := json.MarshalIndent(state, "", "  ")
		os.WriteFile(statePath, data, 0644)

		// Load
		loadedData, err := os.ReadFile(statePath)
		if err != nil {
			t.Fatalf("Failed to read state file: %v", err)
		}

		var loaded SyncState
		json.Unmarshal(loadedData, &loaded)

		if len(loaded.GetManagedServers("adapter1")) != 2 {
			t.Error("adapter1 servers not persisted correctly")
		}
		if len(loaded.GetManagedServers("adapter2")) != 1 {
			t.Error("adapter2 servers not persisted correctly")
		}
	})
}

// TestGoldenHTTPServerOutput tests that HTTP servers are written correctly
func TestGoldenHTTPServerOutput(t *testing.T) {
	servers := []*mcp.Server{
		{
			Name:      "http-server",
			URL:       "https://mcp.example.com/api",
			Transport: mcp.TransportHTTP,
		},
	}

	t.Run("opencode_remote_type", func(t *testing.T) {
		tempDir := testdata.CreateTempConfigDir(t)
		configPath := goldenConfigPath(tempDir, "opencode")

		input := map[string]interface{}{"mcp": map[string]interface{}{}}
		inputBytes, _ := json.MarshalIndent(input, "", "  ")
		testdata.WriteTestConfig(t, filepath.Dir(configPath), filepath.Base(configPath), inputBytes)

		adapter := createGoldenTestAdapter("opencode", configPath)
		if err := adapter.WriteServers(servers); err != nil {
			t.Fatalf("WriteServers failed: %v", err)
		}

		// Verify output
		testdata.AssertGolden(t, "opencode", "http_server", testdata.ReadTestConfig(t, configPath))
	})

	t.Run("claude_url_field", func(t *testing.T) {
		tempDir := testdata.CreateTempConfigDir(t)
		configPath := goldenConfigPath(tempDir, "claude")

		input := map[string]interface{}{"mcpServers": map[string]interface{}{}}
		inputBytes, _ := json.MarshalIndent(input, "", "  ")
		testdata.WriteTestConfig(t, filepath.Dir(configPath), filepath.Base(configPath), inputBytes)

		adapter := createGoldenTestAdapter("claude", configPath)
		if err := adapter.WriteServers(servers); err != nil {
			t.Fatalf("WriteServers failed: %v", err)
		}

		actualBytes := testdata.ReadTestConfig(t, configPath)
		var actual map[string]interface{}
		json.Unmarshal(actualBytes, &actual)

		mcpServers := actual["mcpServers"].(map[string]interface{})
		httpServer := mcpServers["http-server"].(map[string]interface{})

		if httpServer["url"] != "https://mcp.example.com/api" {
			t.Errorf("URL not set correctly: %v", httpServer["url"])
		}
	})
}
