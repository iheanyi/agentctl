package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List installed resources",
	Long: `List installed MCP servers, commands, rules, prompts, and skills.

Examples:
  agentctl list                  # List all resources
  agentctl list --type servers   # List only servers
  agentctl list --type commands  # List only commands`,
	RunE: runList,
}

var (
	listType    string
	listProfile string
)

func init() {
	listCmd.Flags().StringVarP(&listType, "type", "t", "", "Filter by resource type (servers, commands, rules, prompts, skills)")
	listCmd.Flags().StringVarP(&listProfile, "profile", "p", "", "List resources from specific profile")
}

func runList(cmd *cobra.Command, args []string) error {
	// Load config (including project config if present)
	cfg, err := config.LoadWithProject()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Show project config notice if applicable
	if cfg.ProjectPath != "" {
		fmt.Printf("Project config: %s\n\n", cfg.ProjectPath)
	}

	// Apply profile filter if specified
	if listProfile != "" {
		// TODO: Apply profile filtering
	}

	hasOutput := false

	// List servers
	if listType == "" || listType == "servers" {
		if len(cfg.Servers) > 0 {
			fmt.Println("MCP Servers:")
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "  NAME\tSOURCE\tSTATUS")
			for name, server := range cfg.Servers {
				status := "enabled"
				if server.Disabled {
					status = "disabled"
				}
				sourceInfo := server.Source.Type
				if server.Source.URL != "" {
					sourceInfo = server.Source.URL
				} else if server.Source.Alias != "" {
					sourceInfo = "alias:" + server.Source.Alias
				}
				fmt.Fprintf(w, "  %s\t%s\t%s\n", name, sourceInfo, status)
			}
			w.Flush()
			hasOutput = true
		}
	}

	// List commands
	if listType == "" || listType == "commands" {
		if len(cfg.Commands) > 0 {
			if hasOutput {
				fmt.Println()
			}
			fmt.Println("Commands:")
			for _, name := range cfg.Commands {
				fmt.Printf("  %s\n", name)
			}
			hasOutput = true
		}
	}

	// List rules
	if listType == "" || listType == "rules" {
		if len(cfg.Rules) > 0 {
			if hasOutput {
				fmt.Println()
			}
			fmt.Println("Rules:")
			for _, name := range cfg.Rules {
				fmt.Printf("  %s\n", name)
			}
			hasOutput = true
		}
	}

	// List prompts
	if listType == "" || listType == "prompts" {
		if len(cfg.Prompts) > 0 {
			if hasOutput {
				fmt.Println()
			}
			fmt.Println("Prompts:")
			for _, name := range cfg.Prompts {
				fmt.Printf("  %s\n", name)
			}
			hasOutput = true
		}
	}

	// List skills
	if listType == "" || listType == "skills" {
		if len(cfg.Skills) > 0 {
			if hasOutput {
				fmt.Println()
			}
			fmt.Println("Skills:")
			for _, name := range cfg.Skills {
				fmt.Printf("  %s\n", name)
			}
			hasOutput = true
		}
	}

	if !hasOutput {
		fmt.Println("No resources installed.")
		fmt.Println("\nGet started:")
		fmt.Println("  agentctl install filesystem  # Install filesystem MCP server")
		fmt.Println("  agentctl search <query>      # Search for MCP servers")
	}

	return nil
}
