package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/iheanyi/agentctl/pkg/mcp"
)

func TestDefaultConfigDir(t *testing.T) {
	// Save and clear env vars
	origHome := os.Getenv("AGENTCTL_HOME")
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		os.Setenv("AGENTCTL_HOME", origHome)
		os.Setenv("XDG_CONFIG_HOME", origXDG)
	}()

	tests := []struct {
		name        string
		agentctlHome string
		xdgConfig   string
		wantSuffix  string
	}{
		{
			name:        "AGENTCTL_HOME takes precedence",
			agentctlHome: "/custom/agentctl",
			xdgConfig:   "/xdg/config",
			wantSuffix:  "/custom/agentctl",
		},
		{
			name:       "XDG_CONFIG_HOME used if no AGENTCTL_HOME",
			xdgConfig:  "/xdg/config",
			wantSuffix: "/xdg/config/agentctl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("AGENTCTL_HOME", tt.agentctlHome)
			os.Setenv("XDG_CONFIG_HOME", tt.xdgConfig)

			got := DefaultConfigDir()
			if got != tt.wantSuffix {
				t.Errorf("DefaultConfigDir() = %q, want %q", got, tt.wantSuffix)
			}
		})
	}
}

func TestLoadFromNonexistent(t *testing.T) {
	cfg, err := LoadFrom("/nonexistent/config.json")
	if err != nil {
		t.Fatalf("LoadFrom should not error for nonexistent file: %v", err)
	}
	if cfg == nil {
		t.Fatal("Config should not be nil")
	}
	if cfg.Version != "1" {
		t.Errorf("Default version should be '1', got %q", cfg.Version)
	}
	if cfg.Servers == nil {
		t.Error("Servers map should be initialized")
	}
}

func TestLoadAndSave(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "agentctl.json")

	// Create a config
	cfg := &Config{
		Version: "1",
		Servers: map[string]*mcp.Server{
			"filesystem": {
				Name:    "filesystem",
				Command: "npx",
				Args:    []string{"-y", "@modelcontextprotocol/server-filesystem"},
				Source: mcp.Source{
					Type:  "alias",
					Alias: "filesystem",
				},
			},
		},
		Commands: []string{"explain", "review"},
		Rules:    []string{"coding-style"},
		Settings: Settings{
			DefaultProfile: "work",
			Tools: map[string]ToolConfig{
				"claude": {Enabled: true},
				"cursor": {Enabled: true},
			},
		},
		Path:      configPath,
		ConfigDir: tmpDir,
	}

	// Save it
	if err := cfg.Save(); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Load it back
	loaded, err := LoadFrom(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify fields
	if loaded.Version != cfg.Version {
		t.Errorf("Version mismatch: got %q, want %q", loaded.Version, cfg.Version)
	}
	if len(loaded.Servers) != 1 {
		t.Errorf("Servers count mismatch: got %d, want 1", len(loaded.Servers))
	}
	if loaded.Servers["filesystem"] == nil {
		t.Error("filesystem server should exist")
	}
	if len(loaded.Commands) != 2 {
		t.Errorf("Commands count mismatch: got %d, want 2", len(loaded.Commands))
	}
	if loaded.Settings.DefaultProfile != "work" {
		t.Errorf("DefaultProfile mismatch: got %q, want %q", loaded.Settings.DefaultProfile, "work")
	}
}

func TestConfigMerge(t *testing.T) {
	base := &Config{
		Version: "1",
		Servers: map[string]*mcp.Server{
			"server1": {Name: "server1", Command: "cmd1"},
			"server2": {Name: "server2", Command: "cmd2"},
		},
		Commands: []string{"cmd1", "cmd2"},
	}

	overlay := &Config{
		Version: "1",
		Servers: map[string]*mcp.Server{
			"server2": {Name: "server2", Command: "cmd2-modified"}, // Override
			"server3": {Name: "server3", Command: "cmd3"},          // Add new
		},
		Commands: []string{"cmd3"},
		Disabled: []string{"server1"}, // Disable server1
	}

	merged := base.Merge(overlay)

	// server1 should be removed (disabled)
	if _, exists := merged.Servers["server1"]; exists {
		t.Error("server1 should be disabled/removed")
	}

	// server2 should be updated
	if merged.Servers["server2"].Command != "cmd2-modified" {
		t.Errorf("server2 should be modified, got command %q", merged.Servers["server2"].Command)
	}

	// server3 should be added
	if merged.Servers["server3"] == nil {
		t.Error("server3 should be added")
	}

	// Commands should be merged
	if len(merged.Commands) != 3 {
		t.Errorf("Commands should have 3 items, got %d", len(merged.Commands))
	}
}

func TestActiveServers(t *testing.T) {
	cfg := &Config{
		Servers: map[string]*mcp.Server{
			"active1":   {Name: "active1", Disabled: false},
			"active2":   {Name: "active2", Disabled: false},
			"disabled1": {Name: "disabled1", Disabled: true},
		},
	}

	active := cfg.ActiveServers()
	if len(active) != 2 {
		t.Errorf("Expected 2 active servers, got %d", len(active))
	}

	// Verify disabled server is not included
	for _, s := range active {
		if s.Name == "disabled1" {
			t.Error("Disabled server should not be in active list")
		}
	}
}

func TestAutoUpdateConfig(t *testing.T) {
	cfg := &Config{
		Settings: Settings{
			AutoUpdate: AutoUpdateConfig{
				Enabled:  true,
				Interval: "24h",
				Servers: map[string]string{
					"filesystem": "auto",
					"custom":     "notify",
				},
			},
		},
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if !loaded.Settings.AutoUpdate.Enabled {
		t.Error("AutoUpdate.Enabled should be true")
	}
	if loaded.Settings.AutoUpdate.Interval != "24h" {
		t.Errorf("AutoUpdate.Interval mismatch: got %q", loaded.Settings.AutoUpdate.Interval)
	}
	if loaded.Settings.AutoUpdate.Servers["filesystem"] != "auto" {
		t.Errorf("filesystem auto-update setting mismatch")
	}
}

func TestDefaultToolSettings(t *testing.T) {
	settings := defaultToolSettings()

	expectedTools := []string{"claude", "cursor", "codex", "opencode", "cline", "windsurf", "zed", "continue"}
	for _, tool := range expectedTools {
		if _, exists := settings[tool]; !exists {
			t.Errorf("Tool %q should be in default settings", tool)
		}
		if !settings[tool].Enabled {
			t.Errorf("Tool %q should be enabled by default", tool)
		}
	}
}

func TestMergeStringSlices(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want int // Expected length (unique items)
	}{
		{
			name: "no overlap",
			a:    []string{"a", "b"},
			b:    []string{"c", "d"},
			want: 4,
		},
		{
			name: "with overlap",
			a:    []string{"a", "b", "c"},
			b:    []string{"b", "c", "d"},
			want: 4,
		},
		{
			name: "empty slices",
			a:    []string{},
			b:    []string{},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeStringSlices(tt.a, tt.b)
			if len(result) != tt.want {
				t.Errorf("mergeStringSlices() = %d items, want %d", len(result), tt.want)
			}
		})
	}
}

func TestRemoveFromSlice(t *testing.T) {
	slice := []string{"a", "b", "c", "d"}

	result := removeFromSlice(slice, "b")
	if len(result) != 3 {
		t.Errorf("Expected 3 items, got %d", len(result))
	}

	for _, s := range result {
		if s == "b" {
			t.Error("'b' should be removed")
		}
	}

	// Remove non-existent item
	result2 := removeFromSlice(slice, "z")
	if len(result2) != 4 {
		t.Errorf("Removing non-existent item should not change length")
	}
}
