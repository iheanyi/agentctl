package cli

import (
	"testing"
)

func TestValidateServer(t *testing.T) {
	tests := []struct {
		name        string
		adapterName string
		serverName  string
		serverData  interface{}
		wantErrors  []string
	}{
		{
			name:        "valid stdio server",
			adapterName: "claude",
			serverName:  "test",
			serverData:  map[string]interface{}{"command": "echo", "args": []interface{}{"hello"}},
			wantErrors:  nil,
		},
		{
			name:        "valid http server",
			adapterName: "claude",
			serverName:  "test",
			serverData:  map[string]interface{}{"url": "http://localhost:8080"},
			wantErrors:  nil,
		},
		{
			name:        "missing command and url",
			adapterName: "claude",
			serverName:  "broken",
			serverData:  map[string]interface{}{"notCommand": "oops"},
			wantErrors:  []string{`Server "broken": missing 'command' or 'url' field`},
		},
		{
			name:        "both command and url",
			adapterName: "claude",
			serverName:  "both",
			serverData:  map[string]interface{}{"command": "echo", "url": "http://localhost"},
			wantErrors:  []string{`Server "both": has both 'command' and 'url' (should have only one)`},
		},
		{
			name:        "server not an object",
			adapterName: "claude",
			serverName:  "bad",
			serverData:  "not an object",
			wantErrors:  []string{`Server "bad": should be an object`},
		},
		{
			name:        "opencode valid local",
			adapterName: "opencode",
			serverName:  "test",
			serverData:  map[string]interface{}{"type": "local", "command": []interface{}{"echo"}},
			wantErrors:  nil,
		},
		{
			name:        "opencode valid remote",
			adapterName: "opencode",
			serverName:  "test",
			serverData:  map[string]interface{}{"type": "remote", "url": "http://localhost"},
			wantErrors:  nil,
		},
		{
			name:        "opencode missing type",
			adapterName: "opencode",
			serverName:  "broken",
			serverData:  map[string]interface{}{"command": []interface{}{"echo"}},
			wantErrors:  []string{`Server "broken": missing 'type' field`},
		},
		{
			name:        "opencode invalid type",
			adapterName: "opencode",
			serverName:  "broken",
			serverData:  map[string]interface{}{"type": "invalid"},
			wantErrors:  []string{`Server "broken": invalid type "invalid" (expected 'local' or 'remote')`},
		},
		{
			name:        "opencode local missing command",
			adapterName: "opencode",
			serverName:  "broken",
			serverData:  map[string]interface{}{"type": "local"},
			wantErrors:  []string{`Server "broken": local server missing 'command' array`},
		},
		{
			name:        "opencode remote missing url",
			adapterName: "opencode",
			serverName:  "broken",
			serverData:  map[string]interface{}{"type": "remote"},
			wantErrors:  []string{`Server "broken": remote server missing 'url' field`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateServer(tt.adapterName, tt.serverName, tt.serverData)

			if len(got) != len(tt.wantErrors) {
				t.Errorf("validateServer() returned %d errors, want %d\ngot: %v\nwant: %v",
					len(got), len(tt.wantErrors), got, tt.wantErrors)
				return
			}

			for i, err := range got {
				if err != tt.wantErrors[i] {
					t.Errorf("validateServer() error[%d] = %q, want %q", i, err, tt.wantErrors[i])
				}
			}
		})
	}
}

func TestGetServerKey(t *testing.T) {
	tests := []struct {
		adapterName string
		want        string
	}{
		{"claude", "mcpServers"},
		{"cursor", "mcpServers"},
		{"zed", "context_servers"},
		{"opencode", "mcp"},
		{"unknown", "mcpServers"},
	}

	for _, tt := range tests {
		t.Run(tt.adapterName, func(t *testing.T) {
			got := getServerKey(tt.adapterName)
			if got != tt.want {
				t.Errorf("getServerKey(%q) = %q, want %q", tt.adapterName, got, tt.want)
			}
		})
	}
}
