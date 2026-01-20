package sync

import (
	"fmt"
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
	helper := NewJSONConfigHelper(a.ConfigPath())
	raw, err := helper.LoadRaw()
	if err != nil {
		return nil, err
	}

	mcpServers, _ := GetMCPServersSection(raw, "mcpServers")
	return ServersFromMCPSection(mcpServers), nil
}

func (a *CursorAdapter) WriteServers(servers []*mcp.Server) error {
	helper := NewJSONConfigHelper(a.ConfigPath())
	raw, err := helper.LoadRaw()
	if err != nil {
		return err
	}

	mcpServers, _ := GetMCPServersSection(raw, "mcpServers")
	RemoveManagedServers(mcpServers)

	// Add new servers (only stdio transport - Cursor doesn't support HTTP/SSE)
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

func (a *CursorAdapter) ReadCommands() ([]*command.Command, error) {
	return ReadCommandsFromDir(a.commandsDir(), parseCursorCommand)
}

func (a *CursorAdapter) WriteCommands(commands []*command.Command) error {
	return WriteCommandsToDir(a.commandsDir(), commands, formatCursorCommand)
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

	// Validate rule name to prevent path traversal (without extension)
	baseName := strings.TrimSuffix(name, ".mdc")
	if err := SanitizeName(baseName); err != nil {
		return fmt.Errorf("invalid rule name: %w", err)
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
	helper := NewJSONConfigHelper(a.WorkspaceConfigPath(projectDir))
	raw, err := helper.LoadRaw()
	if err != nil {
		return nil, err
	}

	mcpServers, _ := GetMCPServersSection(raw, "mcpServers")
	servers := ServersFromMCPSection(mcpServers)

	// Mark as local scope
	for _, s := range servers {
		s.Scope = "local"
	}

	return servers, nil
}

// WriteWorkspaceServers writes MCP servers to the project's .cursor/mcp.json file
func (a *CursorAdapter) WriteWorkspaceServers(projectDir string, servers []*mcp.Server) error {
	helper := NewJSONConfigHelper(a.WorkspaceConfigPath(projectDir))
	raw, err := helper.LoadRaw()
	if err != nil {
		return err
	}

	mcpServers, _ := GetMCPServersSection(raw, "mcpServers")
	RemoveManagedServers(mcpServers)

	// Add new servers (only stdio - Cursor doesn't support HTTP/SSE)
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
