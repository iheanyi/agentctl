package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/output"
	"github.com/iheanyi/agentctl/pkg/profile"
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
	Use:   "create [name]",
	Short: "Create a new profile",
	Long: `Create a new profile with the given name.

If no name is provided, launches an interactive form.

Examples:
  agentctl profile create           # Interactive mode
  agentctl profile create work      # Create profile named "work"
  agentctl profile create work -d "Work servers"`,
	Args: cobra.MaximumNArgs(1),
	RunE: runProfileCreate,
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

var profileShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show profile details",
	Long: `Show details of a profile including its servers, commands, and rules.
If no name is provided, shows the active profile.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runProfileShow,
}

var profileAddServerCmd = &cobra.Command{
	Use:   "add-server <profile> <server>",
	Short: "Add a server to a profile",
	Args:  cobra.ExactArgs(2),
	RunE:  runProfileAddServer,
}

var profileRemoveServerCmd = &cobra.Command{
	Use:   "remove-server <profile> <server>",
	Short: "Remove a server from a profile",
	Args:  cobra.ExactArgs(2),
	RunE:  runProfileRemoveServer,
}

var (
	profileCreateDesc string
)

func init() {
	profileCreateCmd.Flags().StringVarP(&profileCreateDesc, "description", "d", "", "Profile description")

	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileCreateCmd)
	profileCmd.AddCommand(profileSwitchCmd)
	profileCmd.AddCommand(profileExportCmd)
	profileCmd.AddCommand(profileImportCmd)
	profileCmd.AddCommand(profileDeleteCmd)
	profileCmd.AddCommand(profileShowCmd)
	profileCmd.AddCommand(profileAddServerCmd)
	profileCmd.AddCommand(profileRemoveServerCmd)
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
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	out := output.DefaultWriter()
	profilesDir := filepath.Join(cfg.ConfigDir, "profiles")

	var name string
	var description string
	var selectedServers []string

	// If no name provided, use interactive mode
	if len(args) == 0 {
		// Check if we're in an interactive terminal
		if err := requireInteractive("profile create"); err != nil {
			return err
		}

		// Run interactive form
		name, description, selectedServers, err = runInteractiveProfileCreate(cfg, profilesDir)
		if err != nil {
			return err
		}
	} else {
		name = args[0]
		description = profileCreateDesc

		// Check if profile already exists
		if profile.Exists(profilesDir, name) {
			return fmt.Errorf("profile %q already exists", name)
		}
	}

	// Create the profile
	p, err := profile.Create(profilesDir, name, description)
	if err != nil {
		return fmt.Errorf("failed to create profile: %w", err)
	}

	// Add selected servers if any
	if len(selectedServers) > 0 {
		p.Servers = selectedServers
		if err := p.Save(); err != nil {
			return fmt.Errorf("failed to save profile with servers: %w", err)
		}
	}

	out.Success("Created profile %q", name)
	if len(selectedServers) > 0 {
		out.Println("  Servers: %d", len(selectedServers))
		for _, s := range selectedServers {
			out.Println("    - %s", s)
		}
	}
	out.Println("")
	out.Println("Add servers to this profile:")
	out.Println("  agentctl profile add-server %s <server>", name)
	out.Println("")
	out.Println("Or switch to it and install servers:")
	out.Println("  agentctl profile switch %s", name)
	out.Println("  agentctl install <server>")
	out.Println("")
	out.Info("Profile stored at: %s", p.Path)

	return nil
}

// runInteractiveProfileCreate launches an interactive form to create a profile
func runInteractiveProfileCreate(cfg *config.Config, profilesDir string) (name, description string, servers []string, err error) {
	// Get sorted list of available servers
	var serverNames []string
	for serverName := range cfg.Servers {
		serverNames = append(serverNames, serverName)
	}
	sort.Strings(serverNames)

	// Step 1: Get profile name
	nameForm := newStyledForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Profile name").
				Description("A unique name for this profile").
				Placeholder("e.g., work, personal, project-x").
				Value(&name).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("name is required")
					}
					if s == "default" {
						return fmt.Errorf("cannot use reserved name 'default'")
					}
					if profile.Exists(profilesDir, s) {
						return fmt.Errorf("profile %q already exists", s)
					}
					return nil
				}),
		),
	)

	ok, err := runFormWithCancel(nameForm, "profile create")
	if err != nil {
		return "", "", nil, err
	}
	if !ok {
		return "", "", nil, fmt.Errorf("cancelled")
	}

	// Step 2: Get optional description
	descForm := newStyledForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Description (optional)").
				Description("A brief description of this profile").
				Placeholder("e.g., Servers for work projects").
				Value(&description),
		),
	)

	ok, err = runFormWithCancel(descForm, "profile create")
	if err != nil {
		return "", "", nil, err
	}
	if !ok {
		return "", "", nil, fmt.Errorf("cancelled")
	}

	// Step 3: Select servers (if any are available)
	if len(serverNames) > 0 {
		// Build options for multi-select
		var serverOptions []huh.Option[string]
		for _, s := range serverNames {
			serverOptions = append(serverOptions, huh.NewOption(s, s))
		}

		serverForm := newStyledForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Servers to include").
					Description("Select servers to add to this profile (space to toggle, enter to confirm)").
					Options(serverOptions...).
					Value(&servers),
			),
		)

		ok, err = runFormWithCancel(serverForm, "profile create")
		if err != nil {
			return "", "", nil, err
		}
		if !ok {
			return "", "", nil, fmt.Errorf("cancelled")
		}
	}

	return name, description, servers, nil
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

func runProfileShow(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	out := output.DefaultWriter()
	profilesDir := filepath.Join(cfg.ConfigDir, "profiles")

	// Determine which profile to show
	name := cfg.Settings.DefaultProfile
	if name == "" {
		name = "default"
	}
	if len(args) > 0 {
		name = args[0]
	}

	// Handle default profile specially
	if name == "default" {
		out.Println("Profile: default (main config)")
		out.Println("")
		out.Println("Servers (%d):", len(cfg.Servers))
		for serverName, server := range cfg.Servers {
			transport := string(server.Transport)
			if transport == "" {
				transport = "stdio"
			}
			out.Println("  • %s (%s)", serverName, transport)
		}
		if len(cfg.Servers) == 0 {
			out.Println("  (none)")
		}
		return nil
	}

	// Load the profile
	p, err := profile.Load(filepath.Join(profilesDir, name+".json"))
	if err != nil {
		return fmt.Errorf("profile %q not found", name)
	}

	// Check if active
	isActive := cfg.Settings.DefaultProfile == name
	status := ""
	if isActive {
		status = " (active)"
	}

	out.Println("Profile: %s%s", p.Name, status)
	if p.Description != "" {
		out.Println("Description: %s", p.Description)
	}
	out.Println("")

	// Show servers
	out.Println("Servers (%d):", len(p.Servers))
	if len(p.Servers) > 0 {
		for _, serverName := range p.Servers {
			out.Println("  • %s", serverName)
		}
	} else {
		out.Println("  (none - will use base config servers)")
	}

	// Show disabled
	if len(p.Disabled) > 0 {
		out.Println("")
		out.Println("Disabled:")
		for _, name := range p.Disabled {
			out.Println("  • %s", name)
		}
	}

	// Show additive model info
	out.Println("")
	out.Info("Profiles use an additive model: profile servers are added to base config")

	return nil
}

func runProfileAddServer(cmd *cobra.Command, args []string) error {
	profileName := args[0]
	serverName := args[1]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	out := output.DefaultWriter()
	profilesDir := filepath.Join(cfg.ConfigDir, "profiles")

	// Check profile exists
	if !profile.Exists(profilesDir, profileName) {
		return fmt.Errorf("profile %q does not exist", profileName)
	}

	// Load the profile
	p, err := profile.Load(filepath.Join(profilesDir, profileName+".json"))
	if err != nil {
		return fmt.Errorf("failed to load profile: %w", err)
	}

	// Check if server already in profile
	for _, s := range p.Servers {
		if s == serverName {
			out.Info("Server %q is already in profile %q", serverName, profileName)
			return nil
		}
	}

	// Add server to profile
	p.Servers = append(p.Servers, serverName)
	if err := p.Save(); err != nil {
		return fmt.Errorf("failed to save profile: %w", err)
	}

	out.Success("Added %q to profile %q", serverName, profileName)

	// Hint about syncing if this is the active profile
	if cfg.Settings.DefaultProfile == profileName {
		out.Info("Run 'agentctl sync' to apply changes")
	}

	return nil
}

func runProfileRemoveServer(cmd *cobra.Command, args []string) error {
	profileName := args[0]
	serverName := args[1]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	out := output.DefaultWriter()
	profilesDir := filepath.Join(cfg.ConfigDir, "profiles")

	// Check profile exists
	if !profile.Exists(profilesDir, profileName) {
		return fmt.Errorf("profile %q does not exist", profileName)
	}

	// Load the profile
	p, err := profile.Load(filepath.Join(profilesDir, profileName+".json"))
	if err != nil {
		return fmt.Errorf("failed to load profile: %w", err)
	}

	// Find and remove server
	found := false
	var newServers []string
	for _, s := range p.Servers {
		if s == serverName {
			found = true
		} else {
			newServers = append(newServers, s)
		}
	}

	if !found {
		return fmt.Errorf("server %q not found in profile %q", serverName, profileName)
	}

	p.Servers = newServers
	if err := p.Save(); err != nil {
		return fmt.Errorf("failed to save profile: %w", err)
	}

	out.Success("Removed %q from profile %q", serverName, profileName)

	// Hint about syncing if this is the active profile
	if cfg.Settings.DefaultProfile == profileName {
		out.Info("Run 'agentctl sync' to apply changes")
	}

	return nil
}
