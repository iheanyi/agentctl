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

// ContinueAdapter syncs configuration to Continue.dev
type ContinueAdapter struct{}

// ContinueServerConfig represents a server in Continue's config format
type ContinueServerConfig struct {
	Command   string            `json:"command"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	ManagedBy string            `json:"_managedBy,omitempty"`
}

// ContinueConfig represents Continue's configuration
type ContinueConfig struct {
	MCPServers map[string]ContinueServerConfig `json:"mcpServers,omitempty"`
}

func init() {
	Register(&ContinueAdapter{})
}

func (a *ContinueAdapter) Name() string {
	return "continue"
}

func (a *ContinueAdapter) Detect() (bool, error) {
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

func (a *ContinueAdapter) ConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	switch runtime.GOOS {
	case "darwin", "linux":
		return filepath.Join(homeDir, ".continue", "config.json")
	case "windows":
		return filepath.Join(homeDir, ".continue", "config.json")
	default:
		return ""
	}
}

func (a *ContinueAdapter) SupportedResources() []ResourceType {
	return []ResourceType{ResourceMCP, ResourceRules}
}

func (a *ContinueAdapter) ReadServers() ([]*mcp.Server, error) {
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

func (a *ContinueAdapter) WriteServers(servers []*mcp.Server) error {
	config, err := a.loadConfig()
	if err != nil {
		return err
	}

	if config.MCPServers == nil {
		config.MCPServers = make(map[string]ContinueServerConfig)
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

		config.MCPServers[name] = ContinueServerConfig{
			Command:   server.Command,
			Args:      server.Args,
			Env:       server.Env,
			ManagedBy: ManagedValue,
		}
	}

	return a.saveConfig(config)
}

func (a *ContinueAdapter) ReadCommands() ([]*command.Command, error) {
	return nil, nil
}

func (a *ContinueAdapter) WriteCommands(commands []*command.Command) error {
	return nil
}

func (a *ContinueAdapter) ReadRules() ([]*rule.Rule, error) {
	// Continue uses .continuerules or INSTRUCTIONS.md
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	rulesPath := filepath.Join(homeDir, ".continue", "rules.md")
	if _, err := os.Stat(rulesPath); os.IsNotExist(err) {
		return nil, nil
	}

	r, err := rule.Load(rulesPath)
	if err != nil {
		return nil, err
	}

	return []*rule.Rule{r}, nil
}

func (a *ContinueAdapter) WriteRules(rules []*rule.Rule) error {
	if len(rules) == 0 {
		return nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	rulesPath := filepath.Join(homeDir, ".continue", "rules.md")

	dir := filepath.Dir(rulesPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	var content string
	for i, r := range rules {
		if i > 0 {
			content += "\n\n---\n\n"
		}
		content += r.Content
	}

	return os.WriteFile(rulesPath, []byte(content), 0644)
}

func (a *ContinueAdapter) loadConfig() (*ContinueConfig, error) {
	path := a.ConfigPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ContinueConfig{}, nil
		}
		return nil, err
	}

	var config ContinueConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (a *ContinueAdapter) saveConfig(config *ContinueConfig) error {
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
