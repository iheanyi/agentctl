package sync

import (
	"os"
	"path/filepath"

	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/iheanyi/agentctl/pkg/skill"
)

// WorkBuddyAdapter syncs configuration to WorkBuddy.
// Verified on Windows: MCP config lives at ~/.workbuddy/.mcp.json using the
// standard "mcpServers" JSON shape; skills live in ~/.workbuddy/skills/.
type WorkBuddyAdapter struct{}

// WorkBuddyServerConfig represents a server in WorkBuddy's config format.
type WorkBuddyServerConfig struct {
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	URL         string            `json:"url,omitempty"`
	Type        string            `json:"type,omitempty"`
	ManagedBy   string            `json:"_managedBy,omitempty"`
	Description string            `json:"description,omitempty"`
}

func init() {
	Register(&WorkBuddyAdapter{})
}

func (a *WorkBuddyAdapter) Name() string {
	return "workbuddy"
}

func (a *WorkBuddyAdapter) baseDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".workbuddy")
}

func (a *WorkBuddyAdapter) Detect() (bool, error) {
	dir := a.baseDir()
	if dir == "" {
		return false, nil
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return false, nil
	}
	return true, nil
}

func (a *WorkBuddyAdapter) ConfigPath() string {
	return filepath.Join(a.baseDir(), ".mcp.json")
}

func (a *WorkBuddyAdapter) SupportedResources() []ResourceType {
	return []ResourceType{ResourceMCP, ResourceSkills}
}

// ReadServers reads MCP server configurations from WorkBuddy.
func (a *WorkBuddyAdapter) ReadServers() ([]*mcp.Server, error) {
	helper := NewJSONConfigHelper(a.ConfigPath())
	raw, err := helper.LoadRaw()
	if err != nil {
		return nil, err
	}

	mcpServers, _ := GetMCPServersSection(raw, "mcpServers")
	return ServersFromMCPSection(mcpServers), nil
}

// WriteServers writes MCP server configurations to WorkBuddy.
func (a *WorkBuddyAdapter) WriteServers(servers []*mcp.Server) error {
	helper := NewJSONConfigHelper(a.ConfigPath())
	raw, err := helper.LoadRaw()
	if err != nil {
		return err
	}

	mcpServers, _ := GetMCPServersSection(raw, "mcpServers")
	RemoveManagedServers(mcpServers)

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

// skillsDir returns WorkBuddy's skills directory.
func (a *WorkBuddyAdapter) skillsDir() string {
	return filepath.Join(a.baseDir(), "skills")
}

// ReadSkills reads installed skills from WorkBuddy's skills directory.
func (a *WorkBuddyAdapter) ReadSkills() ([]*skill.Skill, error) {
	return ReadSkillsFromDir(a.skillsDir())
}

// WriteSkills writes skills to WorkBuddy's skills directory.
func (a *WorkBuddyAdapter) WriteSkills(skills []*skill.Skill) error {
	return WriteSkillsToDir(a.skillsDir(), skills)
}
