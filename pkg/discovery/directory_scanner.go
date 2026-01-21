package discovery

import (
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

	// Scan rules from each configured directory
	for _, localDir := range s.cfg.LocalDirs {
		for _, rulesDir := range s.cfg.RulesDirs {
			fullPath := filepath.Join(dir, localDir, rulesDir)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				continue
			}

			loadedRules, err := rule.LoadAll(fullPath)
			if err != nil {
				continue // Log warning, continue
			}

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
			// Only load if it's a markdown-like file
			if s.hasAllowedExtension(detectFile) || !strings.Contains(detectFile, ".") {
				// For files like .cursorrules (no extension), try to load as rule
				r, err := rule.Load(filePath)
				if err == nil {
					r.Scope = "local"
					r.Tool = s.cfg.Name
					rules = append(rules, r)
				}
			}
		}
	}

	return rules, nil
}

func (s *DirectoryScanner) ScanSkills(dir string) ([]*skill.Skill, error) {
	var skills []*skill.Skill

	for _, localDir := range s.cfg.LocalDirs {
		for _, skillsDir := range s.cfg.SkillsDirs {
			fullPath := filepath.Join(dir, localDir, skillsDir)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				continue
			}

			loadedSkills, err := skill.LoadAll(fullPath)
			if err != nil {
				continue
			}

			for _, sk := range loadedSkills {
				sk.Scope = "local"
				sk.Tool = s.cfg.Name
			}
			skills = append(skills, loadedSkills...)
		}
	}

	return skills, nil
}

func (s *DirectoryScanner) ScanHooks(dir string) ([]*hook.Hook, error) {
	// DirectoryScanner doesn't scan hooks by default - hooks are typically
	// in settings files which are tool-specific
	return nil, nil
}

func (s *DirectoryScanner) ScanCommands(dir string) ([]*command.Command, error) {
	var commands []*command.Command

	for _, localDir := range s.cfg.LocalDirs {
		for _, commandsDir := range s.cfg.CommandsDirs {
			fullPath := filepath.Join(dir, localDir, commandsDir)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				continue
			}

			loadedCommands, err := command.LoadAll(fullPath)
			if err != nil {
				continue
			}

			for _, c := range loadedCommands {
				c.Scope = "local"
				c.Tool = s.cfg.Name
			}
			commands = append(commands, loadedCommands...)
		}
	}

	return commands, nil
}

func (s *DirectoryScanner) ScanServers(dir string) ([]*mcp.Server, error) {
	// DirectoryScanner doesn't scan servers by default - servers are typically
	// in JSON config files which are tool-specific
	return nil, nil
}

// ScanAgents discovers agents from the tool's local agents directory
func (s *DirectoryScanner) ScanAgents(dir string) ([]*agent.Agent, error) {
	var agents []*agent.Agent

	for _, localDir := range s.cfg.LocalDirs {
		for _, agentsDir := range s.cfg.AgentsDirs {
			fullPath := filepath.Join(dir, localDir, agentsDir)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				continue
			}

			loadedAgents, err := agent.LoadFromDirectory(fullPath, "local", s.cfg.Name)
			if err != nil {
				continue
			}

			agents = append(agents, loadedAgents...)
		}
	}

	return agents, nil
}

// ScanGlobalAgents discovers agents from the tool's global agents directory
func (s *DirectoryScanner) ScanGlobalAgents() ([]*agent.Agent, error) {
	var agents []*agent.Agent

	for _, globalDir := range s.cfg.GlobalDirs {
		expandedDir := expandHomeDir(globalDir)
		for _, agentsDir := range s.cfg.AgentsDirs {
			fullPath := filepath.Join(expandedDir, agentsDir)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				continue
			}

			loadedAgents, err := agent.LoadFromDirectory(fullPath, "global", s.cfg.Name)
			if err != nil {
				continue
			}

			agents = append(agents, loadedAgents...)
		}
	}

	return agents, nil
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
