package discovery

import (
	"os"
	"path/filepath"

	"github.com/iheanyi/agentctl/pkg/agent"
	"github.com/iheanyi/agentctl/pkg/command"
	"github.com/iheanyi/agentctl/pkg/hook"
	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/iheanyi/agentctl/pkg/rule"
	"github.com/iheanyi/agentctl/pkg/skill"
)

// ClaudeScanner discovers resources from Claude Code's .claude/ directory
type ClaudeScanner struct{}

func init() {
	Register(&ClaudeScanner{})
}

func (s *ClaudeScanner) Name() string {
	return "claude"
}

func (s *ClaudeScanner) Detect(dir string) bool {
	claudeDir := filepath.Join(dir, ".claude")
	if _, err := os.Stat(claudeDir); err == nil {
		return true
	}
	return false
}

func (s *ClaudeScanner) ScanRules(dir string) ([]*rule.Rule, error) {
	rulesDir := filepath.Join(dir, ".claude", "rules")
	if _, err := os.Stat(rulesDir); os.IsNotExist(err) {
		return nil, nil
	}

	rules, err := rule.LoadAll(rulesDir)
	if err != nil {
		return nil, err
	}

	// Mark all as local scope and set tool
	for _, r := range rules {
		r.Scope = "local"
		r.Tool = "claude"
	}

	return rules, nil
}

func (s *ClaudeScanner) ScanSkills(dir string) ([]*skill.Skill, error) {
	skillsDir := filepath.Join(dir, ".claude", "skills")
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		return nil, nil
	}

	skills, err := skill.LoadAll(skillsDir)
	if err != nil {
		return nil, err
	}

	// Mark all as local scope and set tool
	for _, sk := range skills {
		sk.Scope = "local"
		sk.Tool = "claude"
	}

	return skills, nil
}

func (s *ClaudeScanner) ScanHooks(dir string) ([]*hook.Hook, error) {
	return hook.LoadFromClaudeProjectSettings(dir)
}

func (s *ClaudeScanner) ScanCommands(dir string) ([]*command.Command, error) {
	commandsDir := filepath.Join(dir, ".claude", "commands")
	if _, err := os.Stat(commandsDir); os.IsNotExist(err) {
		return nil, nil
	}

	commands, err := command.LoadAll(commandsDir)
	if err != nil {
		return nil, err
	}

	// Mark all as local scope and set tool
	for _, c := range commands {
		c.Scope = "local"
		c.Tool = "claude"
	}

	return commands, nil
}

func (s *ClaudeScanner) ScanServers(dir string) ([]*mcp.Server, error) {
	// Claude Code doesn't store MCP servers in .claude/ directory
	// It uses ~/.claude/settings.json for global MCP config
	// and .mcp.json for project-local servers
	mcpPath := filepath.Join(dir, ".mcp.json")
	if _, err := os.Stat(mcpPath); os.IsNotExist(err) {
		return nil, nil
	}

	// TODO: Parse .mcp.json for local servers
	// This would require integrating with the sync package's claude adapter
	return nil, nil
}

// ScanGlobalRules discovers rules from Claude's global config directory
func (s *ClaudeScanner) ScanGlobalRules() ([]*rule.Rule, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	rulesDir := filepath.Join(homeDir, ".claude", "rules")
	if _, err := os.Stat(rulesDir); os.IsNotExist(err) {
		return nil, nil
	}

	rules, err := rule.LoadAll(rulesDir)
	if err != nil {
		return nil, err
	}

	// Mark all as global scope and set tool
	for _, r := range rules {
		r.Scope = "global"
		r.Tool = "claude"
	}

	return rules, nil
}

// ScanGlobalSkills discovers skills from Claude's global config directory
func (s *ClaudeScanner) ScanGlobalSkills() ([]*skill.Skill, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	skillsDir := filepath.Join(homeDir, ".claude", "skills")
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		return nil, nil
	}

	skills, err := skill.LoadAll(skillsDir)
	if err != nil {
		return nil, err
	}

	// Mark all as global scope and set tool
	for _, sk := range skills {
		sk.Scope = "global"
		sk.Tool = "claude"
	}

	return skills, nil
}

// ScanGlobalCommands discovers commands from Claude's global config directory
func (s *ClaudeScanner) ScanGlobalCommands() ([]*command.Command, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	commandsDir := filepath.Join(homeDir, ".claude", "commands")
	if _, err := os.Stat(commandsDir); os.IsNotExist(err) {
		return nil, nil
	}

	commands, err := command.LoadAll(commandsDir)
	if err != nil {
		return nil, err
	}

	// Mark all as global scope and set tool
	for _, c := range commands {
		c.Scope = "global"
		c.Tool = "claude"
	}

	return commands, nil
}

// ScanGlobalHooks discovers hooks from Claude's global settings
func (s *ClaudeScanner) ScanGlobalHooks() ([]*hook.Hook, error) {
	return hook.LoadFromClaudeSettings()
}

// ScanPlugins discovers plugins from the project's local plugin config
func (s *ClaudeScanner) ScanPlugins(dir string) ([]*Plugin, error) {
	return LoadClaudeProjectPlugins(dir)
}

// ScanGlobalPlugins discovers plugins from Claude's global plugin config
func (s *ClaudeScanner) ScanGlobalPlugins() ([]*Plugin, error) {
	return LoadClaudePlugins()
}

// ScanAgents discovers agents from the project's local .claude/agents/ directory
func (s *ClaudeScanner) ScanAgents(dir string) ([]*agent.Agent, error) {
	agentsDir := filepath.Join(dir, ".claude", "agents")
	if _, err := os.Stat(agentsDir); os.IsNotExist(err) {
		return nil, nil
	}

	agents, err := agent.LoadFromDirectory(agentsDir, "local", "claude")
	if err != nil {
		return nil, err
	}

	return agents, nil
}

// ScanGlobalAgents discovers agents from Claude's global ~/.claude/agents/ directory
func (s *ClaudeScanner) ScanGlobalAgents() ([]*agent.Agent, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	agentsDir := filepath.Join(homeDir, ".claude", "agents")
	if _, err := os.Stat(agentsDir); os.IsNotExist(err) {
		return nil, nil
	}

	agents, err := agent.LoadFromDirectory(agentsDir, "global", "claude")
	if err != nil {
		return nil, err
	}

	return agents, nil
}
