package prompt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPromptSerialization(t *testing.T) {
	p := Prompt{
		Name:        "code-review",
		Description: "Standard code review prompt",
		Template:    "Review this code for:\n1. Bugs\n2. Performance\n3. Security\n\n{{code}}",
		Variables:   []string{"code"},
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Failed to marshal prompt: %v", err)
	}

	var decoded Prompt
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal prompt: %v", err)
	}

	if decoded.Name != p.Name {
		t.Errorf("Name mismatch: got %q, want %q", decoded.Name, p.Name)
	}
	if decoded.Description != p.Description {
		t.Errorf("Description mismatch: got %q, want %q", decoded.Description, p.Description)
	}
	if decoded.Template != p.Template {
		t.Errorf("Template mismatch: got %q, want %q", decoded.Template, p.Template)
	}
	if len(decoded.Variables) != 1 || decoded.Variables[0] != "code" {
		t.Errorf("Variables mismatch: got %v", decoded.Variables)
	}
}

func TestPromptRender(t *testing.T) {
	tests := []struct {
		name     string
		template string
		vars     map[string]string
		want     string
	}{
		{
			name:     "single variable",
			template: "Hello, {{name}}!",
			vars:     map[string]string{"name": "World"},
			want:     "Hello, World!",
		},
		{
			name:     "multiple variables",
			template: "{{greeting}}, {{name}}!",
			vars:     map[string]string{"greeting": "Hi", "name": "User"},
			want:     "Hi, User!",
		},
		{
			name:     "repeated variable",
			template: "{{x}} + {{x}} = 2{{x}}",
			vars:     map[string]string{"x": "a"},
			want:     "a + a = 2a",
		},
		{
			name:     "no variables",
			template: "Static text",
			vars:     map[string]string{},
			want:     "Static text",
		},
		{
			name:     "missing variable",
			template: "Hello, {{name}}!",
			vars:     map[string]string{},
			want:     "Hello, {{name}}!",
		},
		{
			name:     "multiline template",
			template: "Line 1: {{a}}\nLine 2: {{b}}",
			vars:     map[string]string{"a": "first", "b": "second"},
			want:     "Line 1: first\nLine 2: second",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Prompt{Template: tt.template}
			got := p.Render(tt.vars)
			if got != tt.want {
				t.Errorf("Render() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadPrompt(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prompt-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	promptJSON := `{
		"name": "test-prompt",
		"description": "A test prompt",
		"template": "Do something with {{input}}",
		"variables": ["input"]
	}`

	promptPath := filepath.Join(tmpDir, "test.json")
	if err := os.WriteFile(promptPath, []byte(promptJSON), 0644); err != nil {
		t.Fatalf("Failed to write prompt file: %v", err)
	}

	p, err := Load(promptPath)
	if err != nil {
		t.Fatalf("Failed to load prompt: %v", err)
	}

	if p.Name != "test-prompt" {
		t.Errorf("Name mismatch: got %q", p.Name)
	}
	if p.Description != "A test prompt" {
		t.Errorf("Description mismatch: got %q", p.Description)
	}
	if len(p.Variables) != 1 {
		t.Errorf("Variables length mismatch: got %d", len(p.Variables))
	}
}

func TestLoadPromptInvalidJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prompt-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	promptPath := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(promptPath, []byte("not json"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	_, err = Load(promptPath)
	if err == nil {
		t.Error("Load should return error for invalid JSON")
	}
}

func TestLoadPromptNonexistent(t *testing.T) {
	_, err := Load("/nonexistent/prompt.json")
	if err == nil {
		t.Error("Load should return error for nonexistent file")
	}
}

func TestLoadAllPrompts(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prompts-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create multiple prompt files
	prompts := map[string]string{
		"prompt1.json": `{"name": "p1", "template": "template 1"}`,
		"prompt2.json": `{"name": "p2", "template": "template 2"}`,
		"prompt3.json": `{"name": "p3", "template": "template 3"}`,
		"not-json.txt": "should be ignored",
	}

	for name, content := range prompts {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", name, err)
		}
	}

	loaded, err := LoadAll(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load all prompts: %v", err)
	}

	// Should only load .json files
	if len(loaded) != 3 {
		t.Errorf("Expected 3 prompts, got %d", len(loaded))
	}
}

func TestLoadAllPromptsNonexistentDir(t *testing.T) {
	prompts, err := LoadAll("/nonexistent/directory")
	if err != nil {
		t.Errorf("LoadAll should return nil error for nonexistent dir, got: %v", err)
	}
	if prompts != nil {
		t.Errorf("LoadAll should return nil for nonexistent dir, got: %v", prompts)
	}
}

func TestPromptWithEmptyVariables(t *testing.T) {
	p := Prompt{
		Name:     "simple",
		Template: "No variables here",
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded Prompt
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.Variables != nil && len(decoded.Variables) != 0 {
		t.Errorf("Expected nil or empty variables, got %v", decoded.Variables)
	}
}
