package cli

import (
	"fmt"
	"strings"

	"github.com/iheanyi/agentctl/pkg/aliases"
	"github.com/iheanyi/agentctl/pkg/builder"
	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/mcp"
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

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
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
		fmt.Printf("Server %q is already installed (source: %s)\n", server.Name, existing.Source.URL)
		fmt.Println("Use 'agentctl update' to update, or 'agentctl remove' then reinstall.")
		return nil
	}

	// For git sources, clone and build
	if server.Source.Type == "git" {
		b := builder.New(cfg.CacheDir())

		fmt.Printf("Cloning %s...\n", server.Source.URL)
		if err := b.Clone(server); err != nil {
			return fmt.Errorf("failed to clone: %w", err)
		}

		fmt.Println("Building...")
		if err := b.Build(server); err != nil {
			return fmt.Errorf("failed to build: %w", err)
		}

		// Resolve the command after building
		if server.Command == "" {
			server.Command = b.ResolveCommand(server)
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

	fmt.Printf("Installed %q\n", server.Name)
	fmt.Println("Run 'agentctl sync' to sync to your tools.")

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
