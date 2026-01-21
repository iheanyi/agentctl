package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseAgentMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantName string
		wantDesc string
		wantErr  bool
	}{
		{
			name: "basic agent",
			content: `---
name: code-reviewer
description: Reviews code for best practices
model: sonnet
---

You are an expert code reviewer.
`,
			wantName: "code-reviewer",
			wantDesc: "Reviews code for best practices",
		},
		{
			name: "no frontmatter",
			content: `You are a helpful assistant.`,
			wantName: "",
			wantDesc: "",
		},
		{
			name: "with tools",
			content: `---
name: read-only-agent
description: Can only read files
tools:
  - Read
  - Grep
  - Glob
readonly: true
---

You can only read files.
`,
			wantName: "read-only-agent",
			wantDesc: "Can only read files",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, err := ParseAgentMarkdown([]byte(tt.content))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseAgentMarkdown() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if agent.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", agent.Name, tt.wantName)
			}
			if agent.Description != tt.wantDesc {
				t.Errorf("Description = %q, want %q", agent.Description, tt.wantDesc)
			}
		})
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create a temp directory
	tmpDir := t.TempDir()

	// Create a test agent file
	content := `---
name: test-agent
description: A test agent
model: opus
tools:
  - Read
  - Write
---

You are a test agent.
`
	agentPath := filepath.Join(tmpDir, "test-agent.md")
	if err := os.WriteFile(agentPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	agent, err := LoadFromFile(agentPath)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}

	if agent.Name != "test-agent" {
		t.Errorf("Name = %q, want %q", agent.Name, "test-agent")
	}
	if agent.Description != "A test agent" {
		t.Errorf("Description = %q, want %q", agent.Description, "A test agent")
	}
	if agent.Model != "opus" {
		t.Errorf("Model = %q, want %q", agent.Model, "opus")
	}
	if len(agent.Tools) != 2 {
		t.Errorf("Tools length = %d, want 2", len(agent.Tools))
	}
	if agent.Path != agentPath {
		t.Errorf("Path = %q, want %q", agent.Path, agentPath)
	}
}

func TestLoadFromDirectory(t *testing.T) {
	// Create a temp directory
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, ".claude", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test agent files
	files := map[string]string{
		"code-reviewer.md": `---
name: code-reviewer
description: Reviews code
---

Review code for issues.
`,
		"security-auditor.md": `---
name: security-auditor
description: Security analysis
---

Audit for security issues.
`,
		"not-an-agent.txt": `This is not an agent file`,
	}

	for name, content := range files {
		path := filepath.Join(agentsDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	agents, err := LoadFromDirectory(agentsDir, "local", "claude")
	if err != nil {
		t.Fatalf("LoadFromDirectory() error = %v", err)
	}

	if len(agents) != 2 {
		t.Errorf("Got %d agents, want 2", len(agents))
	}

	// Check that all agents have correct scope and tool
	for _, agent := range agents {
		if agent.Scope != "local" {
			t.Errorf("Agent %s scope = %q, want %q", agent.Name, agent.Scope, "local")
		}
		if agent.Tool != "claude" {
			t.Errorf("Agent %s tool = %q, want %q", agent.Name, agent.Tool, "claude")
		}
	}
}

func TestSaveToFile(t *testing.T) {
	tmpDir := t.TempDir()
	agentPath := filepath.Join(tmpDir, "agents", "new-agent.md")

	agent := &Agent{
		Name:        "new-agent",
		Description: "A newly created agent",
		Model:       "sonnet",
		Tools:       []string{"Read", "Grep"},
		Content:     "You are a helpful agent.\n\nBe concise.",
	}

	if err := agent.SaveToFile(agentPath); err != nil {
		t.Fatalf("SaveToFile() error = %v", err)
	}

	// Read it back
	loaded, err := LoadFromFile(agentPath)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}

	if loaded.Name != agent.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, agent.Name)
	}
	if loaded.Description != agent.Description {
		t.Errorf("Description = %q, want %q", loaded.Description, agent.Description)
	}
	if loaded.Model != agent.Model {
		t.Errorf("Model = %q, want %q", loaded.Model, agent.Model)
	}
	if len(loaded.Tools) != len(agent.Tools) {
		t.Errorf("Tools length = %d, want %d", len(loaded.Tools), len(agent.Tools))
	}
}

func TestAgentInspectable(t *testing.T) {
	agent := &Agent{
		Name:        "test-agent",
		Description: "Test description",
		Tool:        "claude",
		Scope:       "global",
		Model:       "opus",
		Content:     "Test prompt content",
	}

	title := agent.InspectTitle()
	if title != "Agent: test-agent" {
		t.Errorf("InspectTitle() = %q, want %q", title, "Agent: test-agent")
	}

	content := agent.InspectContent()
	if content == "" {
		t.Error("InspectContent() returned empty string")
	}
	if !contains(content, "test-agent") {
		t.Error("InspectContent() missing name")
	}
	if !contains(content, "claude") {
		t.Error("InspectContent() missing tool")
	}
}

func TestDeriveNameFromFilename(t *testing.T) {
	tmpDir := t.TempDir()

	// Test .md extension
	path1 := filepath.Join(tmpDir, "my-agent.md")
	if err := os.WriteFile(path1, []byte("---\ndescription: test\n---\nprompt"), 0644); err != nil {
		t.Fatal(err)
	}
	agent1, err := LoadFromFile(path1)
	if err != nil {
		t.Fatal(err)
	}
	if agent1.Name != "my-agent" {
		t.Errorf("Name from .md = %q, want %q", agent1.Name, "my-agent")
	}

	// Test .agent.md extension (Copilot format)
	path2 := filepath.Join(tmpDir, "copilot-agent.agent.md")
	if err := os.WriteFile(path2, []byte("---\ndescription: test\n---\nprompt"), 0644); err != nil {
		t.Fatal(err)
	}
	agent2, err := LoadFromFile(path2)
	if err != nil {
		t.Fatal(err)
	}
	if agent2.Name != "copilot-agent" {
		t.Errorf("Name from .agent.md = %q, want %q", agent2.Name, "copilot-agent")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))
}
