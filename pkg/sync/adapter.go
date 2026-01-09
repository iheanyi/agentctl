package sync

import (
	"sync"

	"github.com/iheanyi/agentctl/pkg/command"
	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/iheanyi/agentctl/pkg/rule"
)

// ResourceType represents the type of resource being synced
type ResourceType string

const (
	ResourceMCP      ResourceType = "mcp"
	ResourceCommands ResourceType = "commands"
	ResourceRules    ResourceType = "rules"
	ResourcePrompts  ResourceType = "prompts"
	ResourceSkills   ResourceType = "skills"
)

// ManagedMarker is the key used to mark entries managed by agentctl
const ManagedMarker = "_managedBy"

// ManagedValue is the value used to identify agentctl-managed entries
const ManagedValue = "agentctl"

// Adapter is the interface that tool-specific adapters must implement
type Adapter interface {
	// Name returns the adapter name (e.g., "claude", "cursor")
	Name() string

	// Detect checks if this tool is installed on the system
	Detect() (bool, error)

	// ConfigPath returns the path to the tool's config file
	ConfigPath() string

	// SupportedResources returns the resource types this adapter supports
	SupportedResources() []ResourceType

	// ReadServers reads MCP server configurations from the tool
	ReadServers() ([]*mcp.Server, error)

	// WriteServers writes MCP server configurations to the tool
	WriteServers(servers []*mcp.Server) error

	// ReadCommands reads command configurations from the tool (if supported)
	ReadCommands() ([]*command.Command, error)

	// WriteCommands writes command configurations to the tool (if supported)
	WriteCommands(commands []*command.Command) error

	// ReadRules reads rule configurations from the tool (if supported)
	ReadRules() ([]*rule.Rule, error)

	// WriteRules writes rule configurations to the tool (if supported)
	WriteRules(rules []*rule.Rule) error
}

// WorkspaceAdapter is an optional interface for adapters that support
// project-level (workspace) configurations in addition to global configs.
// Tools like Claude Code (.mcp.json) and Cursor (.cursor/mcp.json) support this.
type WorkspaceAdapter interface {
	Adapter

	// SupportsWorkspace returns true if this tool has workspace-level config support
	SupportsWorkspace() bool

	// WorkspaceConfigPath returns the path to the workspace config file for a given project
	WorkspaceConfigPath(projectDir string) string

	// ReadWorkspaceServers reads MCP servers from the workspace config
	ReadWorkspaceServers(projectDir string) ([]*mcp.Server, error)

	// WriteWorkspaceServers writes MCP servers to the workspace config
	WriteWorkspaceServers(projectDir string, servers []*mcp.Server) error
}

// SupportsWorkspace checks if an adapter implements WorkspaceAdapter
func SupportsWorkspace(a Adapter) bool {
	wa, ok := a.(WorkspaceAdapter)
	return ok && wa.SupportsWorkspace()
}

// AsWorkspaceAdapter returns the adapter as a WorkspaceAdapter if supported
func AsWorkspaceAdapter(a Adapter) (WorkspaceAdapter, bool) {
	wa, ok := a.(WorkspaceAdapter)
	if !ok || !wa.SupportsWorkspace() {
		return nil, false
	}
	return wa, true
}

// Registry holds all registered adapters
var registry = make(map[string]Adapter)

// Register registers an adapter
func Register(adapter Adapter) {
	registry[adapter.Name()] = adapter
}

// Get returns an adapter by name
func Get(name string) (Adapter, bool) {
	adapter, ok := registry[name]
	return adapter, ok
}

// All returns all registered adapters
func All() []Adapter {
	adapters := make([]Adapter, 0, len(registry))
	for _, adapter := range registry {
		adapters = append(adapters, adapter)
	}
	return adapters
}

// Detected returns all adapters for installed tools
func Detected() []Adapter {
	var detected []Adapter
	for _, adapter := range registry {
		if installed, _ := adapter.Detect(); installed {
			detected = append(detected, adapter)
		}
	}
	return detected
}

// SyncResult represents the result of syncing to a tool
type SyncResult struct {
	Tool    string
	Success bool
	Error   error
	Changes int // Number of changes made
}

// SyncAll syncs configuration to all detected tools
func SyncAll(servers []*mcp.Server, commands []*command.Command, rules []*rule.Rule) []SyncResult {
	adapters := Detected()
	results := make([]SyncResult, len(adapters))
	var wg sync.WaitGroup

	for i, adapter := range adapters {
		wg.Add(1)
		go func(i int, adapter Adapter) {
			defer wg.Done()
			result := SyncResult{Tool: adapter.Name()}

			supported := adapter.SupportedResources()

			// Sync servers if supported
			if containsResource(supported, ResourceMCP) && len(servers) > 0 {
				if err := adapter.WriteServers(servers); err != nil {
					result.Error = err
				} else {
					result.Changes += len(servers)
				}
			}

			// Sync commands if supported
			if containsResource(supported, ResourceCommands) && len(commands) > 0 {
				if err := adapter.WriteCommands(commands); err != nil {
					if result.Error == nil {
						result.Error = err
					}
				} else {
					result.Changes += len(commands)
				}
			}

			// Sync rules if supported
			if containsResource(supported, ResourceRules) && len(rules) > 0 {
				if err := adapter.WriteRules(rules); err != nil {
					if result.Error == nil {
						result.Error = err
					}
				} else {
					result.Changes += len(rules)
				}
			}

			result.Success = result.Error == nil
			results[i] = result
		}(i, adapter)
	}

	wg.Wait()
	return results
}

func containsResource(resources []ResourceType, target ResourceType) bool {
	for _, r := range resources {
		if r == target {
			return true
		}
	}
	return false
}

// FilterStdioServers returns only servers that use stdio transport
// Use this for tools that don't support HTTP/SSE remote MCP servers
func FilterStdioServers(servers []*mcp.Server) []*mcp.Server {
	var filtered []*mcp.Server
	for _, s := range servers {
		if s.Transport != mcp.TransportHTTP && s.Transport != mcp.TransportSSE {
			filtered = append(filtered, s)
		}
	}
	return filtered
}
