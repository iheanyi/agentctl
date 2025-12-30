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

	if len(cfg.Servers) == 0 {
		fmt.Println("No servers configured.")
		fmt.Println("\nGet started:")
		fmt.Println("  agentctl install filesystem")
		return nil
	}

	fmt.Printf("Detected tools: %v\n\n", toolNames)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SERVER\tSTATUS\tSOURCE\tNAMESPACE")

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

		namespace := server.Namespace
		if namespace == "" {
			namespace = "-"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, status, source, namespace)
	}
	w.Flush()

	// Summary
	fmt.Printf("\n%d server(s) configured, %d tool(s) detected\n", len(cfg.Servers), len(detected))

	return nil
}
