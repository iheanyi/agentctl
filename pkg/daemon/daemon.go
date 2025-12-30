package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/iheanyi/agentctl/pkg/config"
)

// Status represents the daemon status
type Status struct {
	Running     bool      `json:"running"`
	PID         int       `json:"pid"`
	StartedAt   time.Time `json:"startedAt,omitempty"`
	LastCheck   time.Time `json:"lastCheck,omitempty"`
	CheckCount  int       `json:"checkCount"`
	UpdatesAvailable []string `json:"updatesAvailable,omitempty"`
}

// Daemon manages background update checks
type Daemon struct {
	cfg      *config.Config
	listener net.Listener
	status   Status
	updates  []string
	mu       sync.RWMutex
	stopCh   chan struct{}
}

// SocketPath returns the path to the daemon socket
func SocketPath() string {
	return filepath.Join(os.TempDir(), "agentctl.sock")
}

// PIDPath returns the path to the daemon PID file
func PIDPath() string {
	return filepath.Join(config.DefaultConfigDir(), "daemon.pid")
}

// StatusPath returns the path to the daemon status file
func StatusPath() string {
	return filepath.Join(config.DefaultConfigDir(), "daemon.status")
}

// New creates a new daemon
func New(cfg *config.Config) *Daemon {
	return &Daemon{
		cfg:    cfg,
		stopCh: make(chan struct{}),
		status: Status{
			PID: os.Getpid(),
		},
	}
}

// Start starts the daemon
func (d *Daemon) Start(ctx context.Context) error {
	// Remove stale socket
	os.Remove(SocketPath())

	// Create Unix socket listener
	listener, err := net.Listen("unix", SocketPath())
	if err != nil {
		return fmt.Errorf("failed to create socket: %w", err)
	}
	d.listener = listener

	// Write PID file
	if err := os.WriteFile(PIDPath(), []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	d.mu.Lock()
	d.status.Running = true
	d.status.StartedAt = time.Now()
	d.mu.Unlock()

	// Save initial status
	d.saveStatus()

	// Start update check loop
	go d.checkLoop(ctx)

	// Accept connections
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-d.stopCh:
			return nil
		default:
			conn, err := listener.Accept()
			if err != nil {
				continue
			}
			go d.handleConnection(conn)
		}
	}
}

// Stop stops the daemon
func (d *Daemon) Stop() error {
	close(d.stopCh)

	if d.listener != nil {
		d.listener.Close()
	}

	os.Remove(SocketPath())
	os.Remove(PIDPath())

	d.mu.Lock()
	d.status.Running = false
	d.mu.Unlock()
	d.saveStatus()

	return nil
}

func (d *Daemon) checkLoop(ctx context.Context) {
	// Get check interval from config
	interval := 24 * time.Hour
	if d.cfg.Settings.AutoUpdate.Interval != "" {
		if parsed, err := time.ParseDuration(d.cfg.Settings.AutoUpdate.Interval); err == nil {
			interval = parsed
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Do an initial check
	d.checkUpdates()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.checkUpdates()
		}
	}
}

func (d *Daemon) checkUpdates() {
	d.mu.Lock()
	d.status.CheckCount++
	d.status.LastCheck = time.Now()
	d.mu.Unlock()

	// Check each server for updates
	var updates []string
	for name, server := range d.cfg.Servers {
		if server.Disabled {
			continue
		}

		// Simplified check - in reality would compare versions
		if server.Source.Type == "git" && server.Source.Ref == "" {
			// Servers without pinned version may have updates
			// This is a placeholder - real implementation would check remote
			_ = name // Would add to updates if newer version available
		}
	}

	d.mu.Lock()
	d.updates = updates
	d.status.UpdatesAvailable = updates
	d.mu.Unlock()

	d.saveStatus()
}

func (d *Daemon) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Read command
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}

	command := string(buf[:n])

	var response []byte
	switch command {
	case "status":
		d.mu.RLock()
		response, _ = json.Marshal(d.status)
		d.mu.RUnlock()
	case "updates":
		d.mu.RLock()
		response, _ = json.Marshal(d.updates)
		d.mu.RUnlock()
	case "check":
		d.checkUpdates()
		response = []byte(`{"ok": true}`)
	case "stop":
		response = []byte(`{"ok": true}`)
		conn.Write(response)
		d.Stop()
		return
	default:
		response = []byte(`{"error": "unknown command"}`)
	}

	conn.Write(response)
}

func (d *Daemon) saveStatus() {
	d.mu.RLock()
	data, _ := json.MarshalIndent(d.status, "", "  ")
	d.mu.RUnlock()

	os.WriteFile(StatusPath(), data, 0644)
}

// Client functions

// IsRunning checks if the daemon is running
func IsRunning() bool {
	conn, err := net.Dial("unix", SocketPath())
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// GetStatus gets the daemon status
func GetStatus() (*Status, error) {
	conn, err := net.Dial("unix", SocketPath())
	if err != nil {
		return nil, fmt.Errorf("daemon not running")
	}
	defer conn.Close()

	conn.Write([]byte("status"))

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}

	var status Status
	if err := json.Unmarshal(buf[:n], &status); err != nil {
		return nil, err
	}

	return &status, nil
}

// SendCommand sends a command to the daemon
func SendCommand(command string) ([]byte, error) {
	conn, err := net.Dial("unix", SocketPath())
	if err != nil {
		return nil, fmt.Errorf("daemon not running")
	}
	defer conn.Close()

	conn.Write([]byte(command))

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}

	return buf[:n], nil
}
