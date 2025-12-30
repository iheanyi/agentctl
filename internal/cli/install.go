package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/iheanyi/agentctl/pkg/aliases"
	"github.com/iheanyi/agentctl/pkg/builder"
	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/lockfile"
	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/iheanyi/agentctl/pkg/output"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install <server>",
	Short: "Install an MCP server",
	Long: `Install an MCP server by alias, git URL, or local path.

Examples:
  agentctl install filesystem                # Install by alias
  agentctl install filesystem@v1.0.0         # Install specific version
  agentctl install github.com/org/mcp-server # Install from git
  agentctl install ./local/mcp               # Install from local path`,
	Args: cobra.ExactArgs(1),
	RunE: runInstall,
}

var (
	installNamespace string
)

func init() {
	installCmd.Flags().StringVarP(&installNamespace, "namespace", "n", "", "Namespace prefix for tool names")
}

func runInstall(cmd *cobra.Command, args []string) error {
	target := args[0]
	out := output.DefaultWriter()

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Load lockfile
	lf, err := lockfile.Load(cfg.ConfigDir)
	if err != nil {
		return fmt.Errorf("failed to load lockfile: %w", err)
	}

	// Parse the target
	server, err := parseInstallTarget(target)
	if err != nil {
		return err
	}

	// Apply namespace if provided
	if installNamespace != "" {
		server.Namespace = installNamespace
	}

	// Check if already installed
	if existing, ok := cfg.Servers[server.Name]; ok {
		out.Warning("Server %q is already installed (source: %s)", server.Name, existing.Source.URL)
		out.Info("Use 'agentctl update' to update, or 'agentctl remove' then reinstall.")
		return nil
	}

	var lockedEntry *lockfile.LockedEntry

	// For git sources, clone and build
	if server.Source.Type == "git" {
		b := builder.New(cfg.CacheDir())

		out.Info("Cloning %s...", server.Source.URL)
		if err := b.Clone(server); err != nil {
			return fmt.Errorf("failed to clone: %w", err)
		}

		out.Info("Building...")
		if err := b.Build(server); err != nil {
			return fmt.Errorf("failed to build: %w", err)
		}

		// Resolve the command after building
		if server.Command == "" {
			server.Command = b.ResolveCommand(server)
		}

		// Get commit hash for lockfile
		commit, _ := b.GetCommit(server)

		// Calculate integrity hash
		serverDir := filepath.Join(cfg.CacheDir(), "downloads", server.Name)
		integrity, _ := lockfile.CalculateIntegrity(serverDir)

		lockedEntry = &lockfile.LockedEntry{
			Source:    server.Source.URL,
			Version:   server.Source.Ref,
			Commit:    commit,
			Integrity: integrity,
		}
	} else if server.Source.Type == "alias" {
		// For aliases, record the source info
		lockedEntry = &lockfile.LockedEntry{
			Source:  server.Source.URL,
			Version: server.Source.Ref,
		}
	}

	// Add to config
	if cfg.Servers == nil {
		cfg.Servers = make(map[string]*mcp.Server)
	}
	cfg.Servers[server.Name] = server

	// Save config
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Update lockfile
	if lockedEntry != nil {
		lf.Lock(server.Name, lockedEntry)
		if err := lf.Save(); err != nil {
			out.Warning("Failed to save lockfile: %v", err)
		}
	}

	out.Success("Installed %q", server.Name)
	out.Info("Run 'agentctl sync' to sync to your tools.")

	return nil
}

func parseInstallTarget(target string) (*mcp.Server, error) {
	// Check for version suffix (name@version)
	var version string
	if idx := strings.LastIndex(target, "@"); idx > 0 {
		version = target[idx+1:]
		target = target[:idx]
	}

	// Check if it's a local path
	if strings.HasPrefix(target, "./") || strings.HasPrefix(target, "/") || strings.HasPrefix(target, "../") {
		return &mcp.Server{
			Name: pathToName(target),
			Source: mcp.Source{
				Type: "local",
				URL:  target,
			},
			Transport: mcp.TransportStdio,
		}, nil
	}

	// Check if it's a git URL
	if strings.Contains(target, "/") && strings.Contains(target, ".") {
		return &mcp.Server{
			Name: pathToName(target),
			Source: mcp.Source{
				Type: "git",
				URL:  target,
				Ref:  version,
			},
			Transport: mcp.TransportStdio,
		}, nil
	}

	// Try to resolve as alias
	alias, ok := aliases.Resolve(target)
	if !ok {
		return nil, fmt.Errorf("unknown alias %q - use full git URL or 'agentctl search' to find servers", target)
	}

	server := &mcp.Server{
		Name: target,
		Source: mcp.Source{
			Type:  "alias",
			Alias: target,
			URL:   alias.URL,
			Ref:   version,
		},
		Transport: mcp.TransportStdio,
	}

	// Set command based on runtime
	switch alias.Runtime {
	case "node":
		server.Command = "npx"
		server.Args = []string{"-y", target} // Will be resolved to actual package
	case "python":
		server.Command = "uvx"
		server.Args = []string{target}
	case "go":
		server.Command = "go"
		server.Args = []string{"run", alias.URL}
	default:
		// Default to node
		server.Command = "npx"
		server.Args = []string{"-y", target}
	}

	return server, nil
}

func pathToName(path string) string {
	// Extract name from path
	parts := strings.Split(path, "/")
	name := parts[len(parts)-1]

	// Remove common suffixes
	name = strings.TrimSuffix(name, "-mcp")
	name = strings.TrimSuffix(name, "-server")
	name = strings.TrimSuffix(name, ".git")

	return name
}
