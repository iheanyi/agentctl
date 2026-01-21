package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/lockfile"
	"github.com/iheanyi/agentctl/pkg/output"
)

var removeCmd = &cobra.Command{
	Use:     "remove <server>",
	Aliases: []string{"rm", "uninstall"},
	Short:   "Remove an MCP server",
	Long: `Remove an MCP server from your configuration.

The server will be removed from agentctl's config. Run 'agentctl sync'
to remove it from your tools.

Scope:
  By default, removes from the config where the server exists.
  Use --scope to explicitly specify local or global config.

Examples:
  agentctl remove filesystem
  agentctl remove filesystem --scope local   # Remove from local config only
  agentctl remove filesystem --scope global  # Remove from global config only
  agentctl rm github`,
	Args: cobra.ExactArgs(1),
	RunE: runRemove,
}

var removeScope string

func init() {
	removeCmd.Flags().StringVarP(&removeScope, "scope", "s", "", "Config scope: local, global (default: auto-detect)")
}

func runRemove(cmd *cobra.Command, args []string) error {
	name := args[0]
	out := output.DefaultWriter()

	// Determine scope
	var scope config.Scope
	if removeScope != "" {
		var err error
		scope, err = config.ParseScope(removeScope)
		if err != nil {
			return err
		}
	}

	// Load config based on scope
	var cfg *config.Config
	var err error
	if scope != "" {
		cfg, err = config.LoadScoped(scope)
	} else {
		// Auto-detect: load merged config to find where server exists
		cfg, err = config.LoadWithProject()
	}
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Load lockfile
	lf, err := lockfile.Load(cfg.ConfigDir)
	if err != nil {
		return fmt.Errorf("failed to load lockfile: %w", err)
	}

	// Check if server exists
	server, ok := cfg.Servers[name]
	if !ok {
		return fmt.Errorf("server %q is not installed", name)
	}

	// Determine which scope to save to
	saveScope := scope
	if saveScope == "" {
		// Auto-detect from server's scope
		if server.Scope == string(config.ScopeLocal) {
			saveScope = config.ScopeLocal
		} else {
			saveScope = config.ScopeGlobal
		}
	}

	// If we loaded merged config but need to save to specific scope, reload that scope
	if scope == "" && saveScope != "" {
		cfg, err = config.LoadScoped(saveScope)
		if err != nil {
			return fmt.Errorf("failed to load %s config: %w", saveScope, err)
		}
		// Check server exists in this specific config
		if _, ok := cfg.Servers[name]; !ok {
			return fmt.Errorf("server %q is not in %s config", name, saveScope)
		}
	}

	// Remove from config
	delete(cfg.Servers, name)

	// Save config to the appropriate scope
	if err := cfg.SaveScoped(saveScope); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Remove from lockfile
	if lf.IsLocked(name) {
		lf.Unlock(name)
		if err := lf.Save(); err != nil {
			out.Warning("Failed to update lockfile: %v", err)
		}
	}

	scopeLabel := ""
	if saveScope == config.ScopeLocal {
		scopeLabel = " (local)"
	} else if saveScope == config.ScopeGlobal {
		scopeLabel = " (global)"
	}
	out.Success("Removed %q%s", name, scopeLabel)
	out.Info("Run 'agentctl sync' to update your tools.")

	return nil
}
