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

// CursorConfig represents Cursor's MCP configuration file structure
type CursorConfig struct {
	MCPServers map[string]CursorServerConfig `json:"mcpServers,omitempty"`
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

func (a *CursorAdapter) WriteServers(servers []*mcp.Server) error {
	config, err := a.loadConfig()
	if err != nil {
		return err
	}

	if config.MCPServers == nil {
		config.MCPServers = make(map[string]CursorServerConfig)
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

		config.MCPServers[name] = CursorServerConfig{
			Command:   server.Command,
			Args:      server.Args,
			Env:       server.Env,
			ManagedBy: ManagedValue,
		}
	}

	return a.saveConfig(config)
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

func (a *CursorAdapter) loadConfig() (*CursorConfig, error) {
	path := a.ConfigPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &CursorConfig{}, nil
		}
		return nil, err
	}

	var config CursorConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (a *CursorAdapter) saveConfig(config *CursorConfig) error {
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
