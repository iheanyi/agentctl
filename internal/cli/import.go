package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/iheanyi/agentctl/pkg/command"
	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/discovery"
	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/iheanyi/agentctl/pkg/output"
	"github.com/iheanyi/agentctl/pkg/rule"
	"github.com/iheanyi/agentctl/pkg/skill"
	"github.com/iheanyi/agentctl/pkg/sync"
)

var importCmd = &cobra.Command{
	Use:   "import [tool]",
	Short: "Import configuration from an existing tool",
	Long: `Import MCP servers, commands, rules, or skills from an existing tool.

This reads the tool's configuration and adds the entries to agentctl,
allowing you to manage them centrally.

Use --local to import from workspace configs (e.g., .mcp.json) into
the local .agentctl.json project config.

Use --all to import from all discovered native resources in tool directories
(.claude/, .cursor/, .codex/, etc.).

Examples:
  agentctl import claude           # Import all from Claude Code global config
  agentctl import cursor --servers # Import only servers from Cursor
  agentctl import claude --local   # Import from .mcp.json to .agentctl.json
  agentctl import cline --rules    # Import only rules from Cline
  agentctl import --all            # Import all discovered native resources
  agentctl import --all --rules    # Import only rules from all tools
  agentctl import --all --force    # Overwrite existing resources
  agentctl import --all --dry-run  # Preview what would be imported`,
	Args: cobra.MaximumNArgs(1),
	RunE: runImport,
}

var (
	importServers  bool
	importCommands bool
	importRules    bool
	importSkills   bool
	importLocal    bool
	importAll      bool
	importForce    bool
	importDryRun   bool
)

func init() {
	importCmd.Flags().BoolVar(&importServers, "servers", false, "Import only servers")
	importCmd.Flags().BoolVar(&importCommands, "commands", false, "Import only commands")
	importCmd.Flags().BoolVar(&importRules, "rules", false, "Import only rules")
	importCmd.Flags().BoolVar(&importSkills, "skills", false, "Import only skills")
	importCmd.Flags().BoolVar(&importLocal, "local", false, "Import from workspace config (e.g., .mcp.json) to local .agentctl.json")
	importCmd.Flags().BoolVar(&importAll, "all", false, "Import from all discovered native resources (.claude/, .cursor/, etc.)")
	importCmd.Flags().BoolVar(&importForce, "force", false, "Overwrite existing resources")
	importCmd.Flags().BoolVar(&importDryRun, "dry-run", false, "Preview what would be imported without making changes")
}

func runImport(cmd *cobra.Command, args []string) error {
	out := output.DefaultWriter()

	// Handle --all flag: import from discovered native resources
	if importAll {
		return runImportAll(out)
	}

	// Require tool argument if not using --all
	if len(args) == 0 {
		return fmt.Errorf("requires a tool argument or --all flag")
	}
	toolName := args[0]

	// Get the adapter for the specified tool
	adapter, ok := sync.Get(toolName)
	if !ok {
		return fmt.Errorf("unknown tool %q", toolName)
	}

	// Check if tool is installed
	detected, err := adapter.Detect()
	if err != nil {
		return fmt.Errorf("failed to detect %s: %w", toolName, err)
	}
	if !detected {
		return fmt.Errorf("%s is not installed or not detected", toolName)
	}

	// Handle local import
	if importLocal {
		return runLocalImport(adapter, out)
	}

	// Load global config for regular import
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if importDryRun {
		out.Println("[dry-run] Importing from %s...", adapter.Name())
	} else {
		out.Println("Importing from %s...", adapter.Name())
	}

	// Determine what to import (default: all types)
	importAllTypes := !importServers && !importCommands && !importRules && !importSkills

	supported := adapter.SupportedResources()
	var serverCount, commandCount, ruleCount int

	// Import servers
	if (importAllTypes || importServers) && containsResourceType(supported, sync.ResourceMCP) {
		sa, ok := sync.AsServerAdapter(adapter)
		if !ok {
			out.Warning("Adapter doesn't support reading servers")
		} else if servers, err := sa.ReadServers(); err != nil {
			out.Warning("Failed to read servers: %v", err)
		} else {
			for _, server := range servers {
				// Skip already managed servers
				if server.Name == "" {
					continue
				}

				// Check if already exists
				if _, exists := cfg.Servers[server.Name]; exists {
					if !importForce {
						out.Info("Skipping %q (already exists, use --force to overwrite)", server.Name)
						continue
					}
					if importDryRun {
						out.Info("[dry-run] Would overwrite server %q", server.Name)
					} else {
						out.Info("Overwriting server %q", server.Name)
					}
				}

				if importDryRun {
					out.Info("[dry-run] Would import server %q", server.Name)
					serverCount++
					continue
				}

				// Add to config
				if cfg.Servers == nil {
					cfg.Servers = make(map[string]*mcp.Server)
				}
				cfg.Servers[server.Name] = server
				serverCount++
			}
		}
	}

	// Import commands
	if (importAllTypes || importCommands) && containsResourceType(supported, sync.ResourceCommands) {
		ca, ok := sync.AsCommandsAdapter(adapter)
		if !ok {
			out.Warning("Adapter doesn't support reading commands")
		} else if commands, err := ca.ReadCommands(); err != nil {
			out.Warning("Failed to read commands: %v", err)
		} else {
			commandsDir := filepath.Join(cfg.ConfigDir, "commands")
			for _, cmd := range commands {
				// Check if already exists
				existingPath := filepath.Join(commandsDir, cmd.Name+".json")
				if command.Exists(existingPath) {
					if !importForce {
						out.Info("Skipping command %q (already exists, use --force to overwrite)", cmd.Name)
						continue
					}
					if importDryRun {
						out.Info("[dry-run] Would overwrite command %q", cmd.Name)
					} else {
						out.Info("Overwriting command %q", cmd.Name)
					}
				}

				if importDryRun {
					out.Info("[dry-run] Would import command %q", cmd.Name)
					commandCount++
					continue
				}

				// Save command
				if err := command.Save(cmd, commandsDir); err != nil {
					out.Warning("Failed to save command %q: %v", cmd.Name, err)
					continue
				}
				commandCount++
			}
		}
	}

	// Import rules
	if (importAllTypes || importRules) && containsResourceType(supported, sync.ResourceRules) {
		ra, ok := sync.AsRulesAdapter(adapter)
		if !ok {
			out.Warning("Adapter doesn't support reading rules")
		} else if rules, err := ra.ReadRules(); err != nil {
			out.Warning("Failed to read rules: %v", err)
		} else {
			rulesDir := filepath.Join(cfg.ConfigDir, "rules")
			for _, r := range rules {
				name := filepath.Base(r.Path)
				if name == "" || name == "." {
					name = "imported-rule"
				}

				// Check if already exists
				existingPath := filepath.Join(rulesDir, name)
				if _, err := os.Stat(existingPath); err == nil {
					if !importForce {
						out.Info("Skipping rule %q (already exists, use --force to overwrite)", name)
						continue
					}
					if importDryRun {
						out.Info("[dry-run] Would overwrite rule %q", name)
					} else {
						out.Info("Overwriting rule %q", name)
					}
				}

				if importDryRun {
					out.Info("[dry-run] Would import rule %q", name)
					ruleCount++
					continue
				}

				// Save rule
				if err := rule.Save(r, rulesDir); err != nil {
					out.Warning("Failed to save rule %q: %v", name, err)
					continue
				}
				ruleCount++
			}
		}
	}

	// Save config if we imported anything (not in dry-run mode)
	if !importDryRun && serverCount > 0 {
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	}

	// Summary
	out.Println("")
	prefix := ""
	if importDryRun {
		prefix = "[dry-run] Would import "
	} else {
		prefix = "Imported "
	}
	if serverCount > 0 {
		out.Success("%s%d server(s)", prefix, serverCount)
	}
	if commandCount > 0 {
		out.Success("%s%d command(s)", prefix, commandCount)
	}
	if ruleCount > 0 {
		out.Success("%s%d rule(s)", prefix, ruleCount)
	}

	if serverCount == 0 && commandCount == 0 && ruleCount == 0 {
		out.Info("No new resources to import")
	} else if !importDryRun {
		out.Println("")
		out.Info("Run 'agentctl sync' to sync to all tools")
	}

	return nil
}

// runLocalImport imports from a tool's workspace config to local .agentctl.json
func runLocalImport(adapter sync.Adapter, out *output.Writer) error {
	// Check if adapter supports workspace configs
	wa, ok := sync.AsWorkspaceAdapter(adapter)
	if !ok {
		return fmt.Errorf("%s doesn't support workspace configs", adapter.Name())
	}

	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Check if workspace config exists
	workspacePath := wa.WorkspaceConfigPath(cwd)
	if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
		return fmt.Errorf("no workspace config found at %s", workspacePath)
	}

	out.Println("Importing from %s workspace config...", adapter.Name())
	out.Println("  Source: %s", workspacePath)

	// Read servers from workspace config
	servers, err := wa.ReadWorkspaceServers(cwd)
	if err != nil {
		return fmt.Errorf("failed to read workspace servers: %w", err)
	}

	if len(servers) == 0 {
		out.Info("No servers found in workspace config")
		return nil
	}

	// Load or create local config
	cfg, err := config.LoadScoped(config.ScopeLocal)
	if err != nil {
		return fmt.Errorf("failed to load local config: %w", err)
	}

	// Import servers
	var serverCount int
	for _, server := range servers {
		if server.Name == "" {
			continue
		}

		// Check if already exists
		if _, exists := cfg.Servers[server.Name]; exists {
			out.Info("Skipping %q (already exists)", server.Name)
			continue
		}

		// Add to config
		if cfg.Servers == nil {
			cfg.Servers = make(map[string]*mcp.Server)
		}
		server.Scope = string(config.ScopeLocal)
		cfg.Servers[server.Name] = server
		serverCount++
		out.Success("Imported %q", server.Name)
	}

	if serverCount == 0 {
		out.Info("No new servers to import")
		return nil
	}

	// Save to local .agentctl.json
	if err := cfg.SaveScoped(config.ScopeLocal); err != nil {
		return fmt.Errorf("failed to save local config: %w", err)
	}

	out.Println("")
	out.Success("Imported %d server(s) to .agentctl.json", serverCount)
	out.Info("Run 'agentctl sync' to sync to workspace configs")

	return nil
}

// runImportAll imports from all discovered native resources
func runImportAll(out *output.Writer) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Load global config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if importDryRun {
		out.Println("[dry-run] Discovering native resources...")
	} else {
		out.Println("Discovering native resources...")
	}

	// Discover all native resources (local + global)
	resources := discovery.DiscoverBoth(cwd)

	if len(resources) == 0 {
		out.Info("No native resources found")
		return nil
	}

	// Group resources by tool for display
	byTool := make(map[string][]*discovery.NativeResource)
	for _, r := range resources {
		byTool[r.Tool] = append(byTool[r.Tool], r)
	}

	// Show what was found
	for tool, toolResources := range byTool {
		out.Info("Found %d resource(s) from %s", len(toolResources), tool)
	}
	out.Println("")

	// Determine what to import (default: all types)
	importAllTypes := !importServers && !importCommands && !importRules && !importSkills

	var (
		serverCount  int
		commandCount int
		ruleCount    int
		skillCount   int
		pluginCount  int
	)

	rulesDir := filepath.Join(cfg.ConfigDir, "rules")
	commandsDir := filepath.Join(cfg.ConfigDir, "commands")
	skillsDir := filepath.Join(cfg.ConfigDir, "skills")

	for _, res := range resources {
		switch res.Type {
		case "server":
			if !importAllTypes && !importServers {
				continue
			}
			server, ok := res.Resource.(*mcp.Server)
			if !ok || server.Name == "" {
				continue
			}

			// Check if already exists
			if _, exists := cfg.Servers[server.Name]; exists {
				if !importForce {
					out.Info("Skipping server %q from %s (already exists)", server.Name, res.Tool)
					continue
				}
				if importDryRun {
					out.Info("[dry-run] Would overwrite server %q from %s", server.Name, res.Tool)
				} else {
					out.Info("Overwriting server %q from %s", server.Name, res.Tool)
				}
			}

			if importDryRun {
				out.Info("[dry-run] Would import server %q from %s", server.Name, res.Tool)
				serverCount++
				continue
			}

			if cfg.Servers == nil {
				cfg.Servers = make(map[string]*mcp.Server)
			}
			cfg.Servers[server.Name] = server
			serverCount++

		case "rule":
			if !importAllTypes && !importRules {
				continue
			}
			r, ok := res.Resource.(*rule.Rule)
			if !ok {
				continue
			}
			name := r.Name
			if name == "" {
				name = filepath.Base(r.Path)
			}

			// Check if already exists
			existingPath := filepath.Join(rulesDir, name+".md")
			if _, err := os.Stat(existingPath); err == nil {
				if !importForce {
					out.Info("Skipping rule %q from %s (already exists)", name, res.Tool)
					continue
				}
				if importDryRun {
					out.Info("[dry-run] Would overwrite rule %q from %s", name, res.Tool)
				} else {
					out.Info("Overwriting rule %q from %s", name, res.Tool)
				}
			}

			if importDryRun {
				out.Info("[dry-run] Would import rule %q from %s", name, res.Tool)
				ruleCount++
				continue
			}

			if err := rule.Save(r, rulesDir); err != nil {
				out.Warning("Failed to save rule %q: %v", name, err)
				continue
			}
			ruleCount++

		case "command":
			if !importAllTypes && !importCommands {
				continue
			}
			cmd, ok := res.Resource.(*command.Command)
			if !ok || cmd.Name == "" {
				continue
			}

			// Check if already exists
			existingPath := filepath.Join(commandsDir, cmd.Name+".json")
			if command.Exists(existingPath) {
				if !importForce {
					out.Info("Skipping command %q from %s (already exists)", cmd.Name, res.Tool)
					continue
				}
				if importDryRun {
					out.Info("[dry-run] Would overwrite command %q from %s", cmd.Name, res.Tool)
				} else {
					out.Info("Overwriting command %q from %s", cmd.Name, res.Tool)
				}
			}

			if importDryRun {
				out.Info("[dry-run] Would import command %q from %s", cmd.Name, res.Tool)
				commandCount++
				continue
			}

			if err := command.Save(cmd, commandsDir); err != nil {
				out.Warning("Failed to save command %q: %v", cmd.Name, err)
				continue
			}
			commandCount++

		case "skill":
			if !importAllTypes && !importSkills {
				continue
			}
			s, ok := res.Resource.(*skill.Skill)
			if !ok || s.Name == "" {
				continue
			}

			// Check if already exists
			existingPath := filepath.Join(skillsDir, s.Name)
			if _, err := os.Stat(existingPath); err == nil {
				if !importForce {
					out.Info("Skipping skill %q from %s (already exists)", s.Name, res.Tool)
					continue
				}
				if importDryRun {
					out.Info("[dry-run] Would overwrite skill %q from %s", s.Name, res.Tool)
				} else {
					out.Info("Overwriting skill %q from %s", s.Name, res.Tool)
				}
			}

			if importDryRun {
				out.Info("[dry-run] Would import skill %q from %s", s.Name, res.Tool)
				skillCount++
				continue
			}

			// Skills are saved to a directory named after the skill
			skillDir := filepath.Join(skillsDir, s.Name)
			if err := s.Save(skillDir); err != nil {
				out.Warning("Failed to save skill %q: %v", s.Name, err)
				continue
			}
			skillCount++

		case "plugin":
			// Plugins can't be "imported" - they're installed via the tool
			// Just count them for display
			pluginCount++
		}
	}

	// Save config if we imported servers (not in dry-run mode)
	if !importDryRun && serverCount > 0 {
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	}

	// Summary
	out.Println("")
	prefix := ""
	if importDryRun {
		prefix = "[dry-run] Would import "
	} else {
		prefix = "Imported "
	}

	total := serverCount + commandCount + ruleCount + skillCount
	if serverCount > 0 {
		out.Success("%s%d server(s)", prefix, serverCount)
	}
	if commandCount > 0 {
		out.Success("%s%d command(s)", prefix, commandCount)
	}
	if ruleCount > 0 {
		out.Success("%s%d rule(s)", prefix, ruleCount)
	}
	if skillCount > 0 {
		out.Success("%s%d skill(s)", prefix, skillCount)
	}
	if pluginCount > 0 {
		out.Info("Found %d plugin(s) (plugins are managed by the tool, not imported)", pluginCount)
	}

	if total == 0 {
		out.Info("No new resources to import")
	} else if !importDryRun {
		out.Println("")
		out.Info("Run 'agentctl sync' to sync to all tools")
	}

	return nil
}
