package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryScannerDetect(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "scanner-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name  string
		cfg   ScannerConfig
		setup func(dir string) error
		want  bool
	}{
		{
			name: "detects tool directory",
			cfg: ScannerConfig{
				Name:      "test",
				LocalDirs: []string{".test"},
			},
			setup: func(dir string) error {
				return os.MkdirAll(filepath.Join(dir, ".test"), 0755)
			},
			want: true,
		},
		{
			name: "detects detect file",
			cfg: ScannerConfig{
				Name:        "test",
				DetectFiles: []string{".testconfig"},
			},
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, ".testconfig"), []byte("test"), 0644)
			},
			want: true,
		},
		{
			name: "returns false when nothing exists",
			cfg: ScannerConfig{
				Name:        "test",
				LocalDirs:   []string{".test"},
				DetectFiles: []string{".testconfig"},
			},
			setup: func(dir string) error {
				return nil
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh subdir for each test
			testDir := filepath.Join(tmpDir, tt.name)
			if err := os.MkdirAll(testDir, 0755); err != nil {
				t.Fatal(err)
			}

			if err := tt.setup(testDir); err != nil {
				t.Fatal(err)
			}

			scanner := NewDirectoryScanner(tt.cfg)
			got := scanner.Detect(testDir)
			if got != tt.want {
				t.Errorf("Detect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDirectoryScannerScanRules(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scanner-rules-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Setup: Create .cursor/rules/ with a rule file
	rulesDir := filepath.Join(tmpDir, ".cursor", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatal(err)
	}

	ruleContent := `---
priority: 1
---
# Test Rule

This is a test rule.
`
	if err := os.WriteFile(filepath.Join(rulesDir, "test-rule.md"), []byte(ruleContent), 0644); err != nil {
		t.Fatal(err)
	}

	scanner := NewDirectoryScanner(ScannerConfig{
		Name:      "cursor",
		LocalDirs: []string{".cursor"},
		RulesDirs: []string{"rules"},
	})

	rules, err := scanner.ScanRules(tmpDir)
	if err != nil {
		t.Fatalf("ScanRules() error = %v", err)
	}

	if len(rules) != 1 {
		t.Fatalf("ScanRules() got %d rules, want 1", len(rules))
	}

	rule := rules[0]
	if rule.Name != "test-rule" {
		t.Errorf("rule.Name = %q, want %q", rule.Name, "test-rule")
	}
	if rule.Tool != "cursor" {
		t.Errorf("rule.Tool = %q, want %q", rule.Tool, "cursor")
	}
	if rule.Scope != "local" {
		t.Errorf("rule.Scope = %q, want %q", rule.Scope, "local")
	}
}

func TestDirectoryScannerScanSkills(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scanner-skills-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Setup: Create .codex/skills/test-skill/SKILL.md
	skillDir := filepath.Join(tmpDir, ".codex", "skills", "test-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}

	skillContent := `---
name: test-skill
description: A test skill
---
# Test Skill

This is a test skill.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatal(err)
	}

	scanner := NewDirectoryScanner(ScannerConfig{
		Name:       "codex",
		LocalDirs:  []string{".codex"},
		SkillsDirs: []string{"skills"},
	})

	skills, err := scanner.ScanSkills(tmpDir)
	if err != nil {
		t.Fatalf("ScanSkills() error = %v", err)
	}

	if len(skills) != 1 {
		t.Fatalf("ScanSkills() got %d skills, want 1", len(skills))
	}

	skill := skills[0]
	if skill.Name != "test-skill" {
		t.Errorf("skill.Name = %q, want %q", skill.Name, "test-skill")
	}
	if skill.Tool != "codex" {
		t.Errorf("skill.Tool = %q, want %q", skill.Tool, "codex")
	}
	if skill.Scope != "local" {
		t.Errorf("skill.Scope = %q, want %q", skill.Scope, "local")
	}
}

func TestDirectoryScannerScanCommands(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scanner-commands-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Setup: Create .opencode/commands/ with a command file
	commandsDir := filepath.Join(tmpDir, ".opencode", "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatal(err)
	}

	commandContent := `---
name: test-command
description: A test command
---
# Test Command

Do something test-like.
`
	if err := os.WriteFile(filepath.Join(commandsDir, "test-command.md"), []byte(commandContent), 0644); err != nil {
		t.Fatal(err)
	}

	scanner := NewDirectoryScanner(ScannerConfig{
		Name:         "opencode",
		LocalDirs:    []string{".opencode"},
		CommandsDirs: []string{"commands"},
	})

	commands, err := scanner.ScanCommands(tmpDir)
	if err != nil {
		t.Fatalf("ScanCommands() error = %v", err)
	}

	if len(commands) != 1 {
		t.Fatalf("ScanCommands() got %d commands, want 1", len(commands))
	}

	cmd := commands[0]
	if cmd.Name != "test-command" {
		t.Errorf("cmd.Name = %q, want %q", cmd.Name, "test-command")
	}
	if cmd.Tool != "opencode" {
		t.Errorf("cmd.Tool = %q, want %q", cmd.Tool, "opencode")
	}
	if cmd.Scope != "local" {
		t.Errorf("cmd.Scope = %q, want %q", cmd.Scope, "local")
	}
}

func TestSafeReader(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "safe-reader-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("reads normal file", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "normal.txt")
		content := []byte("hello world")
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			t.Fatal(err)
		}

		reader := NewSafeReader()
		data, err := reader.ReadFile(filePath)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if string(data) != string(content) {
			t.Errorf("ReadFile() = %q, want %q", string(data), string(content))
		}
	})

	t.Run("rejects file over max size", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "large.txt")
		// Create a file with 100 bytes
		content := make([]byte, 100)
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			t.Fatal(err)
		}

		// Create reader with small max size
		reader := &SafeReader{MaxSize: 50}
		_, err := reader.ReadFile(filePath)
		if err == nil {
			t.Fatal("ReadFile() expected error for large file")
		}
	})
}

func TestCursorScannerIntegration(t *testing.T) {
	// Test that the cursor scanner is registered and works
	scanner, ok := Get("cursor")
	if !ok {
		t.Fatal("cursor scanner not registered")
	}

	if scanner.Name() != "cursor" {
		t.Errorf("scanner.Name() = %q, want %q", scanner.Name(), "cursor")
	}
}

func TestCodexScannerIntegration(t *testing.T) {
	scanner, ok := Get("codex")
	if !ok {
		t.Fatal("codex scanner not registered")
	}

	if scanner.Name() != "codex" {
		t.Errorf("scanner.Name() = %q, want %q", scanner.Name(), "codex")
	}
}

func TestOpenCodeScannerIntegration(t *testing.T) {
	scanner, ok := Get("opencode")
	if !ok {
		t.Fatal("opencode scanner not registered")
	}

	if scanner.Name() != "opencode" {
		t.Errorf("scanner.Name() = %q, want %q", scanner.Name(), "opencode")
	}
}

func TestCopilotScannerIntegration(t *testing.T) {
	scanner, ok := Get("copilot")
	if !ok {
		t.Fatal("copilot scanner not registered")
	}

	if scanner.Name() != "copilot" {
		t.Errorf("scanner.Name() = %q, want %q", scanner.Name(), "copilot")
	}
}

func TestTopLevelScannerIntegration(t *testing.T) {
	scanner, ok := Get("toplevel")
	if !ok {
		t.Fatal("toplevel scanner not registered")
	}

	if scanner.Name() != "toplevel" {
		t.Errorf("scanner.Name() = %q, want %q", scanner.Name(), "toplevel")
	}
}
