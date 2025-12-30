package cli

import (
	"fmt"

	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/sync"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync configuration to tools",
	Long: `Sync your agentctl configuration to all detected tools.

This will update MCP servers, commands, and rules in each tool's
configuration file. Manually added entries (without the agentctl
marker) are preserved.

Examples:
  agentctl sync                  # Sync to all detected tools
  agentctl sync --tool claude    # Sync only to Claude Code
  agentctl sync --dry-run        # Preview changes without applying`,
	RunE: runSync,
}

var (
	syncTool   string
	syncDryRun bool
	syncClean  bool
)

func init() {
	syncCmd.Flags().StringVarP(&syncTool, "tool", "t", "", "Sync to specific tool only")
	syncCmd.Flags().BoolVarP(&syncDryRun, "dry-run", "n", false, "Preview changes without applying")
	syncCmd.Flags().BoolVar(&syncClean, "clean", false, "Remove stale managed entries")
}

func runSync(cmd *cobra.Command, args []string) error {
	// Load config (including project config if present)
	cfg, err := config.LoadWithProject()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Show project config notice if applicable
	if cfg.ProjectPath != "" {
		fmt.Printf("Using project config: %s\n\n", cfg.ProjectPath)
	}

	// Get active servers, commands, and rules
	servers := cfg.ActiveServers()
	commands := cfg.LoadedCommands
	rules := cfg.LoadedRules

	if len(servers) == 0 && len(commands) == 0 && len(rules) == 0 {
		fmt.Println("No resources to sync.")
		fmt.Println("Use 'agentctl install <server>' to add servers.")
		fmt.Println("Use 'agentctl import <tool>' to import existing config.")
		return nil
	}

	// Get adapters to sync to
	var adapters []sync.Adapter
	if syncTool != "" {
		adapter, ok := sync.Get(syncTool)
		if !ok {
			return fmt.Errorf("unknown tool %q", syncTool)
		}
		adapters = []sync.Adapter{adapter}
	} else {
		adapters = sync.Detected()
	}

	if len(adapters) == 0 {
		fmt.Println("No supported tools detected.")
		fmt.Println("\nSupported tools: Claude Code, Cursor, Codex, OpenCode, Cline, Windsurf, Zed, Continue")
		return nil
	}

	if syncDryRun {
		fmt.Println("Dry run - no changes will be made")
	}

	// Sync to each adapter
	var successCount, errorCount int
	for _, adapter := range adapters {
		detected, err := adapter.Detect()
		if err != nil || !detected {
			continue
		}

		fmt.Printf("Syncing to %s...\n", adapter.Name())

		// Get supported resources
		supported := adapter.SupportedResources()

		if syncDryRun {
			if containsResourceType(supported, sync.ResourceMCP) && len(servers) > 0 {
				fmt.Printf("  Would sync %d server(s)\n", len(servers))
			}
			if containsResourceType(supported, sync.ResourceCommands) && len(commands) > 0 {
				fmt.Printf("  Would sync %d command(s)\n", len(commands))
			}
			if containsResourceType(supported, sync.ResourceRules) && len(rules) > 0 {
				fmt.Printf("  Would sync %d rule(s)\n", len(rules))
			}
			successCount++
			continue
		}

		var syncedAny bool

		// Sync servers if supported
		if containsResourceType(supported, sync.ResourceMCP) && len(servers) > 0 {
			if err := adapter.WriteServers(servers); err != nil {
				fmt.Printf("  Error syncing servers: %v\n", err)
				errorCount++
			} else {
				fmt.Printf("  Synced %d server(s)\n", len(servers))
				syncedAny = true
			}
		}

		// Sync commands if supported
		if containsResourceType(supported, sync.ResourceCommands) && len(commands) > 0 {
			if err := adapter.WriteCommands(commands); err != nil {
				fmt.Printf("  Error syncing commands: %v\n", err)
			} else {
				fmt.Printf("  Synced %d command(s)\n", len(commands))
				syncedAny = true
			}
		}

		// Sync rules if supported
		if containsResourceType(supported, sync.ResourceRules) && len(rules) > 0 {
			if err := adapter.WriteRules(rules); err != nil {
				fmt.Printf("  Error syncing rules: %v\n", err)
			} else {
				fmt.Printf("  Synced %d rule(s)\n", len(rules))
				syncedAny = true
			}
		}

		if syncedAny {
			successCount++
		}
	}

	fmt.Println()
	if errorCount > 0 {
		fmt.Printf("Synced to %d tool(s) with %d error(s)\n", successCount, errorCount)
	} else {
		fmt.Printf("Synced to %d tool(s)\n", successCount)
	}

	return nil
}

func containsResourceType(types []sync.ResourceType, target sync.ResourceType) bool {
	for _, t := range types {
		if t == target {
			return true
		}
	}
	return false
}
