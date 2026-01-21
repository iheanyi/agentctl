package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// InspectTitle returns the display name for the inspector modal header
func (h *Hook) InspectTitle() string {
	if h.Name != "" {
		return fmt.Sprintf("Hook: %s", h.Name)
	}
	return fmt.Sprintf("Hook: %s", h.Type)
}

// InspectContent returns the formatted content for the inspector viewport
func (h *Hook) InspectContent() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Source:  %s\n", h.Source))
	b.WriteString(fmt.Sprintf("Type:    %s\n", h.Type))
	if h.Matcher != "" && h.Matcher != "*" {
		b.WriteString(fmt.Sprintf("Matcher: %s\n", h.Matcher))
	}
	if h.Name != "" {
		b.WriteString(fmt.Sprintf("Name:    %s\n", h.Name))
	}
	b.WriteString("\n")

	b.WriteString("Command:\n")
	b.WriteString(h.Command)

	return b.String()
}

// Hook represents a configured hook
type Hook struct {
	Type    string `json:"type"`    // PreToolUse, PostToolUse, Notification, Stop, UserPromptSubmit, etc.
	Matcher string `json:"matcher"` // Tool matcher (e.g., "Bash", "Edit", "*")
	Command string `json:"command"` // Shell command to run
	Source  string `json:"-"`       // Which tool this hook came from (e.g., "claude", "claude-local", "gemini")
	Name    string `json:"name"`    // Optional hook name (Gemini)
}

// ClaudeHookEntry represents a single hook entry in Claude Code's settings
type ClaudeHookEntry struct {
	Matcher string `json:"matcher"`
	Hooks   []any  `json:"hooks"` // Can be strings or objects with command/type
}

// ClaudeHookCommand represents a hook command object
type ClaudeHookCommand struct {
	Command string `json:"command"`
	Type    string `json:"type"`
}

// GeminiHookEntry represents a single hook entry in Gemini CLI's settings
type GeminiHookEntry struct {
	Matcher string           `json:"matcher"`
	Hooks   []GeminiHookItem `json:"hooks"`
}

// GeminiHookItem represents a hook command in Gemini CLI format
type GeminiHookItem struct {
	Name        string `json:"name,omitempty"`
	Type        string `json:"type,omitempty"` // "command"
	Command     string `json:"command"`
	Description string `json:"description,omitempty"`
	Timeout     int    `json:"timeout,omitempty"`
}

// parseClaudeSettings parses Claude settings and extracts hooks
func parseClaudeSettings(data []byte, source string) ([]*Hook, error) {
	var settings struct {
		Hooks map[string][]ClaudeHookEntry `json:"hooks"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}

	var hooks []*Hook
	for hookType, entries := range settings.Hooks {
		for _, entry := range entries {
			for _, h := range entry.Hooks {
				var cmd string
				switch v := h.(type) {
				case string:
					cmd = v
				case map[string]any:
					if c, ok := v["command"].(string); ok {
						cmd = c
					}
				}
				if cmd != "" {
					// Generate a name from type and matcher
					name := hookType
					if entry.Matcher != "" && entry.Matcher != "*" {
						name = hookType + ":" + entry.Matcher
					}
					hooks = append(hooks, &Hook{
						Type:    hookType,
						Matcher: entry.Matcher,
						Command: cmd,
						Source:  source,
						Name:    name,
					})
				}
			}
		}
	}

	return hooks, nil
}

// LoadFromClaudeSettings loads hooks from Claude Code's global settings.json
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

	return parseClaudeSettings(data, "claude")
}

// LoadFromClaudeProjectSettings loads hooks from a project-local .claude/settings.json
func LoadFromClaudeProjectSettings(projectDir string) ([]*Hook, error) {
	if projectDir == "" {
		return nil, nil
	}

	settingsPath := filepath.Join(projectDir, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	return parseClaudeSettings(data, "claude-local")
}

// parseGeminiSettings parses Gemini settings and extracts hooks
func parseGeminiSettings(data []byte, source string) ([]*Hook, error) {
	var settings struct {
		Hooks struct {
			Enabled bool `json:"enabled"`
			// Dynamic hook event types (SessionStart, BeforeTool, AfterTool, etc.)
			// We need to handle this dynamically since event names are keys
		} `json:"hooks"`
	}

	// First, check if hooks are enabled
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}

	// Parse the raw hooks object to get event types dynamically
	var raw struct {
		Hooks map[string]json.RawMessage `json:"hooks"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	if raw.Hooks == nil {
		return nil, nil
	}

	var hooks []*Hook
	for eventType, eventData := range raw.Hooks {
		// Skip non-event keys
		if eventType == "enabled" || eventType == "disabled" {
			continue
		}

		// Parse the entries for this event type
		var entries []GeminiHookEntry
		if err := json.Unmarshal(eventData, &entries); err != nil {
			// Might be a different type, skip
			continue
		}

		for _, entry := range entries {
			for _, h := range entry.Hooks {
				if h.Command != "" {
					hooks = append(hooks, &Hook{
						Type:    eventType,
						Matcher: entry.Matcher,
						Command: h.Command,
						Name:    h.Name,
						Source:  source,
					})
				}
			}
		}
	}

	return hooks, nil
}

// geminiSettingsPath returns the path to Gemini's global settings.json
func geminiSettingsPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	switch runtime.GOOS {
	case "darwin", "linux":
		return filepath.Join(homeDir, ".gemini", "settings.json")
	case "windows":
		return filepath.Join(homeDir, ".gemini", "settings.json")
	default:
		return ""
	}
}

// LoadFromGeminiSettings loads hooks from Gemini CLI's global settings.json
func LoadFromGeminiSettings() ([]*Hook, error) {
	settingsPath := geminiSettingsPath()
	if settingsPath == "" {
		return nil, nil
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	return parseGeminiSettings(data, "gemini")
}

// LoadFromGeminiProjectSettings loads hooks from a project-local .gemini/settings.json
func LoadFromGeminiProjectSettings(projectDir string) ([]*Hook, error) {
	if projectDir == "" {
		return nil, nil
	}

	settingsPath := filepath.Join(projectDir, ".gemini", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	return parseGeminiSettings(data, "gemini-local")
}

// LoadAll loads hooks from all supported tools (global only)
func LoadAll() ([]*Hook, error) {
	return LoadAllWithProject("")
}

// LoadAllWithProject loads hooks from all supported tools, including project-local
func LoadAllWithProject(projectDir string) ([]*Hook, error) {
	var allHooks []*Hook

	// Load from Claude Code global settings
	claudeHooks, err := LoadFromClaudeSettings()
	if err == nil && len(claudeHooks) > 0 {
		allHooks = append(allHooks, claudeHooks...)
	}

	// Load from Claude Code project-local settings
	if projectDir != "" {
		localHooks, err := LoadFromClaudeProjectSettings(projectDir)
		if err == nil && len(localHooks) > 0 {
			allHooks = append(allHooks, localHooks...)
		}
	}

	// Load from Gemini CLI global settings
	geminiHooks, err := LoadFromGeminiSettings()
	if err == nil && len(geminiHooks) > 0 {
		allHooks = append(allHooks, geminiHooks...)
	}

	// Load from Gemini CLI project-local settings
	if projectDir != "" {
		localGeminiHooks, err := LoadFromGeminiProjectSettings(projectDir)
		if err == nil && len(localGeminiHooks) > 0 {
			allHooks = append(allHooks, localGeminiHooks...)
		}
	}

	return allHooks, nil
}
