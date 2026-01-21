package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/output"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View and edit configuration",
	Long: `View and edit agentctl configuration.

Examples:
  agentctl config                       # Show config location and summary
  agentctl config show                  # Show full config as JSON
  agentctl config get settings.defaultProfile
  agentctl config set settings.defaultProfile work
  agentctl config edit                  # Open in editor`,
	RunE: runConfig,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show full configuration",
	RunE:  runConfigShow,
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigGet,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE:  runConfigSet,
}

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open configuration in editor",
	RunE:  runConfigEdit,
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show configuration file path",
	RunE:  runConfigPath,
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configEditCmd)
	configCmd.AddCommand(configPathCmd)
}

func runConfig(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	out := output.DefaultWriter()

	out.Println("Configuration")
	out.Println("")
	out.Println("  Path: %s", cfg.Path)
	out.Println("  Config dir: %s", cfg.ConfigDir)
	out.Println("  Cache dir: %s", cfg.CacheDir())
	out.Println("")
	out.Println("  Servers: %d", len(cfg.Servers))
	out.Println("  Commands: %d", len(cfg.LoadedCommands))
	out.Println("  Rules: %d", len(cfg.LoadedRules))
	out.Println("  Skills: %d", len(cfg.LoadedSkills))
	out.Println("")

	if cfg.Settings.DefaultProfile != "" {
		out.Println("  Active profile: %s", cfg.Settings.DefaultProfile)
	} else {
		out.Println("  Active profile: default")
	}

	out.Println("")
	out.Info("Use 'agentctl config show' to see full config")
	out.Info("Use 'agentctl config edit' to edit in your editor")

	return nil
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	key := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Convert config to map for key lookup
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	var configMap map[string]interface{}
	if err := json.Unmarshal(data, &configMap); err != nil {
		return err
	}

	// Navigate to the key
	value, err := getNestedValue(configMap, key)
	if err != nil {
		return err
	}

	// Print value
	switch v := value.(type) {
	case string:
		fmt.Println(v)
	case nil:
		fmt.Println("null")
	default:
		jsonValue, _ := json.MarshalIndent(v, "", "  ")
		fmt.Println(string(jsonValue))
	}

	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	out := output.DefaultWriter()

	// For now, only support settings.defaultProfile
	switch key {
	case "settings.defaultProfile":
		cfg.Settings.DefaultProfile = value
	case "settings.autoUpdate.enabled":
		cfg.Settings.AutoUpdate.Enabled = value == "true"
	case "settings.autoUpdate.interval":
		cfg.Settings.AutoUpdate.Interval = value
	default:
		return fmt.Errorf("setting %q is not supported via CLI (edit config file directly)", key)
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	out.Success("Set %s = %s", key, value)
	return nil
}

func runConfigEdit(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Determine editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vim"
	}

	// Open editor
	editorCmd := exec.Command(editor, cfg.Path)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr

	return editorCmd.Run()
}

func runConfigPath(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Println(cfg.Path)
	return nil
}

func getNestedValue(m map[string]interface{}, key string) (interface{}, error) {
	parts := strings.Split(key, ".")
	current := interface{}(m)

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			val, ok := v[part]
			if !ok {
				return nil, fmt.Errorf("key %q not found", key)
			}
			current = val
		default:
			return nil, fmt.Errorf("cannot navigate into %q", part)
		}
	}

	return current, nil
}
