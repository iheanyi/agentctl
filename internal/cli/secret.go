package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/iheanyi/agentctl/pkg/output"
	"github.com/iheanyi/agentctl/pkg/secrets"
	"github.com/spf13/cobra"
	"golang.org/x/term"
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
	Use:   "set <name>",
	Short: "Store a secret in the keychain",
	Args:  cobra.ExactArgs(1),
	RunE:  runSecretSet,
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
	name := args[0]
	out := output.DefaultWriter()

	// Read secret from stdin or prompt
	var value string
	var err error

	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
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
