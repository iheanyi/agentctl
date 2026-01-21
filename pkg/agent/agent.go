package agent

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Agent represents a custom agent/subagent for AI coding tools.
// Agents are configured via markdown files with YAML frontmatter.
// Supported tools: Claude Code, GitHub Copilot, Cursor, OpenCode
type Agent struct {
	// Core fields (all tools)
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	Content     string `yaml:"-" json:"content,omitempty"` // Markdown body (system prompt)
	Scope       string `yaml:"-" json:"scope"`             // "local" or "global"
	Tool        string `yaml:"-" json:"tool"`              // Source tool (claude, copilot, cursor, opencode)
	Path        string `yaml:"-" json:"path,omitempty"`    // File path

	// Model selection
	Model string `yaml:"model,omitempty" json:"model,omitempty"` // Model to use (inherit, sonnet, opus, haiku, fast, etc.)

	// Tool access control
	Tools           []string `yaml:"tools,omitempty" json:"tools,omitempty"`                     // Allowed tools
	DisallowedTools []string `yaml:"disallowedTools,omitempty" json:"disallowedTools,omitempty"` // Denied tools (Claude)

	// Claude-specific
	PermissionMode string   `yaml:"permissionMode,omitempty" json:"permissionMode,omitempty"` // default, acceptEdits, dontAsk, bypassPermissions, plan
	Skills         []string `yaml:"skills,omitempty" json:"skills,omitempty"`                 // Preloaded skills

	// Cursor-specific
	ReadOnly     bool `yaml:"readonly,omitempty" json:"readonly,omitempty"`         // Restrict write operations
	IsBackground bool `yaml:"is_background,omitempty" json:"is_background,omitempty"` // Run asynchronously

	// OpenCode-specific
	Temperature float64 `yaml:"temperature,omitempty" json:"temperature,omitempty"` // Response randomness (0.0-1.0)
	MaxSteps    int     `yaml:"maxSteps,omitempty" json:"maxSteps,omitempty"`       // Max iterations
	Mode        string  `yaml:"mode,omitempty" json:"mode,omitempty"`               // primary, subagent, all
	Hidden      bool    `yaml:"hidden,omitempty" json:"hidden,omitempty"`           // Exclude from autocomplete
	Disabled    bool    `yaml:"disable,omitempty" json:"disabled,omitempty"`        // Agent disabled

	// Copilot-specific
	Target  string   `yaml:"target,omitempty" json:"target,omitempty"` // vscode, github-copilot
	Infer   bool     `yaml:"infer,omitempty" json:"infer,omitempty"`   // Auto-selection based on context
	Metadata map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"` // Custom annotations
}

// InspectTitle returns the display name for the inspector modal header
func (a *Agent) InspectTitle() string {
	return fmt.Sprintf("Agent: %s", a.Name)
}

// InspectContent returns the formatted content for the inspector viewport
func (a *Agent) InspectContent() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Name:        %s\n", a.Name))
	b.WriteString(fmt.Sprintf("Tool:        %s\n", a.Tool))
	b.WriteString(fmt.Sprintf("Scope:       %s\n", a.Scope))
	if a.Path != "" {
		b.WriteString(fmt.Sprintf("Path:        %s\n", a.Path))
	}

	b.WriteString("\n")

	if a.Description != "" {
		b.WriteString(fmt.Sprintf("Description: %s\n", a.Description))
	}

	if a.Model != "" {
		b.WriteString(fmt.Sprintf("Model:       %s\n", a.Model))
	}

	if len(a.Tools) > 0 {
		b.WriteString(fmt.Sprintf("Tools:       %s\n", strings.Join(a.Tools, ", ")))
	}

	if len(a.DisallowedTools) > 0 {
		b.WriteString(fmt.Sprintf("Disallowed:  %s\n", strings.Join(a.DisallowedTools, ", ")))
	}

	if a.PermissionMode != "" {
		b.WriteString(fmt.Sprintf("Permission:  %s\n", a.PermissionMode))
	}

	if len(a.Skills) > 0 {
		b.WriteString(fmt.Sprintf("Skills:      %s\n", strings.Join(a.Skills, ", ")))
	}

	if a.ReadOnly {
		b.WriteString("Read-only:   true\n")
	}

	if a.IsBackground {
		b.WriteString("Background:  true\n")
	}

	if a.Content != "" {
		b.WriteString("\n--- Prompt ---\n")
		// Truncate if too long
		content := a.Content
		if len(content) > 2000 {
			content = content[:2000] + "\n... (truncated)"
		}
		b.WriteString(content)
	}

	return b.String()
}

// LoadFromFile loads an agent from a markdown file with YAML frontmatter
func LoadFromFile(path string) (*Agent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	agent, err := ParseAgentMarkdown(data)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	agent.Path = path

	// Derive name from filename if not specified
	if agent.Name == "" {
		base := filepath.Base(path)
		// Remove extension (.md or .agent.md)
		name := strings.TrimSuffix(base, ".agent.md")
		if name == base {
			name = strings.TrimSuffix(base, ".md")
		}
		agent.Name = name
	}

	return agent, nil
}

// ParseAgentMarkdown parses a markdown file with YAML frontmatter into an Agent
func ParseAgentMarkdown(data []byte) (*Agent, error) {
	agent := &Agent{}

	// Check for YAML frontmatter (starts with "---")
	if !bytes.HasPrefix(bytes.TrimSpace(data), []byte("---")) {
		// No frontmatter, treat entire content as prompt
		agent.Content = strings.TrimSpace(string(data))
		return agent, nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	var frontmatter strings.Builder
	var content strings.Builder
	inFrontmatter := false
	frontmatterEnded := false

	for scanner.Scan() {
		line := scanner.Text()

		if line == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			} else {
				frontmatterEnded = true
				inFrontmatter = false
				continue
			}
		}

		if inFrontmatter {
			frontmatter.WriteString(line)
			frontmatter.WriteString("\n")
		} else if frontmatterEnded {
			content.WriteString(line)
			content.WriteString("\n")
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning content: %w", err)
	}

	// Parse YAML frontmatter
	if frontmatter.Len() > 0 {
		if err := yaml.Unmarshal([]byte(frontmatter.String()), agent); err != nil {
			return nil, fmt.Errorf("parsing YAML frontmatter: %w", err)
		}
	}

	agent.Content = strings.TrimSpace(content.String())

	return agent, nil
}

// LoadFromDirectory loads all agents from a directory
func LoadFromDirectory(dir string, scope string, tool string) ([]*Agent, error) {
	var agents []*Agent

	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Accept .md and .agent.md files
		if !strings.HasSuffix(name, ".md") {
			continue
		}

		path := filepath.Join(dir, name)
		agent, err := LoadFromFile(path)
		if err != nil {
			// Skip files that can't be parsed
			continue
		}

		agent.Scope = scope
		agent.Tool = tool
		agents = append(agents, agent)
	}

	return agents, nil
}

// SaveToFile saves an agent to a markdown file with YAML frontmatter
func (a *Agent) SaveToFile(path string) error {
	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	var buf bytes.Buffer

	// Write YAML frontmatter
	buf.WriteString("---\n")

	// Only include non-empty fields
	type frontmatter struct {
		Name            string            `yaml:"name,omitempty"`
		Description     string            `yaml:"description,omitempty"`
		Model           string            `yaml:"model,omitempty"`
		Tools           []string          `yaml:"tools,omitempty"`
		DisallowedTools []string          `yaml:"disallowedTools,omitempty"`
		PermissionMode  string            `yaml:"permissionMode,omitempty"`
		Skills          []string          `yaml:"skills,omitempty"`
		ReadOnly        bool              `yaml:"readonly,omitempty"`
		IsBackground    bool              `yaml:"is_background,omitempty"`
		Temperature     float64           `yaml:"temperature,omitempty"`
		MaxSteps        int               `yaml:"maxSteps,omitempty"`
		Mode            string            `yaml:"mode,omitempty"`
		Hidden          bool              `yaml:"hidden,omitempty"`
		Disabled        bool              `yaml:"disable,omitempty"`
		Target          string            `yaml:"target,omitempty"`
		Infer           bool              `yaml:"infer,omitempty"`
		Metadata        map[string]string `yaml:"metadata,omitempty"`
	}

	fm := frontmatter{
		Name:            a.Name,
		Description:     a.Description,
		Model:           a.Model,
		Tools:           a.Tools,
		DisallowedTools: a.DisallowedTools,
		PermissionMode:  a.PermissionMode,
		Skills:          a.Skills,
		ReadOnly:        a.ReadOnly,
		IsBackground:    a.IsBackground,
		Temperature:     a.Temperature,
		MaxSteps:        a.MaxSteps,
		Mode:            a.Mode,
		Hidden:          a.Hidden,
		Disabled:        a.Disabled,
		Target:          a.Target,
		Infer:           a.Infer,
		Metadata:        a.Metadata,
	}

	yamlData, err := yaml.Marshal(fm)
	if err != nil {
		return fmt.Errorf("marshaling frontmatter: %w", err)
	}

	buf.Write(yamlData)
	buf.WriteString("---\n\n")

	// Write content
	if a.Content != "" {
		buf.WriteString(a.Content)
		if !strings.HasSuffix(a.Content, "\n") {
			buf.WriteString("\n")
		}
	}

	return os.WriteFile(path, buf.Bytes(), 0644)
}

// ToToolFormat converts the agent to a tool-specific format
// This can be used when syncing agents to different tools
func (a *Agent) ToToolFormat(targetTool string) *Agent {
	// Create a copy
	copy := *a
	copy.Tool = targetTool

	// Tool-specific transformations could go here
	// For now, we just copy as-is since the format is similar

	return &copy
}
