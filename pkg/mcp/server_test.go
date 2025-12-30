package mcp

import (
	"encoding/json"
	"testing"
)

func TestServerSerialization(t *testing.T) {
	server := Server{
		Name:    "filesystem",
		Command: "npx",
		Args:    []string{"-y", "@modelcontextprotocol/server-filesystem"},
		Env: map[string]string{
			"HOME": "/home/user",
		},
		Transport: TransportStdio,
		Source: Source{
			Type:  "alias",
			Alias: "filesystem",
		},
		Namespace: "fs",
	}

	// Test serialization
	data, err := json.Marshal(server)
	if err != nil {
		t.Fatalf("Failed to marshal server: %v", err)
	}

	// Test deserialization
	var decoded Server
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal server: %v", err)
	}

	// Verify fields
	if decoded.Name != server.Name {
		t.Errorf("Name mismatch: got %q, want %q", decoded.Name, server.Name)
	}
	if decoded.Command != server.Command {
		t.Errorf("Command mismatch: got %q, want %q", decoded.Command, server.Command)
	}
	if len(decoded.Args) != len(server.Args) {
		t.Errorf("Args length mismatch: got %d, want %d", len(decoded.Args), len(server.Args))
	}
	if decoded.Transport != server.Transport {
		t.Errorf("Transport mismatch: got %q, want %q", decoded.Transport, server.Transport)
	}
	if decoded.Namespace != server.Namespace {
		t.Errorf("Namespace mismatch: got %q, want %q", decoded.Namespace, server.Namespace)
	}
}

func TestSourceTypes(t *testing.T) {
	tests := []struct {
		name   string
		source Source
	}{
		{
			name: "git source",
			source: Source{
				Type: "git",
				URL:  "github.com/modelcontextprotocol/servers",
				Ref:  "v1.0.0",
			},
		},
		{
			name: "alias source",
			source: Source{
				Type:  "alias",
				Alias: "filesystem",
			},
		},
		{
			name: "local source",
			source: Source{
				Type: "local",
				URL:  "./tools/my-mcp",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.source)
			if err != nil {
				t.Fatalf("Failed to marshal source: %v", err)
			}

			var decoded Source
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Failed to unmarshal source: %v", err)
			}

			if decoded.Type != tt.source.Type {
				t.Errorf("Type mismatch: got %q, want %q", decoded.Type, tt.source.Type)
			}
		})
	}
}

func TestTransportConstants(t *testing.T) {
	if TransportStdio != "stdio" {
		t.Errorf("TransportStdio should be 'stdio', got %q", TransportStdio)
	}
	if TransportSSE != "sse" {
		t.Errorf("TransportSSE should be 'sse', got %q", TransportSSE)
	}
}

func TestBuildConfig(t *testing.T) {
	server := Server{
		Name:    "custom-mcp",
		Command: "node",
		Args:    []string{"dist/index.js"},
		Build: &BuildConfig{
			Install: "npm install",
			Build:   "npm run build",
			WorkDir: ".",
		},
	}

	data, err := json.Marshal(server)
	if err != nil {
		t.Fatalf("Failed to marshal server with build config: %v", err)
	}

	var decoded Server
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal server: %v", err)
	}

	if decoded.Build == nil {
		t.Fatal("Build config should not be nil")
	}
	if decoded.Build.Install != "npm install" {
		t.Errorf("Build install mismatch: got %q", decoded.Build.Install)
	}
	if decoded.Build.Build != "npm run build" {
		t.Errorf("Build command mismatch: got %q", decoded.Build.Build)
	}
}

func TestServerDisabled(t *testing.T) {
	server := Server{
		Name:     "disabled-server",
		Command:  "node",
		Disabled: true,
	}

	data, err := json.Marshal(server)
	if err != nil {
		t.Fatalf("Failed to marshal server: %v", err)
	}

	var decoded Server
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal server: %v", err)
	}

	if !decoded.Disabled {
		t.Error("Disabled flag should be true")
	}
}
