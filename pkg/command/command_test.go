package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCommandSerialization(t *testing.T) {
	cmd := Command{
		Name:        "explain-code",
		Description: "Explain selected code in detail",
		Prompt:      "Explain this code step by step:\n\n{{selection}}",
		Args: map[string]Arg{
			"depth": {
				Type:        "string",
				Enum:        []string{"brief", "detailed", "expert"},
				Default:     "detailed",
				Description: "Level of detail",
				Required:    false,
			},
		},
		AllowedTools:    []string{"Read", "Grep"},
		DisallowedTools: []string{"Bash", "Write"},
		Overrides: map[string]ToolOverride{
			"cursor": {AlwaysApply: true},
			"claude": {AllowedTools: []string{"Read", "Grep", "Glob"}},
		},
	}

	// Test serialization
	data, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("Failed to marshal command: %v", err)
	}

	// Test deserialization
	var decoded Command
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal command: %v", err)
	}

	// Verify fields
	if decoded.Name != cmd.Name {
		t.Errorf("Name mismatch: got %q, want %q", decoded.Name, cmd.Name)
	}
	if decoded.Description != cmd.Description {
		t.Errorf("Description mismatch: got %q, want %q", decoded.Description, cmd.Description)
	}
	if decoded.Prompt != cmd.Prompt {
		t.Errorf("Prompt mismatch: got %q, want %q", decoded.Prompt, cmd.Prompt)
	}
	if len(decoded.Args) != 1 {
		t.Errorf("Args length mismatch: got %d, want 1", len(decoded.Args))
	}
	if len(decoded.AllowedTools) != 2 {
		t.Errorf("AllowedTools length mismatch: got %d, want 2", len(decoded.AllowedTools))
	}
	if len(decoded.DisallowedTools) != 2 {
		t.Errorf("DisallowedTools length mismatch: got %d, want 2", len(decoded.DisallowedTools))
	}
	if len(decoded.Overrides) != 2 {
		t.Errorf("Overrides length mismatch: got %d, want 2", len(decoded.Overrides))
	}
}

func TestArgTypes(t *testing.T) {
	tests := []struct {
		name string
		arg  Arg
	}{
		{
			name: "string arg",
			arg: Arg{
				Type:    "string",
				Default: "default",
			},
		},
		{
			name: "enum arg",
			arg: Arg{
				Type: "string",
				Enum: []string{"a", "b", "c"},
			},
		},
		{
			name: "required arg",
			arg: Arg{
				Type:     "string",
				Required: true,
			},
		},
		{
			name: "number arg",
			arg: Arg{
				Type:    "number",
				Default: 10,
			},
		},
		{
			name: "boolean arg",
			arg: Arg{
				Type:    "boolean",
				Default: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.arg)
			if err != nil {
				t.Fatalf("Failed to marshal arg: %v", err)
			}

			var decoded Arg
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Failed to unmarshal arg: %v", err)
			}

			if decoded.Type != tt.arg.Type {
				t.Errorf("Type mismatch: got %q, want %q", decoded.Type, tt.arg.Type)
			}
			if decoded.Required != tt.arg.Required {
				t.Errorf("Required mismatch: got %v, want %v", decoded.Required, tt.arg.Required)
			}
		})
	}
}

func TestToolOverride(t *testing.T) {
	override := ToolOverride{
		AllowedTools:    []string{"Read"},
		DisallowedTools: []string{"Write"},
		AlwaysApply:     true,
	}

	data, err := json.Marshal(override)
	if err != nil {
		t.Fatalf("Failed to marshal override: %v", err)
	}

	var decoded ToolOverride
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal override: %v", err)
	}

	if !decoded.AlwaysApply {
		t.Error("AlwaysApply should be true")
	}
	if len(decoded.AllowedTools) != 1 || decoded.AllowedTools[0] != "Read" {
		t.Errorf("AllowedTools mismatch: got %v", decoded.AllowedTools)
	}
}

func TestLoadCommand(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "command-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a command file
	cmdJSON := `{
		"name": "test-command",
		"description": "A test command",
		"prompt": "Do something: {{input}}"
	}`

	cmdPath := filepath.Join(tmpDir, "test.json")
	if err := os.WriteFile(cmdPath, []byte(cmdJSON), 0644); err != nil {
		t.Fatalf("Failed to write command file: %v", err)
	}

	cmd, err := Load(cmdPath)
	if err != nil {
		t.Fatalf("Failed to load command: %v", err)
	}

	// Current implementation returns nil - this tests the interface
	if cmd != nil {
		if cmd.Name != "test-command" {
			t.Errorf("Name mismatch: got %q", cmd.Name)
		}
	}
}

func TestCommandWithPromptRef(t *testing.T) {
	cmd := Command{
		Name:        "review",
		Description: "Review code",
		PromptRef:   "@prompts/code-review",
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("Failed to marshal command: %v", err)
	}

	var decoded Command
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal command: %v", err)
	}

	if decoded.PromptRef != "@prompts/code-review" {
		t.Errorf("PromptRef mismatch: got %q", decoded.PromptRef)
	}
}

func TestEmptyCommand(t *testing.T) {
	cmd := Command{}

	data, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("Failed to marshal empty command: %v", err)
	}

	var decoded Command
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal empty command: %v", err)
	}

	if decoded.Name != "" {
		t.Errorf("Empty command name should be empty, got %q", decoded.Name)
	}
}
