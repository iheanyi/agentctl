package config

import "fmt"

// Scope represents the configuration scope (local/project vs global/user)
type Scope string

const (
	// ScopeLocal represents project-specific configuration (.agentctl.json)
	ScopeLocal Scope = "local"

	// ScopeGlobal represents user-wide configuration (~/.config/agentctl/agentctl.json)
	ScopeGlobal Scope = "global"

	// ScopeAll represents both scopes (used for sync operations)
	ScopeAll Scope = "all"
)

// String returns the string representation of the scope
func (s Scope) String() string {
	return string(s)
}

// IsValid returns true if the scope is a valid value
func (s Scope) IsValid() bool {
	switch s {
	case ScopeLocal, ScopeGlobal, ScopeAll:
		return true
	default:
		return false
	}
}

// ParseScope parses a string into a Scope value
// Accepts aliases: "project" for "local", "user" for "global"
func ParseScope(s string) (Scope, error) {
	switch s {
	case "local", "project":
		return ScopeLocal, nil
	case "global", "user":
		return ScopeGlobal, nil
	case "all", "":
		return ScopeAll, nil
	default:
		return "", fmt.Errorf("invalid scope: %q (use local, global, or all)", s)
	}
}

// ShortString returns a short indicator for display (e.g., [G] or [L])
func (s Scope) ShortString() string {
	switch s {
	case ScopeLocal:
		return "[L]"
	case ScopeGlobal:
		return "[G]"
	default:
		return "[?]"
	}
}

// Description returns a human-readable description of the scope
func (s Scope) Description() string {
	switch s {
	case ScopeLocal:
		return "project config (.agentctl.json)"
	case ScopeGlobal:
		return "global config (~/.config/agentctl/agentctl.json)"
	case ScopeAll:
		return "all configs"
	default:
		return "unknown scope"
	}
}
