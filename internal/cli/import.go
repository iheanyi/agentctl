package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/iheanyi/agentctl/pkg/command"
	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/iheanyi/agentctl/pkg/output"
	"github.com/iheanyi/agentctl/pkg/rule"
	"github.com/iheanyi/agentctl/pkg/sync"
	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import <tool>",
	Short: "Import configuration from an existing tool",
	Long: `Import MCP servers, commands, or rules from an existing tool.

This reads the tool's configuration and adds the entries to agentctl,
allowing you to manage them centrally.

Use --local to import from workspace configs (e.g., .mcp.json) into
the local .agentctl.json project config.

Examples:
  agentctl import claude           # Import all from Claude Code global config
  agentctl import cursor --servers # Import only servers from Cursor
  agentctl import claude --local   # Import from .mcp.json to .agentctl.json
  agentctl import cline --rules    # Import only rules from Cline`,
	Args: cobra.ExactArgs(1),
	RunE: runImport,
}

var (
	importServers  bool
	importCommands bool
	importRules    bool
	importLocal    bool
)

func init() {
	importCmd.Flags().BoolVar(&importServers, "servers", false, "Import only servers")
	importCmd.Flags().BoolVar(&importCommands, "commands", false, "Import only commands")
	importCmd.Flags().BoolVar(&importRules, "rules", false, "Import only rules")
	importCmd.Flags().BoolVar(&importLocal, "local", false, "Import from workspace config (e.g., .mcp.json) to local .agentctl.json")
}

func runImport(cmd *cobra.Command, args []string) error {
	toolName := args[0]

	out := output.DefaultWriter()

	// Get the adapter for the specified tool
	adapter, ok := sync.Get(toolName)
	if !ok {
		return fmt.Errorf("unknown tool %q", toolName)
	}

	// Check if tool is installed
	detected, err := adapter.Detect()
	if err != nil {
		return fmt.Errorf("failed to detect %s: %w", toolName, err)
	}
	if !detected {
		return fmt.Errorf("%s is not installed or not detected", toolName)
	}

	// Handle local import
	if importLocal {
		return runLocalImport(adapter, out)
	}

	// Load global config for regular import
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	out.Println("Importing from %s...", adapter.Name())

	// Determine what to import (default: all)
	importAll := !importServers && !importCommands && !importRules

	supported := adapter.SupportedResources()
	var serverCount, commandCount, ruleCount int

	// Import servers
	if (importAll || importServers) && containsResourceType(supported, sync.ResourceMCP) {
		sa, ok := sync.AsServerAdapter(adapter)
		if !ok {
			out.Warning("Adapter doesn't support reading servers")
		} else if servers, err := sa.ReadServers(); err != nil {
			out.Warning("Failed to read servers: %v", err)
		} else {
			for _, server := range servers {
				// Skip already managed servers
				if server.Name == "" {
					continue
				}

				// Check if already exists
				if _, exists := cfg.Servers[server.Name]; exists {
					out.Info("Skipping %q (already exists)", server.Name)
					continue
				}

				// Add to config
				if cfg.Servers == nil {
					cfg.Servers = make(map[string]*mcp.Server)
				}
				cfg.Servers[server.Name] = server
				serverCount++
			}
		}
	}

	// Import commands
	if (importAll || importCommands) && containsResourceType(supported, sync.ResourceCommands) {
		ca, ok := sync.AsCommandsAdapter(adapter)
		if !ok {
			out.Warning("Adapter doesn't support reading commands")
		} else if commands, err := ca.ReadCommands(); err != nil {
			out.Warning("Failed to read commands: %v", err)
		} else {
			commandsDir := filepath.Join(cfg.ConfigDir, "commands")
			for _, cmd := range commands {
				// Check if already exists
				existingPath := filepath.Join(commandsDir, cmd.Name+".json")
				if command.Exists(existingPath) {
					out.Info("Skipping command %q (already exists)", cmd.Name)
					continue
				}

				// Save command
				if err := command.Save(cmd, commandsDir); err != nil {
					out.Warning("Failed to save command %q: %v", cmd.Name, err)
					continue
				}
				commandCount++
			}
		}
	}

	// Import rules
	if (importAll || importRules) && containsResourceType(supported, sync.ResourceRules) {
		ra, ok := sync.AsRulesAdapter(adapter)
		if !ok {
			out.Warning("Adapter doesn't support reading rules")
		} else if rules, err := ra.ReadRules(); err != nil {
			out.Warning("Failed to read rules: %v", err)
		} else {
			rulesDir := filepath.Join(cfg.ConfigDir, "rules")
			for _, r := range rules {
				name := filepath.Base(r.Path)
				if name == "" || name == "." {
					name = "imported-rule"
				}

				// Save rule
				if err := rule.Save(r, rulesDir); err != nil {
					out.Warning("Failed to save rule %q: %v", name, err)
					continue
				}
				ruleCount++
			}
		}
	}

	// Save config if we imported anything
	if serverCount > 0 {
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	}

	// Summary
	out.Println("")
	if serverCount > 0 {
		out.Success("Imported %d server(s)", serverCount)
	}
	if commandCount > 0 {
		out.Success("Imported %d command(s)", commandCount)
	}
	if ruleCount > 0 {
		out.Success("Imported %d rule(s)", ruleCount)
	}

	if serverCount == 0 && commandCount == 0 && ruleCount == 0 {
		out.Info("No new resources to import")
	} else {
		out.Println("")
		out.Info("Run 'agentctl sync' to sync to all tools")
	}

	return nil
}

// runLocalImport imports from a tool's workspace config to local .agentctl.json
func runLocalImport(adapter sync.Adapter, out *output.Writer) error {
	// Check if adapter supports workspace configs
	wa, ok := sync.AsWorkspaceAdapter(adapter)
	if !ok {
		return fmt.Errorf("%s doesn't support workspace configs", adapter.Name())
	}

	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Check if workspace config exists
	workspacePath := wa.WorkspaceConfigPath(cwd)
	if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
		return fmt.Errorf("no workspace config found at %s", workspacePath)
	}

	out.Println("Importing from %s workspace config...", adapter.Name())
	out.Println("  Source: %s", workspacePath)

	// Read servers from workspace config
	servers, err := wa.ReadWorkspaceServers(cwd)
	if err != nil {
		return fmt.Errorf("failed to read workspace servers: %w", err)
	}

	if len(servers) == 0 {
		out.Info("No servers found in workspace config")
		return nil
	}

	// Load or create local config
	cfg, err := config.LoadScoped(config.ScopeLocal)
	if err != nil {
		return fmt.Errorf("failed to load local config: %w", err)
	}

	// Import servers
	var serverCount int
	for _, server := range servers {
		if server.Name == "" {
			continue
		}

		// Check if already exists
		if _, exists := cfg.Servers[server.Name]; exists {
			out.Info("Skipping %q (already exists)", server.Name)
			continue
		}

		// Add to config
		if cfg.Servers == nil {
			cfg.Servers = make(map[string]*mcp.Server)
		}
		server.Scope = string(config.ScopeLocal)
		cfg.Servers[server.Name] = server
		serverCount++
		out.Success("Imported %q", server.Name)
	}

	if serverCount == 0 {
		out.Info("No new servers to import")
		return nil
	}

	// Save to local .agentctl.json
	if err := cfg.SaveScoped(config.ScopeLocal); err != nil {
		return fmt.Errorf("failed to save local config: %w", err)
	}

	out.Println("")
	out.Success("Imported %d server(s) to .agentctl.json", serverCount)
	out.Info("Run 'agentctl sync' to sync to workspace configs")

	return nil
}
