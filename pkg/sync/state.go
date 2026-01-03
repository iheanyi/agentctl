package sync

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/iheanyi/agentctl/pkg/config"
)

// SyncState tracks which servers are managed by agentctl per adapter
// This is stored in ~/.config/agentctl/sync-state.json
type SyncState struct {
	Version int `json:"version"`
	// ManagedServers maps adapter name -> list of server names we manage
	ManagedServers map[string][]string `json:"managedServers"`
}

// stateFilePath returns the path to the sync state file
func stateFilePath() string {
	return filepath.Join(config.DefaultConfigDir(), "sync-state.json")
}

// LoadState loads the sync state from disk
func LoadState() (*SyncState, error) {
	path := stateFilePath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &SyncState{
				Version:        1,
				ManagedServers: make(map[string][]string),
			}, nil
		}
		return nil, err
	}

	var state SyncState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	if state.ManagedServers == nil {
		state.ManagedServers = make(map[string][]string)
	}

	return &state, nil
}

// Save saves the sync state to disk
func (s *SyncState) Save() error {
	path := stateFilePath()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// GetManagedServers returns the list of server names managed for an adapter
func (s *SyncState) GetManagedServers(adapterName string) []string {
	return s.ManagedServers[adapterName]
}

// SetManagedServers sets the list of server names managed for an adapter
func (s *SyncState) SetManagedServers(adapterName string, servers []string) {
	s.ManagedServers[adapterName] = servers
}

// ClearManagedServers removes all managed servers for an adapter
func (s *SyncState) ClearManagedServers(adapterName string) {
	delete(s.ManagedServers, adapterName)
}
