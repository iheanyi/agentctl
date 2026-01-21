package sync

import (
	"testing"
)

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		// Valid names
		{name: "simple name", input: "myserver", wantErr: false},
		{name: "name with dash", input: "my-server", wantErr: false},
		{name: "name with underscore", input: "my_server", wantErr: false},
		{name: "name with dot", input: "my.server", wantErr: false},
		{name: "name with colon", input: "skill:command", wantErr: false},
		{name: "name with numbers", input: "server123", wantErr: false},
		{name: "mixed valid chars", input: "my-skill_v1.0:test", wantErr: false},
		{name: "uppercase letters", input: "MyServer", wantErr: false},
		{name: "single character", input: "a", wantErr: false},

		// Invalid names - empty
		{name: "empty name", input: "", wantErr: true, errMsg: "cannot be empty"},

		// Invalid names - path traversal
		{name: "forward slash", input: "my/server", wantErr: true, errMsg: "path separators"},
		{name: "backslash", input: "my\\server", wantErr: true, errMsg: "path separators"},
		{name: "parent directory", input: "..server", wantErr: true, errMsg: "'..'"},
		{name: "parent traversal", input: "../../../etc/passwd", wantErr: true, errMsg: "path separators"},
		{name: "hidden parent", input: "foo..bar", wantErr: true, errMsg: "'..'"},
		{name: "only dots", input: "..", wantErr: true, errMsg: "'..'"},
		{name: "path with parent", input: "foo/../bar", wantErr: true, errMsg: "path separators"},
		{name: "windows path traversal", input: "..\\..\\etc\\passwd", wantErr: true, errMsg: "path separators"},

		// Invalid names - bad characters
		{name: "starts with dash", input: "-server", wantErr: true, errMsg: "invalid characters"},
		{name: "starts with dot", input: ".server", wantErr: true, errMsg: "invalid characters"},
		{name: "starts with underscore", input: "_server", wantErr: true, errMsg: "invalid characters"},
		{name: "contains space", input: "my server", wantErr: true, errMsg: "invalid characters"},
		{name: "contains special char", input: "my@server", wantErr: true, errMsg: "invalid characters"},
		{name: "contains shell escape", input: "server$(whoami)", wantErr: true, errMsg: "invalid characters"},
		{name: "contains backtick", input: "server`id`", wantErr: true, errMsg: "invalid characters"},
		{name: "contains semicolon", input: "server;rm", wantErr: true, errMsg: "invalid characters"},
		{name: "contains newline", input: "server\nmalicious", wantErr: true, errMsg: "invalid characters"},
		{name: "contains null byte", input: "server\x00evil", wantErr: true, errMsg: "invalid characters"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SanitizeName(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("SanitizeName(%q) expected error containing %q, got nil", tt.input, tt.errMsg)
				} else if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("SanitizeName(%q) error = %q, want error containing %q", tt.input, err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("SanitizeName(%q) unexpected error: %v", tt.input, err)
				}
			}
		})
	}
}

func TestMustSanitizeName(t *testing.T) {
	// Test that valid names don't panic
	t.Run("valid name", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("MustSanitizeName panicked unexpectedly: %v", r)
			}
		}()
		MustSanitizeName("valid-name")
	})

	// Test that invalid names panic
	t.Run("invalid name", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("MustSanitizeName did not panic for invalid name")
			}
		}()
		MustSanitizeName("../etc/passwd")
	})
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
