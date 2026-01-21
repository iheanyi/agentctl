package tui

import (
	"fmt"

	"github.com/charmbracelet/x/ansi"

	"github.com/iheanyi/agentctl/pkg/aliases"
	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/iheanyi/agentctl/pkg/mcpclient"
)

// ServerStatusType represents the installation status of an MCP server
type ServerStatusType int

const (
	ServerStatusInstalled ServerStatusType = iota
	ServerStatusAvailable
	ServerStatusDisabled
	ServerStatusNative // Server exists in tool config but not managed by agentctl
)

// HealthStatusType represents the health check status of an MCP server
type HealthStatusType int

const (
	HealthStatusUnknown HealthStatusType = iota
	HealthStatusChecking
	HealthStatusHealthy
	HealthStatusUnhealthy
)

// Server represents a unified server item for the TUI list
type Server struct {
	Name          string
	Desc          string // Description of the server
	Status        ServerStatusType
	Health        HealthStatusType
	HealthError   error
	HealthLatency string           // e.g., "120ms"
	Transport     string           // stdio, http, sse
	Command       string           // for display
	Selected      bool             // for multi-select
	Tools         []mcpclient.Tool // Discovered tools
	ServerConfig  *mcp.Server
	AliasConfig   *aliases.Alias
	SourceTool    string // Tool this server came from (for native servers)
}

// Title implements list.Item interface
// Returns the server name with a status badge
func (s Server) Title() string {
	badge := ServerStatusBadge(s.Status)
	healthBadge := HealthStatusBadge(s.Health)

	if s.Health != HealthStatusUnknown {
		return fmt.Sprintf("%s %s %s", badge, s.Name, healthBadge)
	}
	return fmt.Sprintf("%s %s", badge, s.Name)
}

// Description implements list.Item interface
// Returns transport info and command
func (s Server) Description() string {
	return FormatServerDescription(&s)
}

// FilterValue implements list.Item interface
// Returns the server name for filtering
func (s Server) FilterValue() string {
	return s.Name
}

// ServerStatusBadge returns a visual badge for the server status
//   - ServerStatusInstalled: filled circle (●)
//   - ServerStatusAvailable: empty circle (○)
//   - ServerStatusDisabled: dotted circle (◌)
//   - ServerStatusNative: filled diamond (◆)
func ServerStatusBadge(status ServerStatusType) string {
	switch status {
	case ServerStatusInstalled:
		return StatusInstalled // ●
	case ServerStatusAvailable:
		return StatusAvailable // ○
	case ServerStatusDisabled:
		return StatusDisabled // ◌
	case ServerStatusNative:
		return StatusNative // ◆
	default:
		return StatusAvailable
	}
}

// HealthStatusBadge returns a visual badge for the health status
//   - HealthStatusUnknown: question mark (?)
//   - HealthStatusChecking: spinner character (◐)
//   - HealthStatusHealthy: checkmark (✓)
//   - HealthStatusUnhealthy: cross (✗)
func HealthStatusBadge(health HealthStatusType) string {
	switch health {
	case HealthStatusUnknown:
		return HealthUnknown // ?
	case HealthStatusChecking:
		return HealthChecking // ◐
	case HealthStatusHealthy:
		return HealthHealthy // ✓
	case HealthStatusUnhealthy:
		return HealthUnhealthy // ✗
	default:
		return HealthUnknown
	}
}

// FormatServerDescription formats the description line for a server
// Shows transport type and command/URL information
func FormatServerDescription(s *Server) string {
	transport := s.Transport
	if transport == "" {
		transport = "stdio"
	}

	var desc string

	// Start with the base description if available
	if s.Desc != "" {
		desc = s.Desc
	} else {
		// Build description from transport info
		desc = fmt.Sprintf("%s transport", transport)

		// Add command for stdio servers
		if transport == "stdio" && s.Command != "" {
			desc += fmt.Sprintf(" - %s", s.Command)
		}

		// Add URL for remote servers
		if (transport == "http" || transport == "sse") && s.ServerConfig != nil && s.ServerConfig.URL != "" {
			desc += fmt.Sprintf(" - %s", s.ServerConfig.URL)
		}
	}

	// Truncate if too long (unicode-safe)
	desc = ansi.Truncate(desc, 60, "...")

	return desc
}
