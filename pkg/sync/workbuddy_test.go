package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iheanyi/agentctl/pkg/mcp"
)

// Regression test: WriteServers-style JSON merge must preserve non-managed
// (manual) servers already present in the config file.
func TestWorkBuddyJSONMergePreservesManualServers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".mcp.json")

	existing := `{
  "mcpServers": {
    "connector-proxy": {
      "type": "http",
      "url": "http://127.0.0.1:7212/mcp",
      "description": "Aggregated proxy containing MCP servers: agent-mail"
    }
  }
}`
	if err := os.WriteFile(path, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	servers := []*mcp.Server{{Name: "graphify-mcp", Command: "graphify-mcp"}}

	helper := NewJSONConfigHelper(path)
	raw, err := helper.LoadRaw()
	if err != nil {
		t.Fatal(err)
	}
	mcpServers, _ := GetMCPServersSection(raw, "mcpServers")
	RemoveManagedServers(mcpServers)
	for _, server := range servers {
		name := GetServerName(server)
		if name == "" {
			continue
		}
		mcpServers[name] = ServerToRawMap(server)
	}
	raw["mcpServers"] = mcpServers
	if err := helper.SaveRaw(raw); err != nil {
		t.Fatal(err)
	}

	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "connector-proxy") {
		t.Fatalf("manual server connector-proxy was dropped; file:\n%s", out)
	}
	if !strings.Contains(string(out), "graphify-mcp") {
		t.Fatalf("managed server graphify-mcp missing; file:\n%s", out)
	}
	if !strings.Contains(string(out), "7212") {
		t.Fatalf("connector-proxy url lost; file:\n%s", out)
	}
}
