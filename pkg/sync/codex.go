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

// CodexAdapter syncs configuration to OpenAI Codex CLI
type CodexAdapter struct{}

// CodexServerConfig represents a server in Codex's config format
type CodexServerConfig struct {
	Command   string            `json:"command"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	ManagedBy string            `json:"_managedBy,omitempty"`
}

// CodexConfig represents Codex's configuration
type CodexConfig struct {
	MCPServers map[string]CodexServerConfig `json:"mcpServers,omitempty"`
}

func init() {
	Register(&CodexAdapter{})
}

func (a *CodexAdapter) Name() string {
	return "codex"
}

func (a *CodexAdapter) Detect() (bool, error) {
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

func (a *CodexAdapter) ConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	switch runtime.GOOS {
	case "darwin", "linux":
		return filepath.Join(homeDir, ".codex", "config.json")
	case "windows":
		return filepath.Join(homeDir, ".codex", "config.json")
	default:
		return ""
	}
}

func (a *CodexAdapter) SupportedResources() []ResourceType {
	return []ResourceType{ResourceMCP, ResourceCommands}
}

func (a *CodexAdapter) ReadServers() ([]*mcp.Server, error) {
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

func (a *CodexAdapter) WriteServers(servers []*mcp.Server) error {
	config, err := a.loadConfig()
	if err != nil {
		return err
	}

	if config.MCPServers == nil {
		config.MCPServers = make(map[string]CodexServerConfig)
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

		config.MCPServers[name] = CodexServerConfig{
			Command:   server.Command,
			Args:      server.Args,
			Env:       server.Env,
			ManagedBy: ManagedValue,
		}
	}

	return a.saveConfig(config)
}

func (a *CodexAdapter) ReadCommands() ([]*command.Command, error) {
	return nil, nil
}

func (a *CodexAdapter) WriteCommands(commands []*command.Command) error {
	return nil
}

func (a *CodexAdapter) ReadRules() ([]*rule.Rule, error) {
	return nil, nil
}

func (a *CodexAdapter) WriteRules(rules []*rule.Rule) error {
	return nil
}

func (a *CodexAdapter) loadConfig() (*CodexConfig, error) {
	path := a.ConfigPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &CodexConfig{}, nil
		}
		return nil, err
	}

	var config CodexConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (a *CodexAdapter) saveConfig(config *CodexConfig) error {
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
