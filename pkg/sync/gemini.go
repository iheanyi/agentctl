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

// GeminiAdapter syncs configuration to Google Gemini CLI
type GeminiAdapter struct{}

// GeminiServerConfig represents a server in Gemini's config format
type GeminiServerConfig struct {
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	URL       string            `json:"url,omitempty"`
	Transport string            `json:"transport,omitempty"`
	ManagedBy string            `json:"_managedBy,omitempty"`
}

// GeminiConfig represents Gemini's configuration
type GeminiConfig struct {
	MCPServers map[string]GeminiServerConfig `json:"mcpServers,omitempty"`
	// Preserve other fields
	Other map[string]interface{} `json:"-"`
}

func init() {
	Register(&GeminiAdapter{})
}

func (a *GeminiAdapter) Name() string {
	return "gemini"
}

func (a *GeminiAdapter) Detect() (bool, error) {
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

func (a *GeminiAdapter) ConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	switch runtime.GOOS {
	case "darwin", "linux":
		return filepath.Join(homeDir, ".gemini", "config.json")
	case "windows":
		return filepath.Join(homeDir, ".gemini", "config.json")
	default:
		return ""
	}
}

func (a *GeminiAdapter) SupportedResources() []ResourceType {
	return []ResourceType{ResourceMCP}
}

func (a *GeminiAdapter) ReadServers() ([]*mcp.Server, error) {
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
		
		if serverCfg.Transport == "http" || serverCfg.Transport == "sse" {
			server.Transport = mcp.Transport(serverCfg.Transport)
			server.URL = serverCfg.URL
		}

		servers = append(servers, server)
	}

	return servers, nil
}

func (a *GeminiAdapter) WriteServers(servers []*mcp.Server) error {
	config, err := a.loadConfig()
	if err != nil {
		return err
	}

	if config.MCPServers == nil {
		config.MCPServers = make(map[string]GeminiServerConfig)
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

		// Skip servers with empty names to prevent corrupting config
		if name == "" {
			continue
		}

		cfg := GeminiServerConfig{
			Env:       server.Env,
			ManagedBy: ManagedValue,
		}

		if server.Transport == mcp.TransportHTTP || server.Transport == mcp.TransportSSE {
			cfg.Transport = string(server.Transport)
			cfg.URL = server.URL
		} else {
			cfg.Command = server.Command
			cfg.Args = server.Args
		}

		config.MCPServers[name] = cfg
	}

	return a.saveConfig(config)
}

func (a *GeminiAdapter) ReadCommands() ([]*command.Command, error) {
	return nil, nil
}

func (a *GeminiAdapter) WriteCommands(commands []*command.Command) error {
	return nil
}

func (a *GeminiAdapter) ReadRules() ([]*rule.Rule, error) {
	return nil, nil
}

func (a *GeminiAdapter) WriteRules(rules []*rule.Rule) error {
	return nil
}

func (a *GeminiAdapter) loadConfig() (*GeminiConfig, error) {
	path := a.ConfigPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &GeminiConfig{}, nil
		}
		return nil, err
	}

	// First unmarshal to capture all fields
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	var config GeminiConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	
	config.Other = raw

	return &config, nil
}

func (a *GeminiAdapter) saveConfig(config *GeminiConfig) error {
	path := a.ConfigPath()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Merge back with original fields to preserve unknown keys
	output := config.Other
	if output == nil {
		output = make(map[string]interface{})
	}

	output["mcpServers"] = config.MCPServers

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
