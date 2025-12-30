# CLAUDE.md - agentctl Development Guide

## Project Overview

**agentctl** is a universal agent configuration manager for syncing MCP servers, commands, rules, prompts, and skills across 8 agentic tools: Claude Code, Cursor, Codex, OpenCode, Cline, Windsurf, Zed, and Continue.

**Module:** `github.com/iheanyi/agentctl`

## Quick Commands

```bash
# Build
go build ./...

# Run tests
go test ./...

# Build binary
go build -o agentctl ./cmd/agentctl

# Run CLI
./agentctl --help
./agentctl install filesystem
./agentctl sync
./agentctl list
```

## Architecture

### Package Structure

```
pkg/
├── mcp/        # MCP server types (Server, Source, Transport, BuildConfig)
├── config/     # Config loading/saving, profiles, project-local inheritance
├── sync/       # Tool adapters (claude, cursor, codex, opencode, cline, windsurf, zed, continue)
├── aliases/    # Bundled + user aliases with search
├── builder/    # Git cloning, auto-build detection (Node/Go/Rust/Python)
├── profile/    # Profile CRUD operations
├── registry/   # mcp.so API client
├── output/     # Terminal output formatting (gh CLI-style)
├── command/    # Slash command schema
├── rule/       # Rules with YAML frontmatter
├── prompt/     # Prompt templates
├── skill/      # Directory-based skills

internal/cli/   # Cobra command implementations
cmd/agentctl/   # Entry point
```

### Key Types

```go
// MCP Server (pkg/mcp/server.go)
type Server struct {
    Name      string
    Source    Source    // Type: "git", "alias", "local"
    Command   string
    Args      []string
    Env       map[string]string
    Transport Transport // "stdio" or "sse"
    Namespace string
    Build     *BuildConfig
    Disabled  bool
}

// Config (pkg/config/config.go)
type Config struct {
    Version   string
    Servers   map[string]*mcp.Server
    Commands  []string
    Rules     []string
    Settings  Settings
    // Loaded resources (not serialized)
    LoadedCommands []*command.Command
    LoadedRules    []*rule.Rule
}

// Sync Adapter (pkg/sync/adapter.go)
type Adapter interface {
    Name() string
    Detect() (bool, error)
    ConfigPath() string
    SupportedResources() []ResourceType
    ReadServers() ([]*mcp.Server, error)
    WriteServers(servers []*mcp.Server) error
    WriteCommands(commands []*command.Command) error
    WriteRules(rules []*rule.Rule) error
}
```

### Config Locations

- **agentctl config:** `~/.config/agentctl/agentctl.json`
- **Cache:** `~/.cache/agentctl/`
- **Project config:** `.agentctl.json` in project root

### Tool Config Paths

| Tool | Config Location |
|------|-----------------|
| Claude Code | `~/.config/claude/claude_desktop_config.json` |
| Cursor | `~/.cursor/mcp.json` |
| Codex | `~/.codex/config.json` |
| OpenCode | `~/.config/opencode/config.json` |
| Cline | `~/.cline/cline_mcp_settings.json` |
| Windsurf | `~/.windsurf/mcp.json` |
| Zed | `~/.config/zed/settings.json` |
| Continue | `~/.continue/config.json` |

## Sync Behavior

Managed entries are marked with `"_managedBy": "agentctl"`. Manual entries (without marker) are preserved during sync.

```json
{
  "mcpServers": {
    "filesystem": {
      "_managedBy": "agentctl",
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem"]
    },
    "manual-server": {
      "command": "/path/to/server"
    }
  }
}
```

## Adding a New Tool Adapter

1. Create `pkg/sync/<tool>.go`:

```go
type ToolAdapter struct{}

func (a *ToolAdapter) Name() string { return "tool" }
func (a *ToolAdapter) Detect() (bool, error) { /* check if installed */ }
func (a *ToolAdapter) ConfigPath() string { return "~/.tool/config.json" }
func (a *ToolAdapter) SupportedResources() []ResourceType { return []ResourceType{ResourceMCP} }
func (a *ToolAdapter) ReadServers() ([]*mcp.Server, error) { /* ... */ }
func (a *ToolAdapter) WriteServers(servers []*mcp.Server) error { /* ... */ }
```

2. Register in `pkg/sync/adapter.go`:

```go
func init() {
    Register(&ToolAdapter{})
}
```

## Testing Conventions

- Each package has `*_test.go` files
- Use `os.MkdirTemp` for temp directories in tests
- Clean up with `defer os.RemoveAll(tmpDir)`
- Mock HTTP servers for API tests (see `pkg/registry/mcpso_test.go`)

## CLI Commands

| Command | Description |
|---------|-------------|
| `install <target>` | Install server (alias, git URL, or local path) |
| `remove <name>` | Remove installed server |
| `list` | List all resources |
| `sync` | Sync config to all detected tools |
| `search <query>` | Search mcp.so and bundled aliases |
| `alias list/add/remove` | Manage aliases |
| `profile list/create/switch/delete` | Manage profiles |
| `doctor` | Diagnose common issues |
| `status` | Show resource status |

## Bundled Aliases

Located in `pkg/aliases/aliases.json`. Add popular MCP servers here:

```json
{
  "filesystem": {
    "url": "github.com/modelcontextprotocol/servers",
    "description": "File system operations",
    "runtime": "node"
  }
}
```

## Environment Variables

- `AGENTCTL_HOME` - Override config directory
- `XDG_CONFIG_HOME` - XDG config (default: `~/.config`)
- `XDG_CACHE_HOME` - XDG cache (default: `~/.cache`)

## Implementation Status

### Completed
- Core types and config
- All 8 tool adapters
- Bundled aliases (16 servers)
- Git cloning and auto-building
- Profile management
- mcp.so search integration
- Output formatting

### TODO
- [ ] Secrets management (keychain integration)
- [ ] Background daemon for auto-updates
- [ ] TUI mode (Bubble Tea)
- [ ] `agentctl new` scaffolding commands
- [ ] `agentctl import` from existing tools
- [ ] Lockfile support
- [ ] GoReleaser setup
- [ ] Homebrew formula

## Code Style

- Use `fmt.Errorf("context: %w", err)` for error wrapping
- Keep functions focused and small
- Add tests for new functionality
- Use the `output` package for user-facing messages
- Prefer composition over inheritance

## Spec Reference

Full specification in `.claude/plans/mcp-pkg-spec.md`
