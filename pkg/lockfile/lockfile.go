package lockfile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Lockfile represents the agentctl.lock file that tracks exact versions
type Lockfile struct {
	Version string                  `json:"version"`
	Locked  map[string]*LockedEntry `json:"locked"`
	path    string
}

// LockedEntry represents a locked server entry
type LockedEntry struct {
	Source      string    `json:"source"`              // Git URL or alias
	Version     string    `json:"version,omitempty"`   // Semver version if available
	Commit      string    `json:"commit,omitempty"`    // Git commit hash
	Integrity   string    `json:"integrity,omitempty"` // SHA256 hash of installed files
	InstalledAt time.Time `json:"installedAt"`
	UpdatedAt   time.Time `json:"updatedAt,omitempty"`
}

// New creates a new lockfile
func New() *Lockfile {
	return &Lockfile{
		Version: "1",
		Locked:  make(map[string]*LockedEntry),
	}
}

// Load loads a lockfile from a config directory
func Load(configDir string) (*Lockfile, error) {
	path := filepath.Join(configDir, "agentctl.lock")
	return LoadFrom(path)
}

// LoadFrom loads a lockfile from a specific path
func LoadFrom(path string) (*Lockfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty lockfile if file doesn't exist
			lf := New()
			lf.path = path
			return lf, nil
		}
		return nil, err
	}

	var lf Lockfile
	if err := json.Unmarshal(data, &lf); err != nil {
		return nil, err
	}

	if lf.Locked == nil {
		lf.Locked = make(map[string]*LockedEntry)
	}
	lf.path = path

	return &lf, nil
}

// Save saves the lockfile to disk
func (lf *Lockfile) Save() error {
	return lf.SaveTo(lf.path)
}

// SaveTo saves the lockfile to a specific path
func (lf *Lockfile) SaveTo(path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// Lock adds or updates a locked entry
func (lf *Lockfile) Lock(name string, entry *LockedEntry) {
	existing, exists := lf.Locked[name]
	if exists {
		entry.InstalledAt = existing.InstalledAt
		entry.UpdatedAt = time.Now()
	} else {
		entry.InstalledAt = time.Now()
	}
	lf.Locked[name] = entry
}

// Unlock removes a locked entry
func (lf *Lockfile) Unlock(name string) {
	delete(lf.Locked, name)
}

// Get returns a locked entry if it exists
func (lf *Lockfile) Get(name string) (*LockedEntry, bool) {
	entry, ok := lf.Locked[name]
	return entry, ok
}

// IsLocked checks if a server is locked
func (lf *Lockfile) IsLocked(name string) bool {
	_, ok := lf.Locked[name]
	return ok
}

// NeedsUpdate checks if a locked entry needs updating
// Returns true if the commit or version has changed
func (lf *Lockfile) NeedsUpdate(name string, newCommit, newVersion string) bool {
	entry, ok := lf.Locked[name]
	if !ok {
		return true // Not locked, needs initial install
	}

	if newCommit != "" && entry.Commit != newCommit {
		return true
	}

	if newVersion != "" && entry.Version != newVersion {
		return true
	}

	return false
}

// CalculateIntegrity calculates a SHA256 hash of a directory's contents
func CalculateIntegrity(dir string) (string, error) {
	hash := sha256.New()

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Add file path relative to dir
		relPath, _ := filepath.Rel(dir, path)
		hash.Write([]byte(relPath))

		// Add file contents
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		hash.Write(data)

		return nil
	})

	if err != nil {
		return "", err
	}

	return "sha256-" + hex.EncodeToString(hash.Sum(nil)), nil
}

// VerifyIntegrity checks if a directory matches the expected integrity hash
func VerifyIntegrity(dir string, expected string) (bool, error) {
	actual, err := CalculateIntegrity(dir)
	if err != nil {
		return false, err
	}
	return actual == expected, nil
}

// Path returns the lockfile path
func (lf *Lockfile) Path() string {
	return lf.path
}

// Entries returns all locked entries
func (lf *Lockfile) Entries() map[string]*LockedEntry {
	return lf.Locked
}

// Count returns the number of locked entries
func (lf *Lockfile) Count() int {
	return len(lf.Locked)
}
