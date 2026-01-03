package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

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

Examples:
  agentctl doctor           # Run all health checks
  agentctl doctor -v        # Show detailed output`,
	RunE: runDoctor,
}

var doctorVerbose bool

func init() {
	doctorCmd.Flags().BoolVarP(&doctorVerbose, "verbose", "v", false, "Show detailed output")
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

	for _, rt := range runtimes {
		version, err := getVersion(rt.command, rt.args)
		if err != nil {
			if rt.required {
				fmt.Printf("  ✗ %s: not found (required)\n", rt.name)
				issues++
			} else {
				fmt.Printf("  - %s: not found\n", rt.name)
			}
		} else {
			fmt.Printf("  ✓ %s: %s\n", rt.name, version)
		}
	}
	fmt.Println()

	// Check detected tools and their configs
	fmt.Println("Tools:")
	adapters := sync.All()
	detectedCount := 0
	for _, adapter := range adapters {
		detected, _ := adapter.Detect()
		if detected {
			detectedCount++
			configValid, serverCount, configErr := validateToolConfig(adapter)
			path := shortenPath(adapter.ConfigPath())

			if configErr != nil {
				fmt.Printf("  ✗ %s: %s\n", adapter.Name(), path)
				fmt.Printf("      Error: %v\n", configErr)
				issues++
			} else if !configValid {
				fmt.Printf("  ⚠ %s: %s (config issues)\n", adapter.Name(), path)
			} else {
				fmt.Printf("  ✓ %s: %s (%d servers)\n", adapter.Name(), path, serverCount)
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
