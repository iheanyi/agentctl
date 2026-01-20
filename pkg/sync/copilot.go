package sync

import (
	"encoding/json"
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

func (a *CopilotAdapter) WriteServers(servers []*mcp.Server) error {
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

	// Add new servers (only stdio transport)
	for _, server := range FilterStdioServers(servers) {
		name := server.Name
		if server.Namespace != "" {
			name = server.Namespace
		}

		// Skip servers with empty names to prevent corrupting config
		if name == "" {
			continue
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

func (a *CopilotAdapter) ReadCommands() ([]*command.Command, error) {
	commandsDir := a.commandsDir()

	entries, err := os.ReadDir(commandsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var commands []*command.Command
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		path := filepath.Join(commandsDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		cmd := parseCopilotCommand(entry.Name(), string(data))
		if cmd != nil {
			commands = append(commands, cmd)
		}
	}

	return commands, nil
}

func (a *CopilotAdapter) WriteCommands(commands []*command.Command) error {
	commandsDir := a.commandsDir()

	// Ensure directory exists
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		return err
	}

	for _, cmd := range commands {
		content := formatCopilotCommand(cmd)
		filename := cmd.Name + ".md"
		path := filepath.Join(commandsDir, filename)

		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
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
	skillsDir := a.skillsDir()

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var skills []*skill.Skill
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
		if _, err := os.Stat(skillPath); os.IsNotExist(err) {
			continue
		}

		s, err := skill.Load(filepath.Join(skillsDir, entry.Name()))
		if err != nil {
			continue
		}
		skills = append(skills, s)
	}

	return skills, nil
}

// WriteSkills writes skills to Copilot's skills directory
func (a *CopilotAdapter) WriteSkills(skills []*skill.Skill) error {
	skillsDir := a.skillsDir()

	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return err
	}

	for _, s := range skills {
		skillDir := filepath.Join(skillsDir, s.Name)
		if err := s.Save(skillDir); err != nil {
			return err
		}
	}

	return nil
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

// loadRawConfig loads the entire config as a raw map to preserve all fields
func (a *CopilotAdapter) loadRawConfig() (map[string]interface{}, error) {
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
func (a *CopilotAdapter) saveRawConfig(raw map[string]interface{}) error {
	path := a.ConfigPath()

	// Ensure directory exists
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
