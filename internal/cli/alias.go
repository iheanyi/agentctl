package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/iheanyi/agentctl/pkg/aliases"
	"github.com/spf13/cobra"
)

var aliasCmd = &cobra.Command{
	Use:   "alias",
	Short: "Manage server aliases",
	Long: `Manage short name aliases for MCP servers.

Aliases let you use short names like 'filesystem' instead of full
git URLs when installing servers.

Examples:
  agentctl alias list                               # List all aliases
  agentctl alias add mydb github.com/org/db-mcp    # Add custom alias
  agentctl alias remove mydb                        # Remove custom alias`,
}

var aliasListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all aliases",
	RunE:  runAliasList,
}

var aliasAddCmd = &cobra.Command{
	Use:   "add <name> <url>",
	Short: "Add a custom alias",
	Args:  cobra.ExactArgs(2),
	RunE:  runAliasAdd,
}

var aliasRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"rm"},
	Short:   "Remove a custom alias",
	Args:    cobra.ExactArgs(1),
	RunE:    runAliasRemove,
}

var (
	aliasDescription string
	aliasRuntime     string
)

func init() {
	aliasCmd.AddCommand(aliasListCmd)
	aliasCmd.AddCommand(aliasAddCmd)
	aliasCmd.AddCommand(aliasRemoveCmd)

	aliasAddCmd.Flags().StringVarP(&aliasDescription, "description", "d", "", "Description for the alias")
	aliasAddCmd.Flags().StringVarP(&aliasRuntime, "runtime", "r", "node", "Runtime (node, python, go, docker)")
}

func runAliasList(cmd *cobra.Command, args []string) error {
	store := aliases.Default()

	bundled := store.ListBundled()
	user := store.ListUser()

	if len(bundled) == 0 && len(user) == 0 {
		fmt.Println("No aliases available.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	if len(bundled) > 0 {
		fmt.Println("Bundled aliases:")
		fmt.Fprintln(w, "  NAME\tRUNTIME\tDESCRIPTION")
		for name, alias := range bundled {
			runtime := alias.Runtime
			if runtime == "" {
				runtime = "node"
			}
			desc := alias.Description
			if len(desc) > 50 {
				desc = desc[:47] + "..."
			}
			fmt.Fprintf(w, "  %s\t%s\t%s\n", name, runtime, desc)
		}
		w.Flush()
	}

	if len(user) > 0 {
		if len(bundled) > 0 {
			fmt.Println()
		}
		fmt.Println("User aliases:")
		fmt.Fprintln(w, "  NAME\tURL")
		for name, alias := range user {
			url := alias.URL
			if len(url) > 50 {
				url = url[:47] + "..."
			}
			fmt.Fprintf(w, "  %s\t%s\n", name, url)
		}
		w.Flush()
	}

	return nil
}

func runAliasAdd(cmd *cobra.Command, args []string) error {
	name := args[0]
	url := args[1]

	store := aliases.Default()

	// Check if it would override a bundled alias
	if store.IsBundled(name) {
		fmt.Printf("Note: This will override the bundled alias %q\n", name)
	}

	alias := aliases.Alias{
		URL:         url,
		Description: aliasDescription,
		Runtime:     aliasRuntime,
	}

	if err := store.Add(name, alias); err != nil {
		return fmt.Errorf("failed to add alias: %w", err)
	}

	fmt.Printf("Added alias %q -> %s\n", name, url)
	return nil
}

func runAliasRemove(cmd *cobra.Command, args []string) error {
	name := args[0]

	store := aliases.Default()

	if !store.IsUser(name) {
		if store.IsBundled(name) {
			return fmt.Errorf("%q is a bundled alias and cannot be removed", name)
		}
		return fmt.Errorf("alias %q does not exist", name)
	}

	if err := store.Remove(name); err != nil {
		return fmt.Errorf("failed to remove alias: %w", err)
	}

	fmt.Printf("Removed alias %q\n", name)
	return nil
}
