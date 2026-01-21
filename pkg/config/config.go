package config

import (
	"encoding/json"
	"fmt"
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
	Enabled   bool           `json:"enabled"`
	Overrides map[string]any `json:"overrides,omitempty"`
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
	Path        string `json:"-"` // Path to config file
	ConfigDir   string `json:"-"` // Config directory
	ProjectPath string `json:"-"` // Path to project config (if loaded from project)
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

	// Mark that we have a project config
	globalCfg.ProjectPath = projectPath

	// Merge: project overrides global
	merged := globalCfg.Merge(projectCfg)
	merged.ProjectPath = projectPath

	// Load local resources from .agentctl/ directory
	if err := merged.loadLocalResources(); err != nil {
		return nil, err
	}

	return merged, nil
}

// LoadWithProject loads global config merged with any project config in cwd
func LoadWithProject() (*Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		// Fall back to just global config
		return Load()
	}
	return LoadProjectConfig(cwd)
}

// FindProjectConfig walks up from dir looking for .agentctl.json
func FindProjectConfig(dir string) (string, bool) {
	for {
		configPath := filepath.Join(dir, ".agentctl.json")
		if _, err := os.Stat(configPath); err == nil {
			return configPath, true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			return "", false
		}
		dir = parent
	}
}

// HasProjectConfig checks if current directory has a project config
func HasProjectConfig() bool {
	cwd, err := os.Getwd()
	if err != nil {
		return false
	}
	_, found := FindProjectConfig(cwd)
	return found
}

// InitProjectConfig creates a .agentctl.json in the given directory
func InitProjectConfig(dir string) (*Config, error) {
	configPath := filepath.Join(dir, ".agentctl.json")

	// Check if already exists
	if _, err := os.Stat(configPath); err == nil {
		return nil, fmt.Errorf("project config already exists at %s", configPath)
	}

	cfg := &Config{
		Version:     "1",
		Servers:     make(map[string]*mcp.Server),
		Path:        configPath,
		ConfigDir:   dir,
		ProjectPath: configPath,
	}

	if err := cfg.Save(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Merge merges another config into this one (other takes precedence)
// Servers from the base config are marked as "global", servers from other are marked as "local"
func (c *Config) Merge(other *Config) *Config {
	merged := &Config{
		Version:   c.Version,
		Servers:   make(map[string]*mcp.Server),
		ConfigDir: c.ConfigDir,
		Path:      c.Path,
		Settings:  c.Settings,
	}

	// Copy servers from base (global)
	for name, server := range c.Servers {
		serverCopy := *server
		if serverCopy.Scope == "" {
			serverCopy.Scope = string(ScopeGlobal)
		}
		merged.Servers[name] = &serverCopy
	}

	// Override/add servers from other (local)
	for name, server := range other.Servers {
		serverCopy := *server
		serverCopy.Scope = string(ScopeLocal)
		merged.Servers[name] = &serverCopy
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
	return c.loadResourcesWithScope(string(ScopeGlobal))
}

// loadResourcesWithScope loads all resources from the config directory and marks them with the given scope
func (c *Config) loadResourcesWithScope(scope string) error {
	var err error

	// Load commands
	c.LoadedCommands, err = command.LoadAll(filepath.Join(c.ConfigDir, "commands"))
	if err != nil {
		return err
	}
	// Mark scope
	for _, cmd := range c.LoadedCommands {
		cmd.Scope = scope
	}

	// Load rules
	c.LoadedRules, err = rule.LoadAll(filepath.Join(c.ConfigDir, "rules"))
	if err != nil {
		return err
	}
	// Mark scope
	for _, r := range c.LoadedRules {
		r.Scope = scope
	}

	// Load prompts
	c.LoadedPrompts, err = prompt.LoadAll(filepath.Join(c.ConfigDir, "prompts"))
	if err != nil {
		return err
	}
	// Mark scope
	for _, p := range c.LoadedPrompts {
		p.Scope = scope
	}

	// Load skills
	c.LoadedSkills, err = skill.LoadAll(filepath.Join(c.ConfigDir, "skills"))
	if err != nil {
		return err
	}
	// Mark scope
	for _, s := range c.LoadedSkills {
		s.Scope = scope
	}

	return nil
}

// loadLocalResources loads resources from the project-local .agentctl/ directory
func (c *Config) loadLocalResources() error {
	if c.ProjectPath == "" {
		return nil
	}

	projectDir := filepath.Dir(c.ProjectPath)
	localResourceDir := filepath.Join(projectDir, ".agentctl")

	// Load local commands
	localCommands, err := command.LoadAll(filepath.Join(localResourceDir, "commands"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, cmd := range localCommands {
		cmd.Scope = string(ScopeLocal)
	}
	c.LoadedCommands = append(c.LoadedCommands, localCommands...)

	// Load local rules
	localRules, err := rule.LoadAll(filepath.Join(localResourceDir, "rules"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, r := range localRules {
		r.Scope = string(ScopeLocal)
	}
	c.LoadedRules = append(c.LoadedRules, localRules...)

	// Load local prompts
	localPrompts, err := prompt.LoadAll(filepath.Join(localResourceDir, "prompts"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, p := range localPrompts {
		p.Scope = string(ScopeLocal)
	}
	c.LoadedPrompts = append(c.LoadedPrompts, localPrompts...)

	// Load local skills
	localSkills, err := skill.LoadAll(filepath.Join(localResourceDir, "skills"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, s := range localSkills {
		s.Scope = string(ScopeLocal)
	}
	c.LoadedSkills = append(c.LoadedSkills, localSkills...)

	return nil
}

// LocalResourceDir returns the local resource directory path for the project
func (c *Config) LocalResourceDir() string {
	if c.ProjectPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(c.ProjectPath), ".agentctl")
}

// ReloadResources reloads all resources from disk (global and local)
// This should be called after creating, editing, or deleting resources
func (c *Config) ReloadResources() error {
	// Reset loaded resources
	c.LoadedCommands = nil
	c.LoadedRules = nil
	c.LoadedPrompts = nil
	c.LoadedSkills = nil

	// Reload global resources
	if err := c.loadResourcesWithScope(string(ScopeGlobal)); err != nil {
		return err
	}

	// Reload local resources
	return c.loadLocalResources()
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

// CacheDir returns the cache directory for this config
func (c *Config) CacheDir() string {
	return DefaultCacheDir()
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

// LoadScoped loads configuration for a specific scope only
func LoadScoped(scope Scope) (*Config, error) {
	switch scope {
	case ScopeLocal:
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
		projectPath := filepath.Join(cwd, ".agentctl.json")
		if _, err := os.Stat(projectPath); os.IsNotExist(err) {
			// Create a new empty project config
			return &Config{
				Version:     "1",
				Servers:     make(map[string]*mcp.Server),
				Path:        projectPath,
				ConfigDir:   cwd,
				ProjectPath: projectPath,
			}, nil
		}
		cfg, err := LoadFrom(projectPath)
		if err != nil {
			return nil, err
		}
		cfg.ProjectPath = projectPath
		// Mark all servers as local scope
		for _, server := range cfg.Servers {
			server.Scope = string(ScopeLocal)
		}
		return cfg, nil

	case ScopeGlobal:
		cfg, err := Load()
		if err != nil {
			return nil, err
		}
		// Mark all servers as global scope
		for _, server := range cfg.Servers {
			server.Scope = string(ScopeGlobal)
		}
		return cfg, nil

	case ScopeAll:
		return LoadWithProject()

	default:
		return nil, fmt.Errorf("invalid scope: %s", scope)
	}
}

// SaveScoped saves the configuration to the appropriate location based on scope
func (c *Config) SaveScoped(scope Scope) error {
	switch scope {
	case ScopeLocal:
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
		projectPath := filepath.Join(cwd, ".agentctl.json")
		return c.SaveTo(projectPath)

	case ScopeGlobal:
		configDir := DefaultConfigDir()
		globalPath := filepath.Join(configDir, "agentctl.json")
		return c.SaveTo(globalPath)

	default:
		return fmt.Errorf("cannot save to scope %q (use local or global)", scope)
	}
}

// ServersForScope returns servers that belong to a specific scope
func (c *Config) ServersForScope(scope Scope) []*mcp.Server {
	var servers []*mcp.Server
	for _, server := range c.Servers {
		if server.Disabled {
			continue
		}
		switch scope {
		case ScopeLocal:
			if server.Scope == string(ScopeLocal) {
				servers = append(servers, server)
			}
		case ScopeGlobal:
			if server.Scope == string(ScopeGlobal) || server.Scope == "" {
				servers = append(servers, server)
			}
		case ScopeAll:
			servers = append(servers, server)
		}
	}
	return servers
}

// GetServerScope returns the scope of a specific server
func (c *Config) GetServerScope(name string) Scope {
	server, ok := c.Servers[name]
	if !ok {
		return ""
	}
	if server.Scope == string(ScopeLocal) {
		return ScopeLocal
	}
	return ScopeGlobal
}

// RulesForScope returns rules that belong to a specific scope
func (c *Config) RulesForScope(scope Scope) []*rule.Rule {
	var rules []*rule.Rule
	for _, r := range c.LoadedRules {
		switch scope {
		case ScopeLocal:
			if r.Scope == string(ScopeLocal) {
				rules = append(rules, r)
			}
		case ScopeGlobal:
			if r.Scope == string(ScopeGlobal) || r.Scope == "" {
				rules = append(rules, r)
			}
		case ScopeAll:
			rules = append(rules, r)
		}
	}
	return rules
}

// CommandsForScope returns commands that belong to a specific scope
func (c *Config) CommandsForScope(scope Scope) []*command.Command {
	var commands []*command.Command
	for _, cmd := range c.LoadedCommands {
		switch scope {
		case ScopeLocal:
			if cmd.Scope == string(ScopeLocal) {
				commands = append(commands, cmd)
			}
		case ScopeGlobal:
			if cmd.Scope == string(ScopeGlobal) || cmd.Scope == "" {
				commands = append(commands, cmd)
			}
		case ScopeAll:
			commands = append(commands, cmd)
		}
	}
	return commands
}

// PromptsForScope returns prompts that belong to a specific scope
func (c *Config) PromptsForScope(scope Scope) []*prompt.Prompt {
	var prompts []*prompt.Prompt
	for _, p := range c.LoadedPrompts {
		switch scope {
		case ScopeLocal:
			if p.Scope == string(ScopeLocal) {
				prompts = append(prompts, p)
			}
		case ScopeGlobal:
			if p.Scope == string(ScopeGlobal) || p.Scope == "" {
				prompts = append(prompts, p)
			}
		case ScopeAll:
			prompts = append(prompts, p)
		}
	}
	return prompts
}

// SkillsForScope returns skills that belong to a specific scope
func (c *Config) SkillsForScope(scope Scope) []*skill.Skill {
	var skills []*skill.Skill
	for _, s := range c.LoadedSkills {
		switch scope {
		case ScopeLocal:
			if s.Scope == string(ScopeLocal) {
				skills = append(skills, s)
			}
		case ScopeGlobal:
			if s.Scope == string(ScopeGlobal) || s.Scope == "" {
				skills = append(skills, s)
			}
		case ScopeAll:
			skills = append(skills, s)
		}
	}
	return skills
}

// ProjectDir returns the project directory if a project config is loaded
func (c *Config) ProjectDir() string {
	if c.ProjectPath == "" {
		return ""
	}
	return filepath.Dir(c.ProjectPath)
}
