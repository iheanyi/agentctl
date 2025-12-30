package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/spf13/cobra"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage configuration profiles",
	Long: `Manage configuration profiles (e.g., work, personal).

Profiles let you maintain different sets of MCP servers, commands,
and rules that you can switch between.

Examples:
  agentctl profile list              # List all profiles
  agentctl profile create work       # Create a new profile
  agentctl profile switch work       # Switch to work profile
  agentctl profile export work       # Export profile to JSON`,
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all profiles",
	RunE:  runProfileList,
}

var profileCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileCreate,
}

var profileSwitchCmd = &cobra.Command{
	Use:   "switch <name>",
	Short: "Switch to a profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileSwitch,
}

var profileExportCmd = &cobra.Command{
	Use:   "export <name>",
	Short: "Export a profile to JSON",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileExport,
}

var profileImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Import a profile from JSON (stdin)",
	RunE:  runProfileImport,
}

func init() {
	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileCreateCmd)
	profileCmd.AddCommand(profileSwitchCmd)
	profileCmd.AddCommand(profileExportCmd)
	profileCmd.AddCommand(profileImportCmd)
}

func runProfileList(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// TODO: Load profiles from profiles/ directory
	defaultProfile := cfg.Settings.DefaultProfile
	if defaultProfile == "" {
		defaultProfile = "default"
	}

	fmt.Println("Profiles:")
	fmt.Printf("  * %s (active)\n", defaultProfile)

	return nil
}

func runProfileCreate(cmd *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create profile in profiles/ directory
	profileDir := cfg.ConfigDir + "/profiles"
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return fmt.Errorf("failed to create profiles directory: %w", err)
	}

	profilePath := profileDir + "/" + name + ".json"
	if _, err := os.Stat(profilePath); err == nil {
		return fmt.Errorf("profile %q already exists", name)
	}

	// Create empty profile
	profile := map[string]interface{}{
		"name":     name,
		"servers":  []string{},
		"commands": []string{},
		"rules":    []string{},
		"disabled": []string{},
	}

	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(profilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to create profile: %w", err)
	}

	fmt.Printf("Created profile %q\n", name)
	fmt.Printf("Edit %s to configure the profile.\n", profilePath)

	return nil
}

func runProfileSwitch(cmd *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check profile exists
	profilePath := cfg.ConfigDir + "/profiles/" + name + ".json"
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		return fmt.Errorf("profile %q does not exist", name)
	}

	// Update default profile
	cfg.Settings.DefaultProfile = name
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Switched to profile %q\n", name)
	fmt.Println("Run 'agentctl sync' to apply the new profile.")

	return nil
}

func runProfileExport(cmd *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	profilePath := cfg.ConfigDir + "/profiles/" + name + ".json"
	data, err := os.ReadFile(profilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("profile %q does not exist", name)
		}
		return fmt.Errorf("failed to read profile: %w", err)
	}

	fmt.Print(string(data))
	return nil
}

func runProfileImport(cmd *cobra.Command, args []string) error {
	// Read from stdin
	var profile map[string]interface{}
	decoder := json.NewDecoder(os.Stdin)
	if err := decoder.Decode(&profile); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	name, ok := profile["name"].(string)
	if !ok || name == "" {
		return fmt.Errorf("profile must have a 'name' field")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create profiles directory
	profileDir := cfg.ConfigDir + "/profiles"
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return fmt.Errorf("failed to create profiles directory: %w", err)
	}

	// Save profile
	profilePath := profileDir + "/" + name + ".json"
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(profilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to save profile: %w", err)
	}

	fmt.Printf("Imported profile %q\n", name)
	return nil
}
