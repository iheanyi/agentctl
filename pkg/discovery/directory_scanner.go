package discovery

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/iheanyi/agentctl/pkg/agent"
	"github.com/iheanyi/agentctl/pkg/command"
	"github.com/iheanyi/agentctl/pkg/hook"
	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/iheanyi/agentctl/pkg/rule"
	"github.com/iheanyi/agentctl/pkg/skill"
)

// ScannerConfig defines the configuration for a DirectoryScanner.
// WARNING: Keep this struct frozen. The moment you add ValidationFunc,
// CustomDetectors, or IgnorePatterns, consider whether 5 simple files
// would be clearer. Config-driven is good; a DSL is not.
type ScannerConfig struct {
	Name         string   // Scanner/tool name (e.g., "cursor")
	LocalDirs    []string // Tool directories relative to project (e.g., [".cursor"])
	GlobalDirs   []string // Global config directories (e.g., ["~/.cursor"])
	DetectFiles  []string // Files that indicate tool presence (e.g., [".cursorrules"])
	RulesDirs    []string // Subdirs containing rules (e.g., ["rules"])
	SkillsDirs   []string // Subdirs containing skills
	CommandsDirs []string // Subdirs containing commands
	AgentsDirs   []string // Subdirs containing agents (e.g., ["agents"])
	FileExts     []string // Allowed file extensions (default: [".md", ".mdc"])
}

// DirectoryScanner is a config-driven scanner for discovering resources
// from tool-native directories.
type DirectoryScanner struct {
	cfg ScannerConfig
}

// NewDirectoryScanner creates a new DirectoryScanner with the given config.
func NewDirectoryScanner(cfg ScannerConfig) *DirectoryScanner {
	// Set default file extensions if not specified
	if len(cfg.FileExts) == 0 {
		cfg.FileExts = []string{".md", ".mdc"}
	}
	return &DirectoryScanner{cfg: cfg}
}

func (s *DirectoryScanner) Name() string {
	return s.cfg.Name
}

func (s *DirectoryScanner) Detect(dir string) bool {
	// Check for tool directories
	for _, localDir := range s.cfg.LocalDirs {
		toolDir := filepath.Join(dir, localDir)
		if info, err := os.Stat(toolDir); err == nil && info.IsDir() {
			return true
		}
	}

	// Check for detect files
	for _, detectFile := range s.cfg.DetectFiles {
		filePath := filepath.Join(dir, detectFile)
		if _, err := os.Stat(filePath); err == nil {
			return true
		}
	}

	return false
}

func (s *DirectoryScanner) ScanRules(dir string) ([]*rule.Rule, error) {
	var rules []*rule.Rule
	var warnings []error

	// Scan rules from each configured directory
	for _, localDir := range s.cfg.LocalDirs {
		for _, rulesDir := range s.cfg.RulesDirs {
			fullPath := filepath.Join(dir, localDir, rulesDir)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				continue
			}

			loadedRules, ruleWarnings := loadRulesWithWarnings(fullPath)
			warnings = append(warnings, ruleWarnings...)

			for _, r := range loadedRules {
				r.Scope = "local"
				r.Tool = s.cfg.Name
			}
			rules = append(rules, loadedRules...)
		}
	}

	// Also check for standalone detect files as rules (e.g., .cursorrules)
	for _, detectFile := range s.cfg.DetectFiles {
		filePath := filepath.Join(dir, detectFile)
		if _, err := os.Stat(filePath); err == nil {
			// Load markdown-like files and extensionless dotfiles (e.g. ".cursorrules").
			if s.hasAllowedExtension(detectFile) || isExtensionlessDotfile(detectFile) {
				// For files like .cursorrules (no extension), try to load as rule
				r, err := rule.Load(filePath)
				if err == nil {
					if r.Name == "" {
						r.Name = fallbackResourceName(filePath)
					}
					r.Scope = "local"
					r.Tool = s.cfg.Name
					rules = append(rules, r)
				} else {
					warnings = append(warnings, fmt.Errorf("%s: %w", filePath, err))
				}
			}
		}
	}

	return rules, joinScanWarnings("rules", s.cfg.Name, warnings)
}

// ScanGlobalRules discovers rules from the tool's global config directories
func (s *DirectoryScanner) ScanGlobalRules() ([]*rule.Rule, error) {
	var rules []*rule.Rule
	var warnings []error

	for _, globalDir := range s.cfg.GlobalDirs {
		expandedDir := expandHomeDir(globalDir)

		// Scan rules from each configured directory
		for _, rulesDir := range s.cfg.RulesDirs {
			fullPath := filepath.Join(expandedDir, rulesDir)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				continue
			}

			loadedRules, ruleWarnings := loadRulesWithWarnings(fullPath)
			warnings = append(warnings, ruleWarnings...)

			for _, r := range loadedRules {
				r.Scope = "global"
				r.Tool = s.cfg.Name
			}
			rules = append(rules, loadedRules...)
		}

		// Also check for standalone detect files as rules
		for _, detectFile := range s.cfg.DetectFiles {
			filePath := filepath.Join(expandedDir, detectFile)
			if _, err := os.Stat(filePath); err == nil {
				if s.hasAllowedExtension(detectFile) || isExtensionlessDotfile(detectFile) {
					r, err := rule.Load(filePath)
					if err == nil {
						if r.Name == "" {
							r.Name = fallbackResourceName(filePath)
						}
						r.Scope = "global"
						r.Tool = s.cfg.Name
						rules = append(rules, r)
					} else {
						warnings = append(warnings, fmt.Errorf("%s: %w", filePath, err))
					}
				}
			}
		}
	}

	return rules, joinScanWarnings("global rules", s.cfg.Name, warnings)
}

func (s *DirectoryScanner) ScanSkills(dir string) ([]*skill.Skill, error) {
	var skills []*skill.Skill
	var warnings []error

	for _, localDir := range s.cfg.LocalDirs {
		for _, skillsDir := range s.cfg.SkillsDirs {
			fullPath := filepath.Join(dir, localDir, skillsDir)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				continue
			}

			loadedSkills, err := skill.LoadAll(fullPath)
			if err != nil {
				warnings = append(warnings, fmt.Errorf("%s: %w", fullPath, err))
				continue
			}

			for _, sk := range loadedSkills {
				sk.Scope = "local"
				sk.Tool = s.cfg.Name
			}
			skills = append(skills, loadedSkills...)
		}
	}

	return skills, joinScanWarnings("skills", s.cfg.Name, warnings)
}

// ScanGlobalSkills discovers skills from the tool's global config directories
func (s *DirectoryScanner) ScanGlobalSkills() ([]*skill.Skill, error) {
	var skills []*skill.Skill
	var warnings []error

	for _, globalDir := range s.cfg.GlobalDirs {
		expandedDir := expandHomeDir(globalDir)
		for _, skillsDir := range s.cfg.SkillsDirs {
			fullPath := filepath.Join(expandedDir, skillsDir)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				continue
			}

			loadedSkills, err := skill.LoadAll(fullPath)
			if err != nil {
				warnings = append(warnings, fmt.Errorf("%s: %w", fullPath, err))
				continue
			}

			for _, sk := range loadedSkills {
				sk.Scope = "global"
				sk.Tool = s.cfg.Name
			}
			skills = append(skills, loadedSkills...)
		}
	}

	return skills, joinScanWarnings("global skills", s.cfg.Name, warnings)
}

func (s *DirectoryScanner) ScanHooks(dir string) ([]*hook.Hook, error) {
	// DirectoryScanner doesn't scan hooks by default - hooks are typically
	// in settings files which are tool-specific
	return nil, nil
}

// ScanGlobalHooks discovers hooks from global settings (not supported for DirectoryScanner)
func (s *DirectoryScanner) ScanGlobalHooks() ([]*hook.Hook, error) {
	return nil, nil
}

func (s *DirectoryScanner) ScanCommands(dir string) ([]*command.Command, error) {
	var commands []*command.Command
	var warnings []error

	for _, localDir := range s.cfg.LocalDirs {
		for _, commandsDir := range s.cfg.CommandsDirs {
			fullPath := filepath.Join(dir, localDir, commandsDir)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				continue
			}

			loadedCommands, err := command.LoadAll(fullPath)
			if err != nil {
				warnings = append(warnings, fmt.Errorf("%s: %w", fullPath, err))
				continue
			}

			for _, c := range loadedCommands {
				c.Scope = "local"
				c.Tool = s.cfg.Name
			}
			commands = append(commands, loadedCommands...)
		}
	}

	return commands, joinScanWarnings("commands", s.cfg.Name, warnings)
}

// ScanGlobalCommands discovers commands from the tool's global config directories
func (s *DirectoryScanner) ScanGlobalCommands() ([]*command.Command, error) {
	var commands []*command.Command
	var warnings []error

	for _, globalDir := range s.cfg.GlobalDirs {
		expandedDir := expandHomeDir(globalDir)
		for _, commandsDir := range s.cfg.CommandsDirs {
			fullPath := filepath.Join(expandedDir, commandsDir)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				continue
			}

			loadedCommands, err := command.LoadAll(fullPath)
			if err != nil {
				warnings = append(warnings, fmt.Errorf("%s: %w", fullPath, err))
				continue
			}

			for _, c := range loadedCommands {
				c.Scope = "global"
				c.Tool = s.cfg.Name
			}
			commands = append(commands, loadedCommands...)
		}
	}

	return commands, joinScanWarnings("global commands", s.cfg.Name, warnings)
}

func (s *DirectoryScanner) ScanServers(dir string) ([]*mcp.Server, error) {
	// DirectoryScanner doesn't scan servers by default - servers are typically
	// in JSON config files which are tool-specific
	return nil, nil
}

// ScanAgents discovers agents from the tool's local agents directory
func (s *DirectoryScanner) ScanAgents(dir string) ([]*agent.Agent, error) {
	var agents []*agent.Agent
	var warnings []error

	for _, localDir := range s.cfg.LocalDirs {
		for _, agentsDir := range s.cfg.AgentsDirs {
			fullPath := filepath.Join(dir, localDir, agentsDir)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				continue
			}

			loadedAgents, err := agent.LoadFromDirectory(fullPath, "local", s.cfg.Name)
			if err != nil {
				warnings = append(warnings, fmt.Errorf("%s: %w", fullPath, err))
				continue
			}

			agents = append(agents, loadedAgents...)
		}
	}

	return agents, joinScanWarnings("agents", s.cfg.Name, warnings)
}

// ScanGlobalAgents discovers agents from the tool's global agents directory
func (s *DirectoryScanner) ScanGlobalAgents() ([]*agent.Agent, error) {
	var agents []*agent.Agent
	var warnings []error

	for _, globalDir := range s.cfg.GlobalDirs {
		expandedDir := expandHomeDir(globalDir)
		for _, agentsDir := range s.cfg.AgentsDirs {
			fullPath := filepath.Join(expandedDir, agentsDir)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				continue
			}

			loadedAgents, err := agent.LoadFromDirectory(fullPath, "global", s.cfg.Name)
			if err != nil {
				warnings = append(warnings, fmt.Errorf("%s: %w", fullPath, err))
				continue
			}

			agents = append(agents, loadedAgents...)
		}
	}

	return agents, joinScanWarnings("global agents", s.cfg.Name, warnings)
}

// hasAllowedExtension checks if the filename has an allowed extension
func (s *DirectoryScanner) hasAllowedExtension(filename string) bool {
	for _, ext := range s.cfg.FileExts {
		if strings.HasSuffix(strings.ToLower(filename), ext) {
			return true
		}
	}
	return false
}

// expandHomeDir expands ~ to the user's home directory
func expandHomeDir(path string) string {
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(homeDir, path[2:])
	}
	return path
}

func isExtensionlessDotfile(path string) bool {
	base := filepath.Base(path)
	return strings.HasPrefix(base, ".") && !strings.Contains(base[1:], ".")
}

func loadRulesWithWarnings(dir string) ([]*rule.Rule, []error) {
	var (
		rules    []*rule.Rule
		warnings []error
	)

	walkErr := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			warnings = append(warnings, fmt.Errorf("%s: %w", path, err))
			return nil
		}
		if d.IsDir() {
			return nil
		}

		name := d.Name()
		lowerName := strings.ToLower(name)
		if !strings.HasSuffix(lowerName, ".md") && !strings.HasSuffix(lowerName, ".mdc") {
			return nil
		}

		r, err := rule.Load(path)
		if err != nil {
			warnings = append(warnings, fmt.Errorf("%s: %w", path, err))
			return nil
		}
		if r.Name == "" {
			r.Name = fallbackResourceName(path)
		}
		rules = append(rules, r)
		return nil
	})
	if walkErr != nil {
		warnings = append(warnings, fmt.Errorf("%s: %w", dir, walkErr))
	}

	return rules, warnings
}

func fallbackResourceName(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	trimmed := strings.TrimSuffix(base, ext)
	if trimmed == "" {
		return base
	}
	return trimmed
}

func joinScanWarnings(resourceType, scannerName string, warnings []error) error {
	if len(warnings) == 0 {
		return nil
	}
	return fmt.Errorf("%s scanner %q had %d %s warning(s): %w",
		"discovery",
		scannerName,
		len(warnings),
		resourceType,
		errors.Join(warnings...),
	)
}
