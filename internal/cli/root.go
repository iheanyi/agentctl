package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Version is set at build time
	Version = "dev"
	// Commit is set at build time
	Commit = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "agentctl",
	Short: "Universal agent configuration manager",
	Long: `agentctl manages MCP servers, commands, rules, prompts, and skills
across multiple agentic frameworks and developer tools.

Supported tools: Claude Code, Cursor, Codex, OpenCode, Cline, Windsurf, Zed, Continue

Examples:
  agentctl install filesystem          # Install MCP server by alias
  agentctl install github.com/org/mcp  # Install from git URL
  agentctl sync                        # Sync config to all tools
  agentctl list                        # List installed resources
  agentctl profile switch work         # Switch to work profile`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(addCmd) // Primary command for adding MCP servers (alias: install)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(aliasCmd)
	rootCmd.AddCommand(profileCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(doctorCmd)
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
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("agentctl version %s (%s)\n", Version, Commit)
	},
}
