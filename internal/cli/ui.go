package cli

import (
	"github.com/spf13/cobra"

	"github.com/iheanyi/agentctl/internal/tui"
)

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Launch interactive TUI",
	Long: `Launch the interactive terminal user interface.

The TUI provides a visual way to manage MCP servers with keyboard navigation.

Keyboard shortcuts:
  ↑/↓ or j/k   Navigate servers
  d            Delete selected server
  s            Sync all servers
  t            Test selected server
  q or Esc     Quit`,
	RunE: runUI,
}

func runUI(cmd *cobra.Command, args []string) error {
	return tui.Run()
}
