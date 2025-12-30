package lockfile

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	lf := New()
	if lf.Version != "1" {
		t.Errorf("Version = %q, want %q", lf.Version, "1")
	}
	if lf.Locked == nil {
		t.Error("Locked should not be nil")
	}
}

func TestLockUnlock(t *testing.T) {
	lf := New()

	entry := &LockedEntry{
		Source:  "github.com/example/mcp",
		Version: "1.0.0",
		Commit:  "abc123",
	}

	lf.Lock("test-server", entry)

	if !lf.IsLocked("test-server") {
		t.Error("Expected test-server to be locked")
	}

	got, ok := lf.Get("test-server")
	if !ok {
		t.Fatal("Expected to get test-server entry")
	}
	if got.Source != "github.com/example/mcp" {
		t.Errorf("Source = %q, want %q", got.Source, "github.com/example/mcp")
	}
	if got.InstalledAt.IsZero() {
		t.Error("InstalledAt should be set")
	}

	lf.Unlock("test-server")
	if lf.IsLocked("test-server") {
		t.Error("Expected test-server to be unlocked")
	}
}

func TestLockUpdate(t *testing.T) {
	lf := New()

	// Initial lock
	entry1 := &LockedEntry{
		Source:  "github.com/example/mcp",
		Version: "1.0.0",
		Commit:  "abc123",
	}
	lf.Lock("test-server", entry1)

	installedAt := lf.Locked["test-server"].InstalledAt

	// Wait a tiny bit and update
	time.Sleep(time.Millisecond)

	entry2 := &LockedEntry{
		Source:  "github.com/example/mcp",
		Version: "1.1.0",
		Commit:  "def456",
	}
	lf.Lock("test-server", entry2)

	got := lf.Locked["test-server"]
	if got.Version != "1.1.0" {
		t.Errorf("Version = %q, want %q", got.Version, "1.1.0")
	}
	if got.InstalledAt != installedAt {
		t.Error("InstalledAt should be preserved on update")
	}
	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set on update")
	}
}

func TestNeedsUpdate(t *testing.T) {
	lf := New()

	// Not locked - needs update
	if !lf.NeedsUpdate("test-server", "abc123", "1.0.0") {
		t.Error("Unlocked server should need update")
	}

	// Lock it
	entry := &LockedEntry{
		Source:  "github.com/example/mcp",
		Version: "1.0.0",
		Commit:  "abc123",
	}
	lf.Lock("test-server", entry)

	// Same version - no update needed
	if lf.NeedsUpdate("test-server", "abc123", "1.0.0") {
		t.Error("Same version should not need update")
	}

	// Different commit - needs update
	if !lf.NeedsUpdate("test-server", "def456", "1.0.0") {
		t.Error("Different commit should need update")
	}

	// Different version - needs update
	if !lf.NeedsUpdate("test-server", "abc123", "1.1.0") {
		t.Error("Different version should need update")
	}
}

func TestSaveLoad(t *testing.T) {
	tmpDir := t.TempDir()

	lf := New()
	lf.Lock("filesystem", &LockedEntry{
		Source:  "github.com/modelcontextprotocol/servers",
		Version: "1.2.3",
		Commit:  "abc1234def5678",
	})

	// Save
	path := filepath.Join(tmpDir, "agentctl.lock")
	if err := lf.SaveTo(path); err != nil {
		t.Fatalf("SaveTo failed: %v", err)
	}

	// Load
	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom failed: %v", err)
	}

	if loaded.Version != "1" {
		t.Errorf("Version = %q, want %q", loaded.Version, "1")
	}

	entry, ok := loaded.Get("filesystem")
	if !ok {
		t.Fatal("Expected filesystem entry")
	}
	if entry.Source != "github.com/modelcontextprotocol/servers" {
		t.Errorf("Source = %q, want %q", entry.Source, "github.com/modelcontextprotocol/servers")
	}
	if entry.Version != "1.2.3" {
		t.Errorf("Version = %q, want %q", entry.Version, "1.2.3")
	}
}

func TestLoadNonexistent(t *testing.T) {
	tmpDir := t.TempDir()

	lf, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if lf.Version != "1" {
		t.Errorf("Version = %q, want %q", lf.Version, "1")
	}
	if lf.Count() != 0 {
		t.Errorf("Count = %d, want 0", lf.Count())
	}
}

func TestCalculateIntegrity(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some test files
	if err := os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("world"), 0644); err != nil {
		t.Fatal(err)
	}

	hash1, err := CalculateIntegrity(tmpDir)
	if err != nil {
		t.Fatalf("CalculateIntegrity failed: %v", err)
	}

	if hash1 == "" {
		t.Error("Hash should not be empty")
	}
	if hash1[:7] != "sha256-" {
		t.Errorf("Hash should start with sha256-, got %s", hash1[:7])
	}

	// Calculate again - should be the same
	hash2, err := CalculateIntegrity(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if hash1 != hash2 {
		t.Error("Same directory should produce same hash")
	}

	// Modify a file - hash should change
	if err := os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}

	hash3, err := CalculateIntegrity(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if hash1 == hash3 {
		t.Error("Modified directory should produce different hash")
	}
}

func TestVerifyIntegrity(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	hash, err := CalculateIntegrity(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Verify with correct hash
	valid, err := VerifyIntegrity(tmpDir, hash)
	if err != nil {
		t.Fatal(err)
	}
	if !valid {
		t.Error("Should be valid with correct hash")
	}

	// Verify with wrong hash
	valid, err = VerifyIntegrity(tmpDir, "sha256-wronghash")
	if err != nil {
		t.Fatal(err)
	}
	if valid {
		t.Error("Should be invalid with wrong hash")
	}
}

func TestCount(t *testing.T) {
	lf := New()

	if lf.Count() != 0 {
		t.Errorf("Count = %d, want 0", lf.Count())
	}

	lf.Lock("server1", &LockedEntry{Source: "test1"})
	if lf.Count() != 1 {
		t.Errorf("Count = %d, want 1", lf.Count())
	}

	lf.Lock("server2", &LockedEntry{Source: "test2"})
	if lf.Count() != 2 {
		t.Errorf("Count = %d, want 2", lf.Count())
	}
}

func TestEntries(t *testing.T) {
	lf := New()

	lf.Lock("server1", &LockedEntry{Source: "test1"})
	lf.Lock("server2", &LockedEntry{Source: "test2"})

	entries := lf.Entries()
	if len(entries) != 2 {
		t.Errorf("Entries count = %d, want 2", len(entries))
	}
}
