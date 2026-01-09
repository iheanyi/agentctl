package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/iheanyi/agentctl/pkg/builder"
	"github.com/iheanyi/agentctl/pkg/mcp"
)

// TestGoInstallationSimulation simulates a Go installation by creating a local git repo
// with a cmd/ directory structure and attempting to "install" it using the builder.
func TestGoInstallationSimulation(t *testing.T) {
	// Create a temporary directory for our "fake" remote repo
	tempDir, err := os.MkdirTemp("", "agentctl-test-go")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	remoteRepoDir := filepath.Join(tempDir, "fake-go-repo")
	if err := os.MkdirAll(remoteRepoDir, 0755); err != nil {
		t.Fatalf("Failed to create remote repo dir: %v", err)
	}

	// Initialize git repo
	runGit(t, remoteRepoDir, "init")
	runGit(t, remoteRepoDir, "config", "user.email", "test@example.com")
	runGit(t, remoteRepoDir, "config", "user.name", "Test User")

	// Create a fake server implementation (Go)
	goMod := `module fake-go-server

go 1.21
`
	mainGo := `package main

import "fmt"

func main() {
	fmt.Println("MCP Server Started")
}
`
	// Create go.mod
	os.WriteFile(filepath.Join(remoteRepoDir, "go.mod"), []byte(goMod), 0644)
	
	// Create cmd/server/main.go structure
	cmdDir := filepath.Join(remoteRepoDir, "cmd", "server")
	os.MkdirAll(cmdDir, 0755)
	os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte(mainGo), 0644)

	// Commit
	runGit(t, remoteRepoDir, "add", ".")
	runGit(t, remoteRepoDir, "commit", "-m", "Initial commit")

	// Now simulate the installation process
	server := &mcp.Server{
		Name: "fake-go-server",
		Source: mcp.Source{
			Type: "git",
			URL:  remoteRepoDir, 
		},
		Transport: mcp.TransportStdio,
	}

	// Use Builder to clone and build
	cacheDir := filepath.Join(tempDir, "cache")
	b := builder.New(cacheDir)

	t.Logf("Cloning from %s to %s", remoteRepoDir, cacheDir)
	if err := b.Clone(server); err != nil {
		t.Fatalf("Builder.Clone failed: %v", err)
	}

	// Verify clone
	clonedDir := b.ServerDir(server)
	if _, err := os.Stat(filepath.Join(clonedDir, "go.mod")); os.IsNotExist(err) {
		t.Errorf("Cloned directory does not contain go.mod")
	}

	// Build (this should detect Go and run go build)
	t.Log("Building...")
	if err := b.Build(server); err != nil {
		t.Fatalf("Builder.Build failed: %v", err)
	}

	// Verify binary was built
	// Based on our builder logic, it should output "server" binary in the root
	if _, err := os.Stat(filepath.Join(clonedDir, "server")); os.IsNotExist(err) {
		t.Errorf("Build artifact 'server' not found")
	}

	// Resolve command
	cmd := b.ResolveCommand(server)
	if cmd == "" {
		t.Error("ResolveCommand returned empty string")
	}
	
	t.Logf("Resolved command: %s", cmd)
}
