package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Profile represents a configuration profile
type Profile struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Servers     []string `json:"servers,omitempty"`  // Server names to include
	Commands    []string `json:"commands,omitempty"` // Command names to include
	Rules       []string `json:"rules,omitempty"`    // Rule names to include
	Prompts     []string `json:"prompts,omitempty"`  // Prompt names to include
	Skills      []string `json:"skills,omitempty"`   // Skill names to include
	Disabled    []string `json:"disabled,omitempty"` // Resources to disable
	Path        string   `json:"-"`                  // Path to profile file (not serialized)
}

// Load loads a profile from a JSON file
func Load(path string) (*Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}

	p.Path = path
	return &p, nil
}

// Save saves the profile to disk
func (p *Profile) Save() error {
	return p.SaveTo(p.Path)
}

// SaveTo saves the profile to a specific path
func (p *Profile) SaveTo(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// LoadAll loads all profiles from a directory
func LoadAll(dir string) ([]*Profile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var profiles []*Profile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		p, err := Load(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue // Skip invalid profiles
		}

		profiles = append(profiles, p)
	}

	return profiles, nil
}

// Exists checks if a profile with the given name exists
func Exists(dir, name string) bool {
	path := filepath.Join(dir, name+".json")
	_, err := os.Stat(path)
	return err == nil
}

// Create creates a new empty profile
func Create(dir, name string, description string) (*Profile, error) {
	p := &Profile{
		Name:        name,
		Description: description,
		Path:        filepath.Join(dir, name+".json"),
	}

	if err := p.Save(); err != nil {
		return nil, err
	}

	return p, nil
}

// Delete removes a profile
func Delete(dir, name string) error {
	path := filepath.Join(dir, name+".json")
	return os.Remove(path)
}
