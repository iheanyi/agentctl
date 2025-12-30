package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/iheanyi/agentctl/pkg/command"
	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/iheanyi/agentctl/pkg/prompt"
	"github.com/iheanyi/agentctl/pkg/rule"
	"github.com/iheanyi/agentctl/pkg/skill"
)

// AutoUpdateConfig configures automatic update behavior
type AutoUpdateConfig struct {
	Enabled  bool              `json:"enabled"`
	Interval string            `json:"interval,omitempty"` // e.g., "24h"
	Servers  map[string]string `json:"servers,omitempty"`  // "auto" or "notify" per server
}

// ToolConfig configures a specific tool
type ToolConfig struct {
	Enabled   bool                `json:"enabled"`
	Overrides map[string]any      `json:"overrides,omitempty"`
}

// Settings contains global settings
type Settings struct {
	DefaultProfile string                `json:"defaultProfile,omitempty"`
	AutoUpdate     AutoUpdateConfig      `json:"autoUpdate,omitempty"`
	Tools          map[string]ToolConfig `json:"tools,omitempty"`
}

// Config represents the main agentctl configuration
type Config struct {
	Version  string                 `json:"version"`
	Servers  map[string]*mcp.Server `json:"servers,omitempty"`
	Commands []string               `json:"commands,omitempty"` // Command names to include
	Rules    []string               `json:"rules,omitempty"`    // Rule names to include
	Prompts  []string               `json:"prompts,omitempty"`  // Prompt names to include
	Skills   []string               `json:"skills,omitempty"`   // Skill names to include
	Disabled []string               `json:"disabled,omitempty"` // Resources to disable
	Profile  string                 `json:"profile,omitempty"`  // Active profile (for project configs)
	Settings Settings               `json:"settings,omitempty"`

	// Loaded resources (not serialized)
	LoadedCommands []*command.Command `json:"-"`
	LoadedRules    []*rule.Rule       `json:"-"`
	LoadedPrompts  []*prompt.Prompt   `json:"-"`
	LoadedSkills   []*skill.Skill     `json:"-"`

	// Path info (not serialized)
	Path      string `json:"-"` // Path to config file
	ConfigDir string `json:"-"` // Config directory
}

// DefaultConfigDir returns the default configuration directory
func DefaultConfigDir() string {
	// Check AGENTCTL_HOME first
	if home := os.Getenv("AGENTCTL_HOME"); home != "" {
		return home
	}

	// Check XDG_CONFIG_HOME
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "agentctl")
	}

	// Default to ~/.config/agentctl
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".agentctl"
	}
	return filepath.Join(homeDir, ".config", "agentctl")
}

// DefaultCacheDir returns the default cache directory
func DefaultCacheDir() string {
	// Check XDG_CACHE_HOME
	if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
		return filepath.Join(xdgCache, "agentctl")
	}

	// Default to ~/.cache/agentctl
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".agentctl-cache"
	}
	return filepath.Join(homeDir, ".cache", "agentctl")
}

// Load loads the configuration from the default location
func Load() (*Config, error) {
	configDir := DefaultConfigDir()
	return LoadFrom(filepath.Join(configDir, "agentctl.json"))
}

// LoadFrom loads configuration from a specific path
func LoadFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if file doesn't exist
			return &Config{
				Version:   "1",
				Servers:   make(map[string]*mcp.Server),
				ConfigDir: filepath.Dir(path),
				Path:      path,
				Settings: Settings{
					Tools: defaultToolSettings(),
				},
			}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	cfg.Path = path
	cfg.ConfigDir = filepath.Dir(path)

	// Load resources from directories
	if err := cfg.loadResources(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// LoadProjectConfig loads a project-local configuration and merges with global
func LoadProjectConfig(projectDir string) (*Config, error) {
	// Load global config first
	globalCfg, err := Load()
	if err != nil {
		return nil, err
	}

	// Check for project config
	projectPath := filepath.Join(projectDir, ".agentctl.json")
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return globalCfg, nil
	}

	// Load project config
	projectCfg, err := LoadFrom(projectPath)
	if err != nil {
		return nil, err
	}

	// Merge: project overrides global
	merged := globalCfg.Merge(projectCfg)
	return merged, nil
}

// Merge merges another config into this one (other takes precedence)
func (c *Config) Merge(other *Config) *Config {
	merged := &Config{
		Version:   c.Version,
		Servers:   make(map[string]*mcp.Server),
		ConfigDir: c.ConfigDir,
		Path:      c.Path,
		Settings:  c.Settings,
	}

	// Copy servers from base
	for name, server := range c.Servers {
		merged.Servers[name] = server
	}

	// Override/add servers from other
	for name, server := range other.Servers {
		merged.Servers[name] = server
	}

	// Merge resource lists
	merged.Commands = mergeStringSlices(c.Commands, other.Commands)
	merged.Rules = mergeStringSlices(c.Rules, other.Rules)
	merged.Prompts = mergeStringSlices(c.Prompts, other.Prompts)
	merged.Skills = mergeStringSlices(c.Skills, other.Skills)

	// Apply disabled list from other
	for _, disabled := range other.Disabled {
		delete(merged.Servers, disabled)
		merged.Commands = removeFromSlice(merged.Commands, disabled)
		merged.Rules = removeFromSlice(merged.Rules, disabled)
		merged.Prompts = removeFromSlice(merged.Prompts, disabled)
		merged.Skills = removeFromSlice(merged.Skills, disabled)
	}

	// Use profile from other if specified
	if other.Profile != "" {
		merged.Profile = other.Profile
	}

	return merged
}

// Save saves the configuration to disk
func (c *Config) Save() error {
	return c.SaveTo(c.Path)
}

// SaveTo saves the configuration to a specific path
func (c *Config) SaveTo(path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// loadResources loads all resources from the config directory
func (c *Config) loadResources() error {
	var err error

	// Load commands
	c.LoadedCommands, err = command.LoadAll(filepath.Join(c.ConfigDir, "commands"))
	if err != nil {
		return err
	}

	// Load rules
	c.LoadedRules, err = rule.LoadAll(filepath.Join(c.ConfigDir, "rules"))
	if err != nil {
		return err
	}

	// Load prompts
	c.LoadedPrompts, err = prompt.LoadAll(filepath.Join(c.ConfigDir, "prompts"))
	if err != nil {
		return err
	}

	// Load skills
	c.LoadedSkills, err = skill.LoadAll(filepath.Join(c.ConfigDir, "skills"))
	if err != nil {
		return err
	}

	return nil
}

// ActiveServers returns the list of non-disabled servers
func (c *Config) ActiveServers() []*mcp.Server {
	var servers []*mcp.Server
	for _, server := range c.Servers {
		if !server.Disabled {
			servers = append(servers, server)
		}
	}
	return servers
}

// defaultToolSettings returns default tool configuration
func defaultToolSettings() map[string]ToolConfig {
	tools := []string{"claude", "cursor", "codex", "opencode", "cline", "windsurf", "zed", "continue"}
	settings := make(map[string]ToolConfig)
	for _, tool := range tools {
		settings[tool] = ToolConfig{Enabled: true}
	}
	return settings
}

// Helper functions
func mergeStringSlices(a, b []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range a {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	for _, s := range b {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

func removeFromSlice(slice []string, item string) []string {
	var result []string
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}
