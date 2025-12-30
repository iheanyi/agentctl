package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/output"
	"github.com/iheanyi/agentctl/pkg/profile"
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

var profileDeleteCmd = &cobra.Command{
	Use:     "delete <name>",
	Aliases: []string{"rm"},
	Short:   "Delete a profile",
	Args:    cobra.ExactArgs(1),
	RunE:    runProfileDelete,
}

func init() {
	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileCreateCmd)
	profileCmd.AddCommand(profileSwitchCmd)
	profileCmd.AddCommand(profileExportCmd)
	profileCmd.AddCommand(profileImportCmd)
	profileCmd.AddCommand(profileDeleteCmd)
}

func runProfileList(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	out := output.DefaultWriter()
	profilesDir := filepath.Join(cfg.ConfigDir, "profiles")

	// Load all profiles
	profiles, err := profile.LoadAll(profilesDir)
	if err != nil {
		return fmt.Errorf("failed to load profiles: %w", err)
	}

	activeProfile := cfg.Settings.DefaultProfile
	if activeProfile == "" {
		activeProfile = "default"
	}

	if len(profiles) == 0 {
		out.Println("No profiles found.")
		out.Info("The 'default' profile is active (using main config)")
		out.Println("")
		out.Println("Create a profile with: agentctl profile create <name>")
		return nil
	}

	table := output.NewTable("Name", "Description", "Status")

	// Add default profile if it's active and not in the list
	hasDefault := false
	for _, p := range profiles {
		if p.Name == "default" {
			hasDefault = true
			break
		}
	}
	if !hasDefault && activeProfile == "default" {
		table.AddRow("default", "(main config)", "*active*")
	}

	for _, p := range profiles {
		status := ""
		if p.Name == activeProfile {
			status = "*active*"
		}
		table.AddRow(p.Name, p.Description, status)
	}

	table.Render()
	return nil
}

func runProfileCreate(cmd *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	out := output.DefaultWriter()
	profilesDir := filepath.Join(cfg.ConfigDir, "profiles")

	// Check if profile already exists
	if profile.Exists(profilesDir, name) {
		return fmt.Errorf("profile %q already exists", name)
	}

	// Create the profile
	p, err := profile.Create(profilesDir, name, "")
	if err != nil {
		return fmt.Errorf("failed to create profile: %w", err)
	}

	out.Success("Created profile %q", name)
	out.Info("Edit %s to configure the profile", p.Path)

	return nil
}

func runProfileSwitch(cmd *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	out := output.DefaultWriter()
	profilesDir := filepath.Join(cfg.ConfigDir, "profiles")

	// Check profile exists (unless switching to default)
	if name != "default" && !profile.Exists(profilesDir, name) {
		return fmt.Errorf("profile %q does not exist", name)
	}

	// Update default profile
	cfg.Settings.DefaultProfile = name
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	out.Success("Switched to profile %q", name)
	out.Info("Run 'agentctl sync' to apply the new profile")

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

func runProfileDelete(cmd *cobra.Command, args []string) error {
	name := args[0]

	if name == "default" {
		return fmt.Errorf("cannot delete the default profile")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	out := output.DefaultWriter()
	profilesDir := filepath.Join(cfg.ConfigDir, "profiles")

	// Check profile exists
	if !profile.Exists(profilesDir, name) {
		return fmt.Errorf("profile %q does not exist", name)
	}

	// Delete the profile
	if err := profile.Delete(profilesDir, name); err != nil {
		return fmt.Errorf("failed to delete profile: %w", err)
	}

	// If this was the active profile, switch to default
	if cfg.Settings.DefaultProfile == name {
		cfg.Settings.DefaultProfile = "default"
		if err := cfg.Save(); err != nil {
			out.Warning("Deleted profile was active, but failed to update config: %v", err)
		} else {
			out.Info("Switched to 'default' profile")
		}
	}

	out.Success("Deleted profile %q", name)
	return nil
}
