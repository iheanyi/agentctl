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
