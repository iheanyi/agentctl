package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	stdsync "sync"
	"time"

	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/sync"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run health checks on tools and configurations",
	Long: `Run comprehensive health checks on all installed tools and configurations.

This checks:
- Tool installation and versions
- Configuration file validity
- Required runtimes (Node.js, Python, etc.)
- MCP server command availability
- Sync state consistency

Use --tools to also run each tool's native doctor command (if available).

Examples:
  agentctl doctor           # Run all health checks
  agentctl doctor -v        # Show detailed output
  agentctl doctor --tools   # Also run tool-specific doctors (claude doctor, etc.)`,
	RunE: runDoctor,
}

var (
	doctorVerbose   bool
	doctorRunTools  bool
)

func init() {
	doctorCmd.Flags().BoolVarP(&doctorVerbose, "verbose", "v", false, "Show detailed output")
	doctorCmd.Flags().BoolVar(&doctorRunTools, "tools", false, "Run tool-specific doctor commands")
}

func runDoctor(cmd *cobra.Command, args []string) error {
	fmt.Println("Running health checks...")
	fmt.Println()

	issues := 0

	// Check agentctl config
	fmt.Println("agentctl Configuration:")
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("  ✗ Failed to load config: %v\n", err)
		issues++
	} else {
		path := shortenPath(cfg.Path)
		fmt.Printf("  ✓ Config: %s\n", path)
		fmt.Printf("    %d server(s) configured\n", len(cfg.Servers))
	}
	fmt.Println()

	// Check runtimes
	fmt.Println("Runtimes:")
	runtimes := []struct {
		name     string
		command  string
		args     []string
		required bool
	}{
		{"Node.js", "node", []string{"--version"}, true},
		{"npm", "npm", []string{"--version"}, true},
		{"npx", "npx", []string{"--version"}, true},
		{"Python", "python3", []string{"--version"}, false},
		{"uv", "uv", []string{"--version"}, false},
		{"Go", "go", []string{"version"}, false},
		{"Docker", "docker", []string{"--version"}, false},
	}

	type runtimeResult struct {
		version string
		err     error
	}
	runtimeResults := make([]runtimeResult, len(runtimes))
	var rtWg stdsync.WaitGroup

	for i, rt := range runtimes {
		rtWg.Add(1)
		go func(i int, cmd string, args []string) {
			defer rtWg.Done()
			v, err := getVersion(cmd, args)
			runtimeResults[i] = runtimeResult{version: v, err: err}
		}(i, rt.command, rt.args)
	}
	rtWg.Wait()

	for i, rt := range runtimes {
		res := runtimeResults[i]
		if res.err != nil {
			if rt.required {
				fmt.Printf("  ✗ %s: not found (required)\n", rt.name)
				issues++
			} else {
				fmt.Printf("  - %s: not found\n", rt.name)
			}
		} else {
			fmt.Printf("  ✓ %s: %s\n", rt.name, res.version)
		}
	}
	fmt.Println()

	// Check detected tools and their configs
	fmt.Println("Tools:")
	adapters := sync.All()
	detectedCount := 0

	type toolResult struct {
		detected    bool
		detectErr   error
		valid       bool
		serverCount int
		configErr   error
	}

	toolResults := make([]toolResult, len(adapters))
	var toolWg stdsync.WaitGroup

	for i, adapter := range adapters {
		toolWg.Add(1)
		go func(i int, adapter sync.Adapter) {
			defer toolWg.Done()
			detected, detectErr := adapter.Detect()
			res := toolResult{detected: detected, detectErr: detectErr}
			if detected {
				res.valid, res.serverCount, res.configErr = validateToolConfig(adapter)
			}
			toolResults[i] = res
		}(i, adapter)
	}
	toolWg.Wait()

	for i, adapter := range adapters {
		res := toolResults[i]
		if res.detected {
			detectedCount++
			path := shortenPath(adapter.ConfigPath())

			if res.configErr != nil {
				fmt.Printf("  ✗ %s: %s\n", adapter.Name(), path)
				fmt.Printf("      Error: %v\n", res.configErr)
				issues++
			} else if !res.valid {
				fmt.Printf("  ⚠ %s: %s (config issues)\n", adapter.Name(), path)
			} else {
				fmt.Printf("  ✓ %s: %s (%d servers)\n", adapter.Name(), path, res.serverCount)
			}
		} else if doctorVerbose {
			fmt.Printf("  - %s: not installed\n", adapter.Name())
		}
	}

	if detectedCount == 0 {
		fmt.Println("  No supported tools detected!")
		fmt.Println("  Supported: Claude Code, Cursor, Codex, OpenCode, Cline, Windsurf, Zed, Continue")
		issues++
	}
	fmt.Println()

	// Run tool-specific doctor commands if requested
	if doctorRunTools {
		fmt.Println("Tool Doctors:")
		toolDoctors := getToolDoctorCommands()
		ranAny := false

		for _, td := range toolDoctors {
			// Check if tool command exists
			if _, err := exec.LookPath(td.command); err != nil {
				if doctorVerbose {
					fmt.Printf("  - %s: not installed\n", td.name)
				}
				continue
			}

			ranAny = true
			fmt.Printf("  Running %s doctor...\n", td.name)

			// Run the doctor command with 30s timeout
			// Use script wrapper on macOS to provide PTY for tools that need it
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			var toolCmd *exec.Cmd
			if td.needsPTY && runtime.GOOS == "darwin" {
				// Use script -q /dev/null to provide a PTY
				args := append([]string{"-q", "/dev/null", td.command}, td.args...)
				toolCmd = exec.CommandContext(ctx, "script", args...)
			} else {
				toolCmd = exec.CommandContext(ctx, td.command, td.args...)
			}
			output, err := toolCmd.CombinedOutput()
			cancel()

			if ctx.Err() == context.DeadlineExceeded {
				fmt.Printf("    ⚠ %s doctor timed out (30s)\n", td.name)
				if td.needsPTY {
					fmt.Printf("      Note: This tool requires an interactive terminal.\n")
					fmt.Printf("      Run '%s %s' directly for full diagnostics.\n", td.command, strings.Join(td.args, " "))
				}
				// Don't count timeout as issue, just informational
			} else if err != nil {
				// Check if it's a TTY-related error
				outputStr := string(output)
				if strings.Contains(outputStr, "Raw mode") || strings.Contains(outputStr, "stdin") {
					fmt.Printf("    ⚠ %s doctor requires interactive terminal\n", td.name)
					fmt.Printf("      Run '%s %s' directly for full diagnostics.\n", td.command, strings.Join(td.args, " "))
				} else {
					fmt.Printf("    ✗ %s doctor failed\n", td.name)
					if doctorVerbose && len(output) > 0 {
						// Indent output
						lines := strings.Split(strings.TrimSpace(outputStr), "\n")
						for _, line := range lines {
							fmt.Printf("      %s\n", line)
						}
					}
					issues++
				}
			} else {
				fmt.Printf("    ✓ %s doctor passed\n", td.name)
				if doctorVerbose && len(output) > 0 {
					lines := strings.Split(strings.TrimSpace(string(output)), "\n")
					for _, line := range lines {
						fmt.Printf("      %s\n", line)
					}
				}
			}
		}

		if !ranAny {
			fmt.Println("  No tool doctor commands available")
		}
		fmt.Println()
	}

	// Check MCP server commands
	if cfg != nil && len(cfg.Servers) > 0 {
		fmt.Println("MCP Servers:")
		for _, server := range cfg.Servers {
			if server.Disabled {
				if doctorVerbose {
					fmt.Printf("  - %s: disabled\n", server.Name)
				}
				continue
			}

			if server.URL != "" {
				fmt.Printf("  ✓ %s (http)\n", server.Name)
			} else if server.Command != "" {
				_, err := exec.LookPath(server.Command)
				if err != nil {
					fmt.Printf("  ✗ %s: command not found: %s\n", server.Name, server.Command)
					issues++
				} else {
					fmt.Printf("  ✓ %s (stdio)\n", server.Name)
				}
			}
		}
		fmt.Println()
	}

	// Check sync state
	fmt.Println("Sync State:")
	state, err := sync.LoadState()
	if err != nil {
		fmt.Printf("  ✗ Cannot load state: %v\n", err)
		issues++
	} else {
		totalManaged := 0
		adapterCount := 0
		for adapterName, servers := range state.ManagedServers {
			if len(servers) > 0 {
				adapterCount++
				totalManaged += len(servers)
				if doctorVerbose {
					fmt.Printf("    %s: %d managed server(s)\n", adapterName, len(servers))
				}
			}
		}

		statePath := shortenPath(filepath.Join(config.DefaultConfigDir(), "sync-state.json"))
		if totalManaged > 0 {
			fmt.Printf("  ✓ State: %s\n", statePath)
			fmt.Printf("    Tracking %d server(s) across %d adapter(s)\n", totalManaged, adapterCount)
		} else {
			fmt.Printf("  - State: %s (empty)\n", statePath)
		}
	}
	fmt.Println()

	// System info
	fmt.Println("System:")
	fmt.Printf("  OS: %s\n", runtime.GOOS)
	fmt.Printf("  Arch: %s\n", runtime.GOARCH)
	fmt.Printf("  agentctl: %s (%s)\n", Version, Commit)
	fmt.Println()

	if issues > 0 {
		fmt.Printf("Issues found: %d\n", issues)
		return fmt.Errorf("health check found %d issue(s)", issues)
	}

	fmt.Println("All health checks passed!")
	return nil
}

// validateToolConfig validates a tool's config file
func validateToolConfig(adapter sync.Adapter) (valid bool, serverCount int, err error) {
	configPath := adapter.ConfigPath()

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return true, 0, nil // Not created yet is OK
		}
		return false, 0, err
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return false, 0, fmt.Errorf("invalid JSON")
	}

	// Get server section
	var serverKey string
	switch adapter.Name() {
	case "zed":
		serverKey = "context_servers"
	case "opencode":
		serverKey = "mcp"
	default:
		serverKey = "mcpServers"
	}

	if servers, ok := raw[serverKey].(map[string]interface{}); ok {
		serverCount = len(servers)
	}

	return true, serverCount, nil
}

// shortenPath replaces home directory with ~
func shortenPath(path string) string {
	home, _ := os.UserHomeDir()
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

func getVersion(command string, args []string) (string, error) {
	cmd := exec.Command(command, args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Clean up version string
	version := string(output)
	if len(version) > 50 {
		version = version[:50] + "..."
	}
	// Remove newlines
	for i := 0; i < len(version); i++ {
		if version[i] == '\n' || version[i] == '\r' {
			version = version[:i]
			break
		}
	}

	return version, nil
}

// toolDoctor represents a tool's doctor command
type toolDoctor struct {
	name     string   // Display name
	command  string   // Command to run
	args     []string // Arguments
	needsPTY bool     // Whether the command needs a PTY to run
}

// getToolDoctorCommands returns the list of known tool doctor commands
func getToolDoctorCommands() []toolDoctor {
	return []toolDoctor{
		{
			name:     "Claude Code",
			command:  "claude",
			args:     []string{"doctor"},
			needsPTY: true, // claude doctor uses Ink which requires a TTY
		},
		// Add more tools here as they add doctor commands
		// {
		// 	name:    "Cursor",
		// 	command: "cursor",
		// 	args:    []string{"--doctor"},
		// },
	}
}
