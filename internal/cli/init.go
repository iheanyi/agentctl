package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize agentctl configuration",
	Long: `Initialize agentctl configuration in the current directory or globally.

This creates the configuration directory structure and a default
agentctl.json file.

Examples:
  agentctl init           # Initialize global config
  agentctl init --local   # Initialize project-local config`,
	RunE: runInit,
}

var (
	initLocal bool
)

func init() {
	initCmd.Flags().BoolVarP(&initLocal, "local", "l", false, "Initialize in current directory (project-local config)")
}

func runInit(cmd *cobra.Command, args []string) error {
	var configDir string
	var configPath string

	if initLocal {
		// Project-local config
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		configPath = filepath.Join(cwd, ".agentctl.json")
		configDir = cwd
	} else {
		// Global config
		configDir = config.DefaultConfigDir()
		configPath = filepath.Join(configDir, "agentctl.json")
	}

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("Configuration already exists at %s\n", configPath)
		return nil
	}

	// Create directory structure
	dirs := []string{
		configDir,
		filepath.Join(configDir, "servers"),
		filepath.Join(configDir, "commands"),
		filepath.Join(configDir, "rules"),
		filepath.Join(configDir, "prompts"),
		filepath.Join(configDir, "skills"),
		filepath.Join(configDir, "profiles"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create default config
	cfg := &config.Config{
		Version:   "1",
		Servers:   make(map[string]*mcp.Server),
		Commands:  []string{},
		Rules:     []string{},
		Prompts:   []string{},
		Skills:    []string{},
		Path:      configPath,
		ConfigDir: configDir,
		Settings: config.Settings{
			Tools: map[string]config.ToolConfig{
				"claude":   {Enabled: true},
				"cursor":   {Enabled: true},
				"codex":    {Enabled: true},
				"opencode": {Enabled: true},
				"cline":    {Enabled: true},
				"windsurf": {Enabled: true},
				"zed":      {Enabled: true},
				"continue": {Enabled: true},
			},
		},
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Initialized agentctl configuration at %s\n", configPath)
	fmt.Println("\nNext steps:")
	fmt.Println("  agentctl install filesystem   # Install your first MCP server")
	fmt.Println("  agentctl alias list           # See available aliases")
	fmt.Println("  agentctl sync                 # Sync config to your tools")

	return nil
}
