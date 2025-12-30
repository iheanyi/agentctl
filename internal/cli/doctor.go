package cli

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/sync"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose common issues",
	Long: `Check for common issues with agentctl and your tool configurations.

This command will:
- Check if agentctl config exists
- Verify required runtimes are installed (Node.js, Python, etc.)
- Check which tools are detected
- Validate configuration syntax`,
	RunE: runDoctor,
}

func runDoctor(cmd *cobra.Command, args []string) error {
	fmt.Println("Checking agentctl setup...")

	issues := 0

	// Check config
	fmt.Println("Configuration:")
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("  [!] Failed to load config: %v\n", err)
		issues++
	} else {
		fmt.Printf("  [+] Config loaded from %s\n", cfg.Path)
		fmt.Printf("  [+] %d server(s) configured\n", len(cfg.Servers))
	}

	fmt.Println()

	// Check runtimes
	fmt.Println("Runtimes:")
	runtimes := []struct {
		name    string
		command string
		args    []string
	}{
		{"Node.js", "node", []string{"--version"}},
		{"npm", "npm", []string{"--version"}},
		{"npx", "npx", []string{"--version"}},
		{"Python", "python3", []string{"--version"}},
		{"uv", "uv", []string{"--version"}},
		{"Go", "go", []string{"version"}},
		{"Docker", "docker", []string{"--version"}},
	}

	for _, rt := range runtimes {
		version, err := getVersion(rt.command, rt.args)
		if err != nil {
			fmt.Printf("  [ ] %s: not found\n", rt.name)
		} else {
			fmt.Printf("  [+] %s: %s\n", rt.name, version)
		}
	}

	fmt.Println()

	// Check detected tools
	fmt.Println("Detected tools:")
	adapters := sync.All()
	detectedCount := 0
	for _, adapter := range adapters {
		detected, _ := adapter.Detect()
		if detected {
			fmt.Printf("  [+] %s: %s\n", adapter.Name(), adapter.ConfigPath())
			detectedCount++
		} else {
			fmt.Printf("  [ ] %s: not found\n", adapter.Name())
		}
	}

	if detectedCount == 0 {
		fmt.Println("\n  No supported tools detected!")
		fmt.Println("  Install one of: Claude Code, Cursor, Codex, OpenCode, Cline, Windsurf, Zed, Continue")
		issues++
	}

	fmt.Println()

	// System info
	fmt.Println("System:")
	fmt.Printf("  OS: %s\n", runtime.GOOS)
	fmt.Printf("  Arch: %s\n", runtime.GOARCH)
	fmt.Printf("  agentctl: %s (%s)\n", Version, Commit)

	fmt.Println()

	if issues > 0 {
		fmt.Printf("Found %d issue(s) that may affect agentctl functionality.\n", issues)
		return nil
	}

	fmt.Println("Everything looks good!")
	return nil
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
