package sync

import (
	"os"
	"path/filepath"

	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/iheanyi/agentctl/pkg/skill"
)

// TraeCNAdapter syncs configuration to TRAE CN (国内版).
// Skills live in ~/.trae-cn/skills/. MCP servers use ~/.trae-cn/mcp.json
// (standard "mcpServers" JSON, mirroring TRAE international's ~/.trae/mcp.json).
// Experimental: the mcp.json file is not created by the IDE itself until the
// user configures an MCP server in the UI; writing it here is additive and
// does not overwrite any existing IDE state (the mcps/ directory is unrelated
// per-session cache data).
type TraeCNAdapter struct{}

func init() {
	Register(&TraeCNAdapter{})
}

func (a *TraeCNAdapter) Name() string {
	return "trae_cn"
}

func (a *TraeCNAdapter) baseDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".trae-cn")
}

func (a *TraeCNAdapter) Detect() (bool, error) {
	dir := a.baseDir()
	if dir == "" {
		return false, nil
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return false, nil
	}
	return true, nil
}

func (a *TraeCNAdapter) ConfigPath() string {
	return filepath.Join(a.baseDir(), "mcp.json")
}

func (a *TraeCNAdapter) SupportedResources() []ResourceType {
	return []ResourceType{ResourceMCP, ResourceSkills}
}

// ReadServers reads MCP server configurations from TRAE CN.
func (a *TraeCNAdapter) ReadServers() ([]*mcp.Server, error) {
	helper := NewJSONConfigHelper(a.ConfigPath())
	raw, err := helper.LoadRaw()
	if err != nil {
		return nil, err
	}

	mcpServers, _ := GetMCPServersSection(raw, "mcpServers")
	return ServersFromMCPSection(mcpServers), nil
}

// WriteServers writes MCP server configurations to TRAE CN.
func (a *TraeCNAdapter) WriteServers(servers []*mcp.Server) error {
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

// skillsDir returns TRAE CN's skills directory.
func (a *TraeCNAdapter) skillsDir() string {
	return filepath.Join(a.baseDir(), "skills")
}

// ReadSkills reads installed skills from TRAE CN's skills directory.
func (a *TraeCNAdapter) ReadSkills() ([]*skill.Skill, error) {
	return ReadSkillsFromDir(a.skillsDir())
}

// WriteSkills writes skills to TRAE CN's skills directory.
func (a *TraeCNAdapter) WriteSkills(skills []*skill.Skill) error {
	return WriteSkillsToDir(a.skillsDir(), skills)
}
