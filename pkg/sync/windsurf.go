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
	raw, err := a.loadRawConfig()
	if err != nil {
		return nil, err
	}

	mcpServers, ok := raw["mcpServers"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	var servers []*mcp.Server
	for name, v := range mcpServers {
		serverData, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		server := &mcp.Server{
			Name: name,
		}

		if cmd, ok := serverData["command"].(string); ok {
			server.Command = cmd
		}

		if args, ok := serverData["args"].([]interface{}); ok {
			for _, arg := range args {
				if str, ok := arg.(string); ok {
					server.Args = append(server.Args, str)
				}
			}
		}

		if envData, ok := serverData["env"].(map[string]interface{}); ok {
			server.Env = make(map[string]string)
			for k, v := range envData {
				if str, ok := v.(string); ok {
					server.Env[k] = str
				}
			}
		}

		servers = append(servers, server)
	}

	return servers, nil
}

func (a *WindsurfAdapter) WriteServers(servers []*mcp.Server) error {
	// Load the full raw config to preserve all fields
	raw, err := a.loadRawConfig()
	if err != nil {
		return err
	}

	// Get or create the mcpServers section
	mcpServers, ok := raw["mcpServers"].(map[string]interface{})
	if !ok {
		mcpServers = make(map[string]interface{})
	}

	// Remove old agentctl-managed entries
	for name, v := range mcpServers {
		if serverData, ok := v.(map[string]interface{}); ok {
			if managedBy, ok := serverData["_managedBy"].(string); ok && managedBy == ManagedValue {
				delete(mcpServers, name)
			}
		}
	}

	// Add new servers
	for _, server := range servers {
		name := server.Name
		if server.Namespace != "" {
			name = server.Namespace
		}

		serverCfg := map[string]interface{}{
			"command":    server.Command,
			"_managedBy": ManagedValue,
		}

		if len(server.Args) > 0 {
			serverCfg["args"] = server.Args
		}

		if len(server.Env) > 0 {
			serverCfg["env"] = server.Env
		}

		mcpServers[name] = serverCfg
	}

	// Update the mcpServers section in raw config
	raw["mcpServers"] = mcpServers

	return a.saveRawConfig(raw)
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

// loadRawConfig loads the entire config as a raw map to preserve all fields
func (a *WindsurfAdapter) loadRawConfig() (map[string]interface{}, error) {
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
func (a *WindsurfAdapter) saveRawConfig(raw map[string]interface{}) error {
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
