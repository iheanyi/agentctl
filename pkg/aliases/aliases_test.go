package aliases

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewStore(t *testing.T) {
	store := NewStore("")
	if store == nil {
		t.Fatal("NewStore should not return nil")
	}
}

func TestBundledAliases(t *testing.T) {
	store := NewStore("")

	// Test that bundled aliases are loaded
	bundled := store.ListBundled()
	if len(bundled) == 0 {
		t.Error("Bundled aliases should not be empty")
	}

	// Test specific bundled aliases
	expectedAliases := []string{"filesystem", "github", "postgres", "sqlite", "memory"}
	for _, name := range expectedAliases {
		if _, ok := bundled[name]; !ok {
			t.Errorf("Expected bundled alias %q to exist", name)
		}
	}
}

func TestResolve(t *testing.T) {
	store := NewStore("")

	// Test resolving a bundled alias
	alias, ok := store.Resolve("filesystem")
	if !ok {
		t.Error("Should resolve 'filesystem' alias")
	}
	if alias.URL == "" {
		t.Error("Resolved alias should have a URL")
	}
	if alias.Runtime != "node" {
		t.Errorf("filesystem runtime should be 'node', got %q", alias.Runtime)
	}

	// Test resolving non-existent alias
	_, ok = store.Resolve("nonexistent")
	if ok {
		t.Error("Should not resolve non-existent alias")
	}
}

func TestUserAliases(t *testing.T) {
	// Create temp directory for user aliases
	tmpDir, err := os.MkdirTemp("", "aliases-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewStore(tmpDir)

	// Add a user alias
	err = store.Add("my-mcp", Alias{
		URL:         "github.com/myorg/my-mcp",
		Description: "My custom MCP server",
		Runtime:     "go",
	})
	if err != nil {
		t.Fatalf("Failed to add user alias: %v", err)
	}

	// Verify it was added
	alias, ok := store.Resolve("my-mcp")
	if !ok {
		t.Error("Should resolve user alias")
	}
	if alias.URL != "github.com/myorg/my-mcp" {
		t.Errorf("URL mismatch: got %q", alias.URL)
	}

	// Verify it's in user list
	userAliases := store.ListUser()
	if _, ok := userAliases["my-mcp"]; !ok {
		t.Error("User alias should be in user list")
	}

	// Verify it's not in bundled list
	if store.IsBundled("my-mcp") {
		t.Error("User alias should not be in bundled list")
	}

	// Verify IsUser returns true
	if !store.IsUser("my-mcp") {
		t.Error("IsUser should return true for user alias")
	}

	// Verify file was created
	aliasPath := filepath.Join(tmpDir, "aliases.json")
	if _, err := os.Stat(aliasPath); os.IsNotExist(err) {
		t.Error("User aliases file should be created")
	}

	// Remove the alias
	err = store.Remove("my-mcp")
	if err != nil {
		t.Fatalf("Failed to remove alias: %v", err)
	}

	// Verify it was removed
	_, ok = store.Resolve("my-mcp")
	if ok {
		t.Error("Removed alias should not resolve")
	}
}

func TestUserAliasOverridesBundled(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "aliases-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewStore(tmpDir)

	// Override a bundled alias
	customURL := "github.com/myorg/custom-filesystem"
	err = store.Add("filesystem", Alias{
		URL:         customURL,
		Description: "My custom filesystem server",
	})
	if err != nil {
		t.Fatalf("Failed to add override: %v", err)
	}

	// Resolve should return user version
	alias, ok := store.Resolve("filesystem")
	if !ok {
		t.Error("Should resolve overridden alias")
	}
	if alias.URL != customURL {
		t.Errorf("Should return user alias URL, got %q", alias.URL)
	}

	// IsBundled should still return true
	if !store.IsBundled("filesystem") {
		t.Error("filesystem should still be in bundled list")
	}

	// IsUser should also return true
	if !store.IsUser("filesystem") {
		t.Error("filesystem should also be in user list")
	}
}

func TestList(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "aliases-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewStore(tmpDir)

	// Add a user alias
	store.Add("custom", Alias{URL: "github.com/custom/mcp"})

	// List should include both bundled and user
	all := store.List()

	// Should have bundled aliases
	if _, ok := all["filesystem"]; !ok {
		t.Error("List should include bundled aliases")
	}

	// Should have user aliases
	if _, ok := all["custom"]; !ok {
		t.Error("List should include user aliases")
	}
}

func TestLoadPersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "aliases-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create first store and add alias
	store1 := NewStore(tmpDir)
	err = store1.Add("persistent", Alias{URL: "github.com/persistent/mcp"})
	if err != nil {
		t.Fatalf("Failed to add alias: %v", err)
	}

	// Create new store pointing to same directory
	store2 := NewStore(tmpDir)

	// Should load the persisted alias
	alias, ok := store2.Resolve("persistent")
	if !ok {
		t.Error("Persisted alias should be loaded")
	}
	if alias.URL != "github.com/persistent/mcp" {
		t.Errorf("URL mismatch: got %q", alias.URL)
	}
}

func TestAliasDescription(t *testing.T) {
	store := NewStore("")

	alias, ok := store.Resolve("filesystem")
	if !ok {
		t.Fatal("Should resolve filesystem")
	}

	if alias.Description == "" {
		t.Error("Bundled aliases should have descriptions")
	}
}
