package cli

import (
	"context"
	"fmt"
	"sort"
	stdsync "sync"
	"time"

	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/iheanyi/agentctl/pkg/mcpclient"
	"github.com/iheanyi/agentctl/pkg/output"
	"github.com/iheanyi/agentctl/pkg/secrets"
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:   "test [server...]",
	Short: "Test MCP server health",
	Long: `Test if MCP servers are working correctly.

This runs a comprehensive health check by:
1. Connecting to the server
2. Performing the MCP initialization handshake
3. Listing available tools
4. Verifying the server responds correctly

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
	testCmd.Flags().DurationVar(&testTimeout, "timeout", 10*time.Second, "Timeout for each server test")
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

	// Sort for consistent output
	sort.Strings(serversToTest)

	if len(serversToTest) == 0 {
		out.Println("No servers to test.")
		return nil
	}

	out.Println("Testing %d server(s)...", len(serversToTest))
	out.Println("")

	type testResult struct {
		name    string
		healthy bool
		err     error
		tools   int
		latency time.Duration
		skipped bool
	}

	results := make([]testResult, len(serversToTest))
	var wg stdsync.WaitGroup

	for i, name := range serversToTest {
		server := cfg.Servers[name]

		if server.Disabled {
			results[i] = testResult{name: name, skipped: true}
			continue
		}

		wg.Add(1)
		go func(i int, s *mcp.Server) {
			defer wg.Done()
			
			// Resolve environment variables
			serverCopy := *s
			if s.Env != nil {
				resolvedEnv, err := secrets.ResolveEnv(s.Env)
				if err != nil {
					results[i] = testResult{name: s.Name, healthy: false, err: fmt.Errorf("env error: %w", err)}
					return
				}
				serverCopy.Env = resolvedEnv
			}

			// Run health check
			client := mcpclient.NewClient().WithTimeout(testTimeout)
			health := client.CheckHealth(context.Background(), &serverCopy)

			results[i] = testResult{
				name:    s.Name,
				healthy: health.Healthy,
				err:     health.Error,
				tools:   len(health.Tools),
				latency: health.Latency,
			}
		}(i, server)
	}

	wg.Wait()

	var passCount, failCount int

	for _, res := range results {
		if res.skipped {
			out.Info("%s: skipped (disabled)", res.name)
			continue
		}

		if res.healthy {
			passCount++
			out.Success("%s: healthy (%d tools, %s)", res.name, res.tools, res.latency.Round(time.Millisecond))
		} else {
			failCount++
			out.Error("%s: failed - %v", res.name, res.err)
		}
	}

	out.Println("")
	if failCount > 0 {
		return fmt.Errorf("tests failed: %d passed, %d failed", passCount, failCount)
	}
	
	out.Success("All %d server(s) passed deep validation", passCount)
	return nil
}
