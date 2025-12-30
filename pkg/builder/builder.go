package builder

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/iheanyi/agentctl/pkg/mcp"
)

// Builder handles cloning and building MCP servers
type Builder struct {
	CacheDir string
}

// New creates a new Builder with the given cache directory
func New(cacheDir string) *Builder {
	return &Builder{CacheDir: cacheDir}
}

// ServerDir returns the directory where a server is installed
func (b *Builder) ServerDir(server *mcp.Server) string {
	// Use namespace/name as directory structure
	if server.Namespace != "" {
		return filepath.Join(b.CacheDir, "servers", server.Namespace, server.Name)
	}
	return filepath.Join(b.CacheDir, "servers", server.Name)
}

// Clone clones a git repository for a server
func (b *Builder) Clone(server *mcp.Server) error {
	if server.Source.Type != "git" {
		return fmt.Errorf("server source is not git: %s", server.Source.Type)
	}

	dir := b.ServerDir(server)

	// Check if already cloned
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		// Already cloned, fetch and checkout
		return b.Update(server)
	}

	// Create parent directory
	if err := os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Clone the repository
	url := server.Source.URL
	if url == "" {
		return fmt.Errorf("server source URL is empty")
	}

	// Ensure URL has protocol
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "git@") {
		url = "https://" + url
	}

	args := []string{"clone", "--depth", "1"}
	if server.Source.Ref != "" {
		args = append(args, "--branch", server.Source.Ref)
	}
	args = append(args, url, dir)

	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	return nil
}

// Update fetches and checks out the latest version
func (b *Builder) Update(server *mcp.Server) error {
	dir := b.ServerDir(server)

	// Fetch latest
	fetchCmd := exec.Command("git", "fetch", "--depth", "1", "origin")
	fetchCmd.Dir = dir
	fetchCmd.Stdout = os.Stdout
	fetchCmd.Stderr = os.Stderr
	if err := fetchCmd.Run(); err != nil {
		return fmt.Errorf("git fetch failed: %w", err)
	}

	// Checkout the ref or default branch
	ref := server.Source.Ref
	if ref == "" {
		ref = "origin/HEAD"
	}

	checkoutCmd := exec.Command("git", "checkout", ref)
	checkoutCmd.Dir = dir
	checkoutCmd.Stdout = os.Stdout
	checkoutCmd.Stderr = os.Stderr
	if err := checkoutCmd.Run(); err != nil {
		return fmt.Errorf("git checkout failed: %w", err)
	}

	return nil
}

// Build builds the server according to its build configuration
func (b *Builder) Build(server *mcp.Server) error {
	dir := b.ServerDir(server)

	// If explicit build config, use it
	if server.Build != nil {
		return b.runBuildConfig(dir, server.Build)
	}

	// Auto-detect build system
	return b.autoBuild(dir, server)
}

func (b *Builder) runBuildConfig(dir string, build *mcp.BuildConfig) error {
	// Run install command if specified
	if build.Install != "" {
		if err := b.runCommand(dir, build.Install); err != nil {
			return fmt.Errorf("install command failed: %w", err)
		}
	}

	// Run build command if specified
	if build.Build != "" {
		if err := b.runCommand(dir, build.Build); err != nil {
			return fmt.Errorf("build command failed: %w", err)
		}
	}

	return nil
}

func (b *Builder) autoBuild(dir string, server *mcp.Server) error {
	// Check for package.json (Node.js)
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		return b.buildNode(dir, server)
	}

	// Check for go.mod (Go)
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		return b.buildGo(dir, server)
	}

	// Check for Cargo.toml (Rust)
	if _, err := os.Stat(filepath.Join(dir, "Cargo.toml")); err == nil {
		return b.buildRust(dir, server)
	}

	// Check for pyproject.toml or setup.py (Python)
	if _, err := os.Stat(filepath.Join(dir, "pyproject.toml")); err == nil {
		return b.buildPython(dir, server)
	}
	if _, err := os.Stat(filepath.Join(dir, "setup.py")); err == nil {
		return b.buildPython(dir, server)
	}

	return nil // No build needed
}

func (b *Builder) buildNode(dir string, server *mcp.Server) error {
	// Read package.json to check for build script
	pkgJSON, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return fmt.Errorf("failed to read package.json: %w", err)
	}

	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(pkgJSON, &pkg); err != nil {
		return fmt.Errorf("failed to parse package.json: %w", err)
	}

	// Determine package manager
	pm := "npm"
	if _, err := os.Stat(filepath.Join(dir, "pnpm-lock.yaml")); err == nil {
		pm = "pnpm"
	} else if _, err := os.Stat(filepath.Join(dir, "yarn.lock")); err == nil {
		pm = "yarn"
	} else if _, err := os.Stat(filepath.Join(dir, "bun.lockb")); err == nil {
		pm = "bun"
	}

	// Install dependencies
	if err := b.runCommand(dir, pm+" install"); err != nil {
		return fmt.Errorf("%s install failed: %w", pm, err)
	}

	// Run build if available
	if _, ok := pkg.Scripts["build"]; ok {
		if err := b.runCommand(dir, pm+" run build"); err != nil {
			return fmt.Errorf("%s build failed: %w", pm, err)
		}
	}

	// Set command if not already set
	if server.Command == "" {
		if pm == "bun" {
			server.Command = "bun"
		} else {
			server.Command = "node"
		}
	}

	return nil
}

func (b *Builder) buildGo(dir string, server *mcp.Server) error {
	// Build the Go project
	if err := b.runCommand(dir, "go build -o server ."); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}

	// Set command to the built binary
	if server.Command == "" {
		server.Command = filepath.Join(dir, "server")
	}

	return nil
}

func (b *Builder) buildRust(dir string, server *mcp.Server) error {
	// Build the Rust project
	if err := b.runCommand(dir, "cargo build --release"); err != nil {
		return fmt.Errorf("cargo build failed: %w", err)
	}

	// Set command to the built binary
	if server.Command == "" {
		// Try to find the binary name from Cargo.toml
		server.Command = filepath.Join(dir, "target", "release", server.Name)
	}

	return nil
}

func (b *Builder) buildPython(dir string, server *mcp.Server) error {
	// Check for uv
	if _, err := exec.LookPath("uv"); err == nil {
		if err := b.runCommand(dir, "uv sync"); err != nil {
			return fmt.Errorf("uv sync failed: %w", err)
		}
		return nil
	}

	// Fall back to pip
	// Create virtual environment if not exists
	venvDir := filepath.Join(dir, ".venv")
	if _, err := os.Stat(venvDir); os.IsNotExist(err) {
		if err := b.runCommand(dir, "python3 -m venv .venv"); err != nil {
			return fmt.Errorf("venv creation failed: %w", err)
		}
	}

	// Install dependencies
	pipPath := filepath.Join(venvDir, "bin", "pip")
	if _, err := os.Stat(filepath.Join(dir, "pyproject.toml")); err == nil {
		if err := b.runCommand(dir, pipPath+" install -e ."); err != nil {
			return fmt.Errorf("pip install failed: %w", err)
		}
	} else {
		if err := b.runCommand(dir, pipPath+" install -r requirements.txt"); err != nil {
			return fmt.Errorf("pip install failed: %w", err)
		}
	}

	return nil
}

func (b *Builder) runCommand(dir, command string) error {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// Installed checks if a server is already installed
func (b *Builder) Installed(server *mcp.Server) bool {
	dir := b.ServerDir(server)
	_, err := os.Stat(dir)
	return err == nil
}

// Remove removes an installed server
func (b *Builder) Remove(server *mcp.Server) error {
	dir := b.ServerDir(server)
	return os.RemoveAll(dir)
}

// ResolveCommand resolves the command path for a server
func (b *Builder) ResolveCommand(server *mcp.Server) string {
	if server.Source.Type == "local" {
		return server.Command
	}

	dir := b.ServerDir(server)

	// Check for common entry points
	entryPoints := []string{
		filepath.Join(dir, "server"),
		filepath.Join(dir, "dist", "index.js"),
		filepath.Join(dir, "build", "index.js"),
		filepath.Join(dir, "index.js"),
		filepath.Join(dir, "main.py"),
		filepath.Join(dir, "src", "main.py"),
	}

	for _, ep := range entryPoints {
		if _, err := os.Stat(ep); err == nil {
			return ep
		}
	}

	return server.Command
}

// GetCommit returns the current git commit hash for a server
func (b *Builder) GetCommit(server *mcp.Server) (string, error) {
	dir := b.ServerDir(server)

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// GetVersion returns the current version tag for a server, if available
func (b *Builder) GetVersion(server *mcp.Server) (string, error) {
	dir := b.ServerDir(server)

	// Try to get the tag at HEAD
	cmd := exec.Command("git", "describe", "--tags", "--exact-match", "HEAD")
	cmd.Dir = dir

	output, err := cmd.Output()
	if err != nil {
		// No tag at HEAD, that's okay
		return "", nil
	}

	return strings.TrimSpace(string(output)), nil
}
