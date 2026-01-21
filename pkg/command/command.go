package command

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/iheanyi/agentctl/pkg/jsonutil"
)

// InspectTitle returns the display name for the inspector modal header
func (c *Command) InspectTitle() string {
	return fmt.Sprintf("Command: %s", c.Name)
}

// InspectContent returns the formatted content for the inspector viewport
func (c *Command) InspectContent() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Tool:  %s\n", c.Tool))
	b.WriteString(fmt.Sprintf("Scope: %s\n", c.Scope))
	if c.Path != "" {
		b.WriteString(fmt.Sprintf("Path:  %s\n", c.Path))
	}
	b.WriteString("\n")

	if c.Description != "" {
		b.WriteString(fmt.Sprintf("Description: %s\n\n", c.Description))
	}

	if c.Model != "" {
		b.WriteString(fmt.Sprintf("Model: %s\n\n", c.Model))
	}

	if c.ArgumentHint != "" {
		b.WriteString(fmt.Sprintf("Argument Hint: %s\n\n", c.ArgumentHint))
	}

	if len(c.Args) > 0 {
		b.WriteString("Arguments:\n")
		for name, arg := range c.Args {
			required := ""
			if arg.Required {
				required = " (required)"
			}
			b.WriteString(fmt.Sprintf("  %s: %s%s\n", name, arg.Type, required))
			if arg.Description != "" {
				b.WriteString(fmt.Sprintf("    %s\n", arg.Description))
			}
		}
		b.WriteString("\n")
	}

	if len(c.AllowedTools) > 0 {
		b.WriteString(fmt.Sprintf("Allowed Tools: %s\n", strings.Join(c.AllowedTools, ", ")))
	}
	if len(c.DisallowedTools) > 0 {
		b.WriteString(fmt.Sprintf("Disallowed Tools: %s\n", strings.Join(c.DisallowedTools, ", ")))
	}

	b.WriteString("\nPrompt:\n")
	b.WriteString(c.Prompt)

	return b.String()
}

// Arg represents a command argument definition
type Arg struct {
	Type        string   `json:"type"`                  // "string", "number", "boolean"
	Enum        []string `json:"enum,omitempty"`        // Allowed values
	Default     any      `json:"default,omitempty"`     // Default value
	Description string   `json:"description,omitempty"` // Help text
	Required    bool     `json:"required,omitempty"`    // Is this arg required?
}

// ToolOverride represents tool-specific command overrides
type ToolOverride struct {
	AllowedTools    []string `json:"allowedTools,omitempty"`
	DisallowedTools []string `json:"disallowedTools,omitempty"`
	AlwaysApply     bool     `json:"alwaysApply,omitempty"`
}

// Command represents a slash command configuration
type Command struct {
	Name            string                  `json:"name"`
	Description     string                  `json:"description"`
	Prompt          string                  `json:"prompt"`
	ArgumentHint    string                  `json:"argumentHint,omitempty"`    // Hint for expected arguments (e.g., "[file.md or feature]")
	Model           string                  `json:"model,omitempty"`           // Preferred model (opus, sonnet, haiku, gpt-4, etc.)
	Args            map[string]Arg          `json:"args,omitempty"`            // Structured argument definitions
	AllowedTools    []string                `json:"allowedTools,omitempty"`    // Tools this command can use
	DisallowedTools []string                `json:"disallowedTools,omitempty"` // Tools this command cannot use
	Overrides       map[string]ToolOverride `json:"overrides,omitempty"`       // Per-tool overrides
	PromptRef       string                  `json:"promptRef,omitempty"`       // Reference to a prompt template

	// Runtime fields (not serialized)
	Scope string `json:"-"` // "local" or "global" - where this command came from
	Tool  string `json:"-"` // Which tool owns this (e.g., "claude", "gemini", "agentctl")
	Path  string `json:"-"` // Path to the source file
}

// Load loads a command from a JSON file
func Load(path string) (*Command, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cmd Command
	if err := json.Unmarshal(data, &cmd); err != nil {
		return nil, err
	}

	return &cmd, nil
}

// MarkdownFrontmatter represents the YAML frontmatter in a markdown command file
type MarkdownFrontmatter struct {
	Name         string `yaml:"name"`
	Description  string `yaml:"description"`
	ArgumentHint string `yaml:"argument-hint"` // Codex-style argument hint
}

// LoadMarkdown loads a command from a markdown file with YAML frontmatter
// This is the Claude Code command format
func LoadMarkdown(path string) (*Command, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	frontmatter, content, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}

	var fm MarkdownFrontmatter
	if len(frontmatter) > 0 {
		if err := yaml.Unmarshal(frontmatter, &fm); err != nil {
			return nil, err
		}
	}

	// Use filename (without extension) as fallback for name
	name := fm.Name
	if name == "" {
		base := filepath.Base(path)
		name = strings.TrimSuffix(base, filepath.Ext(base))
	}

	return &Command{
		Name:         name,
		Description:  fm.Description,
		ArgumentHint: fm.ArgumentHint,
		Prompt:       strings.TrimSpace(content),
		Path:         path,
	}, nil
}

// splitFrontmatter splits YAML frontmatter from markdown content
func splitFrontmatter(data []byte) (frontmatter []byte, content string, err error) {
	const delimiter = "---"

	text := string(data)
	text = strings.TrimLeft(text, "\n\r\t ")

	if !strings.HasPrefix(text, delimiter) {
		// No frontmatter, entire file is content
		return nil, text, nil
	}

	// Skip the opening delimiter
	text = text[len(delimiter):]

	// Find closing delimiter
	endIdx := strings.Index(text, "\n"+delimiter)
	if endIdx == -1 {
		// Unclosed frontmatter, treat as no frontmatter
		return nil, string(data), nil
	}

	frontmatter = []byte(strings.TrimSpace(text[:endIdx]))
	content = strings.TrimSpace(text[endIdx+len("\n"+delimiter):])

	return frontmatter, content, nil
}

// LoadAll loads all commands from a directory (recursively includes subdirectories)
// Supports both JSON format (.json) and markdown format (.md)
func LoadAll(dir string) ([]*Command, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var commands []*Command
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())

		if entry.IsDir() {
			// Recursively load from subdirectories
			subCmds, err := LoadAll(path)
			if err != nil {
				continue
			}
			commands = append(commands, subCmds...)
			continue
		}

		ext := filepath.Ext(entry.Name())
		if ext != ".json" && ext != ".md" {
			continue
		}

		var cmd *Command
		var loadErr error

		if ext == ".json" {
			cmd, loadErr = Load(path)
		} else {
			// .md uses markdown format with YAML frontmatter
			cmd, loadErr = LoadMarkdown(path)
		}

		if loadErr != nil {
			continue
		}
		cmd.Path = path
		commands = append(commands, cmd)
	}

	return commands, nil
}

// Exists checks if a command file exists
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Save saves a command to a directory
func Save(cmd *Command, dir string) error {
	// Ensure directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, cmd.Name+".json")
	data, err := jsonutil.MarshalIndent(cmd, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
