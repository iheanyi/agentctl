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
