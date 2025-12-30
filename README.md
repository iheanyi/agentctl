# agentctl

Universal agent configuration manager for MCP servers across multiple agentic tools.

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## Overview

`agentctl` manages MCP servers, commands, rules, prompts, and skills across 8 agentic frameworks:

- **Claude Code** - Anthropic's CLI assistant
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

# Install an MCP server
agentctl install filesystem

# Sync to all your tools
agentctl sync

# List installed servers
agentctl list
```

## Commands

### Installation & Management

```bash
agentctl install <server>      # Install from alias, git URL, or local path
agentctl install filesystem    # Install by alias
agentctl install github.com/org/mcp  # Install from git
agentctl remove <server>       # Remove installed server
agentctl update [server]       # Update servers
agentctl list                  # List all resources
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
agentctl search --community    # Also search mcp.so (third-party)
agentctl alias list            # List available aliases
agentctl alias add name url    # Add custom alias
```

### Profiles

```bash
agentctl profile list          # List profiles
agentctl profile create work   # Create new profile
agentctl profile switch work   # Switch active profile
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

### Interactive Mode

```bash
agentctl ui                    # Launch TUI
```

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
      "source": { "type": "local", "url": "./tools/db-mcp" }
    }
  },
  "disabled": ["personal-mcp"]
}
```

## Bundled Aliases

agentctl ships with aliases for official MCP servers:

| Alias | Description |
|-------|-------------|
| `filesystem` | File system operations |
| `github` | GitHub API integration |
| `postgres` | PostgreSQL database |
| `sqlite` | SQLite database |
| `memory` | In-memory key-value store |
| `fetch` | HTTP fetch operations |
| `puppeteer` | Browser automation |
| `brave-search` | Brave search integration |
| `google-maps` | Google Maps API |
| `slack` | Slack integration |
| `time` | Time/timezone utilities |
| `sequential-thinking` | Structured reasoning |
| `everything` | macOS file search |
| `aws-kb-retrieval` | AWS knowledge base |
| `google-drive` | Google Drive integration |
| `sentry` | Sentry error tracking |

## Sync Behavior

agentctl marks managed entries with `"_managedBy": "agentctl"` and preserves manually-added configurations:

```json
{
  "mcpServers": {
    "filesystem": {
      "_managedBy": "agentctl",
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem"]
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
