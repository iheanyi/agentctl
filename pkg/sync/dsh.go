package sync

import (
	"os"
	"path/filepath"

	"github.com/iheanyi/agentctl/pkg/skill"
)

// DSHAdapter syncs configuration to DeepSeek Harness (dsh).
// Skills live in ~/.dsh/skills/. MCP server configuration format is not yet
// documented/verified, so this adapter supports skills only for now; extend
// with a ServerAdapter once the MCP config path is confirmed.
type DSHAdapter struct{}

func init() {
	Register(&DSHAdapter{})
}

func (a *DSHAdapter) Name() string {
	return "dsh"
}

func (a *DSHAdapter) baseDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".dsh")
}

func (a *DSHAdapter) Detect() (bool, error) {
	dir := a.baseDir()
	if dir == "" {
		return false, nil
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return false, nil
	}
	return true, nil
}

func (a *DSHAdapter) ConfigPath() string {
	return filepath.Join(a.baseDir(), "mcp.json")
}

func (a *DSHAdapter) SupportedResources() []ResourceType {
	return []ResourceType{ResourceSkills}
}

// skillsDir returns DeepSeek Harness's skills directory.
func (a *DSHAdapter) skillsDir() string {
	return filepath.Join(a.baseDir(), "skills")
}

// ReadSkills reads installed skills from DeepSeek Harness's skills directory.
func (a *DSHAdapter) ReadSkills() ([]*skill.Skill, error) {
	return ReadSkillsFromDir(a.skillsDir())
}

// WriteSkills writes skills to DeepSeek Harness's skills directory.
func (a *DSHAdapter) WriteSkills(skills []*skill.Skill) error {
	return WriteSkillsToDir(a.skillsDir(), skills)
}
