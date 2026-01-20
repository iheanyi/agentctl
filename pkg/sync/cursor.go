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
)

// CursorAdapter syncs configuration to Cursor
type CursorAdapter struct{}

// CursorServerConfig represents a server in Cursor's config format
type CursorServerConfig struct {
	Command   string            `json:"command"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	ManagedBy string            `json:"_managedBy,omitempty"`
}

func init() {
	Register(&CursorAdapter{})
}

func (a *CursorAdapter) Name() string {
	return "cursor"
}

func (a *CursorAdapter) Detect() (bool, error) {
	path := a.ConfigPath()
	if path == "" {
		return false, nil
	}

	// Check if Cursor directory exists
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return false, nil
	}

	return true, nil
}

func (a *CursorAdapter) ConfigPath() string {
	return filepath.Join(a.configDir(), "mcp.json")
}

func (a *CursorAdapter) configDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	switch runtime.GOOS {
	case "darwin", "linux", "windows":
		return filepath.Join(homeDir, ".cursor")
	default:
		return ""
	}
}

func (a *CursorAdapter) rulesDir() string {
	return filepath.Join(a.configDir(), "rules")
}

func (a *CursorAdapter) commandsDir() string {
	return filepath.Join(a.configDir(), "commands")
}

func (a *CursorAdapter) SupportedResources() []ResourceType {
	return []ResourceType{ResourceMCP, ResourceRules, ResourceCommands}
}

func (a *CursorAdapter) ReadServers() ([]*mcp.Server, error) {
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

func (a *CursorAdapter) WriteServers(servers []*mcp.Server) error {
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

	// Add new servers (only stdio transport - Cursor doesn't support HTTP/SSE)
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

func (a *CursorAdapter) ReadCommands() ([]*command.Command, error) {
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

		cmd := parseCursorCommand(entry.Name(), string(data))
		if cmd != nil {
			commands = append(commands, cmd)
		}
	}

	return commands, nil
}

func (a *CursorAdapter) WriteCommands(commands []*command.Command) error {
	commandsDir := a.commandsDir()

	// Ensure directory exists
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		return err
	}

	for _, cmd := range commands {
		content := formatCursorCommand(cmd)
		filename := cmd.Name + ".md"
		path := filepath.Join(commandsDir, filename)

		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}

func (a *CursorAdapter) ReadRules() ([]*rule.Rule, error) {
	var rules []*rule.Rule

	// First check modern .cursor/rules/ directory (takes priority)
	rulesDir := a.rulesDir()
	if entries, err := os.ReadDir(rulesDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			// Cursor uses .mdc files (markdown with frontmatter) or .md files
			if !strings.HasSuffix(entry.Name(), ".mdc") && !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}

			path := filepath.Join(rulesDir, entry.Name())
			r, err := loadCursorRule(path)
			if err != nil {
				continue
			}
			rules = append(rules, r)
		}
	}

	// Also check legacy .cursorrules file if no modern rules found
	if len(rules) == 0 {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return rules, nil
		}

		legacyPath := filepath.Join(homeDir, ".cursorrules")
		if _, err := os.Stat(legacyPath); err == nil {
			r, err := rule.Load(legacyPath)
			if err == nil {
				rules = append(rules, r)
			}
		}
	}

	return rules, nil
}

func (a *CursorAdapter) WriteRules(rules []*rule.Rule) error {
	if len(rules) == 0 {
		return nil
	}

	rulesDir := a.rulesDir()

	// Ensure directory exists
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		return err
	}

	// Write each rule as a .mdc file
	for _, r := range rules {
		if err := saveCursorRule(r, rulesDir); err != nil {
			return err
		}
	}

	return nil
}

// loadCursorRule loads a Cursor rule from .mdc or .md file
// Cursor uses "globs" and "alwaysApply" in frontmatter
func loadCursorRule(path string) (*rule.Rule, error) {
	r, err := rule.Load(path)
	if err != nil {
		return nil, err
	}

	// Cursor uses .mdc extension - normalize name
	r.Name = strings.TrimSuffix(r.Name, ".mdc")

	return r, nil
}

// saveCursorRule saves a rule in Cursor's .mdc format
func saveCursorRule(r *rule.Rule, dir string) error {
	var content strings.Builder

	// Write Cursor-style frontmatter if we have glob patterns
	hasGlobs := r.Frontmatter != nil && (len(r.Frontmatter.Globs) > 0 || len(r.Frontmatter.Paths) > 0)
	if hasGlobs || (r.Frontmatter != nil && r.Frontmatter.Priority != 0) {
		content.WriteString("---\n")
		if r.Frontmatter != nil {
			// Use globs for Cursor, or convert paths to globs
			globs := r.Frontmatter.Globs
			if len(globs) == 0 && len(r.Frontmatter.Paths) > 0 {
				globs = r.Frontmatter.Paths
			}
			if len(globs) > 0 {
				content.WriteString("globs:\n")
				for _, g := range globs {
					content.WriteString("  - \"")
					content.WriteString(g)
					content.WriteString("\"\n")
				}
			}
			// Cursor uses alwaysApply: true/false
			content.WriteString("alwaysApply: false\n")
		}
		content.WriteString("---\n\n")
	}

	content.WriteString(r.Content)

	// Determine filename - use .mdc extension for Cursor
	name := r.Name
	if name == "" {
		name = "imported-rule"
	}
	if !strings.HasSuffix(name, ".mdc") {
		name += ".mdc"
	}

	path := filepath.Join(dir, name)
	return os.WriteFile(path, []byte(content.String()), 0644)
}

// parseCursorCommand parses a Cursor command markdown file
func parseCursorCommand(filename string, content string) *command.Command {
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
				}
			}
			cmd.Prompt = strings.TrimSpace(parts[2])
		}
	} else {
		cmd.Prompt = content
	}

	return cmd
}

// formatCursorCommand formats a command for Cursor
func formatCursorCommand(cmd *command.Command) string {
	var sb strings.Builder

	sb.WriteString("---\n")
	if cmd.Description != "" {
		sb.WriteString("description: ")
		sb.WriteString(cmd.Description)
		sb.WriteString("\n")
	}
	sb.WriteString("---\n\n")
	sb.WriteString(cmd.Prompt)

	return sb.String()
}

// loadRawConfig loads the entire config as a raw map to preserve all fields
func (a *CursorAdapter) loadRawConfig() (map[string]interface{}, error) {
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
func (a *CursorAdapter) saveRawConfig(raw map[string]interface{}) error {
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

// WorkspaceAdapter implementation for Cursor

// SupportsWorkspace returns true - Cursor supports .cursor/mcp.json in project root
func (a *CursorAdapter) SupportsWorkspace() bool {
	return true
}

// WorkspaceConfigPath returns the path to .cursor/mcp.json in the project directory
func (a *CursorAdapter) WorkspaceConfigPath(projectDir string) string {
	return filepath.Join(projectDir, ".cursor", "mcp.json")
}

// ReadWorkspaceServers reads MCP servers from the project's .cursor/mcp.json file
func (a *CursorAdapter) ReadWorkspaceServers(projectDir string) ([]*mcp.Server, error) {
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

		servers = append(servers, server)
	}

	return servers, nil
}

// WriteWorkspaceServers writes MCP servers to the project's .cursor/mcp.json file
func (a *CursorAdapter) WriteWorkspaceServers(projectDir string, servers []*mcp.Server) error {
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

	// Add new servers (only stdio - Cursor doesn't support HTTP/SSE)
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

	raw["mcpServers"] = mcpServers

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
