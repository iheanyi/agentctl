package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/daemon"
	"github.com/iheanyi/agentctl/pkg/output"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage background daemon",
	Long: `Manage the agentctl background daemon for automatic updates.

The daemon runs in the background and periodically checks for updates
to your installed MCP servers.

Examples:
  agentctl daemon start    # Start the daemon
  agentctl daemon stop     # Stop the daemon
  agentctl daemon status   # Check daemon status`,
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the background daemon",
	RunE:  runDaemonStart,
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the background daemon",
	RunE:  runDaemonStop,
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check daemon status",
	RunE:  runDaemonStatus,
}

var (
	daemonForeground bool
)

func init() {
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonStatusCmd)

	daemonStartCmd.Flags().BoolVarP(&daemonForeground, "foreground", "f", false, "Run in foreground (don't daemonize)")
}

func runDaemonStart(cmd *cobra.Command, args []string) error {
	out := output.DefaultWriter()

	// Check if already running
	if daemon.IsRunning() {
		out.Warning("Daemon is already running")
		return nil
	}

	if daemonForeground {
		// Run in foreground
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		out.Info("Starting daemon in foreground (Ctrl+C to stop)...")
		d := daemon.New(cfg)
		ctx := context.Background()
		return d.Start(ctx)
	}

	// Daemonize - start a new process
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to find executable: %w", err)
	}

	daemonCmd := exec.Command(executable, "daemon", "start", "-f")
	daemonCmd.Stdout = nil
	daemonCmd.Stderr = nil
	daemonCmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	if err := daemonCmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	out.Success("Daemon started (PID: %d)", daemonCmd.Process.Pid)
	return nil
}

func runDaemonStop(cmd *cobra.Command, args []string) error {
	out := output.DefaultWriter()

	if !daemon.IsRunning() {
		out.Warning("Daemon is not running")
		return nil
	}

	// Try to send stop command
	_, err := daemon.SendCommand("stop")
	if err != nil {
		// If socket doesn't work, try to kill by PID
		pidData, err := os.ReadFile(daemon.PIDPath())
		if err != nil {
			return fmt.Errorf("failed to read PID file: %w", err)
		}

		pid, err := strconv.Atoi(string(pidData))
		if err != nil {
			return fmt.Errorf("invalid PID: %w", err)
		}

		process, err := os.FindProcess(pid)
		if err != nil {
			return fmt.Errorf("failed to find process: %w", err)
		}

		if err := process.Kill(); err != nil {
			return fmt.Errorf("failed to kill daemon: %w", err)
		}
	}

	// Clean up
	os.Remove(daemon.SocketPath())
	os.Remove(daemon.PIDPath())

	out.Success("Daemon stopped")
	return nil
}

func runDaemonStatus(cmd *cobra.Command, args []string) error {
	out := output.DefaultWriter()

	if !daemon.IsRunning() {
		out.Println("Daemon is not running")
		out.Info("Start with: agentctl daemon start")
		return nil
	}

	status, err := daemon.GetStatus()
	if err != nil {
		return err
	}

	out.Println("Daemon Status")
	out.Println("")
	out.Println("  Running: %v", status.Running)
	out.Println("  PID: %d", status.PID)
	if !status.StartedAt.IsZero() {
		out.Println("  Started: %s", status.StartedAt.Format("2006-01-02 15:04:05"))
	}
	if !status.LastCheck.IsZero() {
		out.Println("  Last check: %s", status.LastCheck.Format("2006-01-02 15:04:05"))
	}
	out.Println("  Check count: %d", status.CheckCount)

	if len(status.UpdatesAvailable) > 0 {
		out.Println("")
		out.Warning("Updates available:")
		for _, update := range status.UpdatesAvailable {
			out.Println("  • %s", update)
		}
	}

	return nil
}
