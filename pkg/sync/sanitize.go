package sync

import (
	"github.com/iheanyi/agentctl/pkg/pathutil"
)

// SanitizeName validates and sanitizes a resource name (server, command, skill, rule)
// to prevent path traversal attacks when used in filepath.Join().
// This is a convenience wrapper around pathutil.SanitizeName.
func SanitizeName(name string) error {
	return pathutil.SanitizeName(name)
}

// MustSanitizeName is like SanitizeName but panics on error.
// Use only in contexts where the name has already been validated.
func MustSanitizeName(name string) {
	pathutil.MustSanitizeName(name)
}
