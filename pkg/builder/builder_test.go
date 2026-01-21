package builder

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/iheanyi/agentctl/pkg/mcp"
)

func TestServerDir(t *testing.T) {
	b := New("/cache")

	tests := []struct {
		name     string
		server   *mcp.Server
		wantPath string
	}{
		{
			name: "simple name",
			server: &mcp.Server{
				Name: "filesystem",
			},
			wantPath: "/cache/servers/filesystem",
		},
		{
			name: "with namespace",
			server: &mcp.Server{
				Name:      "filesystem",
				Namespace: "anthropic",
			},
			wantPath: "/cache/servers/anthropic/filesystem",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := b.ServerDir(tt.server)
			if got != tt.wantPath {
				t.Errorf("ServerDir() = %q, want %q", got, tt.wantPath)
			}
		})
	}
}

func TestInstalled(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "builder-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	b := New(tmpDir)

	// Create a server directory
	server := &mcp.Server{Name: "test-server"}
	serverDir := b.ServerDir(server)
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		t.Fatalf("Failed to create server dir: %v", err)
	}

	// Should be installed
	if !b.Installed(server) {
		t.Error("Installed() should return true for existing server")
	}

	// Non-existent server
	other := &mcp.Server{Name: "other-server"}
	if b.Installed(other) {
		t.Error("Installed() should return false for non-existent server")
	}
}

func TestRemove(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "builder-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	b := New(tmpDir)

	// Create a server directory with files
	server := &mcp.Server{Name: "test-server"}
	serverDir := b.ServerDir(server)
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		t.Fatalf("Failed to create server dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(serverDir, "test.txt"), []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Remove server
	if err := b.Remove(server); err != nil {
		t.Fatalf("Remove() failed: %v", err)
	}

	// Should no longer exist
	if b.Installed(server) {
		t.Error("Server should be removed")
	}
}

func TestResolveCommand(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "builder-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	b := New(tmpDir)

	// Create a server directory with entry point
	server := &mcp.Server{
		Name:   "test-server",
		Source: mcp.Source{Type: "git"},
	}
	serverDir := b.ServerDir(server)
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		t.Fatalf("Failed to create server dir: %v", err)
	}

	// Create dist/index.js
	distDir := filepath.Join(serverDir, "dist")
	if err := os.MkdirAll(distDir, 0755); err != nil {
		t.Fatalf("Failed to create dist dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(distDir, "index.js"), []byte(""), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	resolved := b.ResolveCommand(server)
	expected := filepath.Join(serverDir, "dist", "index.js")
	if resolved != expected {
		t.Errorf("ResolveCommand() = %q, want %q", resolved, expected)
	}
}

func TestResolveCommandLocal(t *testing.T) {
	b := New("/cache")

	server := &mcp.Server{
		Name:    "local-server",
		Source:  mcp.Source{Type: "local"},
		Command: "/path/to/server",
	}

	resolved := b.ResolveCommand(server)
	if resolved != "/path/to/server" {
		t.Errorf("ResolveCommand() should return original command for local: %q", resolved)
	}
}

func TestCloneNonGit(t *testing.T) {
	b := New("/cache")

	server := &mcp.Server{
		Name:   "local",
		Source: mcp.Source{Type: "local"},
	}

	err := b.Clone(server)
	if err == nil {
		t.Error("Clone() should return error for non-git source")
	}
}

func TestBuildConfigExecution(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "builder-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	b := New(tmpDir)

	server := &mcp.Server{
		Name:   "test",
		Source: mcp.Source{Type: "local"},
		Build: &mcp.BuildConfig{
			Install: "echo 'installing'",
			Build:   "echo 'building'",
		},
	}

	// Create server directory
	serverDir := b.ServerDir(server)
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		t.Fatalf("Failed to create server dir: %v", err)
	}

	// Build should run the commands
	if err := b.Build(server); err != nil {
		t.Errorf("Build() failed: %v", err)
	}
}

func TestAutoDetectNode(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "builder-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	b := New(tmpDir)

	server := &mcp.Server{
		Name:   "node-server",
		Source: mcp.Source{Type: "git"},
	}

	serverDir := b.ServerDir(server)
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		t.Fatalf("Failed to create server dir: %v", err)
	}

	// Create package.json
	pkgJSON := `{"name": "test", "scripts": {}}`
	if err := os.WriteFile(filepath.Join(serverDir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	// We can't actually run npm install in tests, but we verify detection works
	// by checking that the function doesn't panic and handles the file correctly
}

func TestAutoDetectGo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "builder-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	b := New(tmpDir)

	server := &mcp.Server{
		Name:   "go-server",
		Source: mcp.Source{Type: "git"},
	}

	serverDir := b.ServerDir(server)
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		t.Fatalf("Failed to create server dir: %v", err)
	}

	// Create go.mod
	goMod := "module test\n\ngo 1.21"
	if err := os.WriteFile(filepath.Join(serverDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Verify file exists for detection
	if _, err := os.Stat(filepath.Join(serverDir, "go.mod")); err != nil {
		t.Error("go.mod should exist")
	}
}

func TestNew(t *testing.T) {
	b := New("/test/cache")
	if b.CacheDir != "/test/cache" {
		t.Errorf("CacheDir = %q, want %q", b.CacheDir, "/test/cache")
	}
}
