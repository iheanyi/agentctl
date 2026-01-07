package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Hook represents a configured hook
type Hook struct {
	Type    string   `json:"type"`    // PreToolUse, PostToolUse, Notification, Stop, UserPromptSubmit
	Matcher string   `json:"matcher"` // Tool matcher (e.g., "Bash", "Edit", "*")
	Command string   `json:"command"` // Shell command to run
	Source  string   `json:"-"`       // Which tool this hook came from (e.g., "claude")
}

// ClaudeHookEntry represents a single hook entry in Claude Code's settings
type ClaudeHookEntry struct {
	Matcher string   `json:"matcher"`
	Hooks   []string `json:"hooks"`
}

// LoadFromClaudeSettings loads hooks from Claude Code's settings.json
func LoadFromClaudeSettings() ([]*Hook, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var settings struct {
		Hooks map[string][]ClaudeHookEntry `json:"hooks"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}

	var hooks []*Hook
	for hookType, entries := range settings.Hooks {
		for _, entry := range entries {
			for _, cmd := range entry.Hooks {
				hooks = append(hooks, &Hook{
					Type:    hookType,
					Matcher: entry.Matcher,
					Command: cmd,
					Source:  "claude",
				})
			}
		}
	}

	return hooks, nil
}

// LoadAll loads hooks from all supported tools
func LoadAll() ([]*Hook, error) {
	var allHooks []*Hook

	// Load from Claude Code
	claudeHooks, err := LoadFromClaudeSettings()
	if err == nil && len(claudeHooks) > 0 {
		allHooks = append(allHooks, claudeHooks...)
	}

	// Future: Load from other tools that support hooks

	return allHooks, nil
}
