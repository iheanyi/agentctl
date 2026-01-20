package sync

import (
	"os"
	"path/filepath"
	"runtime"

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
	helper := NewJSONConfigHelper(a.ConfigPath())
	raw, err := helper.LoadRaw()
	if err != nil {
		return nil, err
	}

	mcpServers, _ := GetMCPServersSection(raw, "mcpServers")
	return ServersFromMCPSection(mcpServers), nil
}

func (a *WindsurfAdapter) WriteServers(servers []*mcp.Server) error {
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
