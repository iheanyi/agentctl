package rule

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRuleWithFrontmatter(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "rule-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a rule file with frontmatter
	content := `---
priority: 1
tools: ["claude", "cursor"]
applies: "*.ts"
---

# TypeScript Rules

Always use strict TypeScript.
No any types allowed.`

	rulePath := filepath.Join(tmpDir, "typescript.md")
	if err := os.WriteFile(rulePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write rule file: %v", err)
	}

	// Load the rule
	rule, err := Load(rulePath)
	if err != nil {
		t.Fatalf("Failed to load rule: %v", err)
	}

	// Verify frontmatter
	if rule.Frontmatter == nil {
		t.Fatal("Frontmatter should not be nil")
	}
	if rule.Frontmatter.Priority != 1 {
		t.Errorf("Priority mismatch: got %d, want 1", rule.Frontmatter.Priority)
	}
	if len(rule.Frontmatter.Tools) != 2 {
		t.Errorf("Tools length mismatch: got %d, want 2", len(rule.Frontmatter.Tools))
	}
	if rule.Frontmatter.Applies != "*.ts" {
		t.Errorf("Applies mismatch: got %q, want %q", rule.Frontmatter.Applies, "*.ts")
	}

	// Verify content (should not include frontmatter)
	if rule.Content == "" {
		t.Error("Content should not be empty")
	}
	if rule.Content[0:2] == "---" {
		t.Error("Content should not start with frontmatter delimiter")
	}

	// Verify name extracted from filename
	if rule.Name != "typescript" {
		t.Errorf("Name mismatch: got %q, want %q", rule.Name, "typescript")
	}
}

func TestLoadRuleWithoutFrontmatter(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rule-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	content := `# Simple Rules

Just some plain markdown content.
No frontmatter here.`

	rulePath := filepath.Join(tmpDir, "simple.md")
	if err := os.WriteFile(rulePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write rule file: %v", err)
	}

	rule, err := Load(rulePath)
	if err != nil {
		t.Fatalf("Failed to load rule: %v", err)
	}

	// Frontmatter should be nil for files without it
	if rule.Frontmatter != nil {
		t.Error("Frontmatter should be nil for files without it")
	}

	// Content should be the entire file
	if rule.Content != content {
		t.Errorf("Content mismatch: got %q, want %q", rule.Content, content)
	}
}

func TestLoadAllRules(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rules-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create multiple rule files
	rules := map[string]string{
		"rule1.md":       "# Rule 1\nContent 1",
		"rule2.md":       "# Rule 2\nContent 2",
		"rule3.md":       "# Rule 3\nContent 3",
		"not-a-rule.txt": "This should be ignored",
	}

	for name, content := range rules {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", name, err)
		}
	}

	// Load all rules
	loaded, err := LoadAll(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load all rules: %v", err)
	}

	// Should only load .md files
	if len(loaded) != 3 {
		t.Errorf("Expected 3 rules, got %d", len(loaded))
	}
}

func TestLoadAllRulesNonexistentDir(t *testing.T) {
	rules, err := LoadAll("/nonexistent/directory")
	if err != nil {
		t.Errorf("LoadAll should return nil error for nonexistent dir, got: %v", err)
	}
	if rules != nil {
		t.Errorf("LoadAll should return nil for nonexistent dir, got: %v", rules)
	}
}

func TestLoadRuleInvalidPath(t *testing.T) {
	_, err := Load("/nonexistent/rule.md")
	if err == nil {
		t.Error("Load should return error for nonexistent file")
	}
}

func TestFrontmatterParsing(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantPrio int
		wantErr  bool
	}{
		{
			name: "valid frontmatter",
			content: `---
priority: 5
---
Content`,
			wantPrio: 5,
		},
		{
			name: "empty frontmatter",
			content: `---
---
Content`,
			wantPrio: 0,
		},
		{
			name:     "no frontmatter",
			content:  "Just content",
			wantPrio: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "frontmatter-test")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			rulePath := filepath.Join(tmpDir, "test.md")
			if err := os.WriteFile(rulePath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write file: %v", err)
			}

			rule, err := Load(rulePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if rule.Frontmatter != nil && rule.Frontmatter.Priority != tt.wantPrio {
				t.Errorf("Priority = %d, want %d", rule.Frontmatter.Priority, tt.wantPrio)
			}
		})
	}
}
