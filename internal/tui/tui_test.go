package tui

import (
	"testing"

	"github.com/iheanyi/agentctl/pkg/mcp"
)

func TestItem(t *testing.T) {
	item := Item{
		name:        "test-server",
		description: "A test server",
		status:      "enabled",
		server: &mcp.Server{
			Name:    "test-server",
			Command: "node",
		},
	}

	if item.Title() != "test-server" {
		t.Errorf("Title() = %q, want %q", item.Title(), "test-server")
	}

	if item.Description() != "A test server" {
		t.Errorf("Description() = %q, want %q", item.Description(), "A test server")
	}

	if item.FilterValue() != "test-server" {
		t.Errorf("FilterValue() = %q, want %q", item.FilterValue(), "test-server")
	}
}

func TestNewKeyMap(t *testing.T) {
	km := newKeyMap()

	// Verify key bindings exist
	if len(km.Up.Keys()) == 0 {
		t.Error("Up key binding should have keys")
	}
	if len(km.Down.Keys()) == 0 {
		t.Error("Down key binding should have keys")
	}
	if len(km.Quit.Keys()) == 0 {
		t.Error("Quit key binding should have keys")
	}
}
