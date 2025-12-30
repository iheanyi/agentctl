package command

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
	// Implementation will be added
	return nil, nil
}

// LoadAll loads all commands from a directory
func LoadAll(dir string) ([]*Command, error) {
	// Implementation will be added
	return nil, nil
}
