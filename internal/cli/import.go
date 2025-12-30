package cli

import (
	"fmt"

	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/iheanyi/agentctl/pkg/output"
	"github.com/iheanyi/agentctl/pkg/sync"
	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import <tool>",
	Short: "Import configuration from an existing tool",
	Long: `Import MCP servers, commands, or rules from an existing tool.

This reads the tool's configuration and adds the entries to agentctl,
allowing you to manage them centrally.

Examples:
  agentctl import claude           # Import all from Claude Code
  agentctl import cursor --servers # Import only servers from Cursor
  agentctl import cline --rules    # Import only rules from Cline`,
	Args: cobra.ExactArgs(1),
	RunE: runImport,
}

var (
	importServers  bool
	importCommands bool
	importRules    bool
)

func init() {
	importCmd.Flags().BoolVar(&importServers, "servers", false, "Import only servers")
	importCmd.Flags().BoolVar(&importCommands, "commands", false, "Import only commands")
	importCmd.Flags().BoolVar(&importRules, "rules", false, "Import only rules")
}

func runImport(cmd *cobra.Command, args []string) error {
	toolName := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

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

	out.Println("Importing from %s...", adapter.Name())

	// Determine what to import (default: all)
	importAll := !importServers && !importCommands && !importRules

	supported := adapter.SupportedResources()
	var serverCount, commandCount, ruleCount int

	// Import servers
	if (importAll || importServers) && containsResourceType(supported, sync.ResourceMCP) {
		servers, err := adapter.ReadServers()
		if err != nil {
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
		commands, err := adapter.ReadCommands()
		if err != nil {
			out.Warning("Failed to read commands: %v", err)
		} else {
			commandCount = len(commands)
			// Commands would need to be saved to the commands/ directory
			// For now, just count them
			if commandCount > 0 {
				out.Info("Found %d commands (command import not yet implemented)", commandCount)
				commandCount = 0
			}
		}
	}

	// Import rules
	if (importAll || importRules) && containsResourceType(supported, sync.ResourceRules) {
		rules, err := adapter.ReadRules()
		if err != nil {
			out.Warning("Failed to read rules: %v", err)
		} else {
			ruleCount = len(rules)
			// Rules would need to be saved to the rules/ directory
			// For now, just count them
			if ruleCount > 0 {
				out.Info("Found %d rules (rule import not yet implemented)", ruleCount)
				ruleCount = 0
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
