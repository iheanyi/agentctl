package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iheanyi/agentctl/pkg/builder"
	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/output"
)

var updateCmd = &cobra.Command{
	Use:   "update [server...]",
	Short: "Update installed MCP servers",
	Long: `Update installed MCP servers to their latest versions.

With no arguments, updates all servers. Otherwise, updates only the
specified servers.

Examples:
  agentctl update                  # Update all servers
  agentctl update filesystem       # Update specific server
  agentctl update --check          # Check for updates without applying`,
	RunE: runUpdate,
}

var (
	updateCheck bool
)

func init() {
	updateCmd.Flags().BoolVar(&updateCheck, "check", false, "Check for updates without applying")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	out := output.DefaultWriter()
	b := builder.New(cfg.CacheDir())

	// Determine which servers to update
	var serversToUpdate []string
	if len(args) > 0 {
		// Update specified servers
		for _, name := range args {
			if _, ok := cfg.Servers[name]; !ok {
				out.Warning("Server %q not found, skipping", name)
				continue
			}
			serversToUpdate = append(serversToUpdate, name)
		}
	} else {
		// Update all servers
		for name := range cfg.Servers {
			serversToUpdate = append(serversToUpdate, name)
		}
	}

	if len(serversToUpdate) == 0 {
		out.Println("No servers to update.")
		return nil
	}

	if updateCheck {
		out.Println("Checking for updates...")
	} else {
		out.Println("Updating servers...")
	}

	var updatedCount, errorCount int

	for _, name := range serversToUpdate {
		server := cfg.Servers[name]

		// Only git sources can be updated
		if server.Source.Type != "git" {
			if !updateCheck {
				out.Info("Skipping %q (not a git source)", name)
			}
			continue
		}

		// Check if installed
		if !b.Installed(server) {
			out.Info("Skipping %q (not installed)", name)
			continue
		}

		if updateCheck {
			out.Println("  %s: would update from %s", name, server.Source.URL)
			continue
		}

		out.Println("Updating %s...", name)

		// Update (fetch and checkout)
		if err := b.Update(server); err != nil {
			out.Error("Failed to update %s: %v", name, err)
			errorCount++
			continue
		}

		// Rebuild
		if err := b.Build(server); err != nil {
			out.Error("Failed to build %s: %v", name, err)
			errorCount++
			continue
		}

		out.Success("Updated %s", name)
		updatedCount++
	}

	out.Println("")
	if updateCheck {
		out.Info("Run 'agentctl update' to apply updates")
	} else if errorCount > 0 {
		out.Println("Updated %d server(s) with %d error(s)", updatedCount, errorCount)
	} else if updatedCount > 0 {
		out.Success("Updated %d server(s)", updatedCount)
		out.Info("Run 'agentctl sync' to sync to your tools")
	} else {
		out.Println("No servers were updated")
	}

	return nil
}
