# agentctl

Universal agent configuration manager for MCP servers, commands, rules, skills, agents, and hooks across multiple agentic tools.

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## Overview

`agentctl` manages configuration across 10 agentic frameworks:

- **Claude Code** - Anthropic's CLI assistant
- **Claude Desktop** - Anthropic's desktop app
- **Cursor** - AI-powered code editor
- **Codex** - OpenAI's CLI tool
- **OpenCode** - Open-source coding assistant
- **Cline** - VS Code AI extension
- **Windsurf** - AI IDE
- **Zed** - High-performance editor
- **Continue** - Open-source AI assistant
- **Gemini** - Google's Gemini CLI

One config, all tools. Install once, sync everywhere.

## Features

- **MCP Server Management** - Add, remove, and sync MCP servers across all tools
- **Native Discovery** - Discover and import resources from existing tool directories (`.claude/`, `.cursor/`, etc.)
- **Commands, Rules & Skills** - Manage slash commands, coding rules, and skills
- **Agents** - Create and manage custom agents/subagents
- **Hooks** - Configure lifecycle hooks for automation
- **Profiles** - Switch between different server configurations
- **Project-Local Config** - Override global settings per-project
- **Interactive TUI** - Visual interface for managing everything

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

# Import existing config from another tool
agentctl import claude

# List all resources (including native discoveries)
agentctl list --native

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

### Import from Existing Tools

Import MCP servers, commands, rules, and skills from existing tool configurations:

```bash
# Import all from a specific tool
agentctl import claude              # Import from Claude Code
agentctl import cursor              # Import from Cursor
agentctl import codex               # Import from Codex

# Import specific resource types
agentctl import claude --servers    # Import only MCP servers
agentctl import claude --commands   # Import only commands
agentctl import claude --rules      # Import only rules
agentctl import claude --skills     # Import only skills

# Import from all discovered native resources
agentctl import --all               # Import from .claude/, .cursor/, etc.
agentctl import --all --rules       # Import only rules from all tools
agentctl import --all --dry-run     # Preview what would be imported

# Import workspace config to local project
agentctl import claude --local      # Import .mcp.json to .agentctl.json
```

### Copy Between Scopes

Copy resources between global and project-local configurations:

```bash
agentctl copy command my-cmd --to local    # Copy global command to project
agentctl copy rule my-rule --to global     # Promote local rule to global
agentctl copy skill my-skill --to local    # Copy global skill to project
```

### Syncing

```bash
agentctl sync                  # Sync to all detected tools
agentctl sync --tool claude    # Sync to specific tool
agentctl sync --dry-run        # Preview changes with diff output
agentctl sync --verbose        # Show detailed sync output
```

### List Resources

```bash
agentctl list                      # List all agentctl-managed resources
agentctl list --native             # Include tool-native discoveries
agentctl list --type servers       # List only MCP servers
agentctl list --type commands      # List only commands
agentctl list --type rules         # List only rules
agentctl list --type skills        # List only skills
agentctl list --type agents        # List only agents
agentctl list --scope local        # List only project-local resources
agentctl list --scope global       # List only global resources
```

### Resource Creation

```bash
agentctl new command <name>    # Create new slash command
agentctl new rule <name>       # Create new rule
agentctl new skill <name>      # Create new skill
agentctl new agent <name>      # Create new custom agent
agentctl new prompt <name>     # Create new prompt template
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

### Backups

```bash
agentctl backup list           # List available backups
agentctl backup create         # Create manual backup
agentctl backup restore <id>   # Restore from backup
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

## Interactive TUI

Launch with `agentctl ui` or just `agentctl`:

```bash
agentctl ui
```

### Tabs

- **Servers** - Manage MCP servers
- **Commands** - Manage slash commands
- **Rules** - Manage coding rules
- **Skills** - Manage skills
- **Hooks** - Manage lifecycle hooks
- **Agents** - Manage custom agents
- **Tools** - View detected tools and their capabilities

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Tab` | Switch between tabs |
| `j/k` or `Up/Down` | Navigate list |
| `/` | Filter/search list |
| `Enter` | View details / select |
| `a` | Add new item |
| `d` | Delete selected item |
| `e` | Edit selected item |
| `s` | Sync all servers |
| `t` | Test selected server |
| `i` | Import wizard |
| `b` | Backup modal |
| `?` | Show help |
| `q` | Quit |

### Import Wizard

Press `i` in the TUI to launch the import wizard:

1. **Select Tool** - Choose which tool to import from (Claude, Cursor, etc.)
2. **Select Resources** - Choose which resource types to import
3. **Review & Import** - Preview and confirm the import

## Native Discovery

agentctl can discover resources from tool-native directories:

| Tool | Directory | Discovered Resources |
|------|-----------|---------------------|
| Claude Code | `.claude/` | commands, rules, skills, agents |
| Cursor | `.cursor/rules/`, `.cursorrules` | rules |
| Codex | `.codex/` | commands, rules |
| Gemini | `.gemini/` | rules |

Use `agentctl list --native` to see discovered resources, or `agentctl import --all` to import them into agentctl management.

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
| Gemini | Yes | Yes |

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
