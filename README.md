# agentctl

Universal agent configuration manager for MCP servers across multiple agentic tools.

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## Overview

`agentctl` manages MCP servers, commands, rules, prompts, and skills across 8 agentic frameworks:

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

### Homebrew (macOS/Linux)

```bash
brew install iheanyi/tap/agentctl
```

### Go Install

```bash
go install github.com/iheanyi/agentctl/cmd/agentctl@latest
```

### Script

```bash
curl -sSL https://raw.githubusercontent.com/iheanyi/agentctl/main/install.sh | bash
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

# Add from git URL
agentctl add github.com/org/mcp-server

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

# Note: `agentctl install` is an alias for `agentctl add`
```

### Syncing

```bash
agentctl sync                  # Sync to all detected tools
agentctl sync --tool claude    # Sync to specific tool
agentctl sync --dry-run        # Preview changes
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
# Profile management
agentctl profile list                    # List all profiles
agentctl profile create work -d "Work MCPs"  # Create with description
agentctl profile switch work             # Switch active profile
agentctl profile show [name]             # Show profile details
agentctl profile delete work             # Delete profile

# Add/remove servers from profiles
agentctl profile add-server work sentry
agentctl profile remove-server work sentry
```

### Project Configuration

Create `.agentctl.json` in your project root for project-specific servers:

```bash
# Initialize project config
agentctl init --local

# Commands automatically detect and merge project config
agentctl list    # Shows "Project config: /path/to/.agentctl.json"
agentctl sync    # Syncs merged global + project config
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
agentctl config set <key> <value>  # Set config value
```

### Secrets

```bash
agentctl secret set <name>     # Store secret in keychain
agentctl secret get <name>     # Retrieve secret
agentctl secret list           # List stored secrets
agentctl secret delete <name>  # Remove secret
```

### Health & Status

```bash
agentctl test [server]         # Health check servers
agentctl status                # Show resource status
agentctl doctor                # Diagnose common issues
```

### Interactive TUI

```bash
agentctl ui
```

The TUI provides two views:
- **Installed** - Manage installed MCP servers (delete, test, sync)
- **Browse** - Discover and install available MCPs

Keyboard shortcuts:
- `Tab` - Switch between Installed/Browse views
- `↑/↓` or `j/k` - Navigate
- `/` - Filter list
- `d` - Delete server (Installed view)
- `i` - Install server (Browse view)
- `s` - Sync all servers
- `t` - Test selected server
- `q` - Quit

### Background Daemon

```bash
agentctl daemon start          # Start auto-update daemon
agentctl daemon stop           # Stop daemon
agentctl daemon status         # Check daemon status
```

## Configuration

Configuration is stored in `~/.config/agentctl/agentctl.json`:

```json
{
  "version": "1",
  "servers": {
    "figma": {
      "source": { "type": "remote", "url": "https://mcp.figma.com/mcp" },
      "transport": "http",
      "url": "https://mcp.figma.com/mcp"
    },
    "filesystem": {
      "source": { "type": "alias", "alias": "filesystem" },
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem"]
    }
  },
  "settings": {
    "defaultProfile": "default",
    "autoUpdate": {
      "enabled": true,
      "interval": "24h"
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

### Project-Local Config

Create `.agentctl.json` in your project root for project-specific configuration:

```json
{
  "version": "1",
  "servers": {
    "project-db": {
      "source": { "type": "local", "url": "./tools/db-mcp" },
      "command": "./tools/db-mcp/server"
    }
  },
  "disabled": ["personal-mcp"]
}
```

Project config is automatically detected and merged with global config. Same-name servers override global, others are added.

## Bundled Aliases

agentctl ships with aliases for popular MCP servers:

### Remote MCPs (OAuth, HTTP transport)

| Alias | Description |
|-------|-------------|
| `sentry` | Sentry error tracking (remote + local variants) |
| `figma` | Figma design-to-code (remote + local variants) |
| `linear` | Linear issue tracking |
| `notion` | Notion workspace integration |

### Local MCPs (stdio transport)

| Alias | Description |
|-------|-------------|
| `filesystem` | File system operations |
| `github` | GitHub API integration |
| `postgres` | PostgreSQL database |
| `sqlite` | SQLite database |
| `memory` | Knowledge graph storage |
| `fetch` | HTTP fetch operations |
| `puppeteer` | Browser automation |
| `playwright` | Browser automation |
| `brave-search` | Brave search integration |
| `google-maps` | Google Maps API |
| `google-drive` | Google Drive integration |
| `slack` | Slack integration |
| `git` | Git repository operations |
| `sequential-thinking` | Structured reasoning |
| `context7` | Up-to-date library documentation |

## Transport Support

Different tools support different MCP transports:

| Tool | stdio | HTTP/SSE |
|------|-------|----------|
| Claude Code | ✓ | ✓ |
| Claude Desktop | ✓ | ✓ |
| Cursor | ✓ | ✗ |
| Codex | ✓ | ✗ |
| Cline | ✓ | ✗ |

When installing a remote MCP (like Figma), agentctl automatically:
- Uses HTTP transport for Claude (supports it natively)
- Suggests local npm variant for Cursor/others (no HTTP support)

## Sync Behavior

agentctl marks managed entries with `"_managedBy": "agentctl"` and preserves manually-added configurations:

```json
{
  "mcpServers": {
    "figma": {
      "_managedBy": "agentctl",
      "transport": "http",
      "url": "https://mcp.figma.com/mcp"
    },
    "my-manual-server": {
      "command": "/path/to/server"
    }
  }
}
```

## Environment Variables

- `AGENTCTL_HOME` - Override config directory
- `XDG_CONFIG_HOME` - XDG config (default: `~/.config`)
- `XDG_CACHE_HOME` - XDG cache (default: `~/.cache`)
- `EDITOR` - Editor for `agentctl config edit`

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions welcome! Please read the development guide in [CLAUDE.md](CLAUDE.md).
