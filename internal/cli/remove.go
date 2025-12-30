package cli

import (
	"fmt"

	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/lockfile"
	"github.com/iheanyi/agentctl/pkg/output"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:     "remove <server>",
	Aliases: []string{"rm", "uninstall"},
	Short:   "Remove an MCP server",
	Long: `Remove an MCP server from your configuration.

The server will be removed from agentctl's config. Run 'agentctl sync'
to remove it from your tools.

Examples:
  agentctl remove filesystem
  agentctl rm github`,
	Args: cobra.ExactArgs(1),
	RunE: runRemove,
}

func runRemove(cmd *cobra.Command, args []string) error {
	name := args[0]
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

	// Check if server exists
	if _, ok := cfg.Servers[name]; !ok {
		return fmt.Errorf("server %q is not installed", name)
	}

	// Remove from config
	delete(cfg.Servers, name)

	// Save config
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Remove from lockfile
	if lf.IsLocked(name) {
		lf.Unlock(name)
		if err := lf.Save(); err != nil {
			out.Warning("Failed to update lockfile: %v", err)
		}
	}

	out.Success("Removed %q", name)
	out.Info("Run 'agentctl sync' to update your tools.")

	return nil
}
