package tui

import (
	"testing"

	"github.com/iheanyi/agentctl/pkg/mcp"
)

func TestServer(t *testing.T) {
	server := Server{
		Name:      "test-server",
		Desc:      "A test server",
		Status:    ServerStatusInstalled,
		Health:    HealthStatusUnknown,
		Transport: "stdio",
		Command:   "node",
		ServerConfig: &mcp.Server{
			Name:    "test-server",
			Command: "node",
		},
	}

	// Title should include status badge and name
	title := server.Title()
	if title == "" {
		t.Error("Title() should not be empty")
	}
	if len(title) < len(server.Name) {
		t.Error("Title() should contain the server name")
	}

	// Description should return formatted info
	desc := server.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}

	// FilterValue should return the name for filtering
	if server.FilterValue() != "test-server" {
		t.Errorf("FilterValue() = %q, want %q", server.FilterValue(), "test-server")
	}
}

func TestServerStatusBadge(t *testing.T) {
	tests := []struct {
		status ServerStatusType
		want   string
	}{
		{ServerStatusInstalled, StatusInstalled},
		{ServerStatusAvailable, StatusAvailable},
		{ServerStatusDisabled, StatusDisabled},
	}

	for _, tc := range tests {
		got := ServerStatusBadge(tc.status)
		if got != tc.want {
			t.Errorf("ServerStatusBadge(%d) = %q, want %q", tc.status, got, tc.want)
		}
	}
}

func TestHealthStatusBadge(t *testing.T) {
	tests := []struct {
		health HealthStatusType
		want   string
	}{
		{HealthStatusUnknown, HealthUnknown},
		{HealthStatusChecking, HealthChecking},
		{HealthStatusHealthy, HealthHealthy},
		{HealthStatusUnhealthy, HealthUnhealthy},
	}

	for _, tc := range tests {
		got := HealthStatusBadge(tc.health)
		if got != tc.want {
			t.Errorf("HealthStatusBadge(%d) = %q, want %q", tc.health, got, tc.want)
		}
	}
}

func TestFormatServerDescription(t *testing.T) {
	// With explicit description
	s1 := &Server{
		Name:      "test",
		Desc:      "Custom description",
		Transport: "stdio",
	}
	if desc := FormatServerDescription(s1); desc != "Custom description" {
		t.Errorf("Expected custom description, got %q", desc)
	}

	// Without description, uses transport
	s2 := &Server{
		Name:      "test",
		Transport: "http",
	}
	desc := FormatServerDescription(s2)
	if desc == "" {
		t.Error("Expected generated description, got empty")
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
	if len(km.Search.Keys()) == 0 {
		t.Error("Search key binding should have keys")
	}
	if len(km.Install.Keys()) == 0 {
		t.Error("Install key binding should have keys")
	}
	if len(km.Delete.Keys()) == 0 {
		t.Error("Delete key binding should have keys")
	}
}

func TestKeyMapShortHelp(t *testing.T) {
	km := newKeyMap()
	help := km.ShortHelp()

	if len(help) == 0 {
		t.Error("ShortHelp() should return keybindings")
	}
}

func TestKeyMapFullHelp(t *testing.T) {
	km := newKeyMap()
	help := km.FullHelp()

	if len(help) == 0 {
		t.Error("FullHelp() should return keybinding groups")
	}

	// Should have multiple columns
	if len(help) < 3 {
		t.Error("FullHelp() should have multiple columns")
	}
}
