package mcp

import (
	"fmt"
	"strings"
)

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
	Command   string            `json:"command,omitempty"` // For local servers (stdio)
	Args      []string          `json:"args,omitempty"`    // For local servers
	URL       string            `json:"url,omitempty"`     // For remote servers (http/sse)
	Headers   map[string]string `json:"headers,omitempty"` // For remote servers (http/sse)
	Env       map[string]string `json:"env,omitempty"`
	Transport Transport         `json:"transport,omitempty"`
	Namespace string            `json:"namespace,omitempty"` // For conflict resolution
	Build     *BuildConfig      `json:"build,omitempty"`
	Disabled  bool              `json:"disabled,omitempty"`

	// Runtime fields (not serialized to JSON)
	Scope string `json:"-"` // "local" or "global" - where this server came from
}

// InspectTitle returns the display name for the inspector modal header
func (s *Server) InspectTitle() string {
	return fmt.Sprintf("MCP Server: %s", s.Name)
}

// InspectContent returns the formatted content for the inspector viewport
func (s *Server) InspectContent() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Name:      %s\n", s.Name))
	b.WriteString(fmt.Sprintf("Scope:     %s\n", s.Scope))
	b.WriteString(fmt.Sprintf("Transport: %s\n", s.Transport))

	if s.Disabled {
		b.WriteString("Status:    DISABLED\n")
	}
	b.WriteString("\n")

	// Source info
	if s.Source.Type != "" {
		b.WriteString("Source:\n")
		b.WriteString(fmt.Sprintf("  Type:  %s\n", s.Source.Type))
		if s.Source.URL != "" {
			b.WriteString(fmt.Sprintf("  URL:   %s\n", s.Source.URL))
		}
		if s.Source.Ref != "" {
			b.WriteString(fmt.Sprintf("  Ref:   %s\n", s.Source.Ref))
		}
		if s.Source.Alias != "" {
			b.WriteString(fmt.Sprintf("  Alias: %s\n", s.Source.Alias))
		}
		b.WriteString("\n")
	}

	// Connection details
	if s.Command != "" {
		b.WriteString(fmt.Sprintf("Command: %s\n", s.Command))
	}
	if len(s.Args) > 0 {
		b.WriteString(fmt.Sprintf("Args:    %s\n", strings.Join(s.Args, " ")))
	}
	if s.URL != "" {
		b.WriteString(fmt.Sprintf("URL:     %s\n", s.URL))
	}

	// Environment
	if len(s.Env) > 0 {
		b.WriteString("\nEnvironment:\n")
		for k, v := range s.Env {
			// Mask potentially sensitive values
			displayVal := v
			if strings.Contains(strings.ToLower(k), "key") ||
				strings.Contains(strings.ToLower(k), "secret") ||
				strings.Contains(strings.ToLower(k), "token") ||
				strings.Contains(strings.ToLower(k), "password") {
				displayVal = "***"
			}
			b.WriteString(fmt.Sprintf("  %s=%s\n", k, displayVal))
		}
	}

	// Headers
	if len(s.Headers) > 0 {
		b.WriteString("\nHeaders:\n")
		for k, v := range s.Headers {
			// Mask authorization headers
			displayVal := v
			if strings.ToLower(k) == "authorization" {
				displayVal = "***"
			}
			b.WriteString(fmt.Sprintf("  %s: %s\n", k, displayVal))
		}
	}

	// Build config
	if s.Build != nil {
		b.WriteString("\nBuild:\n")
		if s.Build.Install != "" {
			b.WriteString(fmt.Sprintf("  Install: %s\n", s.Build.Install))
		}
		if s.Build.Build != "" {
			b.WriteString(fmt.Sprintf("  Build:   %s\n", s.Build.Build))
		}
		if s.Build.WorkDir != "" {
			b.WriteString(fmt.Sprintf("  WorkDir: %s\n", s.Build.WorkDir))
		}
	}

	return b.String()
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
