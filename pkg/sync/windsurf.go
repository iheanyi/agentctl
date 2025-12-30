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

// WindsurfAdapter syncs configuration to Windsurf (Codeium)
type WindsurfAdapter struct{}

// WindsurfServerConfig represents a server in Windsurf's config format
type WindsurfServerConfig struct {
	Command   string            `json:"command"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	ManagedBy string            `json:"_managedBy,omitempty"`
}

// WindsurfConfig represents Windsurf's MCP configuration
type WindsurfConfig struct {
	MCPServers map[string]WindsurfServerConfig `json:"mcpServers,omitempty"`
}

func init() {
	Register(&WindsurfAdapter{})
}

func (a *WindsurfAdapter) Name() string {
	return "windsurf"
}

func (a *WindsurfAdapter) Detect() (bool, error) {
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

func (a *WindsurfAdapter) ConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	switch runtime.GOOS {
	case "darwin", "linux":
		return filepath.Join(homeDir, ".windsurf", "mcp.json")
	case "windows":
		return filepath.Join(homeDir, ".windsurf", "mcp.json")
	default:
		return ""
	}
}

func (a *WindsurfAdapter) SupportedResources() []ResourceType {
	return []ResourceType{ResourceMCP, ResourceRules}
}

func (a *WindsurfAdapter) ReadServers() ([]*mcp.Server, error) {
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

func (a *WindsurfAdapter) WriteServers(servers []*mcp.Server) error {
	config, err := a.loadConfig()
	if err != nil {
		return err
	}

	if config.MCPServers == nil {
		config.MCPServers = make(map[string]WindsurfServerConfig)
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

		config.MCPServers[name] = WindsurfServerConfig{
			Command:   server.Command,
			Args:      server.Args,
			Env:       server.Env,
			ManagedBy: ManagedValue,
		}
	}

	return a.saveConfig(config)
}

func (a *WindsurfAdapter) ReadCommands() ([]*command.Command, error) {
	return nil, nil
}

func (a *WindsurfAdapter) WriteCommands(commands []*command.Command) error {
	return nil
}

func (a *WindsurfAdapter) ReadRules() ([]*rule.Rule, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	rulesPath := filepath.Join(homeDir, ".windsurfrules")
	if _, err := os.Stat(rulesPath); os.IsNotExist(err) {
		return nil, nil
	}

	r, err := rule.Load(rulesPath)
	if err != nil {
		return nil, err
	}

	return []*rule.Rule{r}, nil
}

func (a *WindsurfAdapter) WriteRules(rules []*rule.Rule) error {
	if len(rules) == 0 {
		return nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	rulesPath := filepath.Join(homeDir, ".windsurfrules")

	var content string
	for i, r := range rules {
		if i > 0 {
			content += "\n\n---\n\n"
		}
		content += r.Content
	}

	return os.WriteFile(rulesPath, []byte(content), 0644)
}

func (a *WindsurfAdapter) loadConfig() (*WindsurfConfig, error) {
	path := a.ConfigPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &WindsurfConfig{}, nil
		}
		return nil, err
	}

	var config WindsurfConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (a *WindsurfAdapter) saveConfig(config *WindsurfConfig) error {
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
