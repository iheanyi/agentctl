package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/iheanyi/agentctl/pkg/aliases"
	"github.com/iheanyi/agentctl/pkg/builder"
	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/lockfile"
	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/iheanyi/agentctl/pkg/output"
	"github.com/iheanyi/agentctl/pkg/sync"
)

var addCmd = &cobra.Command{
	Use:     "add [name]",
	Aliases: []string{"install"},
	Short:   "Add an MCP server",
	Long: `Add an MCP server from the registry or with explicit config.

If no arguments are provided, launches an interactive form.

Scope:
  By default, servers are added to the local project config (.agentctl.json)
  if it exists, otherwise to the global config (~/.config/agentctl/agentctl.json).
  Use --scope to explicitly choose local or global.

  Local servers sync to workspace configs (.mcp.json, .cursor/mcp.json).
  Global servers sync to global tool configs.

MCP Transport Types:
  stdio  - Local server with command/args
  http   - Remote server with URL
  sse    - Remote server with URL using Server-Sent Events

Examples:
  # Interactive mode
  agentctl add

  # Add from registry
  agentctl add figma
  agentctl add filesystem

  # Add to local project config (explicit)
  agentctl add filesystem --scope local

  # Add to global config (explicit)
  agentctl add figma --scope global

  # Add with explicit URL (http transport)
  agentctl add figma --url https://mcp.figma.com/mcp

  # Add with explicit URL (sse transport)
  agentctl add my-api --url https://api.example.com/mcp/sse --type sse

  # Add with explicit command (stdio transport)
  agentctl add playwright --command npx --args "playwriter@latest"
  agentctl add fs --command npx --args "-y,@modelcontextprotocol/server-filesystem"

  # Preview without adding
  agentctl add figma --dry-run`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAdd,
}

var (
	addNamespace   string
	addNoSync      bool
	addTarget      string
	addForceLocal  bool
	addForceRemote bool
	addCommand     string
	addArgs        string
	addURL         string
	addType        string
	addDryRun      bool
	addHeaders     []string
	addScope       string
)

func init() {
	addCmd.Flags().StringVarP(&addNamespace, "namespace", "n", "", "Namespace prefix for tool names")
	addCmd.Flags().BoolVar(&addNoSync, "no-sync", false, "Don't sync to tools after adding")
	addCmd.Flags().StringVar(&addTarget, "target", "", "Sync to specific tool only (e.g., claude, cursor)")
	addCmd.Flags().BoolVar(&addForceLocal, "local", false, "Force local variant (npx/uvx)")
	addCmd.Flags().BoolVar(&addForceRemote, "remote", false, "Force remote variant")

	// Explicit config flags
	addCmd.Flags().StringVar(&addCommand, "command", "", "Command to run (e.g., npx, uvx)")
	addCmd.Flags().StringVar(&addArgs, "args", "", "Command arguments (comma-separated)")
	addCmd.Flags().StringVar(&addURL, "url", "", "Remote MCP URL")
	addCmd.Flags().StringVar(&addType, "type", "", "Transport type: stdio, http, or sse")
	addCmd.Flags().BoolVar(&addDryRun, "dry-run", false, "Preview config without adding")
	addCmd.Flags().StringArrayVarP(&addHeaders, "header", "H", nil, "HTTP headers (Key: Value)")
	addCmd.Flags().StringVarP(&addScope, "scope", "s", "", "Config scope: local, global (default: local if .agentctl.json exists)")
}

func runAdd(cmd *cobra.Command, args []string) error {
	out := output.DefaultWriter()

	// Determine effective scope
	var scope config.Scope
	if addScope != "" {
		var err error
		scope, err = config.ParseScope(addScope)
		if err != nil {
			return err
		}
	} else {
		// Smart default: local if project config exists, otherwise global
		if config.HasProjectConfig() {
			scope = config.ScopeLocal
		} else {
			scope = config.ScopeGlobal
		}
	}

	// Load config for the determined scope
	cfg, err := config.LoadScoped(scope)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	var name string
	var server *mcp.Server

	// If no args provided, launch interactive mode
	if len(args) == 0 && addCommand == "" && addURL == "" {
		if err := requireInteractive("add"); err != nil {
			return err
		}
		server, err = runInteractiveAdd(cfg)
		if err != nil {
			return err
		}
	} else {
		// Get name from args
		if len(args) > 0 {
			name = args[0]
		} else {
			return fmt.Errorf("server name is required")
		}

		// Check if already exists
		if _, exists := cfg.Servers[name]; exists {
			return fmt.Errorf("server %q already exists - use 'agentctl remove %s' first", name, name)
		}

		// If explicit flags provided, use them directly
		if addCommand != "" || addURL != "" {
			server, err = buildExplicitServer(name)
			if err != nil {
				return err
			}
		} else {
			// Parse target as registry alias, URL, or git path
			server, err = parseAddTarget(name)
			if err != nil {
				return err
			}
		}
	}

	// Apply namespace if provided
	if addNamespace != "" {
		server.Namespace = addNamespace
	}

	// Format and display the config that will be added
	configJSON := formatMCPConfig(server.Name, server)

	out.Println("")
	out.Println("Config to be added:")
	out.Println("%s", configJSON)
	out.Println("")

	// Dry run - just show the config without saving
	if addDryRun {
		out.Info("Dry run - no changes made")
		return nil
	}

	// Handle git sources: Prompt for Command (no clone/build)
	if server.Source.Type == "git" && server.Command == "" {
		out.Println("Adding %s (Source: %s)", server.Name, server.Source.URL)
		out.Println("Please configure the launch command.")

		// Launch interactive config form
		if err := runInteractiveConfig(server, ""); err != nil {
			return err
		}

		// Update displayed config
		configJSON = formatMCPConfig(server.Name, server)
		out.Println("")
		out.Println("Config to be added:")
		out.Println("%s", configJSON)
		out.Println("")
	}

	// Add to config
	if cfg.Servers == nil {
		cfg.Servers = make(map[string]*mcp.Server)
	}
	// Set the server's scope for tracking
	server.Scope = string(scope)
	cfg.Servers[server.Name] = server

	// Save config to the appropriate location based on scope
	if err := cfg.SaveScoped(scope); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	scopeLabel := ""
	if scope == config.ScopeLocal {
		scopeLabel = " (local)"
	} else if scope == config.ScopeGlobal {
		scopeLabel = " (global)"
	}
	out.Success("Added %s%s", server.Name, scopeLabel)

	// Update lockfile
	lf, err := lockfile.Load(cfg.ConfigDir)
	if err == nil {
		entry := &lockfile.LockedEntry{
			Source:  server.Source.URL,
			Version: server.Source.Ref,
		}
		// Note: We are not cloning, so we don't resolve the commit hash here.
		lf.Lock(server.Name, entry)
		_ = lf.Save()
	}

	// Sync to tools
	if !addNoSync {
		out.Println("")
		syncCount := performScopedSync(cfg, server, scope, out, addTarget)
		if syncCount > 0 {
			out.Println("")
			out.Success("Synced to %d tool(s)", syncCount)
		}
	} else {
		out.Println("")
		out.Info("Sync skipped - run 'agentctl sync' to sync to your tools")
	}

	// Check for updates to other installed servers
	updates := checkForUpdates(cfg, lf, server.Name)
	showUpdateHint(updates, out)

	return nil
}

// runInteractiveConfig launches a form to configure command/args for a cloned repo
func runInteractiveConfig(server *mcp.Server, defaultCmd string) error {
	var command, argsStr string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Command").
				Description("The command to run this server").
				Placeholder("e.g. npx -y server-pkg, go run ., /path/to/binary").
				Value(&command).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("command is required")
					}
					return nil
				}),
			huh.NewInput().
				Title("Arguments").
				Description("Arguments (space separated)").
				Placeholder("--verbose").
				Value(&argsStr),
		),
	)

	// Set default values
	command = defaultCmd

	if err := form.Run(); err != nil {
		return err
	}

	server.Command = command
	if argsStr != "" {
		server.Args = strings.Fields(argsStr)
	}
	server.Transport = mcp.TransportStdio

	return nil
}

// buildExplicitServer creates a server from explicit --command/--url flags
func buildExplicitServer(name string) (*mcp.Server, error) {
	if addCommand != "" && addURL != "" {
		return nil, fmt.Errorf("cannot specify both --command and --url")
	}

	server := &mcp.Server{
		Name: name,
	}

	// URL-based (http/sse)
	if addURL != "" {
		server.URL = addURL
		server.Source = mcp.Source{
			Type: "remote",
			URL:  addURL,
		}

		// Determine transport type
		switch addType {
		case "sse":
			server.Transport = mcp.TransportSSE
		case "http", "":
			server.Transport = mcp.TransportHTTP
		default:
			return nil, fmt.Errorf("invalid transport type %q (use 'http' or 'sse')", addType)
		}

		// Parse headers
		if len(addHeaders) > 0 {
			server.Headers = make(map[string]string)
			for _, h := range addHeaders {
				parts := strings.SplitN(h, ":", 2)
				if len(parts) == 2 {
					server.Headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
				}
			}
		}

		return server, nil
	}

	// Command-based (stdio)
	server.Command = addCommand
	server.Transport = mcp.TransportStdio
	server.Source = mcp.Source{Type: "manual"}

	if addArgs != "" {
		// Support comma-separated or space-separated
		if strings.Contains(addArgs, ",") {
			server.Args = strings.Split(addArgs, ",")
		} else {
			server.Args = strings.Fields(addArgs)
		}
	}

	return server, nil
}

// runInteractiveAdd launches an interactive form to add an MCP server
func runInteractiveAdd(cfg *config.Config) (*mcp.Server, error) {
	var (
		name      string
		transport string
		command   string
		argsStr   string
		url       string
	)

	// Step 1: Get server name
	nameForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Server name").
				Description("A unique identifier for this MCP server").
				Placeholder("e.g., figma, my-server").
				Value(&name).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("name is required")
					}
					if _, exists := cfg.Servers[s]; exists {
						return fmt.Errorf("server %q already exists", s)
					}
					return nil
				}),
		),
	)

	if err := nameForm.Run(); err != nil {
		return nil, err
	}

	// Step 2: Choose transport type
	transportForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Transport type").
				Description("How does this MCP server communicate?").
				Options(
					huh.NewOption("stdio - Local server with command/args (npx, uvx, etc.)", "stdio"),
					huh.NewOption("http - Remote server with URL", "http"),
					huh.NewOption("sse - Remote server with Server-Sent Events", "sse"),
				).
				Value(&transport),
		),
	)

	if err := transportForm.Run(); err != nil {
		return nil, err
	}

	server := &mcp.Server{
		Name: name,
	}

	// Step 3: Get transport-specific config
	if transport == "stdio" {
		stdioForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Command").
					Description("The command to run (e.g., npx, uvx, node)").
					Placeholder("npx").
					Value(&command).
					Validate(func(s string) error {
						if s == "" {
							return fmt.Errorf("command is required")
						}
						return nil
					}),
				huh.NewInput().
					Title("Arguments").
					Description("Command arguments (space or comma separated)").
					Placeholder("e.g., -y @modelcontextprotocol/server-filesystem").
					Value(&argsStr),
			),
		)

		if err := stdioForm.Run(); err != nil {
			return nil, err
		}

		server.Command = command
		server.Transport = mcp.TransportStdio
		server.Source = mcp.Source{Type: "manual"}

		if argsStr != "" {
			if strings.Contains(argsStr, ",") {
				server.Args = strings.Split(argsStr, ",")
			} else {
				server.Args = strings.Fields(argsStr)
			}
		}
	} else {
		// http or sse
		urlForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("URL").
					Description("The remote MCP server URL").
					Placeholder("https://mcp.example.com/mcp").
					Value(&url).
					Validate(func(s string) error {
						if s == "" {
							return fmt.Errorf("URL is required")
						}
						if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
							return fmt.Errorf("URL must start with http:// or https://")
						}
						return nil
					}),
			),
		)

		if err := urlForm.Run(); err != nil {
			return nil, err
		}

		server.URL = url
		server.Transport = mcp.Transport(transport)
		server.Source = mcp.Source{Type: "remote", URL: url}
	}

	return server, nil
}

// formatMCPConfig formats a server as clean MCP JSON config
func formatMCPConfig(name string, server *mcp.Server) string {
	config := make(map[string]any)

	if server.URL != "" {
		config["url"] = server.URL
		config["type"] = string(server.Transport)
		if len(server.Headers) > 0 {
			config["headers"] = server.Headers
		}
	} else {
		config["command"] = server.Command
		if len(server.Args) > 0 {
			config["args"] = server.Args
		}
	}

	if len(server.Env) > 0 {
		config["env"] = server.Env
	}

	wrapper := map[string]any{name: config}
	data, _ := json.MarshalIndent(wrapper, "", "  ")
	return string(data)
}

// performScopedSync syncs a server to tools based on scope
// Local scope: sync to workspace configs for tools that support it
// Global scope: sync to global tool configs
func performScopedSync(cfg *config.Config, server *mcp.Server, scope config.Scope, out *output.Writer, targetTool string) int {
	out.Println("Syncing to tools...")

	var adapters []sync.Adapter
	if targetTool != "" {
		adapter, ok := sync.Get(targetTool)
		if !ok {
			out.Warning("Unknown tool: %s", targetTool)
			return 0
		}
		adapters = []sync.Adapter{adapter}
	} else {
		adapters = sync.Detected()
	}

	servers := []*mcp.Server{server}
	syncedCount := 0

	// Get project directory for workspace configs
	projectDir := ""
	if scope == config.ScopeLocal {
		projectDir = cfg.ProjectDir()
		if projectDir == "" {
			// Try current working directory
			if cwd, err := os.Getwd(); err == nil {
				projectDir = cwd
			}
		}
	}

	for _, adapter := range adapters {
		detected, err := adapter.Detect()
		if err != nil || !detected {
			continue
		}

		toolName := adapter.Name()
		configPath := adapter.ConfigPath()

		supported := adapter.SupportedResources()
		if !containsResourceType(supported, sync.ResourceMCP) {
			continue
		}

		// Check transport compatibility
		if server.Transport == mcp.TransportHTTP || server.Transport == mcp.TransportSSE {
			supportsHTTP := toolName == "claude" || toolName == "claude-desktop"
			if !supportsHTTP {
				out.Println("  - %s (no HTTP/SSE support)", toolName)
				continue
			}
		}

		// Handle scoped sync
		if scope == config.ScopeLocal && projectDir != "" {
			// Try to use workspace config if adapter supports it
			if wa, ok := sync.AsWorkspaceAdapter(adapter); ok {
				workspacePath := wa.WorkspaceConfigPath(projectDir)

				// Read existing workspace servers and merge
				existingServers, _ := wa.ReadWorkspaceServers(projectDir)
				mergedServers := mergeServers(existingServers, servers)

				if err := wa.WriteWorkspaceServers(projectDir, mergedServers); err != nil {
					out.Println("  x %s - %v", toolName, err)
					continue
				}
				out.Println("  + %s (%s)", toolName, workspacePath)
				syncedCount++
				continue
			}
			// Tool doesn't support workspace configs - warn and sync to global
			out.Warning("  %s doesn't support workspace configs, syncing to global", toolName)
		}

		// Global sync: read existing servers and merge
		sa, ok := sync.AsServerAdapter(adapter)
		if !ok {
			out.Println("  x %s - doesn't support servers", toolName)
			continue
		}
		existingServers, _ := sa.ReadServers()
		mergedServers := mergeServers(existingServers, servers)

		if err := sa.WriteServers(mergedServers); err != nil {
			out.Println("  x %s - %v", toolName, err)
			continue
		}

		out.Println("  + %s (%s)", toolName, configPath)
		syncedCount++
	}

	return syncedCount
}

// mergeServers merges new servers into existing servers, replacing by name
func mergeServers(existing, new []*mcp.Server) []*mcp.Server {
	serverMap := make(map[string]*mcp.Server)
	for _, s := range existing {
		serverMap[s.Name] = s
	}
	for _, s := range new {
		serverMap[s.Name] = s
	}
	result := make([]*mcp.Server, 0, len(serverMap))
	for _, s := range serverMap {
		result = append(result, s)
	}
	return result
}

// performSync syncs config to detected tools and returns count (legacy, kept for compatibility)
func performSync(cfg *config.Config, server *mcp.Server, out *output.Writer, targetTool string) int {
	return performScopedSync(cfg, server, config.ScopeGlobal, out, targetTool)
}

func parseAddTarget(target string) (*mcp.Server, error) {
	// Check for version suffix (name@version)
	var version string
	if idx := strings.LastIndex(target, "@"); idx > 0 {
		version = target[idx+1:]
		target = target[:idx]
	}

	// Check if it's a local path
	if strings.HasPrefix(target, "./") || strings.HasPrefix(target, "/") || strings.HasPrefix(target, "../") {
		return &mcp.Server{
			Name: pathToName(target),
			Source: mcp.Source{
				Type: "local",
				URL:  target,
			},
			Transport: mcp.TransportStdio,
		}, nil
	}

	// Check if it's a remote MCP URL (http/https)
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		return &mcp.Server{
			Name: urlToName(target),
			Source: mcp.Source{
				Type: "remote",
				URL:  target,
			},
			URL:       target,
			Transport: mcp.TransportHTTP,
		}, nil
	}

	// Determine variant preference from flags
	variantPref := ""
	if addForceLocal {
		variantPref = "local"
	} else if addForceRemote {
		variantPref = "remote"
	}

	// Try to resolve as alias with variant
	alias, resolvedVariant, ok := aliases.ResolveVariant(target, variantPref)
	if !ok {
		return nil, fmt.Errorf("unknown alias %q - use full git URL or 'agentctl search' to find servers", target)
	}

	server := &mcp.Server{
		Name: target,
		Source: mcp.Source{
			Type:  "alias",
			Alias: target,
			URL:   alias.URL,
			Ref:   version,
		},
	}

	// Handle remote HTTP/SSE MCP servers (like Sentry, Figma)
	if alias.Transport == "http" || alias.Transport == "sse" {
		server.Transport = mcp.Transport(alias.Transport)
		server.URL = alias.MCPURL
		server.Source.URL = alias.MCPURL
		server.Source.Type = "remote"
		if resolvedVariant != "" {
			server.Name = target // Keep base name, variant is transparent
		}
		return server, nil
	}

	// Local server with stdio transport
	server.Transport = mcp.TransportStdio

	// Use package name if available, otherwise fall back to target name
	packageName := alias.Package
	if packageName == "" {
		packageName = target
	}

	// Set command based on runtime
	switch alias.Runtime {
	case "node":
		server.Command = "npx"
		server.Args = []string{"-y", packageName}
	case "python":
		server.Command = "uvx"
		server.Args = []string{packageName}
	case "go":
		server.Command = "go"
		server.Args = []string{"run", alias.URL}
	default:
		// Default to node
		server.Command = "npx"
		server.Args = []string{"-y", packageName}
	}

	return server, nil
}

func pathToName(path string) string {
	// Extract name from path
	parts := strings.Split(path, "/")
	name := parts[len(parts)-1]

	// Remove common suffixes
	name = strings.TrimSuffix(name, "-mcp")
	name = strings.TrimSuffix(name, "-server")
	name = strings.TrimSuffix(name, ".git")

	return name
}

func urlToName(url string) string {
	// Extract name from URL like https://mcp.sentry.dev/mcp -> sentry
	// or https://mcp.figma.com/mcp -> figma
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")

	// Get the domain part
	parts := strings.Split(url, "/")
	domain := parts[0]

	// Extract service name from domain
	// mcp.sentry.dev -> sentry
	// mcp.figma.com -> figma
	domainParts := strings.Split(domain, ".")
	if len(domainParts) >= 2 {
		// If it starts with "mcp.", use the second part
		if domainParts[0] == "mcp" && len(domainParts) > 1 {
			return domainParts[1]
		}
		// Otherwise use the first part
		return domainParts[0]
	}

	return domain
}

// checkForUpdates checks if other installed servers have updates available
// Returns a list of server names that have updates
func checkForUpdates(cfg *config.Config, lf *lockfile.Lockfile, excludeServer string) []string {
	var updatesAvailable []string
	b := builder.New(cfg.CacheDir())

	for name, server := range cfg.Servers {
		// Skip the server we just installed
		if name == excludeServer {
			continue
		}

		// Only check git sources for now
		if server.Source.Type != "git" {
			continue
		}

		// Check if installed
		if !b.Installed(server) {
			continue
		}

		// Check if lockfile entry exists and is old enough to warrant a check
		entry, ok := lf.Get(name)
		if !ok {
			continue
		}

		// Skip if updated within the last 24 hours
		if time.Since(entry.UpdatedAt) < 24*time.Hour && !entry.UpdatedAt.IsZero() {
			continue
		}
		if entry.UpdatedAt.IsZero() && time.Since(entry.InstalledAt) < 24*time.Hour {
			continue
		}

		// Quick check for remote updates using git ls-remote
		if hasRemoteUpdate(b.ServerDir(server), entry.Commit) {
			updatesAvailable = append(updatesAvailable, name)
		}
	}

	return updatesAvailable
}

// hasRemoteUpdate checks if there are remote commits newer than the local commit
func hasRemoteUpdate(dir string, localCommit string) bool {
	if localCommit == "" {
		return false
	}

	// Use git ls-remote to get remote HEAD without fetching
	cmd := exec.Command("git", "ls-remote", "origin", "HEAD")
	cmd.Dir = dir

	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Parse output: "commit_hash\tHEAD"
	parts := strings.Fields(string(output))
	if len(parts) < 1 {
		return false
	}

	remoteCommit := parts[0]
	return remoteCommit != localCommit
}

// showUpdateHint displays a non-blocking hint about available updates
func showUpdateHint(updates []string, out *output.Writer) {
	if len(updates) == 0 {
		return
	}

	out.Println("")
	if len(updates) == 1 {
		out.Info("💡 1 update available: %s. Run 'agentctl update' to upgrade.", updates[0])
	} else if len(updates) <= 3 {
		out.Info("💡 %d updates available: %s. Run 'agentctl update' to upgrade.",
			len(updates), strings.Join(updates, ", "))
	} else {
		out.Info("💡 %d updates available. Run 'agentctl update' to upgrade.", len(updates))
	}
}
