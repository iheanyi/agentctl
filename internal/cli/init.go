package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/iheanyi/agentctl/pkg/sync"
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
	var resourceDir string

	if initLocal {
		// Project-local config
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		configPath = filepath.Join(cwd, ".agentctl.json")
		configDir = cwd
		resourceDir = filepath.Join(cwd, ".agentctl") // Local resources go in .agentctl/
	} else {
		// Global config
		configDir = config.DefaultConfigDir()
		configPath = filepath.Join(configDir, "agentctl.json")
		resourceDir = configDir // Global resources go in config dir
	}

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("Configuration already exists at %s\n", configPath)
		return nil
	}

	// Create directory structure
	dirs := []string{
		configDir,
		resourceDir,
		filepath.Join(resourceDir, "servers"),
		filepath.Join(resourceDir, "commands"),
		filepath.Join(resourceDir, "rules"),
		filepath.Join(resourceDir, "prompts"),
		filepath.Join(resourceDir, "skills"),
		filepath.Join(resourceDir, "profiles"),
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

	// If --local flag is set, skip interactive mode
	// Otherwise, check if we're in interactive mode and run the import wizard
	if !initLocal {
		if err := requireInteractive("init"); err == nil {
			// Run interactive import wizard
			importedServers, err := runInteractiveInit(cfg)
			if err != nil {
				// User cancelled or error occurred
				if err == huh.ErrUserAborted {
					showCancelHint("init")
					return nil
				}
				return err
			}

			// Add imported servers to config
			for name, server := range importedServers {
				cfg.Servers[name] = server
			}
		}
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Initialized agentctl configuration at %s\n", configPath)

	// Show summary of what was imported
	if len(cfg.Servers) > 0 {
		fmt.Printf("\nImported %d server(s):\n", len(cfg.Servers))
		for name := range cfg.Servers {
			fmt.Printf("  - %s\n", name)
		}
	}

	fmt.Println("\nNext steps:")
	fmt.Println("  agentctl install filesystem   # Install your first MCP server")
	fmt.Println("  agentctl alias list           # See available aliases")
	fmt.Println("  agentctl sync                 # Sync config to your tools")

	return nil
}

// toolServers holds servers discovered from a tool
type toolServers struct {
	adapter sync.Adapter
	servers []*mcp.Server
}

// runInteractiveInit runs the interactive import wizard
func runInteractiveInit(cfg *config.Config) (map[string]*mcp.Server, error) {
	fmt.Println("Welcome to agentctl!")
	fmt.Println()

	// Detect existing tool configs
	detectedTools := detectToolsWithServers()

	if len(detectedTools) == 0 {
		fmt.Println("No existing tool configurations detected.")
		fmt.Println("You can add MCP servers later with 'agentctl add'.")
		return nil, nil
	}

	// Build options for multi-select
	var options []huh.Option[string]
	for _, t := range detectedTools {
		label := fmt.Sprintf("%s (%d server%s)", t.adapter.Name(), len(t.servers), pluralize(len(t.servers)))
		options = append(options, huh.NewOption(label, t.adapter.Name()))
	}

	// Ask which tools to import from
	var selectedTools []string
	selectForm := newStyledForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Detected MCP servers in your tools").
				Description("Select which tools to import servers from").
				Options(options...).
				Value(&selectedTools),
		),
	)

	ok, err := runFormWithCancel(selectForm, "init")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}

	if len(selectedTools) == 0 {
		fmt.Println("No tools selected. You can import later with 'agentctl import'.")
		return nil, nil
	}

	// Collect all servers from selected tools
	allServers := make(map[string][]*serverWithSource)

	for _, toolName := range selectedTools {
		for _, t := range detectedTools {
			if t.adapter.Name() == toolName {
				for _, server := range t.servers {
					sws := &serverWithSource{
						server: server,
						source: toolName,
					}
					allServers[server.Name] = append(allServers[server.Name], sws)
				}
				break
			}
		}
	}

	// Build final servers map, handling conflicts
	result := make(map[string]*mcp.Server)

	// Separate unique servers from conflicts
	var conflicts []string
	for name, sources := range allServers {
		if len(sources) == 1 {
			result[name] = sources[0].server
		} else {
			conflicts = append(conflicts, name)
		}
	}

	// Sort conflicts for consistent ordering
	sort.Strings(conflicts)

	// Handle conflicts one by one
	for _, name := range conflicts {
		sources := allServers[name]
		server, err := resolveConflict(name, sources)
		if err != nil {
			return nil, err
		}
		if server != nil {
			result[name] = server
		}
	}

	return result, nil
}

// serverWithSource holds a server and its source tool
type serverWithSource struct {
	server *mcp.Server
	source string
}

// detectToolsWithServers finds all tools that have MCP servers configured
func detectToolsWithServers() []toolServers {
	var result []toolServers

	for _, adapter := range sync.All() {
		detected, err := adapter.Detect()
		if err != nil || !detected {
			continue
		}

		// Check if this adapter supports MCP
		supported := adapter.SupportedResources()
		supportsMCP := false
		for _, r := range supported {
			if r == sync.ResourceMCP {
				supportsMCP = true
				break
			}
		}
		if !supportsMCP {
			continue
		}

		// Read servers
		sa, ok := sync.AsServerAdapter(adapter)
		if !ok {
			continue
		}
		servers, err := sa.ReadServers()
		if err != nil || len(servers) == 0 {
			continue
		}

		result = append(result, toolServers{
			adapter: adapter,
			servers: servers,
		})
	}

	// Sort by tool name for consistent ordering
	sort.Slice(result, func(i, j int) bool {
		return result[i].adapter.Name() < result[j].adapter.Name()
	})

	return result
}

// resolveConflict asks the user to choose which server config to use when
// the same server name exists in multiple tools
func resolveConflict(name string, sources []*serverWithSource) (*mcp.Server, error) {
	fmt.Printf("\nConflict: server %q found in multiple tools\n", name)

	// Build options showing the config from each source
	var options []huh.Option[int]
	for i, s := range sources {
		label := fmt.Sprintf("%s: %s", s.source, formatServerBrief(s.server))
		options = append(options, huh.NewOption(label, i))
	}
	options = append(options, huh.NewOption("Skip (don't import)", -1))

	var choice int
	selectForm := newStyledForm(
		huh.NewGroup(
			huh.NewSelect[int]().
				Title(fmt.Sprintf("Choose config for %q", name)).
				Description("Select which configuration to use").
				Options(options...).
				Value(&choice),
		),
	)

	ok, err := runFormWithCancel(selectForm, "init")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}

	if choice == -1 {
		return nil, nil
	}

	return sources[choice].server, nil
}

// formatServerBrief returns a brief description of a server config
func formatServerBrief(s *mcp.Server) string {
	if s.URL != "" {
		return fmt.Sprintf("url=%s", s.URL)
	}
	if s.Command != "" {
		args := strings.Join(s.Args, " ")
		if len(args) > 30 {
			args = args[:27] + "..."
		}
		if args != "" {
			return fmt.Sprintf("cmd=%s %s", s.Command, args)
		}
		return fmt.Sprintf("cmd=%s", s.Command)
	}
	return "(empty config)"
}

// pluralize returns "s" for plural counts
func pluralize(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
