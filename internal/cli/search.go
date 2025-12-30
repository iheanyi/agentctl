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
	Long: `Search for MCP servers in bundled aliases (verified sources).

Bundled aliases point to official and verified MCP server sources,
primarily from github.com/modelcontextprotocol/servers (Anthropic's official repo).

Use --community to also search mcp.so (third-party, unverified).

Examples:
  agentctl search filesystem         # Search verified aliases
  agentctl search github             # Search for GitHub integrations
  agentctl search database --community  # Include community registry`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

var (
	searchLimit     int
	searchCommunity bool
)

func init() {
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "l", 10, "Maximum number of results")
	searchCmd.Flags().BoolVar(&searchCommunity, "community", false, "Also search mcp.so (third-party, unverified)")
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := args[0]
	out := output.DefaultWriter()

	// Search bundled aliases (verified sources)
	out.Println("Verified aliases (from official sources):")
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

	// Only search mcp.so if --community flag is set
	if searchCommunity {
		out.Println("")
		out.Println("Community registry (mcp.so - unverified):")
		out.Warning("These are third-party servers. Review source code before installing.")
		out.Println("")

		client := registry.NewMCPSoClient()
		results, err := client.Search(query)
		if err != nil {
			out.Warning("Could not search mcp.so: %v", err)
			return nil
		}

		if len(results.Results) == 0 {
			out.Println("  No matches found")
		} else {
			table := output.NewTable("Name", "Description", "Author")
			count := 0
			for _, r := range results.Results {
				if count >= searchLimit {
					break
				}
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
		}
	}

	out.Println("")
	out.Info("Install with: agentctl install <name>")
	out.Info("Or by URL: agentctl install github.com/org/repo")

	if !searchCommunity {
		out.Println("")
		out.Info("Use --community to search mcp.so (third-party, unverified)")
	}

	return nil
}
