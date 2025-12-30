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
type OpenCodeServerConfig struct {
	Command   string            `json:"command"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	ManagedBy string            `json:"_managedBy,omitempty"`
}

// OpenCodeConfig represents OpenCode's configuration
type OpenCodeConfig struct {
	MCPServers map[string]OpenCodeServerConfig `json:"mcpServers,omitempty"`
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

	switch runtime.GOOS {
	case "darwin", "linux":
		return filepath.Join(homeDir, ".config", "opencode", "config.json")
	case "windows":
		return filepath.Join(homeDir, "AppData", "Roaming", "opencode", "config.json")
	default:
		return ""
	}
}

func (a *OpenCodeAdapter) SupportedResources() []ResourceType {
	return []ResourceType{ResourceMCP, ResourceCommands}
}

func (a *OpenCodeAdapter) ReadServers() ([]*mcp.Server, error) {
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

func (a *OpenCodeAdapter) WriteServers(servers []*mcp.Server) error {
	config, err := a.loadConfig()
	if err != nil {
		return err
	}

	if config.MCPServers == nil {
		config.MCPServers = make(map[string]OpenCodeServerConfig)
	}

	for name, serverCfg := range config.MCPServers {
		if serverCfg.ManagedBy == ManagedValue {
			delete(config.MCPServers, name)
		}
	}

	for _, server := range servers {
		name := server.Name
		if server.Namespace != "" {
			name = server.Namespace
		}

		config.MCPServers[name] = OpenCodeServerConfig{
			Command:   server.Command,
			Args:      server.Args,
			Env:       server.Env,
			ManagedBy: ManagedValue,
		}
	}

	return a.saveConfig(config)
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

func (a *OpenCodeAdapter) loadConfig() (*OpenCodeConfig, error) {
	path := a.ConfigPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &OpenCodeConfig{}, nil
		}
		return nil, err
	}

	var config OpenCodeConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (a *OpenCodeAdapter) saveConfig(config *OpenCodeConfig) error {
	path := a.ConfigPath()

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
