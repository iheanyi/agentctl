package mcp

// Transport represents the MCP transport protocol
type Transport string

const (
	TransportStdio Transport = "stdio" // Local process communication
	TransportSSE   Transport = "sse"   // Server-Sent Events
	TransportHTTP  Transport = "http"  // Remote HTTP MCP server (like Sentry)
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
	Install string `json:"install,omitempty"` // e.g., "npm install"
	Build   string `json:"build,omitempty"`   // e.g., "npm run build"
	WorkDir string `json:"workdir,omitempty"` // Working directory for build
}

// Server represents an MCP server configuration
type Server struct {
	Name      string            `json:"name"`
	Source    Source            `json:"source"`
	Command   string            `json:"command,omitempty"`   // For local servers (stdio)
	Args      []string          `json:"args,omitempty"`      // For local servers
	URL       string            `json:"url,omitempty"`       // For remote servers (http/sse)
	Headers   map[string]string `json:"headers,omitempty"`   // For remote servers (http/sse)
	Env       map[string]string `json:"env,omitempty"`
	Transport Transport         `json:"transport,omitempty"`
	Namespace string            `json:"namespace,omitempty"` // For conflict resolution
	Build     *BuildConfig      `json:"build,omitempty"`
	Disabled  bool              `json:"disabled,omitempty"`

	// Runtime fields (not serialized to JSON)
	Scope string `json:"-"` // "local" or "global" - where this server came from
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
