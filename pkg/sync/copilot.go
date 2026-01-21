package sync

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/iheanyi/agentctl/pkg/command"
	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/iheanyi/agentctl/pkg/rule"
	"github.com/iheanyi/agentctl/pkg/skill"
)

// CopilotAdapter syncs configuration to GitHub Copilot CLI
// Copilot CLI uses ~/.config/github-copilot/ on Unix systems
type CopilotAdapter struct{}

// CopilotServerConfig represents a server in Copilot's config format
type CopilotServerConfig struct {
	Command   string            `json:"command"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	ManagedBy string            `json:"_managedBy,omitempty"`
}

// CopilotConfig represents Copilot CLI's configuration structure
type CopilotConfig struct {
	MCPServers map[string]CopilotServerConfig `json:"mcpServers,omitempty"`
	// Preserve other fields
	Other map[string]interface{} `json:"-"`
}

func init() {
	Register(&CopilotAdapter{})
}

func (a *CopilotAdapter) Name() string {
	return "copilot"
}

func (a *CopilotAdapter) Detect() (bool, error) {
	configDir := a.configDir()
	if configDir == "" {
		return false, nil
	}

	// Check if Copilot config directory exists
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		return false, nil
	}

	return true, nil
}

func (a *CopilotAdapter) ConfigPath() string {
	return filepath.Join(a.configDir(), "config.json")
}

func (a *CopilotAdapter) configDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	switch runtime.GOOS {
	case "darwin", "linux":
		// Check XDG_CONFIG_HOME first
		if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
			return filepath.Join(xdgConfig, "github-copilot")
		}
		return filepath.Join(homeDir, ".config", "github-copilot")
	case "windows":
		return filepath.Join(homeDir, "AppData", "Roaming", "github-copilot")
	default:
		return ""
	}
}

func (a *CopilotAdapter) commandsDir() string {
	return filepath.Join(a.configDir(), "commands")
}

func (a *CopilotAdapter) skillsDir() string {
	return filepath.Join(a.configDir(), "skills")
}

func (a *CopilotAdapter) agentsFilePath() string {
	return filepath.Join(a.configDir(), "AGENTS.md")
}

func (a *CopilotAdapter) SupportedResources() []ResourceType {
	return []ResourceType{ResourceMCP, ResourceCommands, ResourceRules, ResourceSkills}
}

func (a *CopilotAdapter) ReadServers() ([]*mcp.Server, error) {
	helper := NewJSONConfigHelper(a.ConfigPath())
	raw, err := helper.LoadRaw()
	if err != nil {
		return nil, err
	}

	mcpServers, _ := GetMCPServersSection(raw, "mcpServers")
	return ServersFromMCPSection(mcpServers), nil
}

func (a *CopilotAdapter) WriteServers(servers []*mcp.Server) error {
	helper := NewJSONConfigHelper(a.ConfigPath())
	raw, err := helper.LoadRaw()
	if err != nil {
		return err
	}

	mcpServers, _ := GetMCPServersSection(raw, "mcpServers")
	RemoveManagedServers(mcpServers)

	// Add new servers (only stdio transport)
	for _, server := range FilterStdioServers(servers) {
		name := GetServerName(server)
		if name == "" {
			continue
		}
		mcpServers[name] = ServerToRawMap(server)
	}

	raw["mcpServers"] = mcpServers
	return helper.SaveRaw(raw)
}

func (a *CopilotAdapter) ReadCommands() ([]*command.Command, error) {
	return ReadCommandsFromDir(a.commandsDir(), parseCopilotCommand)
}

func (a *CopilotAdapter) WriteCommands(commands []*command.Command) error {
	return WriteCommandsToDir(a.commandsDir(), commands, formatCopilotCommand)
}

func (a *CopilotAdapter) ReadRules() ([]*rule.Rule, error) {
	// Copilot uses AGENTS.md for instructions
	agentsPath := a.agentsFilePath()

	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		return nil, nil
	}

	r, err := rule.Load(agentsPath)
	if err != nil {
		return nil, err
	}

	return []*rule.Rule{r}, nil
}

func (a *CopilotAdapter) WriteRules(rules []*rule.Rule) error {
	if len(rules) == 0 {
		return nil
	}

	agentsPath := a.agentsFilePath()

	// Concatenate all rules
	var content strings.Builder
	for i, r := range rules {
		if i > 0 {
			content.WriteString("\n\n---\n\n")
		}
		content.WriteString(r.Content)
	}

	// Ensure config directory exists
	if err := os.MkdirAll(a.configDir(), 0755); err != nil {
		return err
	}

	return os.WriteFile(agentsPath, []byte(content.String()), 0644)
}

// ReadSkills reads skills from Copilot's skills directory
func (a *CopilotAdapter) ReadSkills() ([]*skill.Skill, error) {
	return ReadSkillsFromDir(a.skillsDir())
}

// WriteSkills writes skills to Copilot's skills directory
func (a *CopilotAdapter) WriteSkills(skills []*skill.Skill) error {
	return WriteSkillsToDir(a.skillsDir(), skills)
}

// parseCopilotCommand parses a Copilot command markdown file
func parseCopilotCommand(filename string, content string) *command.Command {
	name := strings.TrimSuffix(filename, ".md")
	cmd := &command.Command{Name: name}

	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			frontmatter := parts[1]
			for _, line := range strings.Split(frontmatter, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "description:") {
					cmd.Description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
				} else if strings.HasPrefix(line, "argument-hint:") {
					cmd.ArgumentHint = strings.TrimSpace(strings.TrimPrefix(line, "argument-hint:"))
				}
			}
			cmd.Prompt = strings.TrimSpace(parts[2])
		}
	} else {
		cmd.Prompt = content
	}

	return cmd
}

// formatCopilotCommand formats a command for Copilot
func formatCopilotCommand(cmd *command.Command) string {
	var sb strings.Builder

	sb.WriteString("---\n")
	if cmd.Description != "" {
		sb.WriteString("description: ")
		sb.WriteString(cmd.Description)
		sb.WriteString("\n")
	}
	if cmd.ArgumentHint != "" {
		sb.WriteString("argument-hint: ")
		sb.WriteString(cmd.ArgumentHint)
		sb.WriteString("\n")
	}
	sb.WriteString("---\n\n")
	sb.WriteString(cmd.Prompt)

	return sb.String()
}

