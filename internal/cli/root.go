package cli

import (
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/iheanyi/agentctl/internal/tui"
)

var (
	// Version is set at build time
	Version = "dev"
	// Commit is set at build time
	Commit = "unknown"
	// JSONOutput is set when --json flag is used
	JSONOutput bool
)

var rootCmd = &cobra.Command{
	Use:   "agentctl",
	Short: "Universal agent configuration manager",
	Long: `agentctl manages MCP servers, commands, rules, prompts, and skills
across multiple agentic frameworks and developer tools.

Supported tools: Claude Code, Cursor, Codex, OpenCode, Cline, Windsurf, Zed, Continue

Examples:
  agentctl                             # Launch interactive TUI
  agentctl add filesystem              # Add MCP server by alias
  agentctl add github.com/org/mcp      # Add from git URL
  agentctl sync                        # Sync config to all tools
  agentctl list                        # List installed resources
  agentctl --help                      # Show all commands`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runRoot,
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVar(&JSONOutput, "json", false, "Output results as JSON (machine-parseable)")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(addCmd) // Primary command for adding MCP servers (alias: install)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(aliasCmd)
	rootCmd.AddCommand(profileCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(newCmd)
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(secretCmd)
	rootCmd.AddCommand(testCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(uiCmd)
	rootCmd.AddCommand(daemonCmd)
	rootCmd.AddCommand(backupCmd)
}

// runRoot handles the default behavior when no subcommand is given
func runRoot(cmd *cobra.Command, args []string) error {
	// If we're in a TTY, launch the TUI
	if isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		return tui.Run()
	}

	// Otherwise, show help
	return cmd.Help()
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("agentctl version %s (%s)\n", Version, Commit)
	},
}
