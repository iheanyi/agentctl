package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"

	"github.com/iheanyi/agentctl/pkg/command"
	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/iheanyi/agentctl/pkg/rule"
)

// CursorAdapter syncs configuration to Cursor
type CursorAdapter struct{}

// CursorServerConfig represents a server in Cursor's config format
type CursorServerConfig struct {
	Command   string            `json:"command"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	ManagedBy string            `json:"_managedBy,omitempty"`
}

func init() {
	Register(&CursorAdapter{})
}

func (a *CursorAdapter) Name() string {
	return "cursor"
}

func (a *CursorAdapter) Detect() (bool, error) {
	path := a.ConfigPath()
	if path == "" {
		return false, nil
	}

	// Check if Cursor directory exists
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return false, nil
	}

	return true, nil
}

func (a *CursorAdapter) ConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	switch runtime.GOOS {
	case "darwin", "linux":
		return filepath.Join(homeDir, ".cursor", "mcp.json")
	case "windows":
		return filepath.Join(homeDir, ".cursor", "mcp.json")
	default:
		return ""
	}
}

func (a *CursorAdapter) SupportedResources() []ResourceType {
	return []ResourceType{ResourceMCP, ResourceRules}
}

func (a *CursorAdapter) ReadServers() ([]*mcp.Server, error) {
	raw, err := a.loadRawConfig()
	if err != nil {
		return nil, err
	}

	mcpServers, ok := raw["mcpServers"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	var servers []*mcp.Server
	for name, v := range mcpServers {
		serverData, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		server := &mcp.Server{
			Name: name,
		}

		if cmd, ok := serverData["command"].(string); ok {
			server.Command = cmd
		}

		if args, ok := serverData["args"].([]interface{}); ok {
			for _, arg := range args {
				if str, ok := arg.(string); ok {
					server.Args = append(server.Args, str)
				}
			}
		}

		if envData, ok := serverData["env"].(map[string]interface{}); ok {
			server.Env = make(map[string]string)
			for k, v := range envData {
				if str, ok := v.(string); ok {
					server.Env[k] = str
				}
			}
		}

		servers = append(servers, server)
	}

	return servers, nil
}

func (a *CursorAdapter) WriteServers(servers []*mcp.Server) error {
	// Load the full raw config to preserve all fields
	raw, err := a.loadRawConfig()
	if err != nil {
		return err
	}

	// Get or create the mcpServers section
	mcpServers, ok := raw["mcpServers"].(map[string]interface{})
	if !ok {
		mcpServers = make(map[string]interface{})
	}

	// Remove old agentctl-managed entries
	for name, v := range mcpServers {
		if serverData, ok := v.(map[string]interface{}); ok {
			if managedBy, ok := serverData["_managedBy"].(string); ok && managedBy == ManagedValue {
				delete(mcpServers, name)
			}
		}
	}

	// Add new servers (only stdio transport - Cursor doesn't support HTTP/SSE)
	for _, server := range FilterStdioServers(servers) {
		name := server.Name
		if server.Namespace != "" {
			name = server.Namespace
		}

		// Skip servers with empty names to prevent corrupting config
		if name == "" {
			continue
		}

		serverCfg := map[string]interface{}{
			"command":    server.Command,
			"_managedBy": ManagedValue,
		}

		if len(server.Args) > 0 {
			serverCfg["args"] = server.Args
		}

		if len(server.Env) > 0 {
			serverCfg["env"] = server.Env
		}

		mcpServers[name] = serverCfg
	}

	// Update the mcpServers section in raw config
	raw["mcpServers"] = mcpServers

	return a.saveRawConfig(raw)
}

func (a *CursorAdapter) ReadCommands() ([]*command.Command, error) {
	return nil, nil // Cursor doesn't have commands
}

func (a *CursorAdapter) WriteCommands(commands []*command.Command) error {
	return nil // Cursor doesn't have commands
}

func (a *CursorAdapter) ReadRules() ([]*rule.Rule, error) {
	// Cursor uses .cursorrules file
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	// Check for global .cursorrules
	rulesPath := filepath.Join(homeDir, ".cursorrules")
	if _, err := os.Stat(rulesPath); os.IsNotExist(err) {
		return nil, nil
	}

	r, err := rule.Load(rulesPath)
	if err != nil {
		return nil, err
	}

	return []*rule.Rule{r}, nil
}

func (a *CursorAdapter) WriteRules(rules []*rule.Rule) error {
	if len(rules) == 0 {
		return nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	rulesPath := filepath.Join(homeDir, ".cursorrules")

	// Concatenate all rules with frontmatter indicator
	var content string
	for i, r := range rules {
		if i > 0 {
			content += "\n\n---\n\n"
		}
		content += r.Content
	}

	return os.WriteFile(rulesPath, []byte(content), 0644)
}

// loadRawConfig loads the entire config as a raw map to preserve all fields
func (a *CursorAdapter) loadRawConfig() (map[string]interface{}, error) {
	path := a.ConfigPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]interface{}), nil
		}
		return nil, err
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	return raw, nil
}

// saveRawConfig saves the entire config, preserving all fields
func (a *CursorAdapter) saveRawConfig(raw map[string]interface{}) error {
	path := a.ConfigPath()

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// WorkspaceAdapter implementation for Cursor

// SupportsWorkspace returns true - Cursor supports .cursor/mcp.json in project root
func (a *CursorAdapter) SupportsWorkspace() bool {
	return true
}

// WorkspaceConfigPath returns the path to .cursor/mcp.json in the project directory
func (a *CursorAdapter) WorkspaceConfigPath(projectDir string) string {
	return filepath.Join(projectDir, ".cursor", "mcp.json")
}

// ReadWorkspaceServers reads MCP servers from the project's .cursor/mcp.json file
func (a *CursorAdapter) ReadWorkspaceServers(projectDir string) ([]*mcp.Server, error) {
	path := a.WorkspaceConfigPath(projectDir)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	mcpServers, ok := raw["mcpServers"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	var servers []*mcp.Server
	for name, v := range mcpServers {
		serverData, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		server := &mcp.Server{
			Name:  name,
			Scope: "local",
		}

		if cmd, ok := serverData["command"].(string); ok {
			server.Command = cmd
		}

		if args, ok := serverData["args"].([]interface{}); ok {
			for _, arg := range args {
				if str, ok := arg.(string); ok {
					server.Args = append(server.Args, str)
				}
			}
		}

		if envData, ok := serverData["env"].(map[string]interface{}); ok {
			server.Env = make(map[string]string)
			for k, ev := range envData {
				if str, ok := ev.(string); ok {
					server.Env[k] = str
				}
			}
		}

		servers = append(servers, server)
	}

	return servers, nil
}

// WriteWorkspaceServers writes MCP servers to the project's .cursor/mcp.json file
func (a *CursorAdapter) WriteWorkspaceServers(projectDir string, servers []*mcp.Server) error {
	path := a.WorkspaceConfigPath(projectDir)

	// Load existing config if present
	var raw map[string]interface{}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &raw); err != nil {
			raw = make(map[string]interface{})
		}
	} else {
		raw = make(map[string]interface{})
	}

	// Get or create mcpServers section
	mcpServers, ok := raw["mcpServers"].(map[string]interface{})
	if !ok {
		mcpServers = make(map[string]interface{})
	}

	// Remove old agentctl-managed entries
	for name, v := range mcpServers {
		if serverData, ok := v.(map[string]interface{}); ok {
			if managedBy, ok := serverData["_managedBy"].(string); ok && managedBy == ManagedValue {
				delete(mcpServers, name)
			}
		}
	}

	// Add new servers (only stdio - Cursor doesn't support HTTP/SSE)
	for _, server := range FilterStdioServers(servers) {
		name := server.Name
		if server.Namespace != "" {
			name = server.Namespace
		}

		// Skip servers with empty names to prevent corrupting config
		if name == "" {
			continue
		}

		serverCfg := map[string]interface{}{
			"command":    server.Command,
			"_managedBy": ManagedValue,
		}

		if len(server.Args) > 0 {
			serverCfg["args"] = server.Args
		}

		if len(server.Env) > 0 {
			serverCfg["env"] = server.Env
		}

		mcpServers[name] = serverCfg
	}

	raw["mcpServers"] = mcpServers

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
