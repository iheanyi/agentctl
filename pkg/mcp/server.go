package mcp

// Transport represents the MCP transport protocol
type Transport string

const (
	TransportStdio Transport = "stdio"
	TransportSSE   Transport = "sse"
)

// Source represents where an MCP server comes from
type Source struct {
	Type  string `json:"type"`            // "git", "alias", "local"
	URL   string `json:"url,omitempty"`   // Git URL or local path
	Ref   string `json:"ref,omitempty"`   // Tag, branch, or commit
	Alias string `json:"alias,omitempty"` // Short name alias
}

// BuildConfig defines how to build an MCP server
type BuildConfig struct {
	Command string `json:"command"`           // e.g., "npm run build"
	WorkDir string `json:"workdir,omitempty"` // Working directory for build
}

// Server represents an MCP server configuration
type Server struct {
	Name      string            `json:"name"`
	Source    Source            `json:"source"`
	Command   string            `json:"command"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Transport Transport         `json:"transport,omitempty"`
	Namespace string            `json:"namespace,omitempty"` // For conflict resolution
	Build     *BuildConfig      `json:"build,omitempty"`
	Disabled  bool              `json:"disabled,omitempty"`
}

// Runtime represents the runtime required by an MCP server
type Runtime string

const (
	RuntimeNode   Runtime = "node"
	RuntimePython Runtime = "python"
	RuntimeGo     Runtime = "go"
	RuntimeDocker Runtime = "docker"
)

// ServerInfo contains metadata about an MCP server from a registry/alias
type ServerInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Source      Source   `json:"source"`
	Versions    []string `json:"versions,omitempty"`
	Runtime     Runtime  `json:"runtime,omitempty"`
	Deprecated  bool     `json:"deprecated,omitempty"`
}
