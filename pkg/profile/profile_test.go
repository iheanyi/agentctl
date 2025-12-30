package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestProfileSerialization(t *testing.T) {
	p := Profile{
		Name:        "work",
		Description: "Work profile",
		Servers:     []string{"github-mcp", "jira-mcp"},
		Commands:    []string{"code-review"},
		Rules:       []string{"security"},
		Disabled:    []string{"slack-mcp"},
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Failed to marshal profile: %v", err)
	}

	var decoded Profile
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal profile: %v", err)
	}

	if decoded.Name != p.Name {
		t.Errorf("Name mismatch: got %q, want %q", decoded.Name, p.Name)
	}
	if decoded.Description != p.Description {
		t.Errorf("Description mismatch: got %q, want %q", decoded.Description, p.Description)
	}
	if len(decoded.Servers) != 2 {
		t.Errorf("Servers length mismatch: got %d, want 2", len(decoded.Servers))
	}
	if len(decoded.Disabled) != 1 {
		t.Errorf("Disabled length mismatch: got %d, want 1", len(decoded.Disabled))
	}
}

func TestLoadProfile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "profile-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	profileJSON := `{
		"name": "test-profile",
		"description": "A test profile",
		"servers": ["server1", "server2"]
	}`

	profilePath := filepath.Join(tmpDir, "test.json")
	if err := os.WriteFile(profilePath, []byte(profileJSON), 0644); err != nil {
		t.Fatalf("Failed to write profile file: %v", err)
	}

	p, err := Load(profilePath)
	if err != nil {
		t.Fatalf("Failed to load profile: %v", err)
	}

	if p.Name != "test-profile" {
		t.Errorf("Name mismatch: got %q", p.Name)
	}
	if p.Path != profilePath {
		t.Errorf("Path mismatch: got %q, want %q", p.Path, profilePath)
	}
	if len(p.Servers) != 2 {
		t.Errorf("Servers length mismatch: got %d", len(p.Servers))
	}
}

func TestLoadAllProfiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "profiles-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create multiple profiles
	profiles := []string{
		`{"name": "work"}`,
		`{"name": "personal"}`,
		`{"name": "testing"}`,
	}

	for i, content := range profiles {
		path := filepath.Join(tmpDir, "profile"+string(rune('0'+i+1))+".json")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	// Also create a non-JSON file (should be ignored)
	if err := os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("ignored"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	loaded, err := LoadAll(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load all profiles: %v", err)
	}

	if len(loaded) != 3 {
		t.Errorf("Expected 3 profiles, got %d", len(loaded))
	}
}

func TestLoadAllProfilesNonexistentDir(t *testing.T) {
	profiles, err := LoadAll("/nonexistent/directory")
	if err != nil {
		t.Errorf("LoadAll should return nil error for nonexistent dir, got: %v", err)
	}
	if profiles != nil {
		t.Errorf("LoadAll should return nil for nonexistent dir, got: %v", profiles)
	}
}

func TestExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "profile-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a profile
	if err := os.WriteFile(filepath.Join(tmpDir, "work.json"), []byte(`{"name":"work"}`), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	if !Exists(tmpDir, "work") {
		t.Error("Exists should return true for existing profile")
	}

	if Exists(tmpDir, "nonexistent") {
		t.Error("Exists should return false for nonexistent profile")
	}
}

func TestCreate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "profile-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p, err := Create(tmpDir, "newprofile", "A new profile")
	if err != nil {
		t.Fatalf("Failed to create profile: %v", err)
	}

	if p.Name != "newprofile" {
		t.Errorf("Name mismatch: got %q", p.Name)
	}

	// Verify file was created
	if _, err := os.Stat(filepath.Join(tmpDir, "newprofile.json")); err != nil {
		t.Error("Profile file should exist")
	}
}

func TestDelete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "profile-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a profile
	path := filepath.Join(tmpDir, "deleteme.json")
	if err := os.WriteFile(path, []byte(`{"name":"deleteme"}`), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	if err := Delete(tmpDir, "deleteme"); err != nil {
		t.Errorf("Delete failed: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("Profile file should be deleted")
	}
}

func TestSaveProfile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "profile-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p := &Profile{
		Name:        "saved",
		Description: "Saved profile",
		Servers:     []string{"server1"},
		Path:        filepath.Join(tmpDir, "saved.json"),
	}

	if err := p.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load and verify
	loaded, err := Load(p.Path)
	if err != nil {
		t.Fatalf("Failed to load saved profile: %v", err)
	}

	if loaded.Name != "saved" {
		t.Errorf("Name mismatch: got %q", loaded.Name)
	}
	if len(loaded.Servers) != 1 {
		t.Errorf("Servers mismatch: got %d", len(loaded.Servers))
	}
}

func TestPathNotSerialized(t *testing.T) {
	p := Profile{
		Name: "test",
		Path: "/some/path",
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	if contains(string(data), "/some/path") {
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
