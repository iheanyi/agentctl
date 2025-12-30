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

// ZedAdapter syncs configuration to Zed editor
type ZedAdapter struct{}

// ZedServerConfig represents a server in Zed's config format
type ZedServerConfig struct {
	Command   string            `json:"command"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	ManagedBy string            `json:"_managedBy,omitempty"`
}

// ZedConfig represents Zed's configuration
type ZedConfig struct {
	MCPServers map[string]ZedServerConfig `json:"mcp_servers,omitempty"`
}

func init() {
	Register(&ZedAdapter{})
}

func (a *ZedAdapter) Name() string {
	return "zed"
}

func (a *ZedAdapter) Detect() (bool, error) {
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

func (a *ZedAdapter) ConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(homeDir, ".config", "zed", "settings.json")
	case "linux":
		return filepath.Join(homeDir, ".config", "zed", "settings.json")
	case "windows":
		return filepath.Join(homeDir, "AppData", "Roaming", "Zed", "settings.json")
	default:
		return ""
	}
}

func (a *ZedAdapter) SupportedResources() []ResourceType {
	return []ResourceType{ResourceMCP, ResourceCommands}
}

func (a *ZedAdapter) ReadServers() ([]*mcp.Server, error) {
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

func (a *ZedAdapter) WriteServers(servers []*mcp.Server) error {
	config, err := a.loadConfig()
	if err != nil {
		return err
	}

	if config.MCPServers == nil {
		config.MCPServers = make(map[string]ZedServerConfig)
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

		config.MCPServers[name] = ZedServerConfig{
			Command:   server.Command,
			Args:      server.Args,
			Env:       server.Env,
			ManagedBy: ManagedValue,
		}
	}

	return a.saveConfig(config)
}

func (a *ZedAdapter) ReadCommands() ([]*command.Command, error) {
	return nil, nil
}

func (a *ZedAdapter) WriteCommands(commands []*command.Command) error {
	return nil
}

func (a *ZedAdapter) ReadRules() ([]*rule.Rule, error) {
	return nil, nil
}

func (a *ZedAdapter) WriteRules(rules []*rule.Rule) error {
	return nil
}

func (a *ZedAdapter) loadConfig() (*ZedConfig, error) {
	path := a.ConfigPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ZedConfig{}, nil
		}
		return nil, err
	}

	var config ZedConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (a *ZedAdapter) saveConfig(config *ZedConfig) error {
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
