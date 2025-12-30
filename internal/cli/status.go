package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/sync"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of all resources",
	Long: `Show the status of all configured MCP servers and other resources.

This shows which servers are installed, their current state, and
which tools they're synced to.`,
	RunE: runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get detected tools
	detected := sync.Detected()
	toolNames := make([]string, len(detected))
	for i, adapter := range detected {
		toolNames[i] = adapter.Name()
	}

	hasResources := len(cfg.Servers) > 0 || len(cfg.LoadedCommands) > 0 || len(cfg.LoadedRules) > 0

	if !hasResources {
		fmt.Println("No resources configured.")
		fmt.Println("\nGet started:")
		fmt.Println("  agentctl install filesystem    # Install an MCP server")
		fmt.Println("  agentctl import claude         # Import from Claude Code")
		return nil
	}

	fmt.Printf("Detected tools: %v\n", toolNames)

	// Show servers
	if len(cfg.Servers) > 0 {
		fmt.Println("\nMCP Servers:")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "  NAME\tSTATUS\tSOURCE")

		for name, server := range cfg.Servers {
			status := "enabled"
			if server.Disabled {
				status = "disabled"
			}

			source := server.Source.Type
			if server.Source.Alias != "" {
				source = "alias:" + server.Source.Alias
			} else if server.Source.URL != "" {
				url := server.Source.URL
				if len(url) > 40 {
					url = url[:37] + "..."
				}
				source = url
			}

			fmt.Fprintf(w, "  %s\t%s\t%s\n", name, status, source)
		}
		w.Flush()
	}

	// Show commands
	if len(cfg.LoadedCommands) > 0 {
		fmt.Println("\nCommands:")
		for _, c := range cfg.LoadedCommands {
			desc := c.Description
			if len(desc) > 50 {
				desc = desc[:47] + "..."
			}
			if desc == "" {
				desc = "(no description)"
			}
			fmt.Printf("  /%s - %s\n", c.Name, desc)
		}
	}

	// Show rules
	if len(cfg.LoadedRules) > 0 {
		fmt.Println("\nRules:")
		for _, r := range cfg.LoadedRules {
			fmt.Printf("  %s\n", r.Name)
		}
	}

	// Summary
	fmt.Println()
	parts := []string{}
	if len(cfg.Servers) > 0 {
		parts = append(parts, fmt.Sprintf("%d server(s)", len(cfg.Servers)))
	}
	if len(cfg.LoadedCommands) > 0 {
		parts = append(parts, fmt.Sprintf("%d command(s)", len(cfg.LoadedCommands)))
	}
	if len(cfg.LoadedRules) > 0 {
		parts = append(parts, fmt.Sprintf("%d rule(s)", len(cfg.LoadedRules)))
	}
	if len(parts) > 0 {
		fmt.Printf("%s, %d tool(s) detected\n", joinParts(parts), len(detected))
	}

	return nil
}

func joinParts(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += ", " + parts[i]
	}
	return result
}
