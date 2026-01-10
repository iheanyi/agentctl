package config

import (
	"testing"
)

func TestParseScope(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Scope
		wantErr bool
	}{
		// Valid scopes
		{"local", "local", ScopeLocal, false},
		{"project alias", "project", ScopeLocal, false},
		{"global", "global", ScopeGlobal, false},
		{"user alias", "user", ScopeGlobal, false},
		{"all", "all", ScopeAll, false},
		{"empty defaults to all", "", ScopeAll, false},

		// Invalid scopes
		{"invalid", "invalid", "", true},
		{"typo", "locla", "", true},
		{"uppercase", "LOCAL", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseScope(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseScope(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseScope(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestScope_IsValid(t *testing.T) {
	tests := []struct {
		scope Scope
		want  bool
	}{
		{ScopeLocal, true},
		{ScopeGlobal, true},
		{ScopeAll, true},
		{Scope("invalid"), false},
		{Scope(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.scope), func(t *testing.T) {
			if got := tt.scope.IsValid(); got != tt.want {
				t.Errorf("Scope(%q).IsValid() = %v, want %v", tt.scope, got, tt.want)
			}
		})
	}
}

func TestScope_ShortString(t *testing.T) {
	tests := []struct {
		scope Scope
		want  string
	}{
		{ScopeLocal, "[L]"},
		{ScopeGlobal, "[G]"},
		{ScopeAll, "[?]"},
		{Scope("invalid"), "[?]"},
	}

	for _, tt := range tests {
		t.Run(string(tt.scope), func(t *testing.T) {
			if got := tt.scope.ShortString(); got != tt.want {
				t.Errorf("Scope(%q).ShortString() = %v, want %v", tt.scope, got, tt.want)
			}
		})
	}
}

func TestScope_Description(t *testing.T) {
	tests := []struct {
		scope Scope
		want  string
	}{
		{ScopeLocal, "project config (.agentctl.json)"},
		{ScopeGlobal, "global config (~/.config/agentctl/agentctl.json)"},
		{ScopeAll, "all configs"},
	}

	for _, tt := range tests {
		t.Run(string(tt.scope), func(t *testing.T) {
			if got := tt.scope.Description(); got != tt.want {
				t.Errorf("Scope(%q).Description() = %v, want %v", tt.scope, got, tt.want)
			}
		})
	}
}

func TestScope_String(t *testing.T) {
	tests := []struct {
		scope Scope
		want  string
	}{
		{ScopeLocal, "local"},
		{ScopeGlobal, "global"},
		{ScopeAll, "all"},
	}

	for _, tt := range tests {
		t.Run(string(tt.scope), func(t *testing.T) {
			if got := tt.scope.String(); got != tt.want {
				t.Errorf("Scope(%q).String() = %v, want %v", tt.scope, got, tt.want)
			}
		})
	}
}
