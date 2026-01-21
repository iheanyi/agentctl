package discovery

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InspectTitle returns the display name for the inspector modal header
func (p *Plugin) InspectTitle() string {
	return fmt.Sprintf("Plugin: %s", p.Name)
}

// InspectContent returns the formatted content for the inspector viewport
func (p *Plugin) InspectContent() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Name:    %s\n", p.Name))
	b.WriteString(fmt.Sprintf("Tool:    %s\n", p.Tool))
	b.WriteString(fmt.Sprintf("Scope:   %s\n", p.Scope))
	b.WriteString(fmt.Sprintf("Path:    %s\n", p.Path))

	if p.Version != "" {
		b.WriteString(fmt.Sprintf("Version: %s\n", p.Version))
	}

	b.WriteString("\n")

	if p.Enabled {
		b.WriteString("Status:  Enabled\n")
	} else {
		b.WriteString("Status:  Disabled\n")
	}

	if p.InstalledAt != "" {
		b.WriteString(fmt.Sprintf("Installed:    %s\n", p.InstalledAt))
	}
	if p.LastUpdated != "" {
		b.WriteString(fmt.Sprintf("Last Updated: %s\n", p.LastUpdated))
	}
	if p.GitCommitSha != "" {
		b.WriteString(fmt.Sprintf("Git Commit:   %s\n", p.GitCommitSha))
	}

	return b.String()
}

// Plugin represents a Claude Code plugin
type Plugin struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	Version      string `json:"version"`
	Enabled      bool   `json:"enabled"`
	Scope        string `json:"-"` // "local" or "global"
	Tool         string `json:"-"` // Always "claude" for now
	InstalledAt  string `json:"installed_at,omitempty"`
	LastUpdated  string `json:"last_updated,omitempty"`
	GitCommitSha string `json:"git_commit_sha,omitempty"`
}

// installedPluginsFile is the Claude plugins manifest
type installedPluginsFile struct {
	Version int                              `json:"version"`
	Plugins map[string][]installedPluginInfo `json:"plugins"`
}

type installedPluginInfo struct {
	Scope        string `json:"scope"`
	InstallPath  string `json:"installPath"`
	Version      string `json:"version"`
	InstalledAt  string `json:"installedAt"`
	LastUpdated  string `json:"lastUpdated"`
	GitCommitSha string `json:"gitCommitSha"`
}

// LoadClaudePlugins loads plugins from Claude's installed_plugins.json
func LoadClaudePlugins() ([]*Plugin, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	pluginsPath := filepath.Join(homeDir, ".claude", "plugins", "installed_plugins.json")
	return loadPluginsFromFile(pluginsPath, "global")
}

// LoadClaudeProjectPlugins loads plugins from a project's local plugin config
func LoadClaudeProjectPlugins(projectDir string) ([]*Plugin, error) {
	// Check for local installed_plugins.json
	pluginsPath := filepath.Join(projectDir, ".claude", "plugins", "installed_plugins.json")
	return loadPluginsFromFile(pluginsPath, "local")
}

func loadPluginsFromFile(path string, scope string) ([]*Plugin, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var manifest installedPluginsFile
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing plugins file: %w", err)
	}

	var plugins []*Plugin
	for name, infos := range manifest.Plugins {
		// Extract clean name from "plugin-name@repo-name" format
		cleanName := name
		if idx := strings.Index(name, "@"); idx > 0 {
			cleanName = name[:idx]
		}

		for _, info := range infos {
			plugin := &Plugin{
				Name:         cleanName,
				Path:         info.InstallPath,
				Version:      info.Version,
				Enabled:      true, // Plugins in manifest are enabled
				Scope:        scope,
				Tool:         "claude",
				InstalledAt:  info.InstalledAt,
				LastUpdated:  info.LastUpdated,
				GitCommitSha: info.GitCommitSha,
			}
			plugins = append(plugins, plugin)
		}
	}

	return plugins, nil
}
