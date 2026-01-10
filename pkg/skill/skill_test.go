package skill

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSkillSerialization(t *testing.T) {
	s := Skill{
		Name:        "my-skill",
		Description: "A test skill",
		Version:     "1.0.0",
		Author:      "test@example.com",
		Prompts: map[string]string{
			"main":   "Main prompt content",
			"helper": "Helper prompt content",
		},
		Files: []string{"config.yaml", "templates/"},
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Failed to marshal skill: %v", err)
	}

	var decoded Skill
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal skill: %v", err)
	}

	if decoded.Name != s.Name {
		t.Errorf("Name mismatch: got %q, want %q", decoded.Name, s.Name)
	}
	if decoded.Description != s.Description {
		t.Errorf("Description mismatch: got %q, want %q", decoded.Description, s.Description)
	}
	if decoded.Version != s.Version {
		t.Errorf("Version mismatch: got %q, want %q", decoded.Version, s.Version)
	}
	if decoded.Author != s.Author {
		t.Errorf("Author mismatch: got %q, want %q", decoded.Author, s.Author)
	}
	if len(decoded.Prompts) != 2 {
		t.Errorf("Prompts length mismatch: got %d, want 2", len(decoded.Prompts))
	}
	if len(decoded.Files) != 2 {
		t.Errorf("Files length mismatch: got %d, want 2", len(decoded.Files))
	}
}

func TestLoadSkill(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "skill-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create skill directory structure
	skillDir := filepath.Join(tmpDir, "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	// Create skill.json
	skillJSON := `{
		"name": "my-skill",
		"description": "A test skill",
		"version": "1.0.0"
	}`
	if err := os.WriteFile(filepath.Join(skillDir, "skill.json"), []byte(skillJSON), 0644); err != nil {
		t.Fatalf("Failed to write skill.json: %v", err)
	}

	s, err := Load(skillDir)
	if err != nil {
		t.Fatalf("Failed to load skill: %v", err)
	}

	if s.Name != "my-skill" {
		t.Errorf("Name mismatch: got %q", s.Name)
	}
	if s.Path != skillDir {
		t.Errorf("Path mismatch: got %q, want %q", s.Path, skillDir)
	}
}

func TestLoadSkillWithPrompts(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "skill-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create skill directory with prompts
	skillDir := filepath.Join(tmpDir, "skill-with-prompts")
	promptsDir := filepath.Join(skillDir, "prompts")
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		t.Fatalf("Failed to create dirs: %v", err)
	}

	// Create skill.json
	skillJSON := `{"name": "skill-with-prompts"}`
	if err := os.WriteFile(filepath.Join(skillDir, "skill.json"), []byte(skillJSON), 0644); err != nil {
		t.Fatalf("Failed to write skill.json: %v", err)
	}

	// Create prompt files
	if err := os.WriteFile(filepath.Join(promptsDir, "main.md"), []byte("Main prompt"), 0644); err != nil {
		t.Fatalf("Failed to write prompt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(promptsDir, "helper.txt"), []byte("Helper prompt"), 0644); err != nil {
		t.Fatalf("Failed to write prompt: %v", err)
	}

	s, err := Load(skillDir)
	if err != nil {
		t.Fatalf("Failed to load skill: %v", err)
	}

	// Should have loaded prompts from prompts/ directory
	if len(s.Prompts) != 2 {
		t.Errorf("Expected 2 prompts, got %d", len(s.Prompts))
	}
	if s.Prompts["main"] != "Main prompt" {
		t.Errorf("main prompt mismatch: got %q", s.Prompts["main"])
	}
	if s.Prompts["helper"] != "Helper prompt" {
		t.Errorf("helper prompt mismatch: got %q", s.Prompts["helper"])
	}
}

func TestLoadSkillMissingSkillJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "skill-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create empty skill directory
	skillDir := filepath.Join(tmpDir, "empty-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	_, err = Load(skillDir)
	if err == nil {
		t.Error("Load should return error for missing skill.json")
	}
}

func TestLoadSkillInvalidJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "skill-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	skillDir := filepath.Join(tmpDir, "invalid-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	// Write invalid JSON
	if err := os.WriteFile(filepath.Join(skillDir, "skill.json"), []byte("not json"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	_, err = Load(skillDir)
	if err == nil {
		t.Error("Load should return error for invalid JSON")
	}
}

func TestLoadAllSkills(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "skills-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create multiple skill directories
	for i := 1; i <= 3; i++ {
		skillDir := filepath.Join(tmpDir, "skill"+string(rune('0'+i)))
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatalf("Failed to create skill dir: %v", err)
		}
		skillJSON := `{"name": "skill` + string(rune('0'+i)) + `"}`
		if err := os.WriteFile(filepath.Join(skillDir, "skill.json"), []byte(skillJSON), 0644); err != nil {
			t.Fatalf("Failed to write skill.json: %v", err)
		}
	}

	// Also create a file (should be ignored)
	if err := os.WriteFile(filepath.Join(tmpDir, "not-a-skill.txt"), []byte("ignored"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	skills, err := LoadAll(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load all skills: %v", err)
	}

	if len(skills) != 3 {
		t.Errorf("Expected 3 skills, got %d", len(skills))
	}
}

func TestLoadAllSkillsNonexistentDir(t *testing.T) {
	skills, err := LoadAll("/nonexistent/directory")
	if err != nil {
		t.Errorf("LoadAll should return nil error for nonexistent dir, got: %v", err)
	}
	if skills != nil {
		t.Errorf("LoadAll should return nil for nonexistent dir, got: %v", skills)
	}
}

func TestLoadAllSkillsWithInvalidSkill(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "skills-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create one valid skill
	validDir := filepath.Join(tmpDir, "valid")
	if err := os.MkdirAll(validDir, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(validDir, "skill.json"), []byte(`{"name": "valid"}`), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Create one invalid skill (missing skill.json)
	invalidDir := filepath.Join(tmpDir, "invalid")
	if err := os.MkdirAll(invalidDir, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}

	skills, err := LoadAll(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load skills: %v", err)
	}

	// Should only load valid skill
	if len(skills) != 1 {
		t.Errorf("Expected 1 valid skill, got %d", len(skills))
	}
}

func TestSkillPath(t *testing.T) {
	s := Skill{
		Name: "test",
		Path: "/path/to/skill",
	}

	// Path should not be serialized
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	dataStr := string(data)
	if contains(dataStr, "/path/to/skill") {
		t.Error("Path should not be serialized")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Tests for SKILL.md format (Claude Code format)

func TestLoadSkillMd(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "skill-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	skillDir := filepath.Join(tmpDir, "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	// Create SKILL.md with frontmatter
	skillMd := `---
name: my-skill
description: A test skill for Claude Code
---

# My Skill

This is the prompt content for my skill.

Use $ARGUMENTS to reference user input.`

	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMd), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	s, err := Load(skillDir)
	if err != nil {
		t.Fatalf("Failed to load skill: %v", err)
	}

	if s.Name != "my-skill" {
		t.Errorf("Name mismatch: got %q, want %q", s.Name, "my-skill")
	}
	if s.Description != "A test skill for Claude Code" {
		t.Errorf("Description mismatch: got %q", s.Description)
	}
	if !contains(s.Content, "This is the prompt content") {
		t.Errorf("Content should contain prompt text, got: %q", s.Content)
	}
	if s.Path != skillDir {
		t.Errorf("Path mismatch: got %q, want %q", s.Path, skillDir)
	}
}

func TestSkillMdTakesPrecedenceOverJson(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "skill-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	skillDir := filepath.Join(tmpDir, "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	// Create both SKILL.md and skill.json
	skillMd := `---
name: from-md
description: From SKILL.md
---

Content from SKILL.md`

	skillJson := `{"name": "from-json", "description": "From skill.json"}`

	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMd), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "skill.json"), []byte(skillJson), 0644); err != nil {
		t.Fatalf("Failed to write skill.json: %v", err)
	}

	s, err := Load(skillDir)
	if err != nil {
		t.Fatalf("Failed to load skill: %v", err)
	}

	// SKILL.md should take precedence
	if s.Name != "from-md" {
		t.Errorf("SKILL.md should take precedence, got name %q", s.Name)
	}
	if s.Description != "From SKILL.md" {
		t.Errorf("Description mismatch: got %q", s.Description)
	}
}

func TestLoadSkillMdWithoutFrontmatter(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "skill-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	skillDir := filepath.Join(tmpDir, "plain-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	// Create SKILL.md without frontmatter
	skillMd := `# Plain Skill

Just content, no frontmatter.`

	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMd), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	s, err := Load(skillDir)
	if err != nil {
		t.Fatalf("Failed to load skill: %v", err)
	}

	// Name should fall back to directory name
	if s.Name != "plain-skill" {
		t.Errorf("Name should fall back to dir name, got %q", s.Name)
	}
	if !contains(s.Content, "Just content") {
		t.Errorf("Content mismatch: got %q", s.Content)
	}
}

func TestSplitFrontmatter(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantFM      string
		wantContent string
		wantErr     bool
	}{
		{
			name: "valid frontmatter",
			input: `---
name: test
description: A test
---

Content here`,
			wantFM:      "name: test\ndescription: A test",
			wantContent: "Content here",
			wantErr:     false,
		},
		{
			name:        "no frontmatter",
			input:       "Just plain content",
			wantFM:      "",
			wantContent: "Just plain content",
			wantErr:     false,
		},
		{
			name: "unclosed frontmatter",
			input: `---
name: test
no closing delimiter`,
			wantFM:      "",
			wantContent: "",
			wantErr:     true,
		},
		{
			name: "whitespace before frontmatter",
			input: `
  ---
name: test
---

Content`,
			wantFM:      "name: test",
			wantContent: "Content",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, content, err := splitFrontmatter([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if string(fm) != tt.wantFM {
				t.Errorf("Frontmatter mismatch:\ngot:  %q\nwant: %q", string(fm), tt.wantFM)
			}
			if content != tt.wantContent {
				t.Errorf("Content mismatch:\ngot:  %q\nwant: %q", content, tt.wantContent)
			}
		})
	}
}

func TestSkillSave(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "skill-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	s := &Skill{
		Name:        "my-new-skill",
		Description: "A brand new skill",
		Content:     "# My New Skill\n\nDo something amazing with $ARGUMENTS.",
	}

	skillDir := filepath.Join(tmpDir, "my-new-skill")
	if err := s.Save(skillDir); err != nil {
		t.Fatalf("Failed to save skill: %v", err)
	}

	// Verify SKILL.md was created
	data, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("Failed to read SKILL.md: %v", err)
	}

	content := string(data)
	if !contains(content, "---") {
		t.Error("SKILL.md should contain frontmatter delimiters")
	}
	if !contains(content, "name: my-new-skill") {
		t.Error("SKILL.md should contain name in frontmatter")
	}
	if !contains(content, "description: A brand new skill") {
		t.Error("SKILL.md should contain description in frontmatter")
	}
	if !contains(content, "Do something amazing") {
		t.Error("SKILL.md should contain the content")
	}

	// Verify path was set
	if s.Path != skillDir {
		t.Errorf("Path should be set after save, got %q", s.Path)
	}
}

func TestSkillSaveWithDefaultContent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "skill-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	s := &Skill{
		Name:        "empty-skill",
		Description: "A skill with no content",
		// No Content set
	}

	skillDir := filepath.Join(tmpDir, "empty-skill")
	if err := s.Save(skillDir); err != nil {
		t.Fatalf("Failed to save skill: %v", err)
	}

	// Verify SKILL.md has default template content
	data, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("Failed to read SKILL.md: %v", err)
	}

	content := string(data)
	if !contains(content, "TODO: Write your skill prompt") {
		t.Error("SKILL.md should contain default template when content is empty")
	}
	if !contains(content, "$ARGUMENTS") {
		t.Error("SKILL.md should mention $ARGUMENTS in default template")
	}
}

func TestSkillToMarkdown(t *testing.T) {
	s := &Skill{
		Name:        "test-skill",
		Description: "Test description",
		Content:     "Custom content here",
	}

	md := s.ToMarkdown()

	if !contains(md, "---\n") {
		t.Error("ToMarkdown should start with frontmatter delimiter")
	}
	if !contains(md, "name: test-skill") {
		t.Error("ToMarkdown should include name")
	}
	if !contains(md, "description: Test description") {
		t.Error("ToMarkdown should include description")
	}
	if !contains(md, "Custom content here") {
		t.Error("ToMarkdown should include content")
	}
}

func TestSkillValidate(t *testing.T) {
	tests := []struct {
		name    string
		skill   *Skill
		wantErr bool
	}{
		{
			name:    "valid skill",
			skill:   &Skill{Name: "test", Description: "A test skill"},
			wantErr: false,
		},
		{
			name:    "missing name",
			skill:   &Skill{Description: "A test skill"},
			wantErr: true,
		},
		{
			name:    "missing description",
			skill:   &Skill{Name: "test"},
			wantErr: true,
		},
		{
			name:    "empty skill",
			skill:   &Skill{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.skill.Validate()
			if tt.wantErr && err == nil {
				t.Error("Expected validation error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected validation error: %v", err)
			}
		})
	}
}

func TestLoadAllWithSkillMd(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "skills-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create skill with SKILL.md
	mdDir := filepath.Join(tmpDir, "md-skill")
	if err := os.MkdirAll(mdDir, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	skillMd := `---
name: md-skill
description: From SKILL.md
---

Content`
	if err := os.WriteFile(filepath.Join(mdDir, "SKILL.md"), []byte(skillMd), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	// Create skill with skill.json (legacy)
	jsonDir := filepath.Join(tmpDir, "json-skill")
	if err := os.MkdirAll(jsonDir, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(jsonDir, "skill.json"), []byte(`{"name": "json-skill"}`), 0644); err != nil {
		t.Fatalf("Failed to write skill.json: %v", err)
	}

	skills, err := LoadAll(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load skills: %v", err)
	}

	if len(skills) != 2 {
		t.Errorf("Expected 2 skills, got %d", len(skills))
	}

	// Check both types loaded
	names := make(map[string]bool)
	for _, s := range skills {
		names[s.Name] = true
	}
	if !names["md-skill"] {
		t.Error("md-skill should be loaded")
	}
	if !names["json-skill"] {
		t.Error("json-skill should be loaded")
	}
}

func TestSkillRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "skill-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	original := &Skill{
		Name:        "roundtrip-skill",
		Description: "Testing save and load",
		Content:     "# Round Trip\n\nThis should survive save and load.",
	}

	skillDir := filepath.Join(tmpDir, "roundtrip-skill")
	if err := original.Save(skillDir); err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	loaded, err := Load(skillDir)
	if err != nil {
		t.Fatalf("Failed to load: %v", err)
	}

	if loaded.Name != original.Name {
		t.Errorf("Name mismatch: got %q, want %q", loaded.Name, original.Name)
	}
	if loaded.Description != original.Description {
		t.Errorf("Description mismatch: got %q, want %q", loaded.Description, original.Description)
	}
	if loaded.Content != original.Content {
		t.Errorf("Content mismatch:\ngot:  %q\nwant: %q", loaded.Content, original.Content)
	}
}

// ============================================================================
// Multi-Command Skill Tests
// ============================================================================

func TestLoadSkillWithCommands(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "skill-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	skillDir := filepath.Join(tmpDir, "multi-cmd-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	// Create SKILL.md (main skill file)
	skillMd := `---
name: multi-cmd-skill
description: A skill with multiple commands
---

# Multi-Command Skill

This is the default command.`

	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMd), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	// Create review.md subcommand
	reviewMd := `---
name: review
description: Review code for issues
---

# Review Command

Review the code and provide feedback.`

	if err := os.WriteFile(filepath.Join(skillDir, "review.md"), []byte(reviewMd), 0644); err != nil {
		t.Fatalf("Failed to write review.md: %v", err)
	}

	// Create test.md subcommand (no frontmatter - should use filename)
	testMd := `# Test Command

Run tests and check coverage.`

	if err := os.WriteFile(filepath.Join(skillDir, "test.md"), []byte(testMd), 0644); err != nil {
		t.Fatalf("Failed to write test.md: %v", err)
	}

	// Create a non-.md file (should be ignored)
	if err := os.WriteFile(filepath.Join(skillDir, "README.txt"), []byte("Ignored"), 0644); err != nil {
		t.Fatalf("Failed to write README.txt: %v", err)
	}

	s, err := Load(skillDir)
	if err != nil {
		t.Fatalf("Failed to load skill: %v", err)
	}

	// Check main skill
	if s.Name != "multi-cmd-skill" {
		t.Errorf("Skill name mismatch: got %q", s.Name)
	}
	if !contains(s.Content, "default command") {
		t.Errorf("Skill content mismatch: got %q", s.Content)
	}

	// Check commands were loaded
	if len(s.Commands) != 2 {
		t.Fatalf("Expected 2 commands, got %d", len(s.Commands))
	}

	// Check review command
	reviewCmd := s.GetCommand("review")
	if reviewCmd == nil {
		t.Fatal("review command not found")
	}
	if reviewCmd.Description != "Review code for issues" {
		t.Errorf("review description mismatch: got %q", reviewCmd.Description)
	}
	if !contains(reviewCmd.Content, "Review the code") {
		t.Errorf("review content mismatch: got %q", reviewCmd.Content)
	}
	if reviewCmd.FileName != "review.md" {
		t.Errorf("review filename mismatch: got %q", reviewCmd.FileName)
	}

	// Check test command (no frontmatter)
	testCmd := s.GetCommand("test")
	if testCmd == nil {
		t.Fatal("test command not found")
	}
	if testCmd.Name != "test" {
		t.Errorf("test name should default to filename, got %q", testCmd.Name)
	}
	if !contains(testCmd.Content, "Run tests") {
		t.Errorf("test content mismatch: got %q", testCmd.Content)
	}
}

func TestSkillGetCommand(t *testing.T) {
	s := &Skill{
		Name: "test-skill",
		Commands: []*Command{
			{Name: "cmd1", Description: "First command"},
			{Name: "cmd2", Description: "Second command"},
		},
	}

	// Test finding existing command
	cmd := s.GetCommand("cmd1")
	if cmd == nil {
		t.Fatal("cmd1 should be found")
	}
	if cmd.Description != "First command" {
		t.Errorf("cmd1 description mismatch: got %q", cmd.Description)
	}

	// Test finding non-existent command
	cmd = s.GetCommand("nonexistent")
	if cmd != nil {
		t.Error("nonexistent command should return nil")
	}
}

func TestSkillAddCommand(t *testing.T) {
	s := &Skill{
		Name:     "test-skill",
		Commands: []*Command{},
	}

	// Add first command
	cmd1 := &Command{Name: "cmd1", Description: "First command"}
	if err := s.AddCommand(cmd1); err != nil {
		t.Fatalf("Failed to add cmd1: %v", err)
	}
	if len(s.Commands) != 1 {
		t.Errorf("Expected 1 command, got %d", len(s.Commands))
	}
	if cmd1.FileName != "cmd1.md" {
		t.Errorf("FileName should be auto-set, got %q", cmd1.FileName)
	}

	// Add second command with explicit filename
	cmd2 := &Command{Name: "cmd2", FileName: "custom.md"}
	if err := s.AddCommand(cmd2); err != nil {
		t.Fatalf("Failed to add cmd2: %v", err)
	}
	if cmd2.FileName != "custom.md" {
		t.Errorf("Explicit filename should be preserved, got %q", cmd2.FileName)
	}

	// Try to add duplicate
	cmdDup := &Command{Name: "cmd1"}
	if err := s.AddCommand(cmdDup); err == nil {
		t.Error("Adding duplicate command should fail")
	}
}

func TestSkillSaveCommand(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "skill-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	skillDir := filepath.Join(tmpDir, "save-cmd-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	s := &Skill{
		Name: "save-cmd-skill",
		Path: skillDir,
	}

	cmd := &Command{
		Name:        "review",
		Description: "Review code",
		Content:     "# Review\n\nReview the code carefully.",
		FileName:    "review.md",
	}

	if err := s.SaveCommand(cmd); err != nil {
		t.Fatalf("Failed to save command: %v", err)
	}

	// Verify file was created
	data, err := os.ReadFile(filepath.Join(skillDir, "review.md"))
	if err != nil {
		t.Fatalf("Failed to read saved command: %v", err)
	}

	content := string(data)
	if !contains(content, "---") {
		t.Error("Command file should have frontmatter")
	}
	if !contains(content, "name: review") {
		t.Error("Command file should have name in frontmatter")
	}
	if !contains(content, "description: Review code") {
		t.Error("Command file should have description in frontmatter")
	}
	if !contains(content, "Review the code carefully") {
		t.Error("Command file should have content")
	}
}

func TestSkillSaveCommandNoPath(t *testing.T) {
	s := &Skill{
		Name: "no-path-skill",
		// Path not set
	}

	cmd := &Command{Name: "test", FileName: "test.md"}
	err := s.SaveCommand(cmd)
	if err == nil {
		t.Error("SaveCommand should fail when skill path not set")
	}
}

func TestSkillRemoveCommand(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "skill-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	skillDir := filepath.Join(tmpDir, "remove-cmd-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	// Create command file
	cmdPath := filepath.Join(skillDir, "to-remove.md")
	if err := os.WriteFile(cmdPath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create command file: %v", err)
	}

	s := &Skill{
		Name: "remove-cmd-skill",
		Path: skillDir,
		Commands: []*Command{
			{Name: "to-remove", FileName: "to-remove.md"},
			{Name: "to-keep", FileName: "to-keep.md"},
		},
	}

	// Remove command
	if err := s.RemoveCommand("to-remove"); err != nil {
		t.Fatalf("Failed to remove command: %v", err)
	}

	// Verify command removed from slice
	if len(s.Commands) != 1 {
		t.Errorf("Expected 1 command after removal, got %d", len(s.Commands))
	}
	if s.GetCommand("to-remove") != nil {
		t.Error("to-remove should be gone from commands")
	}

	// Verify file deleted
	if _, err := os.Stat(cmdPath); !os.IsNotExist(err) {
		t.Error("Command file should be deleted")
	}

	// Try to remove non-existent command
	if err := s.RemoveCommand("nonexistent"); err == nil {
		t.Error("Removing nonexistent command should fail")
	}
}

func TestCommandToMarkdown(t *testing.T) {
	cmd := &Command{
		Name:        "test-cmd",
		Description: "A test command",
		Content:     "# Test\n\nDo the test.",
	}

	md := cmd.ToMarkdown()

	if !contains(md, "---\n") {
		t.Error("ToMarkdown should have frontmatter")
	}
	if !contains(md, "name: test-cmd") {
		t.Error("ToMarkdown should include name")
	}
	if !contains(md, "description: A test command") {
		t.Error("ToMarkdown should include description")
	}
	if !contains(md, "Do the test") {
		t.Error("ToMarkdown should include content")
	}
}

func TestCommandToMarkdownDefault(t *testing.T) {
	cmd := &Command{
		Name: "empty-cmd",
		// No content set
	}

	md := cmd.ToMarkdown()

	if !contains(md, "TODO: Write your command prompt") {
		t.Error("ToMarkdown should have default template when content is empty")
	}
	if !contains(md, "$ARGUMENTS") {
		t.Error("ToMarkdown should mention $ARGUMENTS in default template")
	}
}

func TestSkillCommandNames(t *testing.T) {
	s := &Skill{
		Name: "test-skill",
		Commands: []*Command{
			{Name: "alpha"},
			{Name: "beta"},
			{Name: "gamma"},
		},
	}

	names := s.CommandNames()
	if len(names) != 3 {
		t.Errorf("Expected 3 names, got %d", len(names))
	}

	expected := []string{"alpha", "beta", "gamma"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("Name %d mismatch: got %q, want %q", i, name, expected[i])
		}
	}
}

func TestSkillHasCommands(t *testing.T) {
	// Skill without commands
	s1 := &Skill{Name: "no-cmds"}
	if s1.HasCommands() {
		t.Error("Skill without commands should return false")
	}

	// Skill with commands
	s2 := &Skill{
		Name:     "with-cmds",
		Commands: []*Command{{Name: "cmd1"}},
	}
	if !s2.HasCommands() {
		t.Error("Skill with commands should return true")
	}
}

func TestParseCommandMd(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		fileName string
		wantName string
		wantDesc string
		wantErr  bool
	}{
		{
			name: "with frontmatter",
			input: `---
name: custom-name
description: Custom description
---

# Content

Body here.`,
			fileName: "test.md",
			wantName: "custom-name",
			wantDesc: "Custom description",
			wantErr:  false,
		},
		{
			name:     "without frontmatter",
			input:    "# Just Content\n\nNo frontmatter here.",
			fileName: "my-command.md",
			wantName: "my-command", // Should default to filename without .md
			wantDesc: "",
			wantErr:  false,
		},
		{
			name: "partial frontmatter",
			input: `---
description: Only description
---

Content`,
			fileName: "partial.md",
			wantName: "partial", // Should default to filename
			wantDesc: "Only description",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := parseCommandMd([]byte(tt.input), tt.fileName)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if cmd.Name != tt.wantName {
				t.Errorf("Name mismatch: got %q, want %q", cmd.Name, tt.wantName)
			}
			if cmd.Description != tt.wantDesc {
				t.Errorf("Description mismatch: got %q, want %q", cmd.Description, tt.wantDesc)
			}
			if cmd.FileName != tt.fileName {
				t.Errorf("FileName mismatch: got %q, want %q", cmd.FileName, tt.fileName)
			}
		})
	}
}

func TestSkillCommandsRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "skill-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	skillDir := filepath.Join(tmpDir, "roundtrip-skill")

	// Create and save skill
	original := &Skill{
		Name:        "roundtrip-skill",
		Description: "Testing command round trip",
		Content:     "Default content",
	}

	if err := original.Save(skillDir); err != nil {
		t.Fatalf("Failed to save skill: %v", err)
	}

	// Add and save commands
	cmd1 := &Command{
		Name:        "review",
		Description: "Review code",
		Content:     "Review the code carefully.",
		FileName:    "review.md",
	}
	cmd2 := &Command{
		Name:        "test",
		Description: "Run tests",
		Content:     "Execute all tests.",
		FileName:    "test.md",
	}

	if err := original.AddCommand(cmd1); err != nil {
		t.Fatalf("Failed to add cmd1: %v", err)
	}
	if err := original.AddCommand(cmd2); err != nil {
		t.Fatalf("Failed to add cmd2: %v", err)
	}
	if err := original.SaveCommand(cmd1); err != nil {
		t.Fatalf("Failed to save cmd1: %v", err)
	}
	if err := original.SaveCommand(cmd2); err != nil {
		t.Fatalf("Failed to save cmd2: %v", err)
	}

	// Reload skill
	loaded, err := Load(skillDir)
	if err != nil {
		t.Fatalf("Failed to reload skill: %v", err)
	}

	// Verify main skill
	if loaded.Name != original.Name {
		t.Errorf("Skill name mismatch: got %q", loaded.Name)
	}

	// Verify commands were loaded
	if len(loaded.Commands) != 2 {
		t.Fatalf("Expected 2 commands, got %d", len(loaded.Commands))
	}

	reviewLoaded := loaded.GetCommand("review")
	if reviewLoaded == nil {
		t.Fatal("review command not found after reload")
	}
	if reviewLoaded.Description != "Review code" {
		t.Errorf("review description mismatch: got %q", reviewLoaded.Description)
	}
	if reviewLoaded.Content != "Review the code carefully." {
		t.Errorf("review content mismatch: got %q", reviewLoaded.Content)
	}

	testLoaded := loaded.GetCommand("test")
	if testLoaded == nil {
		t.Fatal("test command not found after reload")
	}
	if testLoaded.Description != "Run tests" {
		t.Errorf("test description mismatch: got %q", testLoaded.Description)
	}
}
