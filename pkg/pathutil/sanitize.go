// Package pathutil provides utilities for safe path handling.
package pathutil

import (
	"fmt"
	"regexp"
	"strings"
)

// namePattern matches safe characters for resource names:
// alphanumeric, dash, underscore, dot, and colon (for namespaced names like "skill:command")
var namePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:-]*$`)

// SanitizeName validates and sanitizes a resource name (server, command, skill, rule)
// to prevent path traversal attacks when used in filepath.Join().
//
// The function:
// - Rejects empty names
// - Rejects names containing path separators (/, \)
// - Rejects names containing ".." sequences
// - Rejects names that don't match the safe character pattern
//
// Returns an error if the name is invalid.
func SanitizeName(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	// Check for path separators
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("name cannot contain path separators: %q", name)
	}

	// Check for parent directory traversal
	if strings.Contains(name, "..") {
		return fmt.Errorf("name cannot contain '..': %q", name)
	}

	// Validate against safe character pattern
	if !namePattern.MatchString(name) {
		return fmt.Errorf("name contains invalid characters (must be alphanumeric, dash, underscore, dot, or colon, and start with alphanumeric): %q", name)
	}

	return nil
}

// MustSanitizeName is like SanitizeName but panics on error.
// Use only in contexts where the name has already been validated.
func MustSanitizeName(name string) {
	if err := SanitizeName(name); err != nil {
		panic(err)
	}
}
