package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/iheanyi/agentctl/pkg/command"
	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/iheanyi/agentctl/pkg/skill"
)

// JSONConfigHelper provides common JSON config file operations with field preservation.
// Use this for adapters that use JSON config files with mcpServers section.
type JSONConfigHelper struct {
	// ConfigPath is the path to the JSON config file
	ConfigPath string
}

// NewJSONConfigHelper creates a new helper for the given config path
func NewJSONConfigHelper(configPath string) *JSONConfigHelper {
	return &JSONConfigHelper{ConfigPath: configPath}
}

// LoadRaw loads the entire config as a raw map to preserve all fields.
// Returns an empty map if file doesn't exist.
func (h *JSONConfigHelper) LoadRaw() (map[string]interface{}, error) {
	data, err := os.ReadFile(h.ConfigPath)
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

// SaveRaw saves the entire config, preserving all fields.
// Creates the parent directory if it doesn't exist.
func (h *JSONConfigHelper) SaveRaw(raw map[string]interface{}) error {
	dir := filepath.Dir(h.ConfigPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(h.ConfigPath, data, 0644)
}

// GetMCPServersSection gets or creates the mcpServers section from a raw config.
// Returns the section and a boolean indicating if it was newly created.
func GetMCPServersSection(raw map[string]interface{}, key string) (map[string]interface{}, bool) {
	section, ok := raw[key].(map[string]interface{})
	if !ok {
		return make(map[string]interface{}), true
	}
	return section, false
}

// RemoveManagedServers removes all servers marked as managed by agentctl
func RemoveManagedServers(mcpServers map[string]interface{}) {
	for name, v := range mcpServers {
		if serverData, ok := v.(map[string]interface{}); ok {
			if managedBy, ok := serverData["_managedBy"].(string); ok && managedBy == ManagedValue {
				delete(mcpServers, name)
			}
		}
	}
}

// ServerFromRawMap parses a server from a raw map[string]interface{} representation.
// This handles the common mcpServers JSON format with command, args, env fields.
func ServerFromRawMap(name string, serverData map[string]interface{}) *mcp.Server {
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

	// Handle transport/url for remote servers
	if transport, ok := serverData["transport"].(string); ok {
		server.Transport = mcp.Transport(transport)
	}
	if url, ok := serverData["url"].(string); ok {
		server.URL = url
	}

	return server
}

// ServersFromMCPSection parses all servers from an mcpServers section
func ServersFromMCPSection(mcpServers map[string]interface{}) []*mcp.Server {
	var servers []*mcp.Server
	for name, v := range mcpServers {
		serverData, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		servers = append(servers, ServerFromRawMap(name, serverData))
	}
	return servers
}

// ServerToRawMap converts a server to a raw map for JSON serialization.
// Includes the _managedBy marker. Only includes stdio transport fields.
func ServerToRawMap(server *mcp.Server) map[string]interface{} {
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

	return serverCfg
}

// ServerToRawMapWithTransport converts a server to a raw map, including transport fields.
// Use for tools that support HTTP/SSE transport.
func ServerToRawMapWithTransport(server *mcp.Server) map[string]interface{} {
	serverCfg := map[string]interface{}{
		"_managedBy": ManagedValue,
	}

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

	return serverCfg
}

// GetServerName returns the effective name for a server (namespace if set, otherwise name)
func GetServerName(server *mcp.Server) string {
	if server.Namespace != "" {
		return server.Namespace
	}
	return server.Name
}

// CommandFrontmatterField represents a frontmatter field to parse
type CommandFrontmatterField struct {
	Key  string
	Dest *string
	List *[]string // For list fields like allowed-tools
}

// ParseCommandMarkdown parses a markdown command file with YAML frontmatter.
// Returns the parsed command with name derived from filename.
func ParseCommandMarkdown(filename, content string, fields []CommandFrontmatterField) *command.Command {
	name := strings.TrimSuffix(filename, ".md")
	cmd := &command.Command{Name: name}

	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			frontmatter := parts[1]
			for _, line := range strings.Split(frontmatter, "\n") {
				line = strings.TrimSpace(line)
				for _, field := range fields {
					prefix := field.Key + ":"
					if strings.HasPrefix(line, prefix) {
						value := strings.TrimSpace(strings.TrimPrefix(line, prefix))
						if field.Dest != nil {
							*field.Dest = value
						}
						if field.List != nil {
							*field.List = parseToolsList(value)
						}
					}
				}
			}
			cmd.Prompt = strings.TrimSpace(parts[2])
		}
	} else {
		cmd.Prompt = content
	}

	return cmd
}

// FormatCommandMarkdown formats a command as markdown with YAML frontmatter
func FormatCommandMarkdown(cmd *command.Command, fields []CommandFrontmatterField) string {
	var sb strings.Builder

	sb.WriteString("---\n")

	for _, field := range fields {
		if field.Dest != nil && *field.Dest != "" {
			sb.WriteString(field.Key)
			sb.WriteString(": ")
			sb.WriteString(*field.Dest)
			sb.WriteString("\n")
		}
		if field.List != nil && len(*field.List) > 0 {
			sb.WriteString(field.Key)
			sb.WriteString(": [")
			sb.WriteString(strings.Join(*field.List, ", "))
			sb.WriteString("]\n")
		}
	}

	sb.WriteString("---\n\n")
	sb.WriteString(cmd.Prompt)

	return sb.String()
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

// ReadCommandsFromDir reads all .md command files from a directory
func ReadCommandsFromDir(dir string, parseFunc func(filename, content string) *command.Command) ([]*command.Command, error) {
	entries, err := os.ReadDir(dir)
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

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		cmd := parseFunc(entry.Name(), string(data))
		if cmd != nil {
			commands = append(commands, cmd)
		}
	}

	return commands, nil
}

// WriteCommandsToDir writes commands as .md files to a directory
func WriteCommandsToDir(dir string, commands []*command.Command, formatFunc func(cmd *command.Command) string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	for _, cmd := range commands {
		// Validate command name to prevent path traversal
		if err := SanitizeName(cmd.Name); err != nil {
			return fmt.Errorf("invalid command name: %w", err)
		}

		content := formatFunc(cmd)
		filename := cmd.Name + ".md"
		path := filepath.Join(dir, filename)

		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}

// ReadSkillsFromDir reads all skills from a skills directory.
// Each skill should be in its own subdirectory with a SKILL.md file.
func ReadSkillsFromDir(skillsDir string) ([]*skill.Skill, error) {
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

// WriteSkillsToDir writes skills to a skills directory.
// Each skill is written to its own subdirectory.
func WriteSkillsToDir(skillsDir string, skills []*skill.Skill) error {
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return err
	}

	for _, s := range skills {
		// Validate skill name to prevent path traversal
		if err := SanitizeName(s.Name); err != nil {
			return fmt.Errorf("invalid skill name: %w", err)
		}

		skillDir := filepath.Join(skillsDir, s.Name)
		if err := s.Save(skillDir); err != nil {
			return err
		}
	}

	return nil
}
