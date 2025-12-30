package skill

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Skill represents a multi-file skill/plugin configuration
type Skill struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Version     string            `json:"version,omitempty"`
	Author      string            `json:"author,omitempty"`
	Prompts     map[string]string `json:"prompts,omitempty"`  // Embedded prompts
	Files       []string          `json:"files,omitempty"`    // Additional files
	Path        string            `json:"-"`                  // Directory path (not serialized)
}

// Load loads a skill from a directory
func Load(dir string) (*Skill, error) {
	skillPath := filepath.Join(dir, "skill.json")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		return nil, err
	}

	var s Skill
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
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
			// Skip directories that don't have valid skill.json
			continue
		}
		skills = append(skills, s)
	}

	return skills, nil
}
