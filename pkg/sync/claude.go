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

// ClaudeAdapter syncs configuration to Claude Code / Claude Desktop
type ClaudeAdapter struct{}

// ClaudeServerConfig represents a server in Claude's config format
type ClaudeServerConfig struct {
	Command   string            `json:"command"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	ManagedBy string            `json:"_managedBy,omitempty"`
}

// ClaudeConfig represents Claude's configuration file structure
type ClaudeConfig struct {
	MCPServers map[string]ClaudeServerConfig `json:"mcpServers,omitempty"`
}

func init() {
	Register(&ClaudeAdapter{})
}

func (a *ClaudeAdapter) Name() string {
	return "claude"
}

func (a *ClaudeAdapter) Detect() (bool, error) {
	path := a.ConfigPath()
	if path == "" {
		return false, nil
	}

	// Check if Claude config directory exists
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return false, nil
	}

	return true, nil
}

func (a *ClaudeAdapter) ConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(homeDir, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	case "windows":
		return filepath.Join(homeDir, "AppData", "Roaming", "Claude", "claude_desktop_config.json")
	case "linux":
		return filepath.Join(homeDir, ".config", "claude", "claude_desktop_config.json")
	default:
		return ""
	}
}

func (a *ClaudeAdapter) SupportedResources() []ResourceType {
	return []ResourceType{ResourceMCP, ResourceCommands}
}

func (a *ClaudeAdapter) ReadServers() ([]*mcp.Server, error) {
	config, err := a.loadConfig()
	if err != nil {
		return nil, err
	}

	var servers []*mcp.Server
	for name, serverCfg := range config.MCPServers {
		server := &mcp.Server{
			Name:    name,
			Command: serverCfg.Command,
			Args:    serverCfg.Args,
			Env:     serverCfg.Env,
		}
		servers = append(servers, server)
	}

	return servers, nil
}

func (a *ClaudeAdapter) WriteServers(servers []*mcp.Server) error {
	config, err := a.loadConfig()
	if err != nil {
		return err
	}

	if config.MCPServers == nil {
		config.MCPServers = make(map[string]ClaudeServerConfig)
	}

	// Remove old agentctl-managed entries
	for name, serverCfg := range config.MCPServers {
		if serverCfg.ManagedBy == ManagedValue {
			delete(config.MCPServers, name)
		}
	}

	// Add new servers
	for _, server := range servers {
		name := server.Name
		if server.Namespace != "" {
			name = server.Namespace
		}

		config.MCPServers[name] = ClaudeServerConfig{
			Command:   server.Command,
			Args:      server.Args,
			Env:       server.Env,
			ManagedBy: ManagedValue,
		}
	}

	return a.saveConfig(config)
}

func (a *ClaudeAdapter) ReadCommands() ([]*command.Command, error) {
	// Claude Code stores commands differently - in a separate commands directory
	// For now, return empty as this requires more investigation of Claude's format
	return nil, nil
}

func (a *ClaudeAdapter) WriteCommands(commands []*command.Command) error {
	// Claude Code commands are stored in a separate location
	// This will be implemented based on Claude Code's actual format
	return nil
}

func (a *ClaudeAdapter) ReadRules() ([]*rule.Rule, error) {
	return nil, nil // Claude doesn't have rules in the same way
}

func (a *ClaudeAdapter) WriteRules(rules []*rule.Rule) error {
	return nil // Claude doesn't have rules in the same way
}

func (a *ClaudeAdapter) loadConfig() (*ClaudeConfig, error) {
	path := a.ConfigPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ClaudeConfig{}, nil
		}
		return nil, err
	}

	var config ClaudeConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (a *ClaudeAdapter) saveConfig(config *ClaudeConfig) error {
	path := a.ConfigPath()

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
