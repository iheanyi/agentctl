# agentctl

Universal agent configuration manager for MCP servers across multiple agentic tools.

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## Overview

`agentctl` manages MCP servers, commands, rules, prompts, and skills across 9 agentic frameworks:

- **Claude Code** - Anthropic's CLI assistant
- **Claude Desktop** - Anthropic's desktop app
- **Cursor** - AI-powered code editor
- **Codex** - OpenAI's CLI tool
- **OpenCode** - Open-source coding assistant
- **Cline** - VS Code AI extension
- **Windsurf** - AI IDE
- **Zed** - High-performance editor
- **Continue** - Open-source AI assistant

One config, all tools. Install once, sync everywhere.

## Installation

### Go Install

```bash
go install github.com/iheanyi/agentctl/cmd/agentctl@latest
```

### From Source

```bash
git clone https://github.com/iheanyi/agentctl.git
cd agentctl
go build -o agentctl ./cmd/agentctl
```

## Quick Start

```bash
# Initialize configuration
agentctl init

# Add an MCP server (auto-syncs to all detected tools)
agentctl add figma

# Add with explicit config
agentctl add playwright --command npx --args "playwriter@latest"
agentctl add my-api --url https://api.example.com/mcp

# List installed servers
agentctl list

# Launch interactive TUI
agentctl ui
```

## Commands

### Add & Manage MCP Servers

```bash
# Add from registry (auto-syncs to all tools)
agentctl add figma
agentctl add sentry
agentctl add filesystem

# Add with explicit URL (http/sse transport)
agentctl add figma --url https://mcp.figma.com/mcp
agentctl add my-api --url https://api.example.com/mcp/sse --type sse

# Add with explicit command (stdio transport)
agentctl add playwright --command npx --args "playwriter@latest"
agentctl add fs --command npx --args "-y,@modelcontextprotocol/server-filesystem"

# Options
agentctl add sentry --local         # Force local npx variant
agentctl add sentry --remote        # Force remote HTTP variant
agentctl add sentry --no-sync       # Don't sync after adding
agentctl add sentry --target cursor # Sync to specific tool only
agentctl add figma --dry-run        # Preview config without adding

# Remove and update
agentctl remove <server>
agentctl update [server]
agentctl list
```

### Syncing

```bash
agentctl sync                  # Sync to all detected tools
agentctl sync --tool claude    # Sync to specific tool
agentctl sync --dry-run        # Preview changes with diff output
agentctl sync --verbose        # Show detailed sync output
```

### Diagnostics

```bash
agentctl validate              # Validate all tool config syntax
agentctl validate --tool claude  # Validate specific tool
agentctl doctor                # Run comprehensive health checks
agentctl doctor -v             # Verbose health check output
agentctl test [server]         # Health check MCP servers
agentctl status                # Show resource status
```

### Search & Discovery

```bash
agentctl search <query>        # Search bundled aliases
agentctl alias list            # List available aliases
agentctl alias add name url    # Add custom alias
```

### Profiles

Profiles use an **additive model** - they add servers on top of your base config.

```bash
agentctl profile list                    # List all profiles
agentctl profile create work             # Create profile
agentctl profile switch work             # Switch active profile
agentctl profile delete work             # Delete profile

# Add/remove servers from profiles
agentctl profile add-server work sentry
agentctl profile remove-server work sentry
```

### Project Configuration

Create `.agentctl.json` in your project root for project-specific servers:

```bash
agentctl init --local          # Initialize project config
agentctl list                  # Shows merged global + project config
agentctl sync                  # Syncs merged configuration
```

### Resource Creation

```bash
agentctl new command <name>    # Create new slash command
agentctl new rule <name>       # Create new rule
agentctl new prompt <name>     # Create new prompt
agentctl new skill <name>      # Create new skill
```

### Configuration

```bash
agentctl config                # Show config summary
agentctl config show           # Show full config as JSON
agentctl config edit           # Open in $EDITOR
agentctl config get <key>      # Get config value
agentctl config set <key> <val>  # Set config value
```

### Secrets

```bash
agentctl secret set <name>     # Store secret in keychain
agentctl secret get <name>     # Retrieve secret
agentctl secret list           # List stored secrets
agentctl secret delete <name>  # Remove secret
```

### Interactive TUI

```bash
agentctl ui
```

Keyboard shortcuts:
- `Tab` - Switch between Installed/Browse views
- `j/k` or `Up/Down` - Navigate
- `/` - Filter list
- `Enter` - Install/view details
- `d` - Delete server
- `s` - Sync all servers
- `t` - Test selected server
- `?` - Show help
- `q` - Quit

## Configuration

Configuration is stored in `~/.config/agentctl/agentctl.json`:

```json
{
  "version": "1",
  "servers": {
    "figma": {
      "url": "https://mcp.figma.com/mcp",
      "transport": "http"
    },
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem"]
    }
  }
}
```

### Secrets in Config

Reference secrets using environment variables or keychain:

```json
{
  "servers": {
    "github": {
      "env": {
        "GITHUB_TOKEN": "$GITHUB_TOKEN",
        "API_KEY": "keychain:my-api-key"
      }
    }
  }
}
```

## Transport Support

Different tools support different MCP transports:

| Tool | stdio | HTTP/SSE |
|------|-------|----------|
| Claude Code | Yes | Yes |
| Claude Desktop | Yes | Yes |
| Cursor | Yes | No |
| Codex | Yes | Yes |
| OpenCode | Yes | Yes |
| Cline | Yes | Yes |
| Windsurf | Yes | Yes |
| Zed | Yes | Yes |
| Continue | Yes | Yes |

When syncing, agentctl automatically filters servers based on transport support.

## Sync Behavior

agentctl tracks managed servers and preserves manually-added configurations:

- **Managed servers**: Tracked via `_managedBy: "agentctl"` marker (or external state file for OpenCode)
- **Manual servers**: Preserved during sync - agentctl never touches them
- **Unknown config fields**: Preserved (`$schema`, hooks, plugins, etc.)

## Environment Variables

- `AGENTCTL_HOME` - Override config directory
- `XDG_CONFIG_HOME` - XDG config (default: `~/.config`)
- `XDG_CACHE_HOME` - XDG cache (default: `~/.cache`)
- `EDITOR` - Editor for `agentctl config edit`

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions welcome! See [CLAUDE.md](CLAUDE.md) for development guide.
