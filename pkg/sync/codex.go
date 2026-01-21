package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/iheanyi/agentctl/pkg/command"
	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/iheanyi/agentctl/pkg/rule"
	"github.com/iheanyi/agentctl/pkg/skill"
)

// CodexAdapter syncs configuration to OpenAI Codex CLI
// Codex uses TOML format (config.toml) as primary config
type CodexAdapter struct{}

// CodexTOMLConfig represents Codex's TOML configuration structure
type CodexTOMLConfig struct {
	MCPServers map[string]CodexTOMLServerConfig `toml:"mcp_servers"`
	// Preserve other sections
	Other map[string]interface{} `toml:"-"`
}

// CodexTOMLServerConfig represents a server in Codex's TOML format
type CodexTOMLServerConfig struct {
	Command     string            `toml:"command,omitempty"`
	Args        []string          `toml:"args,omitempty"`
	URL         string            `toml:"url,omitempty"`
	Env         map[string]string `toml:"env,omitempty"`
	Enabled     *bool             `toml:"enabled,omitempty"`
	Timeout     int               `toml:"startup_timeout_sec,omitempty"`
	ToolTimeout int               `toml:"tool_timeout_sec,omitempty"`
}

// CodexJSONServerConfig represents a server in Codex's legacy JSON format
type CodexJSONServerConfig struct {
	Command   string            `json:"command"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	ManagedBy string            `json:"_managedBy,omitempty"`
}

// CodexJSONConfig represents Codex's legacy JSON configuration
type CodexJSONConfig struct {
	MCPServers map[string]CodexJSONServerConfig `json:"mcpServers,omitempty"`
}

func init() {
	Register(&CodexAdapter{})
}

func (a *CodexAdapter) Name() string {
	return "codex"
}

func (a *CodexAdapter) Detect() (bool, error) {
	configDir := a.configDir()
	if configDir == "" {
		return false, nil
	}

	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		return false, nil
	}

	return true, nil
}

func (a *CodexAdapter) ConfigPath() string {
	// Prefer TOML, fall back to JSON
	tomlPath := a.tomlConfigPath()
	if _, err := os.Stat(tomlPath); err == nil {
		return tomlPath
	}
	return a.jsonConfigPath()
}

func (a *CodexAdapter) configDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	switch runtime.GOOS {
	case "darwin", "linux", "windows":
		return filepath.Join(homeDir, ".codex")
	default:
		return ""
	}
}

func (a *CodexAdapter) tomlConfigPath() string {
	return filepath.Join(a.configDir(), "config.toml")
}

func (a *CodexAdapter) jsonConfigPath() string {
	return filepath.Join(a.configDir(), "config.json")
}

func (a *CodexAdapter) promptsDir() string {
	return filepath.Join(a.configDir(), "prompts")
}

func (a *CodexAdapter) skillsDir() string {
	return filepath.Join(a.configDir(), "skills")
}

func (a *CodexAdapter) agentsFilePath() string {
	return filepath.Join(a.configDir(), "AGENTS.md")
}

func (a *CodexAdapter) SupportedResources() []ResourceType {
	return []ResourceType{ResourceMCP, ResourceCommands, ResourceRules, ResourceSkills}
}

func (a *CodexAdapter) ReadServers() ([]*mcp.Server, error) {
	// Try TOML first
	if servers, err := a.readServersFromTOML(); err == nil && len(servers) > 0 {
		return servers, nil
	}

	// Fall back to JSON
	return a.readServersFromJSON()
}

func (a *CodexAdapter) readServersFromTOML() ([]*mcp.Server, error) {
	path := a.tomlConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw map[string]interface{}
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	mcpServers, ok := raw["mcp_servers"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	var servers []*mcp.Server
	for name, v := range mcpServers {
		serverData, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		// Check if enabled (defaults to true)
		if enabled, ok := serverData["enabled"].(bool); ok && !enabled {
			continue
		}

		server := &mcp.Server{Name: name}

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

		if url, ok := serverData["url"].(string); ok {
			server.URL = url
			server.Transport = mcp.TransportHTTP
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

func (a *CodexAdapter) readServersFromJSON() ([]*mcp.Server, error) {
	path := a.jsonConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var config CodexJSONConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	var servers []*mcp.Server
	for name, serverCfg := range config.MCPServers {
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

func (a *CodexAdapter) WriteServers(servers []*mcp.Server) error {
	// Check if TOML config exists - if so, write to TOML
	tomlPath := a.tomlConfigPath()
	if _, err := os.Stat(tomlPath); err == nil {
		return a.writeServersToTOML(servers)
	}

	// Otherwise write to JSON (legacy)
	return a.writeServersToJSON(servers)
}

func (a *CodexAdapter) writeServersToTOML(servers []*mcp.Server) error {
	path := a.tomlConfigPath()

	// Load existing TOML config
	var raw map[string]interface{}
	if data, err := os.ReadFile(path); err == nil {
		if err := toml.Unmarshal(data, &raw); err != nil {
			raw = make(map[string]interface{})
		}
	} else {
		raw = make(map[string]interface{})
	}

	// Load sync state to track managed servers
	state, err := LoadState()
	if err != nil {
		return err
	}

	// Get or create mcp_servers section
	mcpServers, ok := raw["mcp_servers"].(map[string]interface{})
	if !ok {
		mcpServers = make(map[string]interface{})
	}

	// Remove previously managed servers
	for _, name := range state.GetManagedServers(a.Name()) {
		delete(mcpServers, name)
	}

	var managedNames []string

	// Add new servers
	for _, server := range servers {
		name := server.Name
		if server.Namespace != "" {
			name = server.Namespace
		}
		if name == "" {
			continue
		}

		serverCfg := map[string]interface{}{
			"enabled": true,
		}

		if server.URL != "" {
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
		managedNames = append(managedNames, name)
	}

	raw["mcp_servers"] = mcpServers

	// Write TOML
	data, err := toml.Marshal(raw)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}

	// Update state
	state.SetManagedServers(a.Name(), managedNames)
	return state.Save()
}

func (a *CodexAdapter) writeServersToJSON(servers []*mcp.Server) error {
	path := a.jsonConfigPath()

	// Load existing config
	var config CodexJSONConfig
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &config) // Ignore error, start with empty config if invalid
	}

	if config.MCPServers == nil {
		config.MCPServers = make(map[string]CodexJSONServerConfig)
	}

	// Remove managed entries
	for name, serverCfg := range config.MCPServers {
		if serverCfg.ManagedBy == ManagedValue {
			delete(config.MCPServers, name)
		}
	}

	// Add new servers
	for _, server := range servers {
		name := server.Name
		if server.Namespace != "" {
			name = server.Namespace
		}
		if name == "" {
			continue
		}

		config.MCPServers[name] = CodexJSONServerConfig{
			Command:   server.Command,
			Args:      server.Args,
			Env:       server.Env,
			ManagedBy: ManagedValue,
		}
	}

	// Ensure directory exists
	if err := os.MkdirAll(a.configDir(), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (a *CodexAdapter) ReadCommands() ([]*command.Command, error) {
	// Codex uses ~/.codex/prompts/ for custom prompts (similar to commands)
	return ReadCommandsFromDir(a.promptsDir(), parseCodexPrompt)
}

func (a *CodexAdapter) WriteCommands(commands []*command.Command) error {
	promptsDir := a.promptsDir()

	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		return err
	}

	for _, cmd := range commands {
		// Validate command name to prevent path traversal
		if err := SanitizeName(cmd.Name); err != nil {
			return fmt.Errorf("invalid command name: %w", err)
		}

		content := formatCodexPrompt(cmd)
		filename := cmd.Name + ".md"
		path := filepath.Join(promptsDir, filename)

		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}

func (a *CodexAdapter) ReadRules() ([]*rule.Rule, error) {
	// Codex uses AGENTS.md for instructions
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

func (a *CodexAdapter) WriteRules(rules []*rule.Rule) error {
	if len(rules) == 0 {
		return nil
	}

	agentsPath := a.agentsFilePath()

	var content strings.Builder
	for i, r := range rules {
		if i > 0 {
			content.WriteString("\n\n---\n\n")
		}
		content.WriteString(r.Content)
	}

	if err := os.MkdirAll(a.configDir(), 0755); err != nil {
		return err
	}

	return os.WriteFile(agentsPath, []byte(content.String()), 0644)
}

// ReadSkills reads skills from Codex's skills directory
func (a *CodexAdapter) ReadSkills() ([]*skill.Skill, error) {
	return ReadSkillsFromDir(a.skillsDir())
}

// WriteSkills writes skills to Codex's skills directory
func (a *CodexAdapter) WriteSkills(skills []*skill.Skill) error {
	return WriteSkillsToDir(a.skillsDir(), skills)
}

// parseCodexPrompt parses a Codex prompt markdown file
// Codex uses description and argument-hint in frontmatter
func parseCodexPrompt(filename string, content string) *command.Command {
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

// formatCodexPrompt formats a command for Codex's prompts format
func formatCodexPrompt(cmd *command.Command) string {
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
