# agentctl: Universal Agent Configuration Manager

## Overview

`agentctl` is a CLI tool and Go library for managing agent configurations across multiple agentic frameworks and developer tools. It provides a single source of truth for MCP servers, commands, rules, prompts, and skills, syncing them to all supported tools.

**Module path:** `github.com/iheanyi/agentctl`
**License:** MIT
**Language:** Go

## Goals

1. **Unified management** - One config for MCP servers, commands, rules, prompts, and skills
2. **8 tool support** - Claude Code, Cursor, Codex, OpenCode, Cline, Windsurf, Zed, Continue
3. **First-class profiles** - Switch between "work" and "personal" configurations
4. **Project-local support** - Team-shareable `.agentctl.json` for project-specific configs
5. **Library-ready** - Clean Go API for tools to integrate agentctl directly
6. **Minimal maintenance** - No registry server; bundled aliases + git URLs

## Supported Tools (v1.0)

| Tool | Config Location | Resources Supported |
|------|-----------------|---------------------|
| Claude Code | `~/.config/claude/` | MCP, commands, skills |
| Cursor | `~/.cursor/` | MCP, rules |
| OpenAI Codex CLI | `~/.codex/` | MCP, commands |
| OpenCode | `~/.config/opencode/` | MCP, commands |
| Cline | `~/.cline/` | MCP, rules |
| Windsurf | `~/.windsurf/` | MCP, rules |
| Zed | `~/.config/zed/` | MCP, commands |
| Continue | `~/.continue/` | MCP, rules |

## Architecture

### Project Structure

```
agentctl/
├── cmd/
│   └── agentctl/              # CLI entry point
│       └── main.go
├── pkg/
│   ├── config/                # Canonical config, lockfile handling
│   ├── aliases/               # Bundled + user aliases
│   ├── sync/                  # Tool adapters for syncing configs
│   ├── profile/               # Profile management
│   ├── mcp/                   # MCP server types, health checks
│   ├── command/               # Command schema and handling
│   ├── rule/                  # Rule parsing (markdown + frontmatter)
│   ├── prompt/                # Prompt template handling
│   ├── skill/                 # Multi-file skill handling
│   ├── secrets/               # Keychain + env var secret handling
│   ├── search/                # mcp.so search integration
│   └── daemon/                # Background update daemon
├── internal/
│   ├── cli/                   # Cobra command implementations
│   ├── tui/                   # Bubble Tea TUI (optional mode)
│   └── schema/                # Tool config schema definitions
├── aliases.json               # Bundled aliases (curated short names)
├── .goreleaser.yaml           # Release automation
└── go.mod
```

### Directory Layout

```
~/.config/agentctl/              # Or $AGENTCTL_HOME
├── agentctl.json                # Main config
├── agentctl.lock                # Lockfile with exact versions
├── servers/                     # MCP server definitions
│   ├── filesystem.json
│   └── custom-mcp/              # Directory-based for complex ones
│       ├── server.json
│       └── scripts/
├── commands/                    # Slash commands
│   └── explain.json
├── rules/                       # Rules/instructions (markdown)
│   └── coding-style.md
├── prompts/                     # Prompt templates
│   └── review.json
├── skills/                      # Multi-file skills
│   └── my-skill/
│       ├── skill.json
│       └── prompts/
├── profiles/                    # Profile configs
│   ├── work.json
│   └── personal.json
└── aliases.json                 # User-defined aliases

~/.cache/agentctl/               # Separate cache (XDG compliant)
├── downloads/                   # Downloaded packages
├── builds/                      # Build artifacts
└── search/                      # mcp.so search cache
```

## Resource Types

### MCP Servers

```go
// pkg/mcp/server.go
type Server struct {
    Name        string            `json:"name"`
    Source      Source            `json:"source"`
    Command     string            `json:"command"`
    Args        []string          `json:"args,omitempty"`
    Env         map[string]string `json:"env,omitempty"`
    Transport   Transport         `json:"transport"`
    Namespace   string            `json:"namespace,omitempty"`
    Build       *BuildConfig      `json:"build,omitempty"`
    Disabled    bool              `json:"disabled,omitempty"`
}

type Source struct {
    Type  string `json:"type"`  // "git", "alias", "local"
    URL   string `json:"url,omitempty"`
    Ref   string `json:"ref,omitempty"`
    Alias string `json:"alias,omitempty"`
}

type Transport string
const (
    TransportStdio Transport = "stdio"
    TransportSSE   Transport = "sse"
)

type BuildConfig struct {
    Command string `json:"command"`
    WorkDir string `json:"workdir,omitempty"`
}
```

### Commands

Medium complexity schema supporting args, tool restrictions, and per-tool overrides:

```json
{
  "name": "explain-code",
  "description": "Explain selected code in detail",
  "prompt": "Explain this code step by step:\n\n{{selection}}",
  "args": {
    "depth": {
      "type": "string",
      "enum": ["brief", "detailed", "expert"],
      "default": "detailed",
      "description": "Level of detail in explanation"
    }
  },
  "allowedTools": ["Read", "Grep"],
  "disallowedTools": ["Bash", "Write"],
  "overrides": {
    "cursor": { "alwaysApply": true },
    "claude": { "allowedTools": ["Read", "Grep", "Glob"] }
  }
}
```

```go
// pkg/command/command.go
type Command struct {
    Name            string                 `json:"name"`
    Description     string                 `json:"description"`
    Prompt          string                 `json:"prompt"`
    Args            map[string]Arg         `json:"args,omitempty"`
    AllowedTools    []string               `json:"allowedTools,omitempty"`
    DisallowedTools []string               `json:"disallowedTools,omitempty"`
    Overrides       map[string]ToolOverride `json:"overrides,omitempty"`
}

type Arg struct {
    Type        string   `json:"type"`
    Enum        []string `json:"enum,omitempty"`
    Default     any      `json:"default,omitempty"`
    Description string   `json:"description,omitempty"`
    Required    bool     `json:"required,omitempty"`
}
```

### Rules

Plain text markdown with optional YAML frontmatter:

```markdown
---
priority: 1
tools: ["claude", "cursor"]
applies: "*.ts"
---

# TypeScript Coding Rules

Always use strict TypeScript with no `any` types.
Prefer interfaces over type aliases for object shapes.
Use `const` assertions for literal types.
```

```go
// pkg/rule/rule.go
type Rule struct {
    Frontmatter *RuleFrontmatter
    Content     string
    Path        string
}

type RuleFrontmatter struct {
    Priority int      `yaml:"priority,omitempty"`
    Tools    []string `yaml:"tools,omitempty"`
    Applies  string   `yaml:"applies,omitempty"`
}
```

### Prompts

Reusable prompt templates that can be referenced by commands:

```json
{
  "name": "code-review",
  "description": "Standard code review prompt",
  "template": "Review this code for:\n1. Bugs and logic errors\n2. Performance issues\n3. Security vulnerabilities\n\n{{code}}",
  "variables": ["code"]
}
```

### Skills

Directory-based multi-file resources (Claude plugins, complex workflows):

```
skills/my-skill/
├── skill.json           # Skill metadata
├── prompts/
│   └── main.md         # Skill prompts
└── examples/
    └── usage.md        # Usage examples
```

## Configuration

### Main Config

```json
// agentctl.json
{
  "version": "1",
  "servers": {
    "filesystem": {
      "source": { "type": "alias", "alias": "filesystem" },
      "namespace": "fs"
    },
    "github": {
      "source": { "type": "git", "url": "github.com/org/github-mcp", "ref": "^1.0.0" },
      "env": { "GITHUB_TOKEN": "$GITHUB_TOKEN" }
    }
  },
  "commands": ["explain", "review"],
  "rules": ["coding-style", "security"],
  "prompts": ["code-review"],
  "skills": ["my-skill"],
  "settings": {
    "defaultProfile": "work",
    "autoUpdate": {
      "enabled": true,
      "interval": "24h"
    },
    "tools": {
      "claude": { "enabled": true },
      "cursor": { "enabled": true },
      "codex": { "enabled": true },
      "opencode": { "enabled": true },
      "cline": { "enabled": true },
      "windsurf": { "enabled": true },
      "zed": { "enabled": true },
      "continue": { "enabled": true }
    }
  }
}
```

### Profiles

```json
// profiles/work.json
{
  "name": "work",
  "servers": ["filesystem", "github", "postgres"],
  "commands": ["explain", "review", "deploy"],
  "rules": ["coding-style", "company-standards"],
  "disabled": ["personal-mcp"]
}
```

### Project-Local Config

When `.agentctl.json` exists in project root, it inherits from global and can override:

```json
// .agentctl.json (in project root)
{
  "version": "1",
  "servers": {
    "project-db": {
      "source": { "type": "local", "url": "./tools/db-mcp" },
      "command": "node",
      "args": ["index.js"]
    }
  },
  "rules": ["project-rules"],
  "disabled": ["github"],
  "profile": "work"
}
```

**Inheritance behavior:**
1. Load global config
2. Merge project config (add new, override existing)
3. Apply `disabled` list to remove specific resources
4. Sync result to tools

### Lockfile

```json
// agentctl.lock
{
  "version": "1",
  "locked": {
    "filesystem": {
      "source": "github.com/modelcontextprotocol/servers",
      "version": "1.2.3",
      "commit": "abc1234def5678",
      "integrity": "sha256-..."
    }
  }
}
```

## Aliases (No Full Registry)

Instead of maintaining a registry server, agentctl uses aliases:

### Bundled Aliases

```json
// aliases.json (ships with agentctl, updated each release)
{
  "filesystem": "github.com/modelcontextprotocol/servers/src/filesystem",
  "github": "github.com/modelcontextprotocol/servers/src/github",
  "postgres": "github.com/modelcontextprotocol/servers/src/postgres",
  "sqlite": "github.com/modelcontextprotocol/servers/src/sqlite",
  "slack": "github.com/modelcontextprotocol/servers/src/slack",
  "memory": "github.com/modelcontextprotocol/servers/src/memory"
}
```

### User Aliases

```bash
agentctl alias add mydb github.com/myorg/custom-db-mcp
agentctl alias remove mydb
agentctl alias list
```

### Search via mcp.so

```bash
agentctl search slack     # Searches mcp.so, returns results
agentctl search --refresh # Clear cache and search again
```

Search results are cached locally, not maintained by agentctl.

## Secret Management

Secrets resolved in order:
1. **System keychain** - macOS Keychain, Windows Credential Manager, Linux Secret Service
2. **Environment variables** - `$VAR` syntax in config

```json
{
  "servers": {
    "github": {
      "env": {
        "GITHUB_TOKEN": "$GITHUB_TOKEN",
        "SLACK_KEY": "keychain:agentctl/slack-api-key"
      }
    }
  }
}
```

```bash
agentctl secret set github-token    # Prompts for value, stores in keychain
agentctl secret get github-token
agentctl secret delete github-token
agentctl secret list
```

## Sync Mechanics

### Coexistence with Manual Configs

agentctl marks its entries and preserves manually-added ones:

```json
// Tool config (e.g., Claude's claude_desktop_config.json)
{
  "mcpServers": {
    "filesystem": {
      "_managedBy": "agentctl",
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem"]
    },
    "my-manual-mcp": {
      "command": "/path/to/custom"
    }
  }
}
```

### Sync Behavior

- Entries with `"_managedBy": "agentctl"` are managed
- Entries without the marker are preserved
- `agentctl sync` updates managed entries, leaves others alone
- `agentctl sync --clean` removes managed entries not in config

### Conflict Resolution

Namespace prefixing for overlapping tool names:

```json
{
  "servers": {
    "filesystem": { "namespace": "fs" },
    "github": { "namespace": "gh" }
  }
}
```

Results in tools like `fs.read_file`, `gh.read_file`.

## CLI Commands

### Core Commands

```bash
# Installation
agentctl install <alias-or-url>           # Install MCP server
agentctl install filesystem               # From alias
agentctl install github.com/org/repo      # From git URL
agentctl install ./local/path             # Local path
agentctl install filesystem@v1.2.3        # Specific version

agentctl remove <server>                  # Remove server
agentctl list                             # List all resources
agentctl list --type servers              # List specific type
agentctl list --profile work              # List from profile
```

### Syncing

```bash
agentctl sync                             # Sync to all detected tools
agentctl sync --tool claude               # Sync to specific tool
agentctl sync --dry-run                   # Preview changes
agentctl sync --clean                     # Remove stale managed entries
```

### Updates

```bash
agentctl update                           # Update all
agentctl update <server>                  # Update specific
agentctl update --check                   # Check without applying
```

### Search & Discovery

```bash
agentctl search <query>                   # Search mcp.so
agentctl info <alias-or-url>              # Show resource details
agentctl alias list                       # List aliases
agentctl alias add <name> <url>           # Add user alias
agentctl alias remove <name>              # Remove user alias
```

### Profiles

```bash
agentctl profile list
agentctl profile create <name>
agentctl profile switch <name>
agentctl profile export <name> > profile.json
agentctl profile import < profile.json
```

### Resources

```bash
# Commands
agentctl new command <name>               # Create from template
agentctl new command <name> --interactive # Interactive creation

# Rules
agentctl new rule <name>
agentctl new rule <name> --interactive

# Prompts
agentctl new prompt <name>

# Skills
agentctl new skill <name>
```

### Utilities

```bash
agentctl init                             # Initialize in current directory
agentctl doctor                           # Diagnose issues
agentctl test <server>                    # Health check server
agentctl status                           # Show all resource status
agentctl config                           # View/edit config
agentctl config get <key>
agentctl config set <key> <value>
agentctl ui                               # Launch TUI mode
```

### Secrets

```bash
agentctl secret set <name>                # Store in keychain
agentctl secret get <name>
agentctl secret delete <name>
agentctl secret list
```

### Daemon

```bash
agentctl daemon start
agentctl daemon stop
agentctl daemon status
```

### Import from Existing Tools

```bash
agentctl import claude                    # Import all from Claude
agentctl import claude --servers          # Import only servers
agentctl import cursor --rules            # Import only rules
```

## Tool Sync Interface

```go
// pkg/sync/adapter.go
type Adapter interface {
    Name() string
    Detect() (bool, error)
    ConfigPath() string

    // Resource-specific methods
    ReadServers() ([]Server, error)
    WriteServers(servers []Server, marker string) error

    ReadCommands() ([]Command, error)
    WriteCommands(commands []Command, marker string) error

    ReadRules() ([]Rule, error)
    WriteRules(rules []Rule, marker string) error

    SupportedResources() []ResourceType
}

type ResourceType string
const (
    ResourceMCP      ResourceType = "mcp"
    ResourceCommands ResourceType = "commands"
    ResourceRules    ResourceType = "rules"
    ResourcePrompts  ResourceType = "prompts"
    ResourceSkills   ResourceType = "skills"
)
```

## Background Daemon

```json
{
  "settings": {
    "autoUpdate": {
      "enabled": true,
      "interval": "24h",
      "servers": {
        "filesystem": "auto",
        "custom-mcp": "notify"
      }
    }
  }
}
```

- Periodic update checks
- CLI message on next run when updates available
- Auto-update for configured servers
- Unix socket (macOS/Linux) / named pipe (Windows)

## Dependency Checking

```bash
# On install, checks for required runtimes
Warning: Server 'filesystem' requires Node.js >= 18.0.0
  Install Node.js: https://nodejs.org/
  Or use nvm: nvm install 18
```

## Library API

```go
import "github.com/iheanyi/agentctl/pkg/config"
import "github.com/iheanyi/agentctl/pkg/sync"

// Load config
cfg, err := config.Load()
if err != nil {
    log.Fatal(err)
}

// Sync to all tools
results := sync.All(cfg)
for tool, err := range results {
    if err != nil {
        log.Printf("%s: %v", tool, err)
    }
}

// Sync to specific tool
adapter := sync.NewClaudeAdapter()
if detected, _ := adapter.Detect(); detected {
    adapter.WriteServers(cfg.ActiveServers(), "agentctl")
}
```

## Distribution

1. **Go install**: `go install github.com/iheanyi/agentctl/cmd/agentctl@latest`
2. **Homebrew**: `brew install iheanyi/tap/agentctl`
3. **Binary releases**: GitHub Releases for darwin/linux/windows (amd64/arm64)
4. **Install script**: `curl -sSL https://get.agentctl.dev | sh`

## Implementation Phases

### Phase 1: Foundation
- Project setup, Go modules, basic CLI structure
- Config loading/saving with JSON schema
- Single tool adapter (Claude Code)
- MCP servers only

### Phase 2: Multi-tool Support
- All 8 tool adapters
- Sync command with coexistence markers
- Basic health check
- Alias system

### Phase 3: Commands & Rules
- Command schema implementation
- Rule parsing (markdown + frontmatter)
- Cross-tool command/rule sync
- Scaffolding commands (`agentctl new`)

### Phase 4: Profiles & Projects
- Profile management
- Project-local config
- Config inheritance/merging
- Import from existing tools

### Phase 5: Advanced Features
- Prompts and skills
- TUI mode (Bubble Tea)
- Background daemon
- Secret management (keychain)
- mcp.so search integration

### Phase 6: Polish & Distribution
- Doctor command
- Comprehensive error messages
- GoReleaser setup
- Homebrew formula
- Install script
- Documentation

## Open Questions

1. **Tool-specific schemas**: How much variation exists in command/rule formats across tools?
2. **MCP spec evolution**: How to handle breaking changes in MCP protocol?
3. **Windows support priority**: Full parity or best-effort?
4. **Telemetry**: Opt-in anonymous usage stats?

---

*Spec version: 2.0*
*Last updated: 2025-12-30*
