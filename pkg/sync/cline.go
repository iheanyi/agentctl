package sync

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/iheanyi/agentctl/pkg/mcp"
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

	// Cline uses cline_mcp_settings.json
	switch runtime.GOOS {
	case "darwin", "linux":
		return filepath.Join(homeDir, ".cline", "cline_mcp_settings.json")
	case "windows":
		return filepath.Join(homeDir, ".cline", "cline_mcp_settings.json")
	default:
		return ""
	}
}

func (a *ClineAdapter) SupportedResources() []ResourceType {
	return []ResourceType{ResourceMCP}
}

func (a *ClineAdapter) ReadServers() ([]*mcp.Server, error) {
	helper := NewJSONConfigHelper(a.ConfigPath())
	raw, err := helper.LoadRaw()
	if err != nil {
		return nil, err
	}

	mcpServers, _ := GetMCPServersSection(raw, "mcpServers")
	return ServersFromMCPSection(mcpServers), nil
}

func (a *ClineAdapter) WriteServers(servers []*mcp.Server) error {
	helper := NewJSONConfigHelper(a.ConfigPath())
	raw, err := helper.LoadRaw()
	if err != nil {
		return err
	}

	mcpServers, _ := GetMCPServersSection(raw, "mcpServers")
	RemoveManagedServers(mcpServers)

	// Add new servers
	for _, server := range servers {
		name := GetServerName(server)
		if name == "" {
			continue
		}
		mcpServers[name] = ServerToRawMap(server)
	}

	raw["mcpServers"] = mcpServers
	return helper.SaveRaw(raw)
}
