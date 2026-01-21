// Package discovery provides unified resource discovery from tool-native configurations.
// This allows agentctl to detect and load resources (rules, skills, hooks, etc.) from
// tool-specific directories like .claude/, .gemini/, .cursor/ even without agentctl.json.
package discovery

import (
	"os"
	"path/filepath"

	"github.com/iheanyi/agentctl/pkg/agent"
	"github.com/iheanyi/agentctl/pkg/command"
	"github.com/iheanyi/agentctl/pkg/hook"
	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/iheanyi/agentctl/pkg/rule"
	"github.com/iheanyi/agentctl/pkg/skill"
)

// NativeResource represents a resource discovered from a tool's native config
type NativeResource struct {
	Type     string // "rule", "skill", "command", "hook", "server", "plugin", "agent"
	Name     string
	Path     string // Path to the file or directory
	Tool     string // Which tool owns this (e.g., "claude", "gemini")
	Scope    string // "local" or "global"
	Resource any    // The actual resource (*rule.Rule, *skill.Skill, *agent.Agent, etc.)
}

// Scanner defines the interface for discovering resources from tool configurations
type Scanner interface {
	// Name returns the scanner/tool name (e.g., "claude", "gemini")
	Name() string

	// Detect checks if the tool's configuration exists in the given directory
	Detect(dir string) bool

	// ScanRules discovers rules from the tool's config directory
	ScanRules(dir string) ([]*rule.Rule, error)

	// ScanSkills discovers skills from the tool's config directory
	ScanSkills(dir string) ([]*skill.Skill, error)

	// ScanHooks discovers hooks from the tool's settings
	ScanHooks(dir string) ([]*hook.Hook, error)

	// ScanCommands discovers commands (slash commands) from the tool's config
	ScanCommands(dir string) ([]*command.Command, error)

	// ScanServers discovers MCP servers from the tool's config
	ScanServers(dir string) ([]*mcp.Server, error)
}

// registry holds all registered scanners
var registry = make(map[string]Scanner)

// Register adds a scanner to the registry
func Register(s Scanner) {
	registry[s.Name()] = s
}

// Get returns a scanner by name
func Get(name string) (Scanner, bool) {
	s, ok := registry[name]
	return s, ok
}

// All returns all registered scanners
func All() []Scanner {
	scanners := make([]Scanner, 0, len(registry))
	for _, s := range registry {
		scanners = append(scanners, s)
	}
	return scanners
}

// Detected returns scanners that detect configs in the given directory
func Detected(dir string) []Scanner {
	var detected []Scanner
	for _, s := range registry {
		if s.Detect(dir) {
			detected = append(detected, s)
		}
	}
	return detected
}

// DiscoverAll runs all scanners against the given directory and returns discovered resources
func DiscoverAll(dir string) []*NativeResource {
	var resources []*NativeResource

	for _, scanner := range registry {
		if !scanner.Detect(dir) {
			continue
		}

		tool := scanner.Name()

		// Scan rules
		if rules, err := scanner.ScanRules(dir); err == nil {
			for _, r := range rules {
				resources = append(resources, &NativeResource{
					Type:     "rule",
					Name:     r.Name,
					Path:     r.Path,
					Tool:     tool,
					Scope:    "local",
					Resource: r,
				})
			}
		}

		// Scan skills
		if skills, err := scanner.ScanSkills(dir); err == nil {
			for _, s := range skills {
				resources = append(resources, &NativeResource{
					Type:     "skill",
					Name:     s.Name,
					Path:     s.Path,
					Tool:     tool,
					Scope:    "local",
					Resource: s,
				})
			}
		}

		// Scan hooks
		if hooks, err := scanner.ScanHooks(dir); err == nil {
			for _, h := range hooks {
				resources = append(resources, &NativeResource{
					Type:     "hook",
					Name:     h.Name,
					Path:     "",
					Tool:     tool,
					Scope:    "local",
					Resource: h,
				})
			}
		}

		// Scan commands
		if commands, err := scanner.ScanCommands(dir); err == nil {
			for _, c := range commands {
				resources = append(resources, &NativeResource{
					Type:     "command",
					Name:     c.Name,
					Path:     c.Path,
					Tool:     tool,
					Scope:    "local",
					Resource: c,
				})
			}
		}

		// Scan servers
		if servers, err := scanner.ScanServers(dir); err == nil {
			for _, s := range servers {
				resources = append(resources, &NativeResource{
					Type:     "server",
					Name:     s.Name,
					Path:     "",
					Tool:     tool,
					Scope:    "local",
					Resource: s,
				})
			}
		}

		// Scan plugins (if scanner supports it)
		if ps, ok := scanner.(PluginScanner); ok {
			if plugins, err := ps.ScanPlugins(dir); err == nil {
				for _, p := range plugins {
					resources = append(resources, &NativeResource{
						Type:     "plugin",
						Name:     p.Name,
						Path:     p.Path,
						Tool:     tool,
						Scope:    "local",
						Resource: p,
					})
				}
			}
		}

		// Scan agents (if scanner supports it)
		if as, ok := scanner.(AgentScanner); ok {
			if agents, err := as.ScanAgents(dir); err == nil {
				for _, a := range agents {
					resources = append(resources, &NativeResource{
						Type:     "agent",
						Name:     a.Name,
						Path:     a.Path,
						Tool:     tool,
						Scope:    "local",
						Resource: a,
					})
				}
			}
		}
	}

	return resources
}

// DiscoverRules scans all detected tools for rules
func DiscoverRules(dir string) []*rule.Rule {
	var rules []*rule.Rule
	for _, scanner := range registry {
		if !scanner.Detect(dir) {
			continue
		}
		if r, err := scanner.ScanRules(dir); err == nil {
			rules = append(rules, r...)
		}
	}
	return rules
}

// DiscoverSkills scans all detected tools for skills
func DiscoverSkills(dir string) []*skill.Skill {
	var skills []*skill.Skill
	for _, scanner := range registry {
		if !scanner.Detect(dir) {
			continue
		}
		if s, err := scanner.ScanSkills(dir); err == nil {
			skills = append(skills, s...)
		}
	}
	return skills
}

// DiscoverHooks scans all detected tools for hooks
func DiscoverHooks(dir string) []*hook.Hook {
	var hooks []*hook.Hook
	for _, scanner := range registry {
		if !scanner.Detect(dir) {
			continue
		}
		if h, err := scanner.ScanHooks(dir); err == nil {
			hooks = append(hooks, h...)
		}
	}
	return hooks
}

// DiscoverWorkingDir discovers resources from the current working directory
func DiscoverWorkingDir() []*NativeResource {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}
	return DiscoverAll(cwd)
}

// HasToolConfig checks if any tool has a config in the given directory
func HasToolConfig(dir string) bool {
	return len(Detected(dir)) > 0
}

// ToolConfigPaths returns paths to all detected tool configs
func ToolConfigPaths(dir string) map[string]string {
	paths := make(map[string]string)
	for _, scanner := range registry {
		if scanner.Detect(dir) {
			// Each scanner implementation should provide its config path
			paths[scanner.Name()] = filepath.Join(dir, "."+scanner.Name())
		}
	}
	return paths
}

// GlobalScanner defines the interface for discovering global resources
type GlobalScanner interface {
	Scanner
	// ScanGlobalRules discovers rules from the tool's global config directory
	ScanGlobalRules() ([]*rule.Rule, error)
	// ScanGlobalSkills discovers skills from the tool's global config directory
	ScanGlobalSkills() ([]*skill.Skill, error)
	// ScanGlobalCommands discovers commands from the tool's global config directory
	ScanGlobalCommands() ([]*command.Command, error)
	// ScanGlobalHooks discovers hooks from the tool's global settings
	ScanGlobalHooks() ([]*hook.Hook, error)
}

// PluginScanner defines the interface for discovering plugins
type PluginScanner interface {
	Scanner
	// ScanPlugins discovers plugins from the tool's local config
	ScanPlugins(dir string) ([]*Plugin, error)
	// ScanGlobalPlugins discovers plugins from the tool's global config
	ScanGlobalPlugins() ([]*Plugin, error)
}

// AgentScanner defines the interface for discovering agents/subagents
type AgentScanner interface {
	Scanner
	// ScanAgents discovers agents from the tool's local config directory
	ScanAgents(dir string) ([]*agent.Agent, error)
	// ScanGlobalAgents discovers agents from the tool's global config directory
	ScanGlobalAgents() ([]*agent.Agent, error)
}

// DiscoverGlobal discovers resources from global tool configurations
func DiscoverGlobal() []*NativeResource {
	var resources []*NativeResource

	for _, scanner := range registry {
		gs, ok := scanner.(GlobalScanner)
		if !ok {
			continue
		}

		tool := scanner.Name()

		// Scan global rules
		if rules, err := gs.ScanGlobalRules(); err == nil {
			for _, r := range rules {
				resources = append(resources, &NativeResource{
					Type:     "rule",
					Name:     r.Name,
					Path:     r.Path,
					Tool:     tool,
					Scope:    "global",
					Resource: r,
				})
			}
		}

		// Scan global skills
		if skills, err := gs.ScanGlobalSkills(); err == nil {
			for _, s := range skills {
				resources = append(resources, &NativeResource{
					Type:     "skill",
					Name:     s.Name,
					Path:     s.Path,
					Tool:     tool,
					Scope:    "global",
					Resource: s,
				})
			}
		}

		// Scan global commands
		if commands, err := gs.ScanGlobalCommands(); err == nil {
			for _, c := range commands {
				resources = append(resources, &NativeResource{
					Type:     "command",
					Name:     c.Name,
					Path:     c.Path,
					Tool:     tool,
					Scope:    "global",
					Resource: c,
				})
			}
		}

		// Scan global hooks
		if hooks, err := gs.ScanGlobalHooks(); err == nil {
			for _, h := range hooks {
				resources = append(resources, &NativeResource{
					Type:     "hook",
					Name:     h.Name,
					Path:     "",
					Tool:     tool,
					Scope:    "global",
					Resource: h,
				})
			}
		}
	}

	// Scan for global plugins
	for _, scanner := range registry {
		ps, ok := scanner.(PluginScanner)
		if !ok {
			continue
		}

		tool := scanner.Name()

		if plugins, err := ps.ScanGlobalPlugins(); err == nil {
			for _, p := range plugins {
				resources = append(resources, &NativeResource{
					Type:     "plugin",
					Name:     p.Name,
					Path:     p.Path,
					Tool:     tool,
					Scope:    "global",
					Resource: p,
				})
			}
		}
	}

	// Scan for global agents
	for _, scanner := range registry {
		as, ok := scanner.(AgentScanner)
		if !ok {
			continue
		}

		tool := scanner.Name()

		if agents, err := as.ScanGlobalAgents(); err == nil {
			for _, a := range agents {
				resources = append(resources, &NativeResource{
					Type:     "agent",
					Name:     a.Name,
					Path:     a.Path,
					Tool:     tool,
					Scope:    "global",
					Resource: a,
				})
			}
		}
	}

	return resources
}

// DiscoverBoth discovers resources from both local and global configurations
func DiscoverBoth(dir string) []*NativeResource {
	resources := DiscoverAll(dir)
	resources = append(resources, DiscoverGlobal()...)
	return resources
}

// DiscoverAgents scans all detected tools for agents in the given directory
func DiscoverAgents(dir string) []*agent.Agent {
	var agents []*agent.Agent
	for _, scanner := range registry {
		if !scanner.Detect(dir) {
			continue
		}
		as, ok := scanner.(AgentScanner)
		if !ok {
			continue
		}
		if a, err := as.ScanAgents(dir); err == nil {
			agents = append(agents, a...)
		}
	}
	return agents
}

// DiscoverGlobalAgents scans all tools for global agents
func DiscoverGlobalAgents() []*agent.Agent {
	var agents []*agent.Agent
	for _, scanner := range registry {
		as, ok := scanner.(AgentScanner)
		if !ok {
			continue
		}
		if a, err := as.ScanGlobalAgents(); err == nil {
			agents = append(agents, a...)
		}
	}
	return agents
}
