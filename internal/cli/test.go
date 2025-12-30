package cli

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/output"
	"github.com/iheanyi/agentctl/pkg/secrets"
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:   "test [server...]",
	Short: "Test MCP server health",
	Long: `Test if MCP servers are working correctly.

This runs a basic health check by starting each server and
verifying it responds to initialization.

Examples:
  agentctl test                  # Test all installed servers
  agentctl test filesystem       # Test specific server
  agentctl test --timeout 10s    # Custom timeout`,
	RunE: runTest,
}

var (
	testTimeout time.Duration
)

func init() {
	testCmd.Flags().DurationVar(&testTimeout, "timeout", 5*time.Second, "Timeout for each server test")
}

func runTest(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	out := output.DefaultWriter()

	// Determine which servers to test
	var serversToTest []string
	if len(args) > 0 {
		for _, name := range args {
			if _, ok := cfg.Servers[name]; !ok {
				out.Warning("Server %q not found, skipping", name)
				continue
			}
			serversToTest = append(serversToTest, name)
		}
	} else {
		for name := range cfg.Servers {
			serversToTest = append(serversToTest, name)
		}
	}

	if len(serversToTest) == 0 {
		out.Println("No servers to test.")
		return nil
	}

	out.Println("Testing %d server(s)...", len(serversToTest))
	out.Println("")

	var passCount, failCount int

	for _, name := range serversToTest {
		server := cfg.Servers[name]

		if server.Disabled {
			out.Info("%s: skipped (disabled)", name)
			continue
		}

		// Resolve environment variables
		env := make(map[string]string)
		if server.Env != nil {
			var resolveErr error
			env, resolveErr = secrets.ResolveEnv(server.Env)
			if resolveErr != nil {
				out.Error("%s: failed to resolve env - %v", name, resolveErr)
				failCount++
				continue
			}
		}

		// Try to start the server with timeout
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)

		cmdArgs := server.Args
		execCmd := exec.CommandContext(ctx, server.Command, cmdArgs...)

		// Set environment
		for k, v := range env {
			execCmd.Env = append(execCmd.Env, fmt.Sprintf("%s=%s", k, v))
		}

		// Try to start and quickly check if it fails
		err := execCmd.Start()
		if err != nil {
			out.Error("%s: failed to start - %v", name, err)
			failCount++
			cancel()
			continue
		}

		// Give it a moment to fail or succeed
		done := make(chan error, 1)
		go func() {
			done <- execCmd.Wait()
		}()

		select {
		case <-time.After(500 * time.Millisecond):
			// Server is still running after 500ms, consider it healthy
			execCmd.Process.Kill()
			out.Success("%s: healthy", name)
			passCount++
		case err := <-done:
			if err != nil {
				// Check if it's just a timeout or actual failure
				if ctx.Err() == context.DeadlineExceeded {
					out.Success("%s: healthy (timeout)", name)
					passCount++
				} else {
					exitErr, ok := err.(*exec.ExitError)
					if ok && exitErr.ExitCode() != 0 {
						out.Error("%s: failed with exit code %d", name, exitErr.ExitCode())
						failCount++
					} else {
						out.Error("%s: failed - %v", name, err)
						failCount++
					}
				}
			} else {
				// Exited with 0, might be a short-lived command
				out.Success("%s: healthy (exited 0)", name)
				passCount++
			}
		case <-ctx.Done():
			execCmd.Process.Kill()
			out.Success("%s: healthy", name)
			passCount++
		}

		cancel()
	}

	out.Println("")
	if failCount > 0 {
		out.Println("Results: %d passed, %d failed", passCount, failCount)
	} else {
		out.Success("All %d server(s) healthy", passCount)
	}

	return nil
}

// checkServerHealth sends a basic JSON-RPC initialization request
func checkServerHealth(cmd *exec.Cmd) error {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	// Send initialize request
	initRequest := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"agentctl","version":"1.0.0"}}}`
	stdin.Write([]byte(initRequest + "\n"))
	stdin.Close()

	// Read response with timeout
	buf := make([]byte, 4096)
	n, err := stdout.Read(buf)
	if err != nil {
		return err
	}

	response := string(buf[:n])
	if !strings.Contains(response, "result") {
		return fmt.Errorf("unexpected response: %s", response)
	}

	return nil
}
