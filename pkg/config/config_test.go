package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/iheanyi/agentctl/pkg/command"
	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/iheanyi/agentctl/pkg/prompt"
	"github.com/iheanyi/agentctl/pkg/rule"
	"github.com/iheanyi/agentctl/pkg/skill"
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

func TestLoadScoped(t *testing.T) {
	// Create temp directories
	tmpDir, err := os.MkdirTemp("", "config-scope-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Save original working directory and env
	origWd, _ := os.Getwd()
	origHome := os.Getenv("AGENTCTL_HOME")
	defer func() {
		os.Chdir(origWd)
		os.Setenv("AGENTCTL_HOME", origHome)
	}()

	// Set up global config directory
	globalDir := filepath.Join(tmpDir, "global")
	os.MkdirAll(globalDir, 0755)
	os.Setenv("AGENTCTL_HOME", globalDir)

	// Create global config
	globalCfg := &Config{
		Version: "1",
		Servers: map[string]*mcp.Server{
			"global-server": {Name: "global-server", Command: "global-cmd"},
		},
	}
	globalData, _ := json.MarshalIndent(globalCfg, "", "  ")
	os.WriteFile(filepath.Join(globalDir, "agentctl.json"), globalData, 0644)

	// Set up project directory with local config
	projectDir := filepath.Join(tmpDir, "project")
	os.MkdirAll(projectDir, 0755)

	localCfg := &Config{
		Version: "1",
		Servers: map[string]*mcp.Server{
			"local-server": {Name: "local-server", Command: "local-cmd"},
		},
	}
	localData, _ := json.MarshalIndent(localCfg, "", "  ")
	os.WriteFile(filepath.Join(projectDir, ".agentctl.json"), localData, 0644)

	// Change to project directory
	os.Chdir(projectDir)

	t.Run("LoadScoped global", func(t *testing.T) {
		cfg, err := LoadScoped(ScopeGlobal)
		if err != nil {
			t.Fatalf("LoadScoped(global) failed: %v", err)
		}
		if cfg.Servers["global-server"] == nil {
			t.Error("Should have global-server")
		}
		if cfg.Servers["global-server"].Scope != string(ScopeGlobal) {
			t.Errorf("Server scope should be 'global', got %q", cfg.Servers["global-server"].Scope)
		}
	})

	t.Run("LoadScoped local", func(t *testing.T) {
		cfg, err := LoadScoped(ScopeLocal)
		if err != nil {
			t.Fatalf("LoadScoped(local) failed: %v", err)
		}
		if cfg.Servers["local-server"] == nil {
			t.Error("Should have local-server")
		}
		if cfg.Servers["local-server"].Scope != string(ScopeLocal) {
			t.Errorf("Server scope should be 'local', got %q", cfg.Servers["local-server"].Scope)
		}
	})

	t.Run("LoadScoped local nonexistent", func(t *testing.T) {
		// Change to a directory without .agentctl.json
		emptyDir := filepath.Join(tmpDir, "empty")
		os.MkdirAll(emptyDir, 0755)
		os.Chdir(emptyDir)
		defer os.Chdir(projectDir)

		cfg, err := LoadScoped(ScopeLocal)
		if err != nil {
			t.Fatalf("LoadScoped(local) should not fail for nonexistent: %v", err)
		}
		if len(cfg.Servers) != 0 {
			t.Error("Should have empty servers for nonexistent local config")
		}
	})
}

func TestServersForScope(t *testing.T) {
	cfg := &Config{
		Servers: map[string]*mcp.Server{
			"global1": {Name: "global1", Scope: string(ScopeGlobal)},
			"global2": {Name: "global2", Scope: string(ScopeGlobal)},
			"local1":  {Name: "local1", Scope: string(ScopeLocal)},
			"local2":  {Name: "local2", Scope: string(ScopeLocal)},
			"unset":   {Name: "unset", Scope: ""},           // Unset scope defaults to global
			"disabled": {Name: "disabled", Scope: string(ScopeGlobal), Disabled: true},
		},
	}

	t.Run("global scope", func(t *testing.T) {
		servers := cfg.ServersForScope(ScopeGlobal)
		if len(servers) != 3 { // global1, global2, unset (excludes disabled)
			t.Errorf("Expected 3 global servers, got %d", len(servers))
		}
	})

	t.Run("local scope", func(t *testing.T) {
		servers := cfg.ServersForScope(ScopeLocal)
		if len(servers) != 2 { // local1, local2
			t.Errorf("Expected 2 local servers, got %d", len(servers))
		}
	})

	t.Run("all scope", func(t *testing.T) {
		servers := cfg.ServersForScope(ScopeAll)
		if len(servers) != 5 { // All except disabled
			t.Errorf("Expected 5 servers, got %d", len(servers))
		}
	})
}

func TestGetServerScope(t *testing.T) {
	cfg := &Config{
		Servers: map[string]*mcp.Server{
			"global": {Name: "global", Scope: string(ScopeGlobal)},
			"local":  {Name: "local", Scope: string(ScopeLocal)},
			"unset":  {Name: "unset", Scope: ""},
		},
	}

	t.Run("global server", func(t *testing.T) {
		scope := cfg.GetServerScope("global")
		if scope != ScopeGlobal {
			t.Errorf("Expected ScopeGlobal, got %v", scope)
		}
	})

	t.Run("local server", func(t *testing.T) {
		scope := cfg.GetServerScope("local")
		if scope != ScopeLocal {
			t.Errorf("Expected ScopeLocal, got %v", scope)
		}
	})

	t.Run("unset scope defaults to global", func(t *testing.T) {
		scope := cfg.GetServerScope("unset")
		if scope != ScopeGlobal {
			t.Errorf("Expected ScopeGlobal for unset, got %v", scope)
		}
	})

	t.Run("nonexistent server", func(t *testing.T) {
		scope := cfg.GetServerScope("nonexistent")
		if scope != "" {
			t.Errorf("Expected empty scope for nonexistent, got %v", scope)
		}
	})
}

func TestMergeScopeTracking(t *testing.T) {
	base := &Config{
		Version: "1",
		Servers: map[string]*mcp.Server{
			"server1": {Name: "server1", Command: "cmd1"},
			"server2": {Name: "server2", Command: "cmd2"},
		},
	}

	overlay := &Config{
		Version: "1",
		Servers: map[string]*mcp.Server{
			"server3": {Name: "server3", Command: "cmd3"},
		},
	}

	merged := base.Merge(overlay)

	// Base servers should be marked as global
	if merged.Servers["server1"].Scope != string(ScopeGlobal) {
		t.Errorf("server1 should have global scope, got %q", merged.Servers["server1"].Scope)
	}
	if merged.Servers["server2"].Scope != string(ScopeGlobal) {
		t.Errorf("server2 should have global scope, got %q", merged.Servers["server2"].Scope)
	}

	// Overlay servers should be marked as local
	if merged.Servers["server3"].Scope != string(ScopeLocal) {
		t.Errorf("server3 should have local scope, got %q", merged.Servers["server3"].Scope)
	}
}

func TestSaveScoped(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "save-scope-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Save original working directory and env
	origWd, _ := os.Getwd()
	origHome := os.Getenv("AGENTCTL_HOME")
	defer func() {
		os.Chdir(origWd)
		os.Setenv("AGENTCTL_HOME", origHome)
	}()

	// Set up directories
	globalDir := filepath.Join(tmpDir, "global")
	projectDir := filepath.Join(tmpDir, "project")
	os.MkdirAll(globalDir, 0755)
	os.MkdirAll(projectDir, 0755)

	os.Setenv("AGENTCTL_HOME", globalDir)
	os.Chdir(projectDir)

	cfg := &Config{
		Version: "1",
		Servers: map[string]*mcp.Server{
			"test-server": {Name: "test-server", Command: "test-cmd"},
		},
	}

	t.Run("save to global", func(t *testing.T) {
		err := cfg.SaveScoped(ScopeGlobal)
		if err != nil {
			t.Fatalf("SaveScoped(global) failed: %v", err)
		}

		// Verify file exists
		globalPath := filepath.Join(globalDir, "agentctl.json")
		if _, err := os.Stat(globalPath); os.IsNotExist(err) {
			t.Error("Global config file should exist")
		}
	})

	t.Run("save to local", func(t *testing.T) {
		err := cfg.SaveScoped(ScopeLocal)
		if err != nil {
			t.Fatalf("SaveScoped(local) failed: %v", err)
		}

		// Verify file exists
		localPath := filepath.Join(projectDir, ".agentctl.json")
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			t.Error("Local config file should exist")
		}
	})

	t.Run("save to all fails", func(t *testing.T) {
		err := cfg.SaveScoped(ScopeAll)
		if err == nil {
			t.Error("SaveScoped(all) should fail")
		}
	})
}

func TestProjectDir(t *testing.T) {
	t.Run("with project path", func(t *testing.T) {
		cfg := &Config{
			ProjectPath: "/path/to/project/.agentctl.json",
		}
		if cfg.ProjectDir() != "/path/to/project" {
			t.Errorf("Expected /path/to/project, got %q", cfg.ProjectDir())
		}
	})

	t.Run("without project path", func(t *testing.T) {
		cfg := &Config{}
		if cfg.ProjectDir() != "" {
			t.Errorf("Expected empty string, got %q", cfg.ProjectDir())
		}
	})
}

func TestRulesForScope(t *testing.T) {
	cfg := &Config{
		LoadedRules: []*rule.Rule{
			{Name: "global1", Scope: string(ScopeGlobal)},
			{Name: "global2", Scope: string(ScopeGlobal)},
			{Name: "local1", Scope: string(ScopeLocal)},
			{Name: "unset", Scope: ""}, // Unset scope defaults to global
		},
	}

	t.Run("global scope", func(t *testing.T) {
		rules := cfg.RulesForScope(ScopeGlobal)
		if len(rules) != 3 { // global1, global2, unset
			t.Errorf("Expected 3 global rules, got %d", len(rules))
		}
	})

	t.Run("local scope", func(t *testing.T) {
		rules := cfg.RulesForScope(ScopeLocal)
		if len(rules) != 1 { // local1
			t.Errorf("Expected 1 local rule, got %d", len(rules))
		}
	})

	t.Run("all scope", func(t *testing.T) {
		rules := cfg.RulesForScope(ScopeAll)
		if len(rules) != 4 { // All rules
			t.Errorf("Expected 4 rules, got %d", len(rules))
		}
	})
}

func TestCommandsForScope(t *testing.T) {
	cfg := &Config{
		LoadedCommands: []*command.Command{
			{Name: "global1", Scope: string(ScopeGlobal)},
			{Name: "local1", Scope: string(ScopeLocal)},
			{Name: "local2", Scope: string(ScopeLocal)},
			{Name: "unset", Scope: ""}, // Unset scope defaults to global
		},
	}

	t.Run("global scope", func(t *testing.T) {
		commands := cfg.CommandsForScope(ScopeGlobal)
		if len(commands) != 2 { // global1, unset
			t.Errorf("Expected 2 global commands, got %d", len(commands))
		}
	})

	t.Run("local scope", func(t *testing.T) {
		commands := cfg.CommandsForScope(ScopeLocal)
		if len(commands) != 2 { // local1, local2
			t.Errorf("Expected 2 local commands, got %d", len(commands))
		}
	})

	t.Run("all scope", func(t *testing.T) {
		commands := cfg.CommandsForScope(ScopeAll)
		if len(commands) != 4 { // All commands
			t.Errorf("Expected 4 commands, got %d", len(commands))
		}
	})
}

func TestPromptsForScope(t *testing.T) {
	cfg := &Config{
		LoadedPrompts: []*prompt.Prompt{
			{Name: "global1", Scope: string(ScopeGlobal)},
			{Name: "local1", Scope: string(ScopeLocal)},
			{Name: "unset", Scope: ""}, // Unset scope defaults to global
		},
	}

	t.Run("global scope", func(t *testing.T) {
		prompts := cfg.PromptsForScope(ScopeGlobal)
		if len(prompts) != 2 { // global1, unset
			t.Errorf("Expected 2 global prompts, got %d", len(prompts))
		}
	})

	t.Run("local scope", func(t *testing.T) {
		prompts := cfg.PromptsForScope(ScopeLocal)
		if len(prompts) != 1 { // local1
			t.Errorf("Expected 1 local prompt, got %d", len(prompts))
		}
	})

	t.Run("all scope", func(t *testing.T) {
		prompts := cfg.PromptsForScope(ScopeAll)
		if len(prompts) != 3 { // All prompts
			t.Errorf("Expected 3 prompts, got %d", len(prompts))
		}
	})
}

func TestSkillsForScope(t *testing.T) {
	cfg := &Config{
		LoadedSkills: []*skill.Skill{
			{Name: "global1", Scope: string(ScopeGlobal)},
			{Name: "global2", Scope: string(ScopeGlobal)},
			{Name: "local1", Scope: string(ScopeLocal)},
		},
	}

	t.Run("global scope", func(t *testing.T) {
		skills := cfg.SkillsForScope(ScopeGlobal)
		if len(skills) != 2 { // global1, global2
			t.Errorf("Expected 2 global skills, got %d", len(skills))
		}
	})

	t.Run("local scope", func(t *testing.T) {
		skills := cfg.SkillsForScope(ScopeLocal)
		if len(skills) != 1 { // local1
			t.Errorf("Expected 1 local skill, got %d", len(skills))
		}
	})

	t.Run("all scope", func(t *testing.T) {
		skills := cfg.SkillsForScope(ScopeAll)
		if len(skills) != 3 { // All skills
			t.Errorf("Expected 3 skills, got %d", len(skills))
		}
	})
}
