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

// OpenCodeAdapter syncs configuration to OpenCode (opencode.ai)
type OpenCodeAdapter struct{}

// OpenCodeServerConfig represents a server in OpenCode's config format
// OpenCode uses a different format than Claude Desktop:
// - "type": "local" or "remote"
// - "command": array of strings (command + args combined)
// - "enabled": boolean
// - "url": for remote servers
// Note: OpenCode's schema is strict - no custom fields allowed
type OpenCodeServerConfig struct {
	Type    string            `json:"type"`
	Command []string          `json:"command,omitempty"`
	URL     string            `json:"url,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Enabled bool              `json:"enabled"`
}

func init() {
	Register(&OpenCodeAdapter{})
}

func (a *OpenCodeAdapter) Name() string {
	return "opencode"
}

func (a *OpenCodeAdapter) Detect() (bool, error) {
	path := a.ConfigPath()
	if path == "" {
		return false, nil
	}

	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return false, nil
	}

	return true, nil
}

func (a *OpenCodeAdapter) ConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// OpenCode uses opencode.json, not config.json
	switch runtime.GOOS {
	case "darwin", "linux":
		return filepath.Join(homeDir, ".config", "opencode", "opencode.json")
	case "windows":
		return filepath.Join(homeDir, "AppData", "Roaming", "opencode", "opencode.json")
	default:
		return ""
	}
}

func (a *OpenCodeAdapter) SupportedResources() []ResourceType {
	return []ResourceType{ResourceMCP, ResourceCommands}
}

func (a *OpenCodeAdapter) ReadServers() ([]*mcp.Server, error) {
	raw, err := a.loadRawConfig()
	if err != nil {
		return nil, err
	}

	mcpSection, ok := raw["mcp"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	var servers []*mcp.Server
	for name, v := range mcpSection {
		serverData, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		// Skip disabled servers
		if enabled, ok := serverData["enabled"].(bool); ok && !enabled {
			continue
		}

		server := &mcp.Server{
			Name: name,
		}

		// Parse env
		if envData, ok := serverData["env"].(map[string]interface{}); ok {
			server.Env = make(map[string]string)
			for k, v := range envData {
				if str, ok := v.(string); ok {
					server.Env[k] = str
				}
			}
		}

		// Handle local servers (command is array)
		serverType, _ := serverData["type"].(string)
		if serverType == "local" {
			if cmdArray, ok := serverData["command"].([]interface{}); ok && len(cmdArray) > 0 {
				if cmd, ok := cmdArray[0].(string); ok {
					server.Command = cmd
				}
				for i := 1; i < len(cmdArray); i++ {
					if arg, ok := cmdArray[i].(string); ok {
						server.Args = append(server.Args, arg)
					}
				}
			}
		}

		// Handle remote servers (URL-based)
		if serverType == "remote" {
			if url, ok := serverData["url"].(string); ok {
				server.URL = url
				server.Transport = mcp.TransportHTTP
			}
		}

		servers = append(servers, server)
	}

	return servers, nil
}

func (a *OpenCodeAdapter) WriteServers(servers []*mcp.Server) error {
	// Load the full raw config to preserve all fields
	raw, err := a.loadRawConfig()
	if err != nil {
		return err
	}

	// Load sync state to know which servers we previously managed
	state, err := LoadState()
	if err != nil {
		return err
	}

	// Get or create the mcp section
	mcpSection, ok := raw["mcp"].(map[string]interface{})
	if !ok {
		mcpSection = make(map[string]interface{})
	}

	// Remove previously managed servers (tracked in state file)
	for _, name := range state.GetManagedServers(a.Name()) {
		delete(mcpSection, name)
	}

	// Track new managed server names
	var managedNames []string

	// Add new servers (NO custom fields - OpenCode schema is strict)
	for _, server := range servers {
		name := server.Name
		if server.Namespace != "" {
			name = server.Namespace
		}

		// Skip servers with empty names to prevent corrupting config
		if name == "" {
			continue
		}

		serverCfg := map[string]interface{}{
			"enabled": true,
		}

		// Add env if present
		if len(server.Env) > 0 {
			serverCfg["env"] = server.Env
		}

		// Determine server type and set appropriate fields
		if server.URL != "" {
			// Remote server
			serverCfg["type"] = "remote"
			serverCfg["url"] = server.URL
		} else {
			// Local server - combine command and args into single array
			serverCfg["type"] = "local"
			cmd := append([]string{server.Command}, server.Args...)
			serverCfg["command"] = cmd
		}

		mcpSection[name] = serverCfg
		managedNames = append(managedNames, name)
	}

	// Update the mcp section in raw config
	raw["mcp"] = mcpSection

	// Save the config
	if err := a.saveRawConfig(raw); err != nil {
		return err
	}

	// Update state with new managed servers
	state.SetManagedServers(a.Name(), managedNames)
	return state.Save()
}

func (a *OpenCodeAdapter) ReadCommands() ([]*command.Command, error) {
	return nil, nil
}

func (a *OpenCodeAdapter) WriteCommands(commands []*command.Command) error {
	return nil
}

func (a *OpenCodeAdapter) ReadRules() ([]*rule.Rule, error) {
	return nil, nil
}

func (a *OpenCodeAdapter) WriteRules(rules []*rule.Rule) error {
	return nil
}

// loadRawConfig loads the entire config as a raw map to preserve all fields
func (a *OpenCodeAdapter) loadRawConfig() (map[string]interface{}, error) {
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
func (a *OpenCodeAdapter) saveRawConfig(raw map[string]interface{}) error {
	path := a.ConfigPath()

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
