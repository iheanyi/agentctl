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

	"github.com/spf13/cobra"

	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/output"
	"github.com/iheanyi/agentctl/pkg/sync"
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
	doctorVerbose  bool
	doctorRunTools bool
)

func init() {
	doctorCmd.Flags().BoolVarP(&doctorVerbose, "verbose", "v", false, "Show detailed output")
	doctorCmd.Flags().BoolVar(&doctorRunTools, "tools", false, "Run tool-specific doctor commands")
}

func runDoctor(cmd *cobra.Command, args []string) error {
	// JSON output data
	doctorOutput := output.DoctorOutput{
		Runtimes: []output.DoctorRuntimeResult{},
		Tools:    []output.DoctorToolResult{},
		Servers:  []output.DoctorServerResult{},
	}

	if !JSONOutput {
		fmt.Println("Running health checks...")
		fmt.Println()
	}

	issues := 0

	// Check agentctl config
	if !JSONOutput {
		fmt.Println("agentctl Configuration:")
	}
	cfg, err := config.Load()
	if err != nil {
		doctorOutput.Config = output.DoctorConfigResult{
			Valid: false,
			Error: err.Error(),
		}
		if !JSONOutput {
			fmt.Printf("  ✗ Failed to load config: %v\n", err)
		}
		issues++
	} else {
		path := shortenPath(cfg.Path)
		doctorOutput.Config = output.DoctorConfigResult{
			Path:        cfg.Path,
			Valid:       true,
			ServerCount: len(cfg.Servers),
		}
		if !JSONOutput {
			fmt.Printf("  ✓ Config: %s\n", path)
			fmt.Printf("    %d server(s) configured\n", len(cfg.Servers))
		}
	}
	if !JSONOutput {
		fmt.Println()
	}

	// Check runtimes
	if !JSONOutput {
		fmt.Println("Runtimes:")
	}
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
		rtResult := output.DoctorRuntimeResult{
			Name:     rt.name,
			Command:  rt.command,
			Required: rt.required,
		}
		if res.err != nil {
			rtResult.Found = false
			if rt.required {
				if !JSONOutput {
					fmt.Printf("  ✗ %s: not found (required)\n", rt.name)
				}
				issues++
			} else {
				if !JSONOutput {
					fmt.Printf("  - %s: not found\n", rt.name)
				}
			}
		} else {
			rtResult.Found = true
			rtResult.Version = res.version
			if !JSONOutput {
				fmt.Printf("  ✓ %s: %s\n", rt.name, res.version)
			}
		}
		doctorOutput.Runtimes = append(doctorOutput.Runtimes, rtResult)
	}
	if !JSONOutput {
		fmt.Println()
	}

	// Check detected tools and their configs
	if !JSONOutput {
		fmt.Println("Tools:")
	}
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
		toolRes := output.DoctorToolResult{
			Name:       adapter.Name(),
			ConfigPath: adapter.ConfigPath(),
			Detected:   res.detected,
		}

		if res.detected {
			detectedCount++
			path := shortenPath(adapter.ConfigPath())

			if res.configErr != nil {
				toolRes.Valid = false
				toolRes.Error = res.configErr.Error()
				if !JSONOutput {
					fmt.Printf("  ✗ %s: %s\n", adapter.Name(), path)
					fmt.Printf("      Error: %v\n", res.configErr)
				}
				issues++
			} else if !res.valid {
				toolRes.Valid = false
				if !JSONOutput {
					fmt.Printf("  ⚠ %s: %s (config issues)\n", adapter.Name(), path)
				}
			} else {
				toolRes.Valid = true
				toolRes.ServerCount = res.serverCount
				if !JSONOutput {
					fmt.Printf("  ✓ %s: %s (%d servers)\n", adapter.Name(), path, res.serverCount)
				}
			}
		} else if doctorVerbose && !JSONOutput {
			fmt.Printf("  - %s: not installed\n", adapter.Name())
		}
		doctorOutput.Tools = append(doctorOutput.Tools, toolRes)
	}

	if detectedCount == 0 {
		if !JSONOutput {
			fmt.Println("  No supported tools detected!")
			fmt.Println("  Supported: Claude Code, Cursor, Codex, OpenCode, Cline, Windsurf, Zed, Continue")
		}
		issues++
	}
	if !JSONOutput {
		fmt.Println()
	}

	// Run tool-specific doctor commands if requested (skip in JSON mode)
	if doctorRunTools && !JSONOutput {
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
			cmdOutput, err := toolCmd.CombinedOutput()
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
				outputStr := string(cmdOutput)
				if strings.Contains(outputStr, "Raw mode") || strings.Contains(outputStr, "stdin") {
					fmt.Printf("    ⚠ %s doctor requires interactive terminal\n", td.name)
					fmt.Printf("      Run '%s %s' directly for full diagnostics.\n", td.command, strings.Join(td.args, " "))
				} else {
					fmt.Printf("    ✗ %s doctor failed\n", td.name)
					if doctorVerbose && len(cmdOutput) > 0 {
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
				if doctorVerbose && len(cmdOutput) > 0 {
					lines := strings.Split(strings.TrimSpace(string(cmdOutput)), "\n")
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
		if !JSONOutput {
			fmt.Println("MCP Servers:")
		}
		for _, server := range cfg.Servers {
			serverRes := output.DoctorServerResult{
				Name:     server.Name,
				Disabled: server.Disabled,
			}

			if server.Disabled {
				serverRes.Available = false
				if doctorVerbose && !JSONOutput {
					fmt.Printf("  - %s: disabled\n", server.Name)
				}
				doctorOutput.Servers = append(doctorOutput.Servers, serverRes)
				continue
			}

			if server.URL != "" {
				serverRes.Type = "http"
				serverRes.Available = true
				if !JSONOutput {
					fmt.Printf("  ✓ %s (http)\n", server.Name)
				}
			} else if server.Command != "" {
				serverRes.Type = "stdio"
				_, err := exec.LookPath(server.Command)
				if err != nil {
					serverRes.Available = false
					serverRes.Error = fmt.Sprintf("command not found: %s", server.Command)
					if !JSONOutput {
						fmt.Printf("  ✗ %s: command not found: %s\n", server.Name, server.Command)
					}
					issues++
				} else {
					serverRes.Available = true
					if !JSONOutput {
						fmt.Printf("  ✓ %s (stdio)\n", server.Name)
					}
				}
			}
			doctorOutput.Servers = append(doctorOutput.Servers, serverRes)
		}
		if !JSONOutput {
			fmt.Println()
		}
	}

	// Check sync state
	if !JSONOutput {
		fmt.Println("Sync State:")
	}
	state, err := sync.LoadState()
	if err != nil {
		doctorOutput.SyncState = output.DoctorSyncState{
			Valid: false,
			Error: err.Error(),
		}
		if !JSONOutput {
			fmt.Printf("  ✗ Cannot load state: %v\n", err)
		}
		issues++
	} else {
		totalManaged := 0
		adapterCount := 0
		for adapterName, servers := range state.ManagedServers {
			if len(servers) > 0 {
				adapterCount++
				totalManaged += len(servers)
				if doctorVerbose && !JSONOutput {
					fmt.Printf("    %s: %d managed server(s)\n", adapterName, len(servers))
				}
			}
		}

		statePath := filepath.Join(config.DefaultConfigDir(), "sync-state.json")
		doctorOutput.SyncState = output.DoctorSyncState{
			Path:           statePath,
			Valid:          true,
			ManagedServers: totalManaged,
			AdapterCount:   adapterCount,
		}

		if !JSONOutput {
			shortStatePath := shortenPath(statePath)
			if totalManaged > 0 {
				fmt.Printf("  ✓ State: %s\n", shortStatePath)
				fmt.Printf("    Tracking %d server(s) across %d adapter(s)\n", totalManaged, adapterCount)
			} else {
				fmt.Printf("  - State: %s (empty)\n", shortStatePath)
			}
		}
	}
	if !JSONOutput {
		fmt.Println()
	}

	// System info
	doctorOutput.System = output.DoctorSystemInfo{
		OS:              runtime.GOOS,
		Arch:            runtime.GOARCH,
		AgentctlVersion: Version,
		AgentctlCommit:  Commit,
	}
	if !JSONOutput {
		fmt.Println("System:")
		fmt.Printf("  OS: %s\n", runtime.GOOS)
		fmt.Printf("  Arch: %s\n", runtime.GOARCH)
		fmt.Printf("  agentctl: %s (%s)\n", Version, Commit)
		fmt.Println()
	}

	doctorOutput.IssueCount = issues

	// JSON output
	if JSONOutput {
		jw := output.NewJSONWriter()
		return jw.Write(output.CLIOutput{
			Success: issues == 0,
			Data:    doctorOutput,
			Error: func() string {
				if issues > 0 {
					return fmt.Sprintf("health check found %d issue(s)", issues)
				}
				return ""
			}(),
		})
	}

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
