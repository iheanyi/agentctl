package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/iheanyi/agentctl/pkg/command"
	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/iheanyi/agentctl/pkg/rule"
	"github.com/iheanyi/agentctl/pkg/skill"
)

// ClaudeAdapter syncs configuration to Claude Code CLI (~/.claude/)
// This is the default "claude" adapter as Claude Code is the primary developer tool
type ClaudeAdapter struct{}

// ClaudeCodeSettings represents Claude Code's settings.json structure
type ClaudeCodeSettings struct {
	MCPServers     map[string]ClaudeServerConfig `json:"mcpServers,omitempty"`
	EnabledPlugins map[string]bool               `json:"enabledPlugins,omitempty"`
	Hooks          map[string]interface{}        `json:"hooks,omitempty"`
	// Preserve other fields
	Other map[string]interface{} `json:"-"`
}

// ClaudeCodePlugin represents an installed plugin
type ClaudeCodePlugin struct {
	Scope        string `json:"scope"`
	InstallPath  string `json:"installPath"`
	Version      string `json:"version"`
	InstalledAt  string `json:"installedAt"`
	LastUpdated  string `json:"lastUpdated"`
	GitCommitSha string `json:"gitCommitSha,omitempty"`
	IsLocal      bool   `json:"isLocal"`
}

// ClaudeCodePluginsFile represents the installed_plugins.json structure
type ClaudeCodePluginsFile struct {
	Version int                           `json:"version"`
	Plugins map[string][]ClaudeCodePlugin `json:"plugins"`
}

func init() {
	Register(&ClaudeAdapter{})
}

func (a *ClaudeAdapter) Name() string {
	return "claude"
}

func (a *ClaudeAdapter) Detect() (bool, error) {
	path := a.configDir()
	if path == "" {
		return false, nil
	}

	// Check if ~/.claude/ exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, nil
	}

	return true, nil
}

func (a *ClaudeAdapter) ConfigPath() string {
	return filepath.Join(a.configDir(), "settings.json")
}

func (a *ClaudeAdapter) configDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".claude")
}

func (a *ClaudeAdapter) commandsDir() string {
	return filepath.Join(a.configDir(), "commands")
}

func (a *ClaudeAdapter) rulesDir() string {
	return filepath.Join(a.configDir(), "rules")
}

func (a *ClaudeAdapter) pluginsDir() string {
	return filepath.Join(a.configDir(), "plugins")
}

func (a *ClaudeAdapter) SupportedResources() []ResourceType {
	return []ResourceType{ResourceMCP, ResourceCommands, ResourceRules, ResourceSkills}
}

func (a *ClaudeAdapter) ReadServers() ([]*mcp.Server, error) {
	settings, err := a.loadSettings()
	if err != nil {
		return nil, err
	}

	var servers []*mcp.Server
	for name, serverCfg := range settings.MCPServers {
		server := &mcp.Server{
			Name:    name,
			Command: serverCfg.Command,
			Args:    serverCfg.Args,
			Env:     serverCfg.Env,
		}
		servers = append(servers, server)
	}

	return servers, nil
}

func (a *ClaudeAdapter) WriteServers(servers []*mcp.Server) error {
	settings, err := a.loadSettings()
	if err != nil {
		return err
	}

	if settings.MCPServers == nil {
		settings.MCPServers = make(map[string]ClaudeServerConfig)
	}

	// Remove old agentctl-managed entries
	for name, serverCfg := range settings.MCPServers {
		if serverCfg.ManagedBy == ManagedValue {
			delete(settings.MCPServers, name)
		}
	}

	// Add new servers
	for _, server := range servers {
		name := server.Name
		if server.Namespace != "" {
			name = server.Namespace
		}

		// Skip servers with empty names to prevent corrupting config
		if name == "" {
			continue
		}

		cfg := ClaudeServerConfig{
			Env:       server.Env,
			ManagedBy: ManagedValue,
		}

		// Handle different transport types
		if server.Transport == mcp.TransportHTTP || server.Transport == mcp.TransportSSE {
			cfg.Transport = string(server.Transport)
			cfg.URL = server.URL
		} else {
			cfg.Command = server.Command
			cfg.Args = server.Args
		}

		settings.MCPServers[name] = cfg
	}

	return a.saveSettings(settings)
}

func (a *ClaudeAdapter) ReadCommands() ([]*command.Command, error) {
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

		// Parse frontmatter and content
		cmd := parseClaudeCommand(entry.Name(), string(data))
		if cmd != nil {
			commands = append(commands, cmd)
		}
	}

	return commands, nil
}

func (a *ClaudeAdapter) WriteCommands(commands []*command.Command) error {
	commandsDir := a.commandsDir()

	// Ensure directory exists
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		return err
	}

	for _, cmd := range commands {
		content := formatClaudeCommand(cmd)
		filename := cmd.Name + ".md"
		path := filepath.Join(commandsDir, filename)

		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}

func (a *ClaudeAdapter) ReadRules() ([]*rule.Rule, error) {
	rulesDir := a.rulesDir()
	return rule.LoadAll(rulesDir)
}

func (a *ClaudeAdapter) WriteRules(rules []*rule.Rule) error {
	rulesDir := a.rulesDir()

	// Ensure directory exists
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		return err
	}

	for _, r := range rules {
		if err := rule.Save(r, rulesDir); err != nil {
			return err
		}
	}

	return nil
}

// ReadSkills reads installed plugins as skills
func (a *ClaudeAdapter) ReadSkills() ([]*skill.Skill, error) {
	pluginsFile := filepath.Join(a.pluginsDir(), "installed_plugins.json")

	data, err := os.ReadFile(pluginsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var plugins ClaudeCodePluginsFile
	if err := json.Unmarshal(data, &plugins); err != nil {
		return nil, err
	}

	var skills []*skill.Skill
	for name, versions := range plugins.Plugins {
		if len(versions) == 0 {
			continue
		}

		// Use the first (most recent) version
		plugin := versions[0]

		// Parse marketplace and skill name from "skill@marketplace" format
		parts := strings.Split(name, "@")
		skillName := parts[0]
		marketplace := ""
		if len(parts) > 1 {
			marketplace = parts[1]
		}

		skills = append(skills, &skill.Skill{
			Name:        skillName,
			Description: "Claude Code plugin from " + marketplace,
			Version:     plugin.Version,
			Path:        plugin.InstallPath,
		})
	}

	return skills, nil
}

// WriteSkills writes skills to Claude Code's skills directory
// Note: This writes to ~/.claude/skills/, not the plugins system
func (a *ClaudeAdapter) WriteSkills(skills []*skill.Skill) error {
	skillsDir := filepath.Join(a.configDir(), "skills")

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

func (a *ClaudeAdapter) loadSettings() (*ClaudeCodeSettings, error) {
	path := a.ConfigPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ClaudeCodeSettings{}, nil
		}
		return nil, err
	}

	// First unmarshal to capture all fields
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	var settings ClaudeCodeSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}

	settings.Other = raw

	return &settings, nil
}

func (a *ClaudeAdapter) saveSettings(settings *ClaudeCodeSettings) error {
	path := a.ConfigPath()

	// Merge back with original fields to preserve unknown keys
	output := settings.Other
	if output == nil {
		output = make(map[string]interface{})
	}

	// Set known fields
	output["mcpServers"] = settings.MCPServers
	if settings.EnabledPlugins != nil {
		output["enabledPlugins"] = settings.EnabledPlugins
	}
	if settings.Hooks != nil {
		output["hooks"] = settings.Hooks
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// parseClaudeCommand parses a Claude Code command markdown file
func parseClaudeCommand(filename string, content string) *command.Command {
	// Extract name from filename
	name := strings.TrimSuffix(filename, ".md")

	cmd := &command.Command{
		Name: name,
	}

	// Check for frontmatter
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			// Parse frontmatter
			frontmatter := parts[1]
			for _, line := range strings.Split(frontmatter, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "description:") {
					cmd.Description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
				} else if strings.HasPrefix(line, "argument-hint:") {
					cmd.ArgumentHint = strings.TrimSpace(strings.TrimPrefix(line, "argument-hint:"))
				} else if strings.HasPrefix(line, "model:") {
					cmd.Model = strings.TrimSpace(strings.TrimPrefix(line, "model:"))
				} else if strings.HasPrefix(line, "allowed-tools:") {
					tools := strings.TrimSpace(strings.TrimPrefix(line, "allowed-tools:"))
					cmd.AllowedTools = parseToolsList(tools)
				} else if strings.HasPrefix(line, "disallowed-tools:") {
					tools := strings.TrimSpace(strings.TrimPrefix(line, "disallowed-tools:"))
					cmd.DisallowedTools = parseToolsList(tools)
				}
			}
			// Rest is the prompt
			cmd.Prompt = strings.TrimSpace(parts[2])
		}
	} else {
		cmd.Prompt = content
	}

	return cmd
}

// parseToolsList parses a YAML list or comma-separated tools string
func parseToolsList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// Handle YAML array format [tool1, tool2]
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		s = strings.TrimPrefix(s, "[")
		s = strings.TrimSuffix(s, "]")
	}
	parts := strings.Split(s, ",")
	var tools []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			tools = append(tools, p)
		}
	}
	return tools
}

// formatClaudeCommand formats a command for Claude Code
func formatClaudeCommand(cmd *command.Command) string {
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
	if cmd.Model != "" {
		sb.WriteString("model: ")
		sb.WriteString(cmd.Model)
		sb.WriteString("\n")
	}
	if len(cmd.AllowedTools) > 0 {
		sb.WriteString("allowed-tools: [")
		sb.WriteString(strings.Join(cmd.AllowedTools, ", "))
		sb.WriteString("]\n")
	}
	if len(cmd.DisallowedTools) > 0 {
		sb.WriteString("disallowed-tools: [")
		sb.WriteString(strings.Join(cmd.DisallowedTools, ", "))
		sb.WriteString("]\n")
	}
	sb.WriteString("---\n\n")
	sb.WriteString(cmd.Prompt)

	return sb.String()
}

// WorkspaceAdapter implementation for Claude Code

// SupportsWorkspace returns true - Claude Code supports .mcp.json in project root
func (a *ClaudeAdapter) SupportsWorkspace() bool {
	return true
}

// WorkspaceConfigPath returns the path to .mcp.json in the project directory
func (a *ClaudeAdapter) WorkspaceConfigPath(projectDir string) string {
	return filepath.Join(projectDir, ".mcp.json")
}

// ReadWorkspaceServers reads MCP servers from the project's .mcp.json file
func (a *ClaudeAdapter) ReadWorkspaceServers(projectDir string) ([]*mcp.Server, error) {
	path := a.WorkspaceConfigPath(projectDir)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
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
			Name:  name,
			Scope: "local",
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
			for k, ev := range envData {
				if str, ok := ev.(string); ok {
					server.Env[k] = str
				}
			}
		}

		// Check for remote transport
		if transport, ok := serverData["transport"].(string); ok {
			server.Transport = mcp.Transport(transport)
		}
		if url, ok := serverData["url"].(string); ok {
			server.URL = url
		}

		servers = append(servers, server)
	}

	return servers, nil
}

// WriteWorkspaceServers writes MCP servers to the project's .mcp.json file
func (a *ClaudeAdapter) WriteWorkspaceServers(projectDir string, servers []*mcp.Server) error {
	path := a.WorkspaceConfigPath(projectDir)

	// Load existing config if present
	var raw map[string]interface{}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &raw); err != nil {
			raw = make(map[string]interface{})
		}
	} else {
		raw = make(map[string]interface{})
	}

	// Get or create mcpServers section
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

	// Add new servers
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
			"_managedBy": ManagedValue,
		}

		// Handle different transport types
		if server.Transport == mcp.TransportHTTP || server.Transport == mcp.TransportSSE {
			serverCfg["transport"] = string(server.Transport)
			serverCfg["url"] = server.URL
		} else {
			serverCfg["command"] = server.Command
			if len(server.Args) > 0 {
				serverCfg["args"] = server.Args
			}
		}

		if len(server.Env) > 0 {
			serverCfg["env"] = server.Env
		}

		mcpServers[name] = serverCfg
	}

	raw["mcpServers"] = mcpServers

	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
