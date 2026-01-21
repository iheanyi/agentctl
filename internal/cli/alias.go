package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/iheanyi/agentctl/internal/tui"
	"github.com/iheanyi/agentctl/pkg/aliases"
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
	Use:   "add [name] [url]",
	Short: "Add a custom alias",
	Long: `Add a custom alias for an MCP server.

If no arguments are provided, launches an interactive form.

Examples:
  agentctl alias add                               # Interactive mode
  agentctl alias add mydb github.com/org/db-mcp    # Direct mode
  agentctl alias add myapi --url https://api.example.com/mcp --type http`,
	Args: cobra.MaximumNArgs(2),
	RunE: runAliasAdd,
}

var aliasRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"rm"},
	Short:   "Remove a custom alias",
	Args:    cobra.ExactArgs(1),
	RunE:    runAliasRemove,
}

var aliasManageCmd = &cobra.Command{
	Use:   "manage",
	Short: "Open interactive alias manager TUI",
	Long: `Open an interactive TUI for managing bundled aliases.

This allows you to add, edit, delete, test, and validate aliases
in the bundled aliases.json file.

Note: Run this from the agentctl source directory, or set
AGENTCTL_ALIASES_PATH to point to the aliases.json file.

Examples:
  agentctl alias manage
  AGENTCTL_ALIASES_PATH=./pkg/aliases/aliases.json agentctl alias manage`,
	RunE: runAliasManage,
}

var (
	aliasDescription string
	aliasRuntime     string
	aliasTransport   string
	aliasURL         string
	aliasPackage     string
)

func init() {
	aliasCmd.AddCommand(aliasListCmd)
	aliasCmd.AddCommand(aliasAddCmd)
	aliasCmd.AddCommand(aliasRemoveCmd)
	aliasCmd.AddCommand(aliasManageCmd)

	aliasAddCmd.Flags().StringVarP(&aliasDescription, "description", "d", "", "Description for the alias")
	aliasAddCmd.Flags().StringVarP(&aliasRuntime, "runtime", "r", "", "Runtime (node, python, go, docker) - for stdio transport")
	aliasAddCmd.Flags().StringVarP(&aliasTransport, "type", "t", "", "Transport type (stdio, http, sse)")
	aliasAddCmd.Flags().StringVarP(&aliasURL, "url", "u", "", "URL for http/sse transport")
	aliasAddCmd.Flags().StringVarP(&aliasPackage, "package", "p", "", "Package name for stdio transport (e.g., @org/package)")
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
	store := aliases.Default()

	var name string
	var alias aliases.Alias

	// Check if we have arguments or flags
	hasArgs := len(args) >= 2
	hasFlags := aliasURL != "" || aliasPackage != ""

	// If no args and no flags, use interactive mode
	if len(args) == 0 && !hasFlags {
		if err := requireInteractive("alias add"); err != nil {
			return err
		}

		var err error
		name, alias, err = runInteractiveAliasAdd(store)
		if err != nil {
			if err == huh.ErrUserAborted {
				showCancelHint("alias add")
				return nil
			}
			return err
		}
	} else if hasArgs {
		// Traditional mode: name and url as args
		name = args[0]
		url := args[1]

		alias = aliases.Alias{
			URL:         url,
			Description: aliasDescription,
			Runtime:     aliasRuntime,
		}
	} else if len(args) == 1 {
		// Name provided as arg, check for flags
		name = args[0]

		if aliasURL != "" {
			// HTTP/SSE transport
			transport := aliasTransport
			if transport == "" {
				transport = "http"
			}
			alias = aliases.Alias{
				Transport:   transport,
				MCPURL:      aliasURL,
				Description: aliasDescription,
			}
		} else if aliasPackage != "" {
			// Stdio transport with package
			runtime := aliasRuntime
			if runtime == "" {
				runtime = "node"
			}
			alias = aliases.Alias{
				Package:     aliasPackage,
				Runtime:     runtime,
				Transport:   "stdio",
				Description: aliasDescription,
			}
		} else {
			return fmt.Errorf("either URL argument, --url flag, or --package flag is required")
		}
	} else {
		// No args but has flags
		return fmt.Errorf("alias name is required")
	}

	// Check if it would override a bundled alias
	if store.IsBundled(name) {
		fmt.Printf("Note: This will override the bundled alias %q\n", name)
	}

	if err := store.Add(name, alias); err != nil {
		return fmt.Errorf("failed to add alias: %w", err)
	}

	// Show appropriate success message based on transport
	if alias.MCPURL != "" {
		fmt.Printf("Added alias %q -> %s (%s)\n", name, alias.MCPURL, alias.Transport)
	} else if alias.Package != "" {
		fmt.Printf("Added alias %q -> %s (%s)\n", name, alias.Package, alias.Runtime)
	} else if alias.URL != "" {
		fmt.Printf("Added alias %q -> %s\n", name, alias.URL)
	} else {
		fmt.Printf("Added alias %q\n", name)
	}

	return nil
}

// runInteractiveAliasAdd launches an interactive form to add an alias
func runInteractiveAliasAdd(store *aliases.Store) (string, aliases.Alias, error) {
	var (
		name        string
		transport   string
		runtime     string
		packageName string
		url         string
		description string
	)

	// Step 1: Get alias name
	nameForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Alias name").
				Description("A short name to reference this MCP server").
				Placeholder("e.g., mydb, my-api").
				Value(&name).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("name is required")
					}
					if strings.ContainsAny(s, " \t\n") {
						return fmt.Errorf("name cannot contain whitespace")
					}
					return nil
				}),
		),
	)

	if err := nameForm.Run(); err != nil {
		return "", aliases.Alias{}, err
	}

	// Check if it would override a bundled alias
	if store.IsBundled(name) {
		fmt.Printf("Note: This will override the bundled alias %q\n", name)
	}

	// Step 2: Choose transport type
	transportForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Transport type").
				Description("How does this MCP server communicate?").
				Options(
					huh.NewOption("stdio - Local server with runtime package (npx, uvx)", "stdio"),
					huh.NewOption("http - Remote server with URL", "http"),
					huh.NewOption("sse - Remote server with Server-Sent Events", "sse"),
				).
				Value(&transport),
		),
	)

	if err := transportForm.Run(); err != nil {
		return "", aliases.Alias{}, err
	}

	alias := aliases.Alias{
		Transport: transport,
	}

	// Step 3: Get transport-specific config
	if transport == "stdio" {
		// Ask for runtime and package
		stdioForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Runtime").
					Description("Which runtime to use for this server?").
					Options(
						huh.NewOption("node - Node.js (npx)", "node"),
						huh.NewOption("python - Python (uvx)", "python"),
					).
					Value(&runtime),
				huh.NewInput().
					Title("Package name").
					Description("The npm or PyPI package to run").
					Placeholder("e.g., @org/mcp-server, mcp-server-package").
					Value(&packageName).
					Validate(func(s string) error {
						if s == "" {
							return fmt.Errorf("package name is required")
						}
						return nil
					}),
			),
		)

		if err := stdioForm.Run(); err != nil {
			return "", aliases.Alias{}, err
		}

		alias.Runtime = runtime
		alias.Package = packageName
	} else {
		// http or sse - ask for URL
		urlForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("URL").
					Description("The remote MCP server URL").
					Placeholder("https://mcp.example.com/mcp").
					Value(&url).
					Validate(func(s string) error {
						if s == "" {
							return fmt.Errorf("URL is required")
						}
						if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
							return fmt.Errorf("URL must start with http:// or https://")
						}
						return nil
					}),
			),
		)

		if err := urlForm.Run(); err != nil {
			return "", aliases.Alias{}, err
		}

		alias.MCPURL = url
	}

	// Step 4: Optional description
	descForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Description (optional)").
				Description("A brief description of this MCP server").
				Placeholder("e.g., Database access server").
				Value(&description),
		),
	)

	if err := descForm.Run(); err != nil {
		return "", aliases.Alias{}, err
	}

	alias.Description = description

	return name, alias, nil
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

func runAliasManage(cmd *cobra.Command, args []string) error {
	return tui.RunAliasManager()
}
