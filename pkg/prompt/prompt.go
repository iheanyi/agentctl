package prompt

import (
	"encoding/json"
	"os"
	"strings"
)

// Prompt represents a reusable prompt template
type Prompt struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Template    string   `json:"template"`
	Variables   []string `json:"variables,omitempty"` // Expected template variables
}

// Load loads a prompt from a JSON file
func Load(path string) (*Prompt, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var p Prompt
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}

	return &p, nil
}

// LoadAll loads all prompts from a directory
func LoadAll(dir string) ([]*Prompt, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var prompts []*Prompt
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		p, err := Load(dir + "/" + entry.Name())
		if err != nil {
			return nil, err
		}
		prompts = append(prompts, p)
	}

	return prompts, nil
}

// Render renders a prompt template with the given variables
func (p *Prompt) Render(vars map[string]string) string {
	result := p.Template
	for k, v := range vars {
		result = strings.ReplaceAll(result, "{{"+k+"}}", v)
	}
	return result
}
