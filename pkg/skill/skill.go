package skill

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// SkillFileName is the standard skill file name (Claude Code format)
	SkillFileName = "SKILL.md"
	// LegacySkillFileName is the legacy JSON format
	LegacySkillFileName = "skill.json"
)

// Skill represents a skill/plugin configuration
// Skills use SKILL.md format with YAML frontmatter (matching Claude Code)
type Skill struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// Content is the markdown prompt content (after frontmatter)
	Content string `yaml:"-" json:"-"`

	// Path is the directory containing this skill
	Path string `yaml:"-" json:"-"`

	// Scope indicates where this skill came from ("local" or "global")
	Scope string `yaml:"-" json:"-"`

	// Legacy fields (for backwards compatibility with skill.json)
	Version string            `yaml:"version,omitempty" json:"version,omitempty"`
	Author  string            `yaml:"author,omitempty" json:"author,omitempty"`
	Prompts map[string]string `yaml:"prompts,omitempty" json:"prompts,omitempty"`
	Files   []string          `yaml:"files,omitempty" json:"files,omitempty"`
}

// Load loads a skill from a directory
// It first tries SKILL.md (Claude Code format), then falls back to skill.json
func Load(dir string) (*Skill, error) {
	// Try SKILL.md first (Claude Code format)
	skillMdPath := filepath.Join(dir, SkillFileName)
	if data, err := os.ReadFile(skillMdPath); err == nil {
		s, err := parseSkillMd(data)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", SkillFileName, err)
		}
		s.Path = dir
		// Use directory name as fallback for name
		if s.Name == "" {
			s.Name = filepath.Base(dir)
		}
		return s, nil
	}

	// Fall back to legacy skill.json
	skillJsonPath := filepath.Join(dir, LegacySkillFileName)
	data, err := os.ReadFile(skillJsonPath)
	if err != nil {
		return nil, fmt.Errorf("no %s or %s found in %s", SkillFileName, LegacySkillFileName, dir)
	}

	var s Skill
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", LegacySkillFileName, err)
	}
	s.Path = dir

	// Load embedded prompts from prompts/ directory if they exist
	promptsDir := filepath.Join(dir, "prompts")
	if entries, err := os.ReadDir(promptsDir); err == nil {
		if s.Prompts == nil {
			s.Prompts = make(map[string]string)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			content, err := os.ReadFile(filepath.Join(promptsDir, entry.Name()))
			if err != nil {
				continue
			}
			// Use filename without extension as key
			name := entry.Name()
			ext := filepath.Ext(name)
			key := name[:len(name)-len(ext)]
			s.Prompts[key] = string(content)
		}
	}

	return &s, nil
}

// parseSkillMd parses a SKILL.md file with YAML frontmatter
func parseSkillMd(data []byte) (*Skill, error) {
	frontmatter, content, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}

	var s Skill
	if len(frontmatter) > 0 {
		if err := yaml.Unmarshal(frontmatter, &s); err != nil {
			return nil, fmt.Errorf("invalid YAML frontmatter: %w", err)
		}
	}

	s.Content = strings.TrimSpace(content)
	return &s, nil
}

// splitFrontmatter splits YAML frontmatter from markdown content
// Frontmatter is delimited by --- at the start and end
func splitFrontmatter(data []byte) (frontmatter []byte, content string, err error) {
	const delimiter = "---"

	text := string(data)
	text = strings.TrimLeft(text, "\n\r\t ")

	if !strings.HasPrefix(text, delimiter) {
		// No frontmatter, entire file is content
		return nil, text, nil
	}

	// Skip the opening delimiter
	text = text[len(delimiter):]

	// Find closing delimiter
	endIdx := strings.Index(text, "\n"+delimiter)
	if endIdx == -1 {
		return nil, "", fmt.Errorf("unclosed frontmatter: missing closing ---")
	}

	frontmatter = []byte(strings.TrimSpace(text[:endIdx]))
	content = strings.TrimSpace(text[endIdx+len("\n"+delimiter):])

	return frontmatter, content, nil
}

// LoadAll loads all skills from a directory
func LoadAll(dir string) ([]*Skill, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var skills []*Skill
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		s, err := Load(filepath.Join(dir, entry.Name()))
		if err != nil {
			// Skip directories that don't have valid skill files
			continue
		}
		skills = append(skills, s)
	}

	return skills, nil
}

// Save saves a skill to a directory in SKILL.md format
func (s *Skill) Save(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating skill directory: %w", err)
	}

	content := s.ToMarkdown()
	skillPath := filepath.Join(dir, SkillFileName)

	if err := os.WriteFile(skillPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", SkillFileName, err)
	}

	s.Path = dir
	return nil
}

// ToMarkdown converts the skill to SKILL.md format
func (s *Skill) ToMarkdown() string {
	var buf bytes.Buffer

	// Write frontmatter
	buf.WriteString("---\n")
	buf.WriteString(fmt.Sprintf("name: %s\n", s.Name))
	if s.Description != "" {
		buf.WriteString(fmt.Sprintf("description: %s\n", s.Description))
	}
	buf.WriteString("---\n\n")

	// Write content
	if s.Content != "" {
		buf.WriteString(s.Content)
	} else {
		// Default template content - capitalize first letter
		title := s.Name
		if len(title) > 0 {
			title = strings.ToUpper(title[:1]) + title[1:]
		}
		buf.WriteString(fmt.Sprintf("# %s\n\n", title))
		buf.WriteString("TODO: Write your skill prompt here.\n\n")
		buf.WriteString("Use $ARGUMENTS to reference user input when they invoke this skill.\n")
	}

	return buf.String()
}

// Validate checks if the skill has required fields
func (s *Skill) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("skill name is required")
	}
	if s.Description == "" {
		return fmt.Errorf("skill description is required")
	}
	return nil
}

// SkillsDir returns the global skills directory path
func SkillsDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".claude", "skills"), nil
}

// GlobalSkills loads all skills from the global skills directory
func GlobalSkills() ([]*Skill, error) {
	dir, err := SkillsDir()
	if err != nil {
		return nil, err
	}

	skills, err := LoadAll(dir)
	if err != nil {
		return nil, err
	}

	for _, s := range skills {
		s.Scope = "global"
	}

	return skills, nil
}

// ProjectSkills loads all skills from a project's .claude/skills directory
func ProjectSkills(projectDir string) ([]*Skill, error) {
	dir := filepath.Join(projectDir, ".claude", "skills")

	skills, err := LoadAll(dir)
	if err != nil {
		return nil, err
	}

	for _, s := range skills {
		s.Scope = "local"
	}

	return skills, nil
}
