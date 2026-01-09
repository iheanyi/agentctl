# CLAUDE.md - agentctl Development Guide

## Project Overview

**agentctl** is a universal agent configuration manager for syncing MCP servers, commands, rules, prompts, and skills across 10 agentic tools: Claude Code, Claude Desktop, Cursor, Codex, OpenCode, Cline, Windsurf, Zed, Continue, and Gemini.

**Module:** `github.com/iheanyi/agentctl`

## Quick Commands

```bash
# Build
go build ./...

# Run all tests
go test ./...

# Run golden tests only
go test ./pkg/sync/... -run Golden -v

# Update golden files
UPDATE_GOLDENS=1 go test ./pkg/sync/... -run Golden

# Interactive golden updates
GOLDEN_INTERACTIVE=1 go test ./pkg/sync/... -run Golden

# Build binary
go build -o agentctl ./cmd/agentctl

# Run CLI
./agentctl --help
./agentctl add filesystem
./agentctl sync --dry-run
./agentctl validate
./agentctl doctor -v
```

## Architecture

### Package Structure

```
cmd/agentctl/       # Entry point
internal/
├── cli/            # Cobra commands (add, sync, validate, doctor, etc.)
├── tui/            # Bubble Tea TUI
pkg/
├── mcp/            # MCP server types (Server, Source, Transport)
├── config/         # Config loading/saving, profiles, project-local
├── sync/           # Tool adapters + golden test infrastructure
│   ├── adapter.go      # Adapter interface and registry
│   ├── claude.go       # Claude Code adapter
│   ├── cursor.go       # Cursor adapter (filters HTTP/SSE)
│   ├── opencode.go     # OpenCode adapter (uses state file)
│   ├── state.go        # External state file for tracking
│   ├── golden_test.go  # Comprehensive golden tests
│   └── testdata/       # Fixtures and golden files
├── aliases/        # Bundled + user aliases
├── builder/        # Git cloning, auto-build detection
├── profile/        # Profile CRUD
├── registry/       # mcp.so API client
├── output/         # Terminal output formatting
├── command/        # Slash command schema
├── rule/           # Rules with YAML frontmatter
├── prompt/         # Prompt templates
├── skill/          # Directory-based skills
├── secrets/        # Keychain integration
├── lockfile/       # Version locking
```

### Key Types

```go
// MCP Server (pkg/mcp/server.go)
type Server struct {
    Name      string
    Command   string
    Args      []string
    URL       string            // For HTTP/SSE transport
    Env       map[string]string
    Transport Transport         // "stdio", "http", or "sse"
    Disabled  bool
}

// Sync Adapter (pkg/sync/adapter.go)
type Adapter interface {
    Name() string
    Detect() (bool, error)
    ConfigPath() string
    SupportedResources() []ResourceType
    ReadServers() ([]*mcp.Server, error)
    WriteServers(servers []*mcp.Server) error
}
```

### Tool Config Paths

| Tool | Config Location | Server Key |
|------|-----------------|------------|
| Claude Code | `~/.claude/settings.json` | `mcpServers` |
| Claude Desktop | `~/Library/Application Support/Claude/claude_desktop_config.json` | `mcpServers` |
| Cursor | `~/.cursor/mcp.json` | `mcpServers` |
| Codex | `~/.codex/config.json` | `mcpServers` |
| OpenCode | `~/.config/opencode/opencode.json` | `mcp` |
| Cline | `~/.cline/cline_mcp_settings.json` | `mcpServers` |
| Windsurf | `~/.windsurf/mcp.json` | `mcpServers` |
| Zed | `~/.config/zed/settings.json` | `context_servers` |
| Continue | `~/.continue/config.json` | `mcpServers` |
| Gemini | `~/.gemini/config.json` | `mcpServers` |

## Sync Behavior

### Managed Server Tracking

Most adapters mark managed entries with `"_managedBy": "agentctl"`:

```json
{
  "mcpServers": {
    "filesystem": {
      "_managedBy": "agentctl",
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem"]
    }
  }
}
```

**Exception: OpenCode** uses an external state file (`~/.config/agentctl/sync-state.json`) because OpenCode's schema is strict and doesn't allow unknown fields.

### Config Preservation

Adapters preserve all unknown fields during sync:
- `$schema` references
- `hook` configurations
- `plugin` arrays
- Nested custom objects
- Numeric/boolean values

### Transport Filtering

Cursor only supports stdio transport. When syncing to Cursor, HTTP/SSE servers are automatically filtered out.

## Testing

### Golden File Tests

Golden tests verify adapter output against expected snapshots:

```bash
# Run golden tests
go test ./pkg/sync/... -run Golden -v

# Update all golden files
UPDATE_GOLDENS=1 go test ./pkg/sync/... -run Golden

# Interactive mode (prompts for each change)
GOLDEN_INTERACTIVE=1 go test ./pkg/sync/... -run Golden
```

### Test Structure

```
pkg/sync/testdata/
├── fixtures/                    # Input test data
│   ├── servers_minimal.json
│   ├── servers_realistic.json
│   └── servers_all_transports.json
├── golden/                      # Expected outputs per adapter
│   ├── claude/
│   │   ├── basic.input.json
│   │   ├── basic.golden.json
│   │   └── preserve_fields.*.json
│   ├── opencode/
│   │   ├── basic.*.json
│   │   └── strict_schema.*.json  # Tests no _managedBy
│   └── cursor/
│       └── http_filtered.*.json  # Tests transport filtering
├── golden.go                    # Comparison utilities
└── diff_viewer.go               # Colored diff output
```

### Test Categories

1. **Golden file tests** - Verify adapter output format
2. **Config preservation tests** - Verify unknown fields preserved
3. **Transport filtering tests** - Verify Cursor filters HTTP/SSE
4. **State file tests** - Verify OpenCode state tracking

## Adding a New Tool Adapter

1. Create `pkg/sync/<tool>.go`:

```go
package sync

type ToolAdapter struct{}

func init() {
    Register(&ToolAdapter{})
}

func (a *ToolAdapter) Name() string { return "tool" }
func (a *ToolAdapter) Detect() (bool, error) { /* check if installed */ }
func (a *ToolAdapter) ConfigPath() string { return "~/.tool/config.json" }
func (a *ToolAdapter) SupportedResources() []ResourceType { return []ResourceType{ResourceMCP} }
func (a *ToolAdapter) ReadServers() ([]*mcp.Server, error) { /* parse config */ }
func (a *ToolAdapter) WriteServers(servers []*mcp.Server) error { /* write config */ }
```

2. Add golden test cases in `pkg/sync/testdata/golden/<tool>/`

3. Update `golden_test.go` with adapter config

## CLI Commands

| Command | Description |
|---------|-------------|
| `add <target>` | Add server (alias, URL, or command) |
| `remove <name>` | Remove installed server |
| `list` | List all resources |
| `sync` | Sync config to all detected tools |
| `sync --dry-run` | Preview changes with diff output |
| `sync --verbose` | Show detailed sync info |
| `validate` | Validate tool config syntax/schema |
| `doctor` | Run comprehensive health checks |
| `search <query>` | Search bundled aliases |
| `profile` | Manage profiles |
| `new` | Scaffold new resources |
| `import <tool>` | Import from existing tools |
| `update` | Check/apply updates |
| `test [server]` | Health check MCP servers |
| `config` | View/edit configuration |
| `secret` | Manage secrets in keychain |
| `ui` | Launch interactive TUI |

## Code Style

- Use `fmt.Errorf("context: %w", err)` for error wrapping
- Keep functions focused and small
- Add golden tests for adapter changes
- Use the `output` package for user-facing messages
- Preserve unknown config fields during sync

## Environment Variables

- `AGENTCTL_HOME` - Override config directory
- `XDG_CONFIG_HOME` - XDG config (default: `~/.config`)
- `XDG_CACHE_HOME` - XDG cache (default: `~/.cache`)
- `UPDATE_GOLDENS=1` - Auto-update golden files in tests
- `GOLDEN_INTERACTIVE=1` - Interactive golden file updates
