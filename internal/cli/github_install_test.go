package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/iheanyi/agentctl/pkg/builder"
	"github.com/iheanyi/agentctl/pkg/mcp"
)

// TestGitHubInstallationSimulation simulates a GitHub installation by creating a local git repo
// and attempting to "install" it using the builder.
func TestGitHubInstallationSimulation(t *testing.T) {
	// Create a temporary directory for our "fake" remote repo
	tempDir, err := os.MkdirTemp("", "agentctl-test-github")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	remoteRepoDir := filepath.Join(tempDir, "fake-repo")
	if err := os.MkdirAll(remoteRepoDir, 0755); err != nil {
		t.Fatalf("Failed to create remote repo dir: %v", err)
	}

	// Initialize git repo
	runGit(t, remoteRepoDir, "init")
	runGit(t, remoteRepoDir, "config", "user.email", "test@example.com")
	runGit(t, remoteRepoDir, "config", "user.name", "Test User")

	// Create a fake server implementation (Node.js)
	packageJSON := `{
  "name": "fake-mcp-server",
  "version": "1.0.0",
  "scripts": {
    "start": "node index.js"
  }
}`
	indexJS := `
console.error("MCP Server Started");
`
	os.WriteFile(filepath.Join(remoteRepoDir, "package.json"), []byte(packageJSON), 0644)
	os.WriteFile(filepath.Join(remoteRepoDir, "index.js"), []byte(indexJS), 0644)

	// Commit
	runGit(t, remoteRepoDir, "add", ".")
	runGit(t, remoteRepoDir, "commit", "-m", "Initial commit")

	// Now simulate the installation process
	// 1. Parse target (using the local path as a "git" source)
	// We manually construct the server config because parseAddTarget expects github.com/ URLs
	server := &mcp.Server{
		Name: "fake-server",
		Source: mcp.Source{
			Type: "git",
			URL:  remoteRepoDir, // Pointing to local dir as git source
		},
		Transport: mcp.TransportStdio,
	}

	// 2. Use Builder to clone and build
	cacheDir := filepath.Join(tempDir, "cache")
	b := builder.New(cacheDir)

	t.Logf("Cloning from %s to %s", remoteRepoDir, cacheDir)
	if err := b.Clone(server); err != nil {
		t.Fatalf("Builder.Clone failed: %v", err)
	}

	// Verify clone
	clonedDir := b.ServerDir(server)
	if _, err := os.Stat(filepath.Join(clonedDir, "package.json")); os.IsNotExist(err) {
		t.Errorf("Cloned directory does not contain package.json")
	}

	// 3. Build (this should detect Node.js and run npm install)
	// We need npm installed for this to work. If not available, we skip this part.
	if _, err := exec.LookPath("npm"); err == nil {
		t.Log("Building...")
		if err := b.Build(server); err != nil {
			// It might fail if network is restricted (npm install), so we just log
			t.Logf("Builder.Build failed (expected if no network/npm): %v", err)
		} else {
			// Verify node_modules or lockfile
			if _, err := os.Stat(filepath.Join(clonedDir, "node_modules")); os.IsNotExist(err) {
				t.Log("node_modules not found (npm install might have been skipped or failed)")
			} else {
				t.Log("Build successful, node_modules created")
			}
		}
	} else {
		t.Log("npm not found, skipping build verification")
	}

	// 4. Resolve command
	cmd := b.ResolveCommand(server)
	if cmd == "" {
		t.Error("ResolveCommand returned empty string")
	}
	
	// Default behavior for Node is "node" or "npx" depending on logic
	// In builder.go:
	// if server.Command == "" { ... server.Command = "node" ... }
	// But ResolveCommand might return "node" if not set
	
	t.Logf("Resolved command: %s", cmd)
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\nOutput: %s", args, err, out)
	}
}
