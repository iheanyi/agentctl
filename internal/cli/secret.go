package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/iheanyi/agentctl/pkg/output"
	"github.com/iheanyi/agentctl/pkg/secrets"
)

var secretCmd = &cobra.Command{
	Use:   "secret",
	Short: "Manage secrets in system keychain",
	Long: `Store and retrieve secrets using the system keychain.

Secrets can be referenced in MCP server configurations using:
  - $ENV_VAR - environment variable
  - keychain:name - stored secret

Examples:
  agentctl secret set github-token     # Store a secret
  agentctl secret get github-token     # Retrieve a secret
  agentctl secret list                 # List stored secrets
  agentctl secret delete github-token  # Delete a secret`,
}

var secretSetCmd = &cobra.Command{
	Use:   "set [name]",
	Short: "Store a secret in the keychain",
	Long: `Store a secret in the system keychain.

If no name is provided, launches an interactive form.

Examples:
  agentctl secret set                  # Interactive mode
  agentctl secret set github-token     # Prompt for value only
  echo "value" | agentctl secret set github-token  # Piped value`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSecretSet,
}

var secretGetCmd = &cobra.Command{
	Use:   "get <name>",
	Short: "Retrieve a secret from the keychain",
	Args:  cobra.ExactArgs(1),
	RunE:  runSecretGet,
}

var secretListCmd = &cobra.Command{
	Use:   "list",
	Short: "List stored secrets",
	RunE:  runSecretList,
}

var secretDeleteCmd = &cobra.Command{
	Use:     "delete <name>",
	Aliases: []string{"rm"},
	Short:   "Delete a secret from the keychain",
	Args:    cobra.ExactArgs(1),
	RunE:    runSecretDelete,
}

func init() {
	secretCmd.AddCommand(secretSetCmd)
	secretCmd.AddCommand(secretGetCmd)
	secretCmd.AddCommand(secretListCmd)
	secretCmd.AddCommand(secretDeleteCmd)
}

func runSecretSet(cmd *cobra.Command, args []string) error {
	out := output.DefaultWriter()

	var name string
	var value string
	var err error

	// Check if we have a name argument
	if len(args) > 0 {
		name = args[0]
	}

	// Check stdin mode
	stat, _ := os.Stdin.Stat()
	isPiped := (stat.Mode() & os.ModeCharDevice) == 0

	// If no name provided and not piped, use interactive form
	if name == "" && !isPiped {
		if err := requireInteractive("secret set"); err != nil {
			return err
		}

		name, value, err = runInteractiveSecretSet()
		if err != nil {
			if err == huh.ErrUserAborted {
				showCancelHint("secret set")
				return nil
			}
			return err
		}
	} else if name == "" && isPiped {
		return fmt.Errorf("secret name is required when using piped input\nUsage: echo 'value' | agentctl secret set <name>")
	} else {
		// Name provided, just get the value
		if isPiped {
			// Piped input
			reader := bufio.NewReader(os.Stdin)
			value, err = reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read secret: %w", err)
			}
			value = strings.TrimSpace(value)
		} else {
			// Interactive - prompt for password
			fmt.Printf("Enter secret value for %q: ", name)
			byteValue, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Println() // newline after password input
			if err != nil {
				return fmt.Errorf("failed to read secret: %w", err)
			}
			value = string(byteValue)
		}
	}

	if value == "" {
		return fmt.Errorf("secret value cannot be empty")
	}

	store := secrets.NewStore()
	if err := store.Set(name, value); err != nil {
		return fmt.Errorf("failed to store secret: %w", err)
	}

	out.Success("Stored secret %q", name)
	out.Info("Reference in config as: keychain:%s", name)

	return nil
}

// runInteractiveSecretSet launches an interactive form to set a secret
func runInteractiveSecretSet() (name string, value string, err error) {
	// Step 1: Get secret name
	nameForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Secret name").
				Description("A unique identifier for this secret").
				Placeholder("e.g., github-token, api-key").
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

	if err = nameForm.Run(); err != nil {
		return "", "", err
	}

	// Step 2: Get secret value with masked input
	valueForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Secret value").
				Description("The secret value to store (input is masked)").
				EchoMode(huh.EchoModePassword).
				Value(&value).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("value is required")
					}
					return nil
				}),
		),
	)

	if err = valueForm.Run(); err != nil {
		return "", "", err
	}

	return name, value, nil
}

func runSecretGet(cmd *cobra.Command, args []string) error {
	name := args[0]

	store := secrets.NewStore()
	value, err := store.Get(name)
	if err != nil {
		return err
	}

	fmt.Println(value)
	return nil
}

func runSecretList(cmd *cobra.Command, args []string) error {
	out := output.DefaultWriter()
	store := secrets.NewStore()

	names, err := store.List()
	if err != nil {
		return err
	}

	if len(names) == 0 {
		out.Println("No secrets stored.")
		out.Info("Store a secret with: agentctl secret set <name>")
		return nil
	}

	out.Println("Stored secrets:")
	for _, name := range names {
		out.Println("  • %s", name)
	}

	return nil
}

func runSecretDelete(cmd *cobra.Command, args []string) error {
	name := args[0]
	out := output.DefaultWriter()

	store := secrets.NewStore()
	if err := store.Delete(name); err != nil {
		return err
	}

	out.Success("Deleted secret %q", name)
	return nil
}
