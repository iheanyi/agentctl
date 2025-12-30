package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/iheanyi/agentctl/pkg/config"
)

func TestSocketPath(t *testing.T) {
	path := SocketPath()
	if path == "" {
		t.Error("SocketPath should not be empty")
	}
	if !filepath.IsAbs(path) {
		t.Error("SocketPath should be absolute")
	}
}

func TestPIDPath(t *testing.T) {
	path := PIDPath()
	if path == "" {
		t.Error("PIDPath should not be empty")
	}
}

func TestStatusPath(t *testing.T) {
	path := StatusPath()
	if path == "" {
		t.Error("StatusPath should not be empty")
	}
}

func TestNew(t *testing.T) {
	cfg := &config.Config{
		Version: "1",
	}

	d := New(cfg)
	if d == nil {
		t.Fatal("New should return a daemon")
	}
	if d.cfg != cfg {
		t.Error("Daemon should store config")
	}
	if d.status.PID != os.Getpid() {
		t.Errorf("PID = %d, want %d", d.status.PID, os.Getpid())
	}
}

func TestIsRunningNotRunning(t *testing.T) {
	// Clean up any existing socket
	os.Remove(SocketPath())

	if IsRunning() {
		t.Error("IsRunning should return false when daemon is not running")
	}
}

func TestStatus(t *testing.T) {
	status := Status{
		Running:    true,
		PID:        12345,
		CheckCount: 5,
	}

	if !status.Running {
		t.Error("Status.Running should be true")
	}
	if status.PID != 12345 {
		t.Errorf("Status.PID = %d, want 12345", status.PID)
	}
	if status.CheckCount != 5 {
		t.Errorf("Status.CheckCount = %d, want 5", status.CheckCount)
	}
}
