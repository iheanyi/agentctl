package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/iheanyi/agentctl/pkg/command"
	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/iheanyi/agentctl/pkg/output"
	"github.com/iheanyi/agentctl/pkg/rule"
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

Scope:
  By default, syncs all servers from both local and global configs.
  Use --scope to sync only specific scope.

  Local servers sync to workspace configs (.mcp.json, .cursor/mcp.json)
  for tools that support it, falling back to global config with a warning.

  Global servers always sync to global tool configs.

Examples:
  agentctl sync                  # Sync all (local + global)
  agentctl sync --scope local    # Sync only local servers to workspace configs
  agentctl sync --scope global   # Sync only global servers to global configs
  agentctl sync --tool claude    # Sync only to Claude Code
  agentctl sync --dry-run        # Preview changes without applying
  agentctl sync --verbose        # Show detailed sync information`,
	RunE: runSync,
}

var (
	syncTool    string
	syncDryRun  bool
	syncClean   bool
	syncVerbose bool
	syncScope   string
)

func init() {
	syncCmd.Flags().StringVarP(&syncTool, "tool", "t", "", "Sync to specific tool only")
	syncCmd.Flags().BoolVarP(&syncDryRun, "dry-run", "n", false, "Preview changes without applying")
	syncCmd.Flags().BoolVar(&syncClean, "clean", false, "Remove stale managed entries")
	syncCmd.Flags().BoolVarP(&syncVerbose, "verbose", "v", false, "Show detailed sync information")
	syncCmd.Flags().StringVarP(&syncScope, "scope", "s", "", "Sync scope: local, global, or all (default: all)")
}

func runSync(cmd *cobra.Command, args []string) error {
	// Parse scope filter
	var scope config.Scope
	if syncScope != "" {
		var err error
		scope, err = config.ParseScope(syncScope)
		if err != nil {
			if JSONOutput {
				jw := output.NewJSONWriter()
				return jw.WriteError(err)
			}
			return err
		}
	} else {
		scope = config.ScopeAll
	}

	// Load config (including project config if present)
	cfg, err := config.LoadWithProject()
	if err != nil {
		if JSONOutput {
			jw := output.NewJSONWriter()
			return jw.WriteError(fmt.Errorf("failed to load config: %w", err))
		}
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Show project config notice if applicable (not in JSON mode)
	if cfg.ProjectPath != "" && !JSONOutput {
		fmt.Printf("Using project config: %s\n\n", cfg.ProjectPath)
	}

	// Get active servers filtered by scope
	var servers []*mcp.Server
	if scope == config.ScopeAll {
		servers = cfg.ActiveServers()
	} else {
		servers = cfg.ServersForScope(scope)
	}

	// Separate local and global servers for scoped sync
	var localServers, globalServers []*mcp.Server
	for _, s := range servers {
		if s.Scope == string(config.ScopeLocal) {
			localServers = append(localServers, s)
		} else {
			globalServers = append(globalServers, s)
		}
	}

	commands := cfg.LoadedCommands
	rules := cfg.LoadedRules

	if len(servers) == 0 && len(commands) == 0 && len(rules) == 0 {
		if JSONOutput {
			jw := output.NewJSONWriter()
			return jw.WriteSuccess(output.SyncOutput{
				DryRun:      syncDryRun,
				ProjectPath: cfg.ProjectPath,
				ToolResults: []output.SyncToolResult{},
				Summary: output.SyncSummary{
					ToolsSucceeded: 0,
					ToolsFailed:    0,
					TotalServers:   0,
					TotalCommands:  0,
					TotalRules:     0,
				},
			})
		}
		fmt.Println("No resources to sync.")
		fmt.Println("Use 'agentctl install <server>' to add servers.")
		fmt.Println("Use 'agentctl import <tool>' to import existing config.")
		return nil
	}

	// Show verbose summary of resources to sync (not in JSON mode)
	if syncVerbose && !JSONOutput {
		fmt.Println("Resources to sync:")
		if len(servers) > 0 {
			fmt.Printf("\nServers (%d):\n", len(servers))
			printVerboseServers(servers, "  ")
		}
		if len(commands) > 0 {
			fmt.Printf("\nCommands (%d):\n", len(commands))
			printVerboseCommands(commands, "  ")
		}
		if len(rules) > 0 {
			fmt.Printf("\nRules (%d):\n", len(rules))
			printVerboseRules(rules, "  ")
		}
		fmt.Println()
	}

	// Get adapters to sync to
	var adapters []sync.Adapter
	if syncTool != "" {
		adapter, ok := sync.Get(syncTool)
		if !ok {
			err := fmt.Errorf("unknown tool %q", syncTool)
			if JSONOutput {
				jw := output.NewJSONWriter()
				return jw.WriteError(err)
			}
			return err
		}
		adapters = []sync.Adapter{adapter}
	} else {
		adapters = sync.Detected()
	}

	if len(adapters) == 0 {
		if JSONOutput {
			jw := output.NewJSONWriter()
			return jw.WriteSuccess(output.SyncOutput{
				DryRun:      syncDryRun,
				ProjectPath: cfg.ProjectPath,
				ToolResults: []output.SyncToolResult{},
				Summary: output.SyncSummary{
					ToolsSucceeded: 0,
					ToolsFailed:    0,
					TotalServers:   len(servers),
					TotalCommands:  len(commands),
					TotalRules:     len(rules),
				},
			})
		}
		fmt.Println("No supported tools detected.")
		fmt.Println("\nSupported tools: Claude Code, Cursor, Codex, OpenCode, Cline, Windsurf, Zed, Continue")
		return nil
	}

	if syncDryRun && !JSONOutput {
		fmt.Println("Dry run - no changes will be made")
	}

	// Load sync state for diff computation
	state, stateErr := sync.LoadState()
	if stateErr != nil {
		state = nil // Continue without state if loading fails
	}

	// JSON output tracking
	var toolResults []output.SyncToolResult

	// Sync to each adapter
	var successCount, errorCount int
	for _, adapter := range adapters {
		detected, err := adapter.Detect()
		if err != nil || !detected {
			continue
		}

		if !JSONOutput {
			fmt.Printf("Syncing to %s...\n", adapter.Name())
		}

		// Show config path in verbose mode
		if syncVerbose && !JSONOutput {
			fmt.Printf("  Config: %s\n", adapter.ConfigPath())
		}

		// Get supported resources
		supported := adapter.SupportedResources()

		// Track tool result for JSON output
		toolResult := output.SyncToolResult{
			Tool:       adapter.Name(),
			ConfigPath: adapter.ConfigPath(),
			Success:    true,
		}

		if syncDryRun {
			if containsResourceType(supported, sync.ResourceMCP) && len(servers) > 0 {
				// Read existing servers and compute diff
				sa, saOk := sync.AsServerAdapter(adapter)
				var existingServers []*mcp.Server
				var readErr error
				if saOk {
					existingServers, readErr = sa.ReadServers()
				}
				var managedNames []string
				if state != nil {
					managedNames = state.GetManagedServers(adapter.Name())
				}

				if readErr == nil {
					diff := computeServerDiff(existingServers, servers, managedNames)

					// Track changes for JSON
					toolResult.ServersAdded = len(diff.toAdd)
					toolResult.ServersUpdated = len(diff.toUpdate)
					toolResult.ServersRemoved = len(diff.toRemove)

					for _, s := range diff.toAdd {
						name := s.Name
						if s.Namespace != "" {
							name = s.Namespace
						}
						toolResult.Changes = append(toolResult.Changes, output.SyncChange{
							Type:     "add",
							Resource: "server",
							Name:     name,
						})
					}
					for _, s := range diff.toUpdate {
						name := s.Name
						if s.Namespace != "" {
							name = s.Namespace
						}
						toolResult.Changes = append(toolResult.Changes, output.SyncChange{
							Type:     "update",
							Resource: "server",
							Name:     name,
						})
					}
					for _, name := range diff.toRemove {
						toolResult.Changes = append(toolResult.Changes, output.SyncChange{
							Type:     "remove",
							Resource: "server",
							Name:     name,
						})
					}
					for _, s := range diff.unmanaged {
						name := s.Name
						if s.Namespace != "" {
							name = s.Namespace
						}
						toolResult.Changes = append(toolResult.Changes, output.SyncChange{
							Type:     "preserve",
							Resource: "server",
							Name:     name,
						})
					}

					if !JSONOutput {
						if syncVerbose {
							printServerDiff(diff, "  ")
						} else {
							// Show summary in non-verbose mode
							fmt.Printf("  Would sync %d server(s)", len(servers))
							if len(diff.toAdd) > 0 {
								fmt.Printf(" (+%d new)", len(diff.toAdd))
							}
							if len(diff.toUpdate) > 0 {
								fmt.Printf(" (~%d update)", len(diff.toUpdate))
							}
							if len(diff.unmanaged) > 0 {
								fmt.Printf(" (=%d preserved)", len(diff.unmanaged))
							}
							fmt.Println()
						}
					}
				} else {
					toolResult.ServersAdded = len(servers)
					if !JSONOutput {
						fmt.Printf("  Would sync %d server(s)\n", len(servers))
						if syncVerbose {
							printVerboseServers(servers, "    ")
						}
					}
				}
			}
			if containsResourceType(supported, sync.ResourceCommands) && len(commands) > 0 {
				toolResult.CommandsSynced = len(commands)
				if !JSONOutput {
					fmt.Printf("  Would sync %d command(s)\n", len(commands))
					if syncVerbose {
						printVerboseCommands(commands, "    ")
					}
				}
			}
			if containsResourceType(supported, sync.ResourceRules) && len(rules) > 0 {
				toolResult.RulesSynced = len(rules)
				if !JSONOutput {
					fmt.Printf("  Would sync %d rule(s)\n", len(rules))
					if syncVerbose {
						printVerboseRules(rules, "    ")
					}
				}
			}
			toolResults = append(toolResults, toolResult)
			successCount++
			continue
		}

		var syncedAny bool

		// Sync servers if supported
		if containsResourceType(supported, sync.ResourceMCP) && len(servers) > 0 {
			// Get project directory for workspace configs
			projectDir := cfg.ProjectDir()
			if projectDir == "" {
				if cwd, err := os.Getwd(); err == nil {
					projectDir = cwd
				}
			}

			// Check if adapter supports workspace configs
			wa, hasWorkspace := sync.AsWorkspaceAdapter(adapter)

			// Sync local servers to workspace config if supported
			if len(localServers) > 0 && hasWorkspace && projectDir != "" {
				workspacePath := wa.WorkspaceConfigPath(projectDir)
				if err := wa.WriteWorkspaceServers(projectDir, localServers); err != nil {
					if !JSONOutput {
						fmt.Printf("  Error syncing local servers to workspace: %v\n", err)
					}
					toolResult.Success = false
					toolResult.Error = fmt.Sprintf("Error syncing local servers: %v", err)
					errorCount++
				} else {
					if !JSONOutput {
						fmt.Printf("  Synced %d local server(s) to %s\n", len(localServers), workspacePath)
						if syncVerbose {
							printVerboseServers(localServers, "    ")
						}
					}
					toolResult.ServersAdded += len(localServers)
					syncedAny = true
				}
			} else if len(localServers) > 0 {
				// Tool doesn't support workspace configs - warn and sync to global
				if !JSONOutput {
					fmt.Printf("  Warning: %s doesn't support workspace configs\n", adapter.Name())
					fmt.Printf("  Syncing %d local server(s) to global config\n", len(localServers))
				}
				globalServers = append(globalServers, localServers...)
			}

			// Sync global servers to global config
			if len(globalServers) > 0 {
				sa, ok := sync.AsServerAdapter(adapter)
				if !ok {
					if !JSONOutput {
						fmt.Printf("  Error: adapter doesn't support servers\n")
					}
					toolResult.Success = false
					toolResult.Error = "Adapter doesn't support servers"
					errorCount++
				} else if err := sa.WriteServers(globalServers); err != nil {
					if !JSONOutput {
						fmt.Printf("  Error syncing global servers: %v\n", err)
					}
					toolResult.Success = false
					toolResult.Error = fmt.Sprintf("Error syncing global servers: %v", err)
					errorCount++
				} else {
					if !JSONOutput {
						fmt.Printf("  Synced %d global server(s)\n", len(globalServers))
						if syncVerbose {
							printVerboseServers(globalServers, "    ")
						}
					}
					toolResult.ServersAdded += len(globalServers)
					syncedAny = true
				}
			}
		}

		// Sync commands if supported
		if containsResourceType(supported, sync.ResourceCommands) && len(commands) > 0 {
			ca, ok := sync.AsCommandsAdapter(adapter)
			if !ok {
				if !JSONOutput {
					fmt.Printf("  Error: adapter doesn't support commands\n")
				}
				toolResult.Error = "Adapter doesn't support commands"
			} else if err := ca.WriteCommands(commands); err != nil {
				if !JSONOutput {
					fmt.Printf("  Error syncing commands: %v\n", err)
				}
				toolResult.Error = fmt.Sprintf("Error syncing commands: %v", err)
			} else {
				if !JSONOutput {
					fmt.Printf("  Synced %d command(s)\n", len(commands))
					if syncVerbose {
						printVerboseCommands(commands, "    ")
					}
				}
				toolResult.CommandsSynced = len(commands)
				syncedAny = true
			}
		}

		// Sync rules if supported
		if containsResourceType(supported, sync.ResourceRules) && len(rules) > 0 {
			ra, ok := sync.AsRulesAdapter(adapter)
			if !ok {
				if !JSONOutput {
					fmt.Printf("  Error: adapter doesn't support rules\n")
				}
				toolResult.Error = "Adapter doesn't support rules"
			} else if err := ra.WriteRules(rules); err != nil {
				if !JSONOutput {
					fmt.Printf("  Error syncing rules: %v\n", err)
				}
				toolResult.Error = fmt.Sprintf("Error syncing rules: %v", err)
			} else {
				if !JSONOutput {
					fmt.Printf("  Synced %d rule(s)\n", len(rules))
					if syncVerbose {
						printVerboseRules(rules, "    ")
					}
				}
				toolResult.RulesSynced = len(rules)
				syncedAny = true
			}
		}

		toolResults = append(toolResults, toolResult)
		if syncedAny {
			successCount++
		}
	}

	// JSON output
	if JSONOutput {
		jw := output.NewJSONWriter()
		return jw.WriteSuccess(output.SyncOutput{
			DryRun:      syncDryRun,
			ProjectPath: cfg.ProjectPath,
			ToolResults: toolResults,
			Summary: output.SyncSummary{
				ToolsSucceeded: successCount,
				ToolsFailed:    errorCount,
				TotalServers:   len(servers),
				TotalCommands:  len(commands),
				TotalRules:     len(rules),
			},
		})
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

// printVerboseServers prints detailed server information
func printVerboseServers(servers []*mcp.Server, indent string) {
	for _, s := range servers {
		name := s.Name
		if s.Namespace != "" {
			name = s.Namespace
		}
		fmt.Printf("%s• %s\n", indent, name)
		if s.URL != "" {
			fmt.Printf("%s    URL: %s\n", indent, s.URL)
		} else {
			cmdLine := s.Command
			if len(s.Args) > 0 {
				cmdLine += " " + strings.Join(s.Args, " ")
			}
			fmt.Printf("%s    Command: %s\n", indent, cmdLine)
		}
		if len(s.Env) > 0 {
			var envKeys []string
			for k := range s.Env {
				envKeys = append(envKeys, k)
			}
			fmt.Printf("%s    Env: %s\n", indent, strings.Join(envKeys, ", "))
		}
	}
}

// serverDiff represents the diff between existing and new servers
type serverDiff struct {
	toAdd      []*mcp.Server // New servers to add
	toUpdate   []*mcp.Server // Existing managed servers to update
	unmanaged  []*mcp.Server // Existing unmanaged servers (won't touch)
	toRemove   []string      // Managed server names that would be removed
}

// computeServerDiff computes what would change when syncing servers
func computeServerDiff(existing []*mcp.Server, incoming []*mcp.Server, managedNames []string) serverDiff {
	diff := serverDiff{}

	// Build lookup maps
	existingByName := make(map[string]*mcp.Server)
	for _, s := range existing {
		name := s.Name
		if s.Namespace != "" {
			name = s.Namespace
		}
		existingByName[name] = s
	}

	managedSet := make(map[string]bool)
	for _, name := range managedNames {
		managedSet[name] = true
	}

	incomingByName := make(map[string]*mcp.Server)
	for _, s := range incoming {
		name := s.Name
		if s.Namespace != "" {
			name = s.Namespace
		}
		incomingByName[name] = s
	}

	// Categorize incoming servers
	for _, s := range incoming {
		name := s.Name
		if s.Namespace != "" {
			name = s.Namespace
		}
		if _, exists := existingByName[name]; exists {
			diff.toUpdate = append(diff.toUpdate, s)
		} else {
			diff.toAdd = append(diff.toAdd, s)
		}
	}

	// Find unmanaged and to-be-removed servers
	for name, s := range existingByName {
		if _, inIncoming := incomingByName[name]; !inIncoming {
			if managedSet[name] {
				diff.toRemove = append(diff.toRemove, name)
			} else {
				diff.unmanaged = append(diff.unmanaged, s)
			}
		}
	}

	return diff
}

// printServerDiff prints a diff-style view of server changes
func printServerDiff(diff serverDiff, indent string) {
	if len(diff.toAdd) > 0 {
		fmt.Printf("%s[+] Adding %d server(s):\n", indent, len(diff.toAdd))
		for _, s := range diff.toAdd {
			name := s.Name
			if s.Namespace != "" {
				name = s.Namespace
			}
			fmt.Printf("%s    + %s\n", indent, name)
		}
	}

	if len(diff.toUpdate) > 0 {
		fmt.Printf("%s[~] Updating %d server(s):\n", indent, len(diff.toUpdate))
		for _, s := range diff.toUpdate {
			name := s.Name
			if s.Namespace != "" {
				name = s.Namespace
			}
			fmt.Printf("%s    ~ %s\n", indent, name)
		}
	}

	if len(diff.toRemove) > 0 {
		fmt.Printf("%s[-] Removing %d stale server(s):\n", indent, len(diff.toRemove))
		for _, name := range diff.toRemove {
			fmt.Printf("%s    - %s\n", indent, name)
		}
	}

	if len(diff.unmanaged) > 0 {
		fmt.Printf("%s[=] Preserving %d unmanaged server(s):\n", indent, len(diff.unmanaged))
		for _, s := range diff.unmanaged {
			name := s.Name
			if s.Namespace != "" {
				name = s.Namespace
			}
			fmt.Printf("%s    = %s\n", indent, name)
		}
	}
}

// printVerboseCommands prints detailed command information
func printVerboseCommands(commands []*command.Command, indent string) {
	for _, c := range commands {
		fmt.Printf("%s• %s\n", indent, c.Name)
		if c.Description != "" {
			fmt.Printf("%s    Description: %s\n", indent, c.Description)
		}
	}
}

// printVerboseRules prints detailed rule information
func printVerboseRules(rules []*rule.Rule, indent string) {
	for _, r := range rules {
		name := r.Name
		if name == "" && r.Path != "" {
			name = r.Path
		}
		if name == "" {
			name = "(unnamed)"
		}
		fmt.Printf("%s• %s\n", indent, name)
		// Show first line of content as preview
		lines := strings.Split(r.Content, "\n")
		if len(lines) > 0 {
			preview := strings.TrimSpace(lines[0])
			if len(preview) > 60 {
				preview = preview[:57] + "..."
			}
			if preview != "" {
				fmt.Printf("%s    Preview: %s\n", indent, preview)
			}
		}
	}
}
