package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/iheanyi/agentctl/pkg/agent"
	"github.com/iheanyi/agentctl/pkg/command"
	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/iheanyi/agentctl/pkg/rule"
	"github.com/iheanyi/agentctl/pkg/skill"
)

// OpenCodeAdapter syncs configuration to OpenCode (opencode.ai)
type OpenCodeAdapter struct{}

// OpenCodeServerConfig represents a server in OpenCode's config format
// OpenCode uses a different format than Claude Desktop:
// - "type": "local" or "remote"
// - "command": array of strings (command + args combined)
// - "enabled": boolean
// - "url": for remote servers
// Note: OpenCode's schema is strict - no custom fields allowed
type OpenCodeServerConfig struct {
	Type    string            `json:"type"`
	Command []string          `json:"command,omitempty"`
	URL     string            `json:"url,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Enabled bool              `json:"enabled"`
}

func init() {
	Register(&OpenCodeAdapter{})
}

func (a *OpenCodeAdapter) Name() string {
	return "opencode"
}

func (a *OpenCodeAdapter) Detect() (bool, error) {
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

func (a *OpenCodeAdapter) ConfigPath() string {
	return filepath.Join(a.configDir(), "opencode.json")
}

func (a *OpenCodeAdapter) configDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	switch runtime.GOOS {
	case "darwin", "linux":
		return filepath.Join(homeDir, ".config", "opencode")
	case "windows":
		return filepath.Join(homeDir, "AppData", "Roaming", "opencode")
	default:
		return ""
	}
}

func (a *OpenCodeAdapter) commandsDir() string {
	return filepath.Join(a.configDir(), "command")
}

func (a *OpenCodeAdapter) skillsDir() string {
	return filepath.Join(a.configDir(), "skill")
}

// agentsFilePath returns path to AGENTS.md (OpenCode prefers this over CLAUDE.md)
func (a *OpenCodeAdapter) agentsFilePath() string {
	return filepath.Join(a.configDir(), "AGENTS.md")
}

func (a *OpenCodeAdapter) agentsDir() string {
	return filepath.Join(a.configDir(), "agent")
}

func (a *OpenCodeAdapter) SupportedResources() []ResourceType {
	return []ResourceType{ResourceMCP, ResourceCommands, ResourceRules, ResourceSkills, ResourceAgents}
}

func (a *OpenCodeAdapter) ReadServers() ([]*mcp.Server, error) {
	helper := NewJSONConfigHelper(a.ConfigPath())
	raw, err := helper.LoadRaw()
	if err != nil {
		return nil, err
	}

	mcpSection, ok := raw["mcp"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	var servers []*mcp.Server
	for name, v := range mcpSection {
		serverData, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		// Skip disabled servers
		if enabled, ok := serverData["enabled"].(bool); ok && !enabled {
			continue
		}

		server := &mcp.Server{
			Name: name,
		}

		// Parse env
		if envData, ok := serverData["env"].(map[string]interface{}); ok {
			server.Env = make(map[string]string)
			for k, v := range envData {
				if str, ok := v.(string); ok {
					server.Env[k] = str
				}
			}
		}

		// Handle local servers (command is array)
		serverType, _ := serverData["type"].(string)
		if serverType == "local" {
			if cmdArray, ok := serverData["command"].([]interface{}); ok && len(cmdArray) > 0 {
				if cmd, ok := cmdArray[0].(string); ok {
					server.Command = cmd
				}
				for i := 1; i < len(cmdArray); i++ {
					if arg, ok := cmdArray[i].(string); ok {
						server.Args = append(server.Args, arg)
					}
				}
			}
		}

		// Handle remote servers (URL-based)
		if serverType == "remote" {
			if url, ok := serverData["url"].(string); ok {
				server.URL = url
				server.Transport = mcp.TransportHTTP
			}
		}

		servers = append(servers, server)
	}

	return servers, nil
}

func (a *OpenCodeAdapter) WriteServers(servers []*mcp.Server) error {
	helper := NewJSONConfigHelper(a.ConfigPath())
	raw, err := helper.LoadRaw()
	if err != nil {
		return err
	}

	// Load sync state to know which servers we previously managed
	state, err := LoadState()
	if err != nil {
		return err
	}

	// Get or create the mcp section
	mcpSection, ok := raw["mcp"].(map[string]interface{})
	if !ok {
		mcpSection = make(map[string]interface{})
	}

	// Remove previously managed servers (tracked in state file)
	for _, name := range state.GetManagedServers(a.Name()) {
		delete(mcpSection, name)
	}

	// Track new managed server names
	var managedNames []string

	// Add new servers (NO custom fields - OpenCode schema is strict)
	for _, server := range servers {
		name := server.Name
		if server.Namespace != "" {
			name = server.Namespace
		}

		// Skip servers with empty names to prevent corrupting config
		if name == "" {
			continue
		}

		serverCfg := map[string]interface{}{
			"enabled": true,
		}

		// Add env if present
		if len(server.Env) > 0 {
			serverCfg["env"] = server.Env
		}

		// Determine server type and set appropriate fields
		if server.URL != "" {
			// Remote server
			serverCfg["type"] = "remote"
			serverCfg["url"] = server.URL
		} else {
			// Local server - combine command and args into single array
			serverCfg["type"] = "local"
			cmd := append([]string{server.Command}, server.Args...)
			serverCfg["command"] = cmd
		}

		mcpSection[name] = serverCfg
		managedNames = append(managedNames, name)
	}

	// Update the mcp section in raw config
	raw["mcp"] = mcpSection

	// Save the config
	if err := helper.SaveRaw(raw); err != nil {
		return err
	}

	// Update state with new managed servers
	state.SetManagedServers(a.Name(), managedNames)
	return state.Save()
}

func (a *OpenCodeAdapter) ReadCommands() ([]*command.Command, error) {
	return ReadCommandsFromDir(a.commandsDir(), parseOpenCodeCommand)
}

func (a *OpenCodeAdapter) WriteCommands(commands []*command.Command) error {
	commandsDir := a.commandsDir()

	// Ensure directory exists
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		return err
	}

	for _, cmd := range commands {
		// Validate command name to prevent path traversal
		if err := SanitizeName(cmd.Name); err != nil {
			return fmt.Errorf("invalid command name: %w", err)
		}

		content := formatOpenCodeCommand(cmd)
		filename := cmd.Name + ".md"
		path := filepath.Join(commandsDir, filename)

		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}

func (a *OpenCodeAdapter) ReadRules() ([]*rule.Rule, error) {
	// OpenCode uses AGENTS.md (similar to CLAUDE.md)
	agentsPath := a.agentsFilePath()

	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		// Also check for CLAUDE.md as fallback
		claudePath := filepath.Join(a.configDir(), "CLAUDE.md")
		if _, err := os.Stat(claudePath); os.IsNotExist(err) {
			return nil, nil
		}
		agentsPath = claudePath
	}

	r, err := rule.Load(agentsPath)
	if err != nil {
		return nil, err
	}

	return []*rule.Rule{r}, nil
}

func (a *OpenCodeAdapter) WriteRules(rules []*rule.Rule) error {
	if len(rules) == 0 {
		return nil
	}

	// OpenCode prefers AGENTS.md
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

// ReadSkills reads skills from OpenCode's skill directory
func (a *OpenCodeAdapter) ReadSkills() ([]*skill.Skill, error) {
	return ReadSkillsFromDir(a.skillsDir())
}

// WriteSkills writes skills to OpenCode's skill directory
func (a *OpenCodeAdapter) WriteSkills(skills []*skill.Skill) error {
	return WriteSkillsToDir(a.skillsDir(), skills)
}

// parseOpenCodeCommand parses an OpenCode command markdown file
func parseOpenCodeCommand(filename string, content string) *command.Command {
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
				} else if strings.HasPrefix(line, "template:") {
					// OpenCode uses "template" for the prompt template
					cmd.Prompt = strings.TrimSpace(strings.TrimPrefix(line, "template:"))
				} else if strings.HasPrefix(line, "model:") {
					cmd.Model = strings.TrimSpace(strings.TrimPrefix(line, "model:"))
				}
			}
			// If no template in frontmatter, use body as prompt
			if cmd.Prompt == "" {
				cmd.Prompt = strings.TrimSpace(parts[2])
			}
		}
	} else {
		cmd.Prompt = content
	}

	return cmd
}

// formatOpenCodeCommand formats a command for OpenCode
func formatOpenCodeCommand(cmd *command.Command) string {
	var sb strings.Builder

	sb.WriteString("---\n")
	if cmd.Description != "" {
		sb.WriteString("description: ")
		sb.WriteString(cmd.Description)
		sb.WriteString("\n")
	}
	if cmd.Model != "" {
		sb.WriteString("model: ")
		sb.WriteString(cmd.Model)
		sb.WriteString("\n")
	}
	sb.WriteString("---\n\n")
	sb.WriteString(cmd.Prompt)

	return sb.String()
}

// AgentsAdapter implementation for OpenCode

// ReadAgents reads agents from OpenCode's agent directory
func (a *OpenCodeAdapter) ReadAgents() ([]*agent.Agent, error) {
	agentsDir := a.agentsDir()
	return agent.LoadAll(agentsDir)
}

// WriteAgents writes agents to OpenCode's agent directory
func (a *OpenCodeAdapter) WriteAgents(agents []*agent.Agent) error {
	return WriteAgentsToDir(a.agentsDir(), agents)
}
