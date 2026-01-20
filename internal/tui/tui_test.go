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

func TestResourceTabConstants(t *testing.T) {
	// Verify tab constants are sequential
	if TabServers != 0 {
		t.Error("TabServers should be 0")
	}
	if TabCommands != 1 {
		t.Error("TabCommands should be 1")
	}
	if TabRules != 2 {
		t.Error("TabRules should be 2")
	}
	if TabSkills != 3 {
		t.Error("TabSkills should be 3")
	}
	if TabPrompts != 4 {
		t.Error("TabPrompts should be 4")
	}
}

func TestTabNames(t *testing.T) {
	// Verify we have names for all tabs
	if len(TabNames) != 7 {
		t.Errorf("TabNames should have 7 entries, got %d", len(TabNames))
	}

	expectedNames := []string{"Servers", "Commands", "Rules", "Skills", "Prompts", "Hooks", "Tools"}
	for i, name := range expectedNames {
		if TabNames[i] != name {
			t.Errorf("TabNames[%d] = %q, want %q", i, TabNames[i], name)
		}
	}
}

func TestKeyMapTabBindings(t *testing.T) {
	km := newKeyMap()

	// Verify tab key bindings exist
	if len(km.NextTab.Keys()) == 0 {
		t.Error("NextTab key binding should have keys")
	}
	if len(km.PrevTab.Keys()) == 0 {
		t.Error("PrevTab key binding should have keys")
	}
	if len(km.Tab1.Keys()) == 0 {
		t.Error("Tab1 key binding should have keys")
	}
	if len(km.Tab2.Keys()) == 0 {
		t.Error("Tab2 key binding should have keys")
	}
	if len(km.Tab3.Keys()) == 0 {
		t.Error("Tab3 key binding should have keys")
	}
	if len(km.Tab4.Keys()) == 0 {
		t.Error("Tab4 key binding should have keys")
	}
	if len(km.Tab5.Keys()) == 0 {
		t.Error("Tab5 key binding should have keys")
	}
	if len(km.Tab6.Keys()) == 0 {
		t.Error("Tab6 key binding should have keys")
	}
}
