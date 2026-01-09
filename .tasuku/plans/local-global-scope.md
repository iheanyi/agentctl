# Implementation Plan: Local vs Global Scope Support

## Overview

Add explicit scope control to agentctl so users can manage project-local vs global configurations for **all resource types** (MCP servers, commands, rules, prompts, skills) without accidentally polluting other projects.

## Resource Types Covered

| Resource | Global Location | Project Location |
|----------|----------------|------------------|
| MCP Servers | `~/.config/agentctl/agentctl.json` | `.agentctl.json` |
| Commands | `~/.config/agentctl/commands/` | `.agentctl/commands/` |
| Rules | `~/.config/agentctl/rules/` | `.agentctl/rules/` |
| Prompts | `~/.config/agentctl/prompts/` | `.agentctl/prompts/` |
| Skills | `~/.config/agentctl/skills/` | `.agentctl/skills/` |

## Research Summary

### Tools with Workspace Config Support

| Tool | Global Path | Project Path | Notes |
|------|-------------|--------------|-------|
| Claude Code | `~/.claude/settings.json` | `.mcp.json` | Also supports `.claude/settings.local.json` for local-only |
| Cursor | `~/.cursor/mcp.json` | `.cursor/mcp.json` | Appears as "Project Managed" in UI |
| Claude Desktop | `~/Library/Application Support/Claude/...` | None | Desktop app, no workspace concept |
| Codex | `~/.codex/config.json` | None | CLI tool |
| OpenCode | `~/.config/opencode/opencode.json` | None | Uses external state file |
| Cline | `~/.cline/cline_mcp_settings.json` | None | VS Code extension |
| Windsurf | `~/.windsurf/mcp.json` | None | |
| Zed | `~/.config/zed/settings.json` | None | Could support `.zed/settings.json` |
| Continue | `~/.continue/config.json` | None | Could support workspace |
| Gemini | `~/.gemini/config.json` | None | |

### Sources
- [Claude Code MCP Docs](https://code.claude.com/docs/en/mcp)
- [Cursor MCP Docs](https://cursor.com/docs/context/mcp)

## Design Decisions

Based on user feedback:
1. **`add` default**: Default to local when `.agentctl.json` exists
2. **Sync target**: Project servers → workspace configs, global servers → global configs
3. **Warnings**: Warn and continue when project servers go to global (for tools without workspace support)
4. **Tool priority**: Implement all tools with workspace support (Claude Code, Cursor)

## Project Directory Structure

When a project has local configuration, the structure looks like:

```
my-project/
├── .agentctl.json           # Project MCP servers, disabled list, profile
├── .agentctl/
│   ├── commands/            # Project-specific slash commands
│   │   └── deploy.yaml
│   ├── rules/               # Project-specific rules
│   │   └── coding-style.md
│   ├── prompts/             # Project-specific prompts
│   │   └── review.md
│   └── skills/              # Project-specific skills
│       └── ci-pipeline/
└── src/
```

**Note:** `.agentctl.json` is the config file (analogous to `agentctl.json`), while `.agentctl/` is the resource directory (analogous to `~/.config/agentctl/`).

## Implementation Phases

### Phase 1: Core Scope Infrastructure

#### 1.1 Add Scope Type and Constants

**File: `pkg/config/scope.go` (new)**

```go
package config

type Scope string

const (
    ScopeLocal  Scope = "local"   // Project-specific (.agentctl.json)
    ScopeGlobal Scope = "global"  // User-wide (~/.config/agentctl/agentctl.json)
    ScopeAll    Scope = "all"     // Both (for sync operations)
)

func (s Scope) String() string { return string(s) }

func ParseScope(s string) (Scope, error) {
    switch s {
    case "local", "project":
        return ScopeLocal, nil
    case "global", "user":
        return ScopeGlobal, nil
    case "all", "":
        return ScopeAll, nil
    default:
        return "", fmt.Errorf("invalid scope: %s (use local, global, or all)", s)
    }
}
```

#### 1.2 Extend Config Loading

**File: `pkg/config/config.go`**

Add methods:
- `LoadScoped(scope Scope) (*Config, error)` - Load only specified scope
- `SaveScoped(scope Scope) error` - Save to specified scope
- `GetServerScope(name string) Scope` - Determine which config a server came from
- `ServersForScope(scope Scope) []*mcp.Server` - Get servers from specific scope

Track server origin during merge:
```go
type Config struct {
    // ... existing fields
    serverOrigins map[string]Scope  // Track where each server came from
}
```

#### 1.3 Update Server Type

**File: `pkg/mcp/server.go`**

Add scope tracking:
```go
type Server struct {
    // ... existing fields
    Scope  string `json:"-"` // Runtime field, not serialized
}
```

### Phase 1.5: Project Resource Loading

#### 1.5.1 Update Config to Load Project Resources

**File: `pkg/config/config.go`**

```go
// loadResources loads all resources from config directories
func (c *Config) loadResources() error {
    var err error

    // Load global resources
    c.LoadedCommands, err = command.LoadAll(filepath.Join(c.ConfigDir, "commands"))
    if err != nil {
        return err
    }

    // ... similar for rules, prompts, skills

    return nil
}

// loadProjectResources loads resources from project directory and merges
func (c *Config) loadProjectResources(projectDir string) error {
    resourceDir := filepath.Join(projectDir, ".agentctl")

    // Load project commands
    projectCommands, err := command.LoadAll(filepath.Join(resourceDir, "commands"))
    if err != nil && !os.IsNotExist(err) {
        return err
    }

    // Mark as project-scoped
    for _, cmd := range projectCommands {
        cmd.Scope = "local"
    }

    // Merge: project commands override global by name
    c.LoadedCommands = mergeCommands(c.LoadedCommands, projectCommands)

    // ... similar for rules, prompts, skills

    return nil
}

// mergeCommands merges two command slices, with second taking precedence
func mergeCommands(global, project []*command.Command) []*command.Command {
    byName := make(map[string]*command.Command)
    for _, cmd := range global {
        cmd.Scope = "global"  // Mark global
        byName[cmd.Name] = cmd
    }
    for _, cmd := range project {
        cmd.Scope = "local"  // Mark local
        byName[cmd.Name] = cmd  // Override
    }

    result := make([]*command.Command, 0, len(byName))
    for _, cmd := range byName {
        result = append(result, cmd)
    }
    return result
}
```

#### 1.5.2 Add Scope Field to Resource Types

**File: `pkg/command/command.go`**

```go
type Command struct {
    Name        string   `yaml:"name"`
    Description string   `yaml:"description"`
    // ... existing fields

    Scope       string   `yaml:"-"`  // Runtime field: "local" or "global"
    SourcePath  string   `yaml:"-"`  // Path to source file
}
```

**Similar changes for:**
- `pkg/rule/rule.go`
- `pkg/prompt/prompt.go`
- `pkg/skill/skill.go`

#### 1.5.3 Update LoadProjectConfig

**File: `pkg/config/config.go`**

```go
func LoadProjectConfig(projectDir string) (*Config, error) {
    globalCfg, err := Load()
    if err != nil {
        return nil, err
    }

    projectConfigPath := filepath.Join(projectDir, ".agentctl.json")
    if _, err := os.Stat(projectConfigPath); os.IsNotExist(err) {
        return globalCfg, nil
    }

    projectCfg, err := LoadFrom(projectConfigPath)
    if err != nil {
        return nil, err
    }

    // Load project resources from .agentctl/ directory
    if err := globalCfg.loadProjectResources(projectDir); err != nil {
        return nil, err
    }

    globalCfg.ProjectPath = projectConfigPath
    merged := globalCfg.Merge(projectCfg)
    merged.ProjectPath = projectConfigPath

    return merged, nil
}
```

### Phase 2: Adapter Workspace Support

#### 2.1 Extend Adapter Interface

**File: `pkg/sync/adapter.go`**

```go
type Adapter interface {
    // ... existing methods

    // New methods for workspace support
    SupportsWorkspace() bool
    WorkspaceConfigPath(projectDir string) string
    ReadWorkspaceServers(projectDir string) ([]*mcp.Server, error)
    WriteWorkspaceServers(projectDir string, servers []*mcp.Server) error
}

// Default implementation for adapters without workspace support
type BaseAdapter struct{}

func (b *BaseAdapter) SupportsWorkspace() bool { return false }
func (b *BaseAdapter) WorkspaceConfigPath(string) string { return "" }
func (b *BaseAdapter) ReadWorkspaceServers(string) ([]*mcp.Server, error) { return nil, nil }
func (b *BaseAdapter) WriteWorkspaceServers(string, []*mcp.Server) error {
    return fmt.Errorf("workspace config not supported")
}
```

#### 2.2 Implement Claude Code Workspace Support

**File: `pkg/sync/claude.go`**

```go
func (a *ClaudeAdapter) SupportsWorkspace() bool { return true }

func (a *ClaudeAdapter) WorkspaceConfigPath(projectDir string) string {
    return filepath.Join(projectDir, ".mcp.json")
}

func (a *ClaudeAdapter) ReadWorkspaceServers(projectDir string) ([]*mcp.Server, error) {
    path := a.WorkspaceConfigPath(projectDir)
    // Read and parse .mcp.json
}

func (a *ClaudeAdapter) WriteWorkspaceServers(projectDir string, servers []*mcp.Server) error {
    path := a.WorkspaceConfigPath(projectDir)
    // Write .mcp.json with mcpServers key
}
```

#### 2.3 Implement Cursor Workspace Support

**File: `pkg/sync/cursor.go`**

```go
func (a *CursorAdapter) SupportsWorkspace() bool { return true }

func (a *CursorAdapter) WorkspaceConfigPath(projectDir string) string {
    return filepath.Join(projectDir, ".cursor", "mcp.json")
}

// Similar read/write methods
```

### Phase 3: CLI Command Updates

#### 3.1 Update `add` Command

**File: `internal/cli/add.go`**

```go
var addCmd = &cobra.Command{
    // ...
}

func init() {
    addCmd.Flags().StringP("scope", "s", "", "Config scope: local, global (default: local if .agentctl.json exists)")
}

func runAdd(cmd *cobra.Command, args []string) error {
    scopeStr, _ := cmd.Flags().GetString("scope")

    // Determine default scope
    scope := config.ScopeGlobal
    if scopeStr != "" {
        scope, err = config.ParseScope(scopeStr)
    } else if config.HasProjectConfig() {
        scope = config.ScopeLocal
        output.Info("Adding to project config (.agentctl.json). Use --scope=global for global config.")
    }

    // Load and save to appropriate config
    cfg, err := config.LoadScoped(scope)
    // ... add server
    return cfg.SaveScoped(scope)
}
```

#### 3.2 Update `remove` Command

**File: `internal/cli/remove.go`**

Add `--scope` flag with similar logic. When no scope specified:
1. Check if server exists in project config → remove from there
2. Otherwise check global config → remove from there
3. If in both, warn and ask (or use `--scope=all` to remove from both)

#### 3.3 Update `sync` Command

**File: `internal/cli/sync.go`**

```go
func init() {
    syncCmd.Flags().StringP("scope", "s", "all", "Which servers to sync: local, global, all")
}

func runSync(cmd *cobra.Command, args []string) error {
    scopeStr, _ := cmd.Flags().GetString("scope")
    scope, _ := config.ParseScope(scopeStr)

    cfg, err := config.LoadWithProject()

    // Get servers by scope
    globalServers := cfg.ServersForScope(config.ScopeGlobal)
    localServers := cfg.ServersForScope(config.ScopeLocal)

    cwd, _ := os.Getwd()

    for _, adapter := range sync.Adapters() {
        // Sync global servers to global config
        if scope == config.ScopeGlobal || scope == config.ScopeAll {
            if err := adapter.WriteServers(globalServers); err != nil {
                // handle error
            }
        }

        // Sync local servers to workspace config (if supported)
        if scope == config.ScopeLocal || scope == config.ScopeAll {
            if len(localServers) > 0 {
                if adapter.SupportsWorkspace() {
                    if err := adapter.WriteWorkspaceServers(cwd, localServers); err != nil {
                        // handle error
                    }
                } else {
                    // Warn: local servers going to global config
                    output.Warn("%s doesn't support workspace configs. Local servers will be synced globally.", adapter.Name())
                    // Continue syncing to global
                }
            }
        }
    }
}
```

#### 3.4 Update `list` Command

**File: `internal/cli/list.go`**

Add `--scope` flag and show scope indicator in output:

```
MCP Servers:
  filesystem     [global]  @modelcontextprotocol/server-filesystem
  sentry         [local]   @sentry/mcp-server
  postgres       [global]  @modelcontextprotocol/server-postgres
```

#### 3.5 Update `config` Command

Show which config is being viewed/edited:

```bash
$ agentctl config show
# Showing: global config (~/.config/agentctl/agentctl.json)
...

$ agentctl config show --scope=local
# Showing: project config (.agentctl.json)
...
```

### Phase 4: TUI Updates

#### 4.1 Add Scope Column/Indicator

**File: `internal/tui/server.go`**

Show scope in server list:
```
┌─ MCP Servers ──────────────────────────────────────┐
│ ● filesystem     [G]  Running                      │
│ ● sentry         [L]  Running                      │
│ ○ postgres       [G]  Disabled                     │
└────────────────────────────────────────────────────┘
[G] = Global  [L] = Local/Project
```

#### 4.2 Add Scope Selection in Add Flow

When adding a server in TUI, show scope selector if project config exists.

### Phase 5: Warnings and UX

#### 5.1 Sync Warnings

When syncing local servers to tools without workspace support:

```
⚠ Warning: The following tools don't support workspace configs:
  - Claude Desktop
  - Codex
  - Cline
  - Windsurf
  - Zed
  - Continue
  - Gemini

Local servers (sentry, custom-tool) will be synced to their global configs.
This means these servers will be available in ALL projects when using these tools.

Use --scope=global to sync only global servers, or press Enter to continue.
```

#### 5.2 Add Operation Info

```bash
$ agentctl add sentry
Adding 'sentry' to project config (.agentctl.json)
Use --scope=global to add to global config instead.

✓ Added sentry to project config
```

### Phase 6: Testing

#### 6.1 Unit Tests

- `pkg/config/scope_test.go` - Scope parsing, scoped loading/saving
- `pkg/sync/workspace_test.go` - Workspace config read/write for each adapter

#### 6.2 Golden Tests

Add golden test cases:
- `testdata/golden/claude/workspace_*.json` - Claude workspace config format
- `testdata/golden/cursor/workspace_*.json` - Cursor workspace config format

#### 6.3 Integration Tests

- Test `add --scope=local` creates/updates `.agentctl.json`
- Test `add --scope=global` updates global config
- Test `sync` properly routes servers to workspace vs global configs
- Test warnings appear for tools without workspace support

## Task Breakdown

### Phase 1: Core Scope Infrastructure
1. [ ] Add `Scope` type and parsing (`pkg/config/scope.go`)
2. [ ] Add scoped config loading/saving to `pkg/config/config.go`
3. [ ] Track resource origin (local/global) during config merge
4. [ ] Add project resource directory support (`.agentctl/commands/`, etc.)
5. [ ] Merge project resources with global resources in `loadResources()`

### Phase 2: CLI Command Updates
6. [ ] Add `--scope` flag to `add` command with smart defaults
7. [ ] Add `--scope` flag to `remove` command
8. [ ] Add `--scope` flag to `sync` command
9. [ ] Add `--scope` flag to `list` command with scope indicator
10. [ ] Update `init --local` to create `.agentctl/` directory structure
11. [ ] Add `config show --scope` support

### Phase 3: Adapter Workspace Support
12. [ ] Extend `Adapter` interface with workspace methods
13. [ ] Implement Claude Code workspace support (`.mcp.json` for servers)
14. [ ] Implement Cursor workspace support (`.cursor/mcp.json`)
15. [ ] Add sync warnings for tools without workspace support

### Phase 4: TUI Updates
16. [ ] Add scope column/indicator to server list
17. [ ] Add scope column/indicator to commands list
18. [ ] Add scope column/indicator to rules list
19. [ ] Add scope selection in add server flow
20. [ ] Add scope selection in add command flow
21. [ ] Add scope selection in add rule flow

### Phase 5: `new` Command Updates
22. [ ] Update `new command` to support `--scope` flag
23. [ ] Update `new rule` to support `--scope` flag
24. [ ] Update `new prompt` to support `--scope` flag
25. [ ] Update `new skill` to support `--scope` flag
26. [ ] Default to local scope when in project with `.agentctl.json`

### Phase 6: Testing
27. [ ] Add unit tests for scope parsing and config loading
28. [ ] Add unit tests for project resource loading
29. [ ] Add golden tests for workspace configs (Claude, Cursor)
30. [ ] Add integration tests for scoped add/remove/sync operations

## Migration Notes

- **Backwards compatible**: Default behavior unchanged when no `.agentctl.json` exists
- **Gradual adoption**: Users can opt-in to project configs by running `agentctl init --local`
- **No breaking changes**: Existing configs continue to work

## Open Questions

1. Should we auto-create `.mcp.json` / `.cursor/mcp.json` when syncing local servers, or require explicit opt-in?
   - **Recommendation**: Auto-create with a first-time notice

2. Should workspace configs be added to `.gitignore` by default?
   - **Recommendation**: No - these are meant to be shared. But mention in docs.

3. How to handle conflicts when same server exists in both global and local with different configs?
   - **Recommendation**: Local wins (current merge behavior). Show warning in `list`.
