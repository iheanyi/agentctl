package discovery

import (
	"os"
	"path/filepath"

	"github.com/iheanyi/agentctl/pkg/command"
	"github.com/iheanyi/agentctl/pkg/hook"
	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/iheanyi/agentctl/pkg/rule"
	"github.com/iheanyi/agentctl/pkg/skill"
)

// GeminiScanner discovers resources from Gemini CLI's .gemini/ directory
type GeminiScanner struct{}

func init() {
	Register(&GeminiScanner{})
}

func (s *GeminiScanner) Name() string {
	return "gemini"
}

func (s *GeminiScanner) Detect(dir string) bool {
	geminiDir := filepath.Join(dir, ".gemini")
	if _, err := os.Stat(geminiDir); err == nil {
		return true
	}
	return false
}

func (s *GeminiScanner) ScanRules(dir string) ([]*rule.Rule, error) {
	rulesDir := filepath.Join(dir, ".gemini", "rules")
	if _, err := os.Stat(rulesDir); os.IsNotExist(err) {
		return nil, nil
	}

	rules, err := rule.LoadAll(rulesDir)
	if err != nil {
		return nil, err
	}

	// Mark all as local scope and set tool
	for _, r := range rules {
		r.Scope = "local"
		r.Tool = "gemini"
	}

	return rules, nil
}

func (s *GeminiScanner) ScanSkills(dir string) ([]*skill.Skill, error) {
	// Gemini doesn't have skills in the same way as Claude
	return nil, nil
}

func (s *GeminiScanner) ScanHooks(dir string) ([]*hook.Hook, error) {
	return hook.LoadFromGeminiProjectSettings(dir)
}

func (s *GeminiScanner) ScanCommands(dir string) ([]*command.Command, error) {
	// Gemini doesn't have commands directory
	return nil, nil
}

func (s *GeminiScanner) ScanServers(dir string) ([]*mcp.Server, error) {
	// TODO: Parse .gemini/settings.json for MCP servers
	return nil, nil
}

// ScanGlobalRules discovers rules from Gemini's global config directory
func (s *GeminiScanner) ScanGlobalRules() ([]*rule.Rule, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	rulesDir := filepath.Join(homeDir, ".gemini", "rules")
	if _, err := os.Stat(rulesDir); os.IsNotExist(err) {
		return nil, nil
	}

	rules, err := rule.LoadAll(rulesDir)
	if err != nil {
		return nil, err
	}

	// Mark all as global scope and set tool
	for _, r := range rules {
		r.Scope = "global"
		r.Tool = "gemini"
	}

	return rules, nil
}

// ScanGlobalSkills discovers skills from Gemini's global config directory
func (s *GeminiScanner) ScanGlobalSkills() ([]*skill.Skill, error) {
	// Gemini doesn't have skills
	return nil, nil
}

// ScanGlobalCommands discovers commands from Gemini's global config directory
func (s *GeminiScanner) ScanGlobalCommands() ([]*command.Command, error) {
	// Gemini doesn't have commands
	return nil, nil
}

// ScanGlobalHooks discovers hooks from Gemini's global settings
func (s *GeminiScanner) ScanGlobalHooks() ([]*hook.Hook, error) {
	return hook.LoadFromGeminiSettings()
}
