package command

import (
	"encoding/json"
	"os"
	"path/filepath"
)

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
	Args            map[string]Arg          `json:"args,omitempty"`
	AllowedTools    []string                `json:"allowedTools,omitempty"`
	DisallowedTools []string                `json:"disallowedTools,omitempty"`
	Overrides       map[string]ToolOverride `json:"overrides,omitempty"`
	PromptRef       string                  `json:"promptRef,omitempty"` // Reference to a prompt template
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

// LoadAll loads all commands from a directory
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
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		cmd, err := Load(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
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
	data, err := json.MarshalIndent(cmd, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
