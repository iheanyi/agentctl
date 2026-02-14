package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/iheanyi/agentctl/pkg/sync"
)

type doctorTestAdapter struct {
	name       string
	configPath string
}

func (a *doctorTestAdapter) Name() string          { return a.name }
func (a *doctorTestAdapter) Detect() (bool, error) { return true, nil }
func (a *doctorTestAdapter) ConfigPath() string    { return a.configPath }
func (a *doctorTestAdapter) SupportedResources() []sync.ResourceType {
	return []sync.ResourceType{sync.ResourceMCP}
}

func TestValidateToolConfigCodexTOML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")
	content := []byte(`
[mcp_servers.test]
command = "echo"
args = ["hello"]
`)
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	adapter := &doctorTestAdapter{name: "codex", configPath: configPath}
	valid, count, err := validateToolConfig(adapter)
	if err != nil {
		t.Fatalf("validateToolConfig() error = %v", err)
	}
	if !valid {
		t.Fatal("validateToolConfig() expected valid=true")
	}
	if count != 1 {
		t.Fatalf("validateToolConfig() serverCount = %d, want 1", count)
	}
}

func TestValidateToolConfigCodexInvalidTOML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(configPath, []byte("[mcp_servers\nbad"), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	adapter := &doctorTestAdapter{name: "codex", configPath: configPath}
	valid, _, err := validateToolConfig(adapter)
	if err == nil {
		t.Fatal("validateToolConfig() expected TOML error")
	}
	if valid {
		t.Fatal("validateToolConfig() expected valid=false")
	}
}

func TestCheckSyncDriftUnknownAdapter(t *testing.T) {
	state := &sync.SyncState{
		Version: 1,
		ManagedServers: map[string][]string{
			"unknown-adapter": []string{"server-a"},
		},
	}

	issues := checkSyncDrift(state)
	if len(issues) == 0 {
		t.Fatal("checkSyncDrift() expected at least one issue")
	}
}
