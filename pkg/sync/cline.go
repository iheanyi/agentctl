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

// ClineAdapter syncs configuration to Cline
type ClineAdapter struct{}

// ClineServerConfig represents a server in Cline's config format
type ClineServerConfig struct {
	Command   string            `json:"command"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	ManagedBy string            `json:"_managedBy,omitempty"`
}

// ClineConfig represents Cline's MCP configuration
type ClineConfig struct {
	MCPServers map[string]ClineServerConfig `json:"mcpServers,omitempty"`
}

func init() {
	Register(&ClineAdapter{})
}

func (a *ClineAdapter) Name() string {
	return "cline"
}

func (a *ClineAdapter) Detect() (bool, error) {
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

func (a *ClineAdapter) ConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	switch runtime.GOOS {
	case "darwin", "linux":
		return filepath.Join(homeDir, ".cline", "mcp_settings.json")
	case "windows":
		return filepath.Join(homeDir, ".cline", "mcp_settings.json")
	default:
		return ""
	}
}

func (a *ClineAdapter) SupportedResources() []ResourceType {
	return []ResourceType{ResourceMCP, ResourceRules}
}

func (a *ClineAdapter) ReadServers() ([]*mcp.Server, error) {
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

func (a *ClineAdapter) WriteServers(servers []*mcp.Server) error {
	config, err := a.loadConfig()
	if err != nil {
		return err
	}

	if config.MCPServers == nil {
		config.MCPServers = make(map[string]ClineServerConfig)
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

		config.MCPServers[name] = ClineServerConfig{
			Command:   server.Command,
			Args:      server.Args,
			Env:       server.Env,
			ManagedBy: ManagedValue,
		}
	}

	return a.saveConfig(config)
}

func (a *ClineAdapter) ReadCommands() ([]*command.Command, error) {
	return nil, nil
}

func (a *ClineAdapter) WriteCommands(commands []*command.Command) error {
	return nil
}

func (a *ClineAdapter) ReadRules() ([]*rule.Rule, error) {
	// Cline uses custom instructions
	return nil, nil
}

func (a *ClineAdapter) WriteRules(rules []*rule.Rule) error {
	// Cline rules would go to custom instructions
	return nil
}

func (a *ClineAdapter) loadConfig() (*ClineConfig, error) {
	path := a.ConfigPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ClineConfig{}, nil
		}
		return nil, err
	}

	var config ClineConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (a *ClineAdapter) saveConfig(config *ClineConfig) error {
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
