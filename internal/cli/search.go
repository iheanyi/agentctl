package cli

import (
	"fmt"

	"github.com/iheanyi/agentctl/pkg/aliases"
	"github.com/iheanyi/agentctl/pkg/output"
	"github.com/iheanyi/agentctl/pkg/registry"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search for MCP servers",
	Long: `Search for MCP servers on mcp.so and in bundled aliases.

Examples:
  agentctl search filesystem    # Search for filesystem-related servers
  agentctl search github        # Search for GitHub integrations
  agentctl search database      # Search for database servers`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

var (
	searchLimit int
)

func init() {
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "l", 10, "Maximum number of results")
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := args[0]
	out := output.DefaultWriter()

	// First search bundled aliases
	out.Println("Bundled aliases:")
	aliasMatches := aliases.Search(query)
	if len(aliasMatches) == 0 {
		out.Println("  No matches found")
	} else {
		table := output.NewTable("Name", "Description", "Runtime")
		for _, a := range aliasMatches {
			table.AddRow(a.Name, a.Description, a.Runtime)
		}
		table.Render()
	}

	out.Println("")

	// Then search mcp.so
	out.Println("mcp.so registry:")
	client := registry.NewMCPSoClient()
	results, err := client.Search(query)
	if err != nil {
		out.Warning("Could not search mcp.so: %v", err)
		out.Info("You can still install servers by URL: agentctl install github.com/org/repo")
		return nil
	}

	if len(results.Results) == 0 {
		out.Println("  No matches found")
		return nil
	}

	table := output.NewTable("Name", "Description", "Author")
	count := 0
	for _, r := range results.Results {
		if count >= searchLimit {
			break
		}
		// Truncate description
		desc := r.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		table.AddRow(r.Name, desc, r.Author)
		count++
	}
	table.Render()

	if results.Total > searchLimit {
		fmt.Printf("\n  ... and %d more results\n", results.Total-searchLimit)
	}

	out.Println("")
	out.Info("Install a server with: agentctl install <name> or agentctl install <url>")

	return nil
}
