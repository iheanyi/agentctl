package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"

	"github.com/iheanyi/agentctl/pkg/mcp"
)

// ClaudeDesktopAdapter syncs configuration to Claude Desktop (the Electron app)
type ClaudeDesktopAdapter struct{}

// ClaudeServerConfig represents a server in Claude's config format
type ClaudeServerConfig struct {
	// For local (stdio) servers
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`

	// For remote (http/sse) servers
	Transport string `json:"transport,omitempty"` // "http" or "sse"
	URL       string `json:"url,omitempty"`       // Remote server URL

	// Common fields
	Env       map[string]string `json:"env,omitempty"`
	ManagedBy string            `json:"_managedBy,omitempty"`
}

// ClaudeConfig represents Claude's configuration file structure
type ClaudeConfig struct {
	MCPServers map[string]ClaudeServerConfig `json:"mcpServers,omitempty"`
}

func init() {
	Register(&ClaudeDesktopAdapter{})
}

func (a *ClaudeDesktopAdapter) Name() string {
	return "claude-desktop"
}

func (a *ClaudeDesktopAdapter) Detect() (bool, error) {
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

func (a *ClaudeDesktopAdapter) ConfigPath() string {
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

func (a *ClaudeDesktopAdapter) SupportedResources() []ResourceType {
	return []ResourceType{ResourceMCP}
}

func (a *ClaudeDesktopAdapter) ReadServers() ([]*mcp.Server, error) {
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

func (a *ClaudeDesktopAdapter) WriteServers(servers []*mcp.Server) error {
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

	// Add new servers (only stdio - Claude Desktop doesn't support HTTP/SSE)
	for _, server := range FilterStdioServers(servers) {
		name := server.Name
		if server.Namespace != "" {
			name = server.Namespace
		}

		// Skip servers with empty names to prevent corrupting config
		if name == "" {
			continue
		}

		// Validate: command is required for stdio servers
		if server.Command == "" {
			continue
		}

		cfg := ClaudeServerConfig{
			Command:   server.Command,
			Args:      server.Args,
			Env:       server.Env,
			ManagedBy: ManagedValue,
		}

		config.MCPServers[name] = cfg
	}

	return a.saveConfig(config)
}

func (a *ClaudeDesktopAdapter) loadConfig() (*ClaudeConfig, error) {
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

func (a *ClaudeDesktopAdapter) saveConfig(config *ClaudeConfig) error {
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
