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
	raw, err := a.loadRawConfig()
	if err != nil {
		return nil, err
	}

	// Zed uses "context_servers" with snake_case
	contextServers, ok := raw["context_servers"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	var servers []*mcp.Server
	for name, v := range contextServers {
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

func (a *ZedAdapter) WriteServers(servers []*mcp.Server) error {
	// Load the full raw config to preserve all fields
	raw, err := a.loadRawConfig()
	if err != nil {
		return err
	}

	// Get or create the context_servers section (Zed uses snake_case)
	contextServers, ok := raw["context_servers"].(map[string]interface{})
	if !ok {
		contextServers = make(map[string]interface{})
	}

	// Remove old agentctl-managed entries
	for name, v := range contextServers {
		if serverData, ok := v.(map[string]interface{}); ok {
			if managedBy, ok := serverData["_managedBy"].(string); ok && managedBy == ManagedValue {
				delete(contextServers, name)
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

		contextServers[name] = serverCfg
	}

	// Update the context_servers section in raw config
	raw["context_servers"] = contextServers

	return a.saveRawConfig(raw)
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

// loadRawConfig loads the entire config as a raw map to preserve all fields
func (a *ZedAdapter) loadRawConfig() (map[string]interface{}, error) {
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
func (a *ZedAdapter) saveRawConfig(raw map[string]interface{}) error {
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
