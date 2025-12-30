# agentctl v2.0: MCP Installation Experience Redesign

## Overview

A comprehensive redesign of agentctl's MCP installation experience based on analysis of industry leaders (Notion, Figma, Sentry, Linear, GitHub, PlanetScale) and user interview feedback.

**Target Release:** Big bang v2.0 release with full feature set

---

## Core Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Install + Sync UX | Auto-sync by default | `agentctl install X` immediately syncs to all tools. Add `--no-sync` to opt out. |
| OAuth Handling | Hybrid (metadata-driven) | MCP registry defines whether agentctl or tool handles OAuth. Remote MCPs delegate, local MCPs with env vars use agentctl. |
| Transport Gaps | Suggest alternatives | If tool doesn't support transport, suggest fallback: "cursor: using npx figma-mcp instead of remote" |
| Registry | Hybrid (embedded + GitHub + npm fallback) | ~100 essential MCPs bundled, extended registry on GitHub, npm/pypi fallback for everything else |
| Multi-distribution | Smart default + flags | `agentctl install notion` picks best default, `--local` / `--docker` / `--remote` to override |
| Output UX | Rich/interactive | Progress spinners, test connection prompt, open tool prompt, usage tips |
| Updates | Check on install | Non-blocking notification of available updates when installing anything |
| Profiles | First-class, additive model | Base MCPs always present, profiles add/remove specific ones |
| Local Config | Critical, merge strategy | Project `.agentctl.json` overrides same-name globals, otherwise merges |
| TUI | Both views (Installed + Browse) | Tabbed interface for management and discovery |
| Branding | Keep agentctl | Name is established, focus on product |

---

## 1. Installation UX Redesign

### 1.1 New Install Flow

```bash
# Simple install - auto-syncs to all detected tools
agentctl install figma

# Output:
# ┌─────────────────────────────────────────────────────────────┐
# │ Installing figma                                             │
# ├─────────────────────────────────────────────────────────────┤
# │ ✓ Resolved: Figma MCP (remote, HTTP transport)              │
# │ ✓ Transport: HTTP → https://mcp.figma.com/mcp               │
# │ ✓ Auth: OAuth (handled by Claude/Cursor on first use)       │
# │                                                              │
# │ Syncing to tools...                                          │
# │   ✓ claude      - configured (HTTP supported)                │
# │   ✓ cursor      - using @figma/mcp-local instead (no HTTP)   │
# │   ✓ cline       - skipped (HTTP not supported, no fallback)  │
# │                                                              │
# │ ✓ Installed figma to 2 tools                                 │
# └─────────────────────────────────────────────────────────────┘
#
# 💡 Test connection now? [Y/n]
# 💡 Open Claude Code to try it? [Y/n]
#
# Example prompts to try:
#   • "Show me the design for the login page"
#   • "Generate code for the selected Figma frame"
```

### 1.2 Install Flags

```bash
agentctl install notion              # Smart default (remote if available)
agentctl install notion --local      # Force npx @notionhq/notion-mcp-server
agentctl install notion --docker     # Force Docker variant
agentctl install notion --remote     # Force remote HTTP variant
agentctl install notion --no-sync    # Install to registry only, don't sync
agentctl install notion --target cursor  # Sync to specific tool only
```

### 1.3 URL-based Installation

```bash
# Remote MCP URLs
agentctl install https://mcp.sentry.dev/mcp
agentctl install https://mcp.figma.com/mcp
agentctl install https://mcp.notion.com/mcp

# Git URLs
agentctl install github.com/company/internal-mcp

# npm packages (fallback)
agentctl install @company/custom-mcp
```

---

## 2. Registry System

### 2.1 Three-Tier Registry

```
┌─────────────────────────────────────────────────────────────┐
│ Tier 1: Embedded Essentials (~100 MCPs)                     │
│ • Bundled in binary for offline/fast access                 │
│ • Vendor-backed: Notion, Figma, Sentry, Linear, GitHub...   │
│ • Core tools: filesystem, postgres, fetch, memory...        │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│ Tier 2: GitHub Extended Registry                            │
│ • Fetched on `agentctl search`, cached locally              │
│ • Community PRs to add MCPs (quality standards)             │
│ • Updated weekly, ~1000+ MCPs                               │
│ • URL: github.com/iheanyi/agentctl-registry/registry.json   │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│ Tier 3: npm/pypi Fallback                                   │
│ • If not in registry: try npm/pypi directly                 │
│ • `agentctl install @company/mcp-server`                    │
│ • Less metadata, but works for any package                  │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 Enhanced Registry Entry Schema

```json
{
  "notion": {
    "name": "Notion",
    "description": "Connect to Notion workspaces - read/write pages, databases, comments",
    "vendor": "Notion",
    "verified": true,
    "categories": ["productivity", "documents", "databases"],
    "popularity": 9823,

    "variants": {
      "remote": {
        "default": true,
        "transport": "http",
        "url": "https://mcp.notion.com/mcp",
        "auth": {
          "type": "oauth",
          "handler": "tool",
          "provider": "notion"
        }
      },
      "local": {
        "transport": "stdio",
        "runtime": "node",
        "package": "@notionhq/notion-mcp-server",
        "auth": {
          "type": "env",
          "handler": "agentctl",
          "vars": ["NOTION_API_KEY"],
          "setup_url": "https://developers.notion.com/docs/getting-started"
        }
      },
      "docker": {
        "transport": "stdio",
        "runtime": "docker",
        "image": "notion/mcp-server:latest"
      }
    },

    "toolSupport": {
      "claude": ["remote", "local", "docker"],
      "cursor": ["local", "docker"],
      "cline": ["local", "docker"]
    },

    "examples": [
      "List all pages in my workspace",
      "Create a new page titled 'Meeting Notes'",
      "Search for pages containing 'Q4 planning'"
    ]
  }
}
```

### 2.3 Priority MCPs for v2.0 Registry

**Vendor-Backed (Remote OAuth):**
- Notion, Figma, Sentry, Linear, GitHub, PlanetScale
- Supabase, Vercel, Cloudflare, Stripe, Slack
- Google (Drive, Calendar, Docs), Atlassian (Jira, Confluence)

**Developer Tools (Local):**
- filesystem, postgres, sqlite, mysql, mongodb
- fetch, puppeteer, playwright, browserbase
- git, docker, kubernetes
- memory, sequential-thinking

---

## 3. OAuth & Authentication

### 3.1 Hybrid OAuth Strategy

```
┌─────────────────────────────────────────────────────────────┐
│ MCP Registry Entry                                          │
│ auth.handler = "tool" | "agentctl" | "manual"               │
└─────────────────────────────────────────────────────────────┘
                            │
           ┌────────────────┼────────────────┐
           ▼                ▼                ▼
    ┌─────────────┐  ┌─────────────┐  ┌─────────────┐
    │ handler:    │  │ handler:    │  │ handler:    │
    │ "tool"      │  │ "agentctl"  │  │ "manual"    │
    │             │  │             │  │             │
    │ Remote HTTP │  │ Local MCPs  │  │ Copy/paste  │
    │ MCPs with   │  │ needing     │  │ API keys    │
    │ vendor OAuth│  │ env vars    │  │ from docs   │
    └─────────────┘  └─────────────┘  └─────────────┘
```

### 3.2 agentctl OAuth Flow (for local MCPs)

```bash
agentctl install github
# Output:
# ┌─────────────────────────────────────────────────────────────┐
# │ GitHub MCP requires authentication                          │
# │                                                              │
# │ Choose setup method:                                         │
# │   1. Browser OAuth (recommended)                             │
# │   2. Enter Personal Access Token manually                    │
# │   3. Skip auth (configure later)                             │
# └─────────────────────────────────────────────────────────────┘
#
# > 1
#
# Opening browser for GitHub authentication...
# Waiting for callback...
#
# ✓ Authenticated as @iheanyi
# ✓ Token stored in system keychain
# ✓ GitHub MCP configured with GITHUB_TOKEN
```

### 3.3 Keychain Integration

```bash
# Tokens stored securely in system keychain
agentctl secret list
# github-token     GitHub (expires: 2025-12-01)
# notion-api-key   Notion (no expiry)

# Automatic refresh for expiring tokens
agentctl secret refresh github-token
```

---

## 4. Profile System

### 4.1 Profile Structure

```
~/.config/agentctl/
├── agentctl.json           # Global config + base MCPs
├── profiles/
│   ├── work.json           # Work profile additions
│   ├── personal.json       # Personal profile additions
│   └── client-acme.json    # Client-specific profile
└── agentctl.lock           # Version lockfile
```

### 4.2 Additive Profile Model

```bash
# Create a profile
agentctl profile create work

# Add MCPs to profile
agentctl install linear --profile work
agentctl install slack --profile work

# Switch profiles
agentctl profile switch work
# Output:
# ┌─────────────────────────────────────────────────────────────┐
# │ Switching to profile: work                                   │
# │                                                              │
# │ Base MCPs (always active):                                   │
# │   ✓ filesystem, fetch, memory                                │
# │                                                              │
# │ Adding work MCPs:                                            │
# │   + linear                                                   │
# │   + slack                                                    │
# │   + github (work org)                                        │
# │                                                              │
# │ Syncing to tools...                                          │
# │   ✓ claude, cursor, cline                                    │
# └─────────────────────────────────────────────────────────────┘

# Show current profile
agentctl profile show
# Active: work
# Base: filesystem, fetch, memory
# Profile: linear, slack, github
```

### 4.3 Profile Configuration

```json
// ~/.config/agentctl/profiles/work.json
{
  "name": "work",
  "description": "Work environment with team tools",
  "add": ["linear", "slack", "github"],
  "remove": ["personal-notion"],
  "overrides": {
    "github": {
      "env": {
        "GITHUB_TOKEN": "keychain:github-work-token"
      }
    }
  }
}
```

---

## 5. Project-Local Configuration

### 5.1 .agentctl.json Schema

```json
// /path/to/project/.agentctl.json
{
  "$schema": "https://agentctl.dev/schema/project.json",

  "disable": ["notion"],  // Disable these globals

  "servers": {
    "postgres": {
      "transport": "stdio",
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-postgres"],
      "env": {
        "POSTGRES_URL": "${PROJECT_DB_URL}"  // From .env
      }
    },
    "github": {
      "extends": "global",  // Inherit from global, override env
      "env": {
        "GITHUB_TOKEN": "keychain:github-acme-token"
      }
    }
  }
}
```

### 5.2 Merge Behavior

```
Global Config              Project Config           Result (in project)
─────────────────────────────────────────────────────────────────────
filesystem    ────────────────────────────────────► filesystem (global)
notion        ──── disable: ["notion"] ──────────► (removed)
github        ──── github (override) ────────────► github (project)
              ──── postgres (new) ───────────────► postgres (project)
```

### 5.3 Project Detection

```bash
cd /path/to/project
agentctl sync
# Output:
# ┌─────────────────────────────────────────────────────────────┐
# │ Detected project config: .agentctl.json                      │
# │                                                              │
# │ Global MCPs:                                                 │
# │   ✓ filesystem (kept)                                        │
# │   ✗ notion (disabled by project)                             │
# │   ↻ github (overridden by project)                           │
# │                                                              │
# │ Project MCPs:                                                │
# │   + postgres                                                 │
# │                                                              │
# │ Syncing 3 MCPs to tools...                                   │
# └─────────────────────────────────────────────────────────────┘
```

---

## 6. Interactive TUI

### 6.1 Two-View Interface

```
┌─────────────────────────────────────────────────────────────────┐
│ agentctl                                    [Installed] [Browse] │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  INSTALLED (5)                              Status    Updated    │
│  ─────────────────────────────────────────────────────────────  │
│  ● filesystem                               enabled   built-in   │
│  ● github                                   enabled   v1.2.3     │
│  ● notion                                   enabled   remote     │
│  ○ linear (profile: work)                   disabled  remote     │
│  ● postgres                                 enabled   v2.1.0     │
│                                                                  │
│  ─────────────────────────────────────────────────────────────  │
│  [Enter] Toggle  [u] Update  [r] Remove  [Tab] Browse  [q] Quit │
└─────────────────────────────────────────────────────────────────┘
```

```
┌─────────────────────────────────────────────────────────────────┐
│ agentctl                                    [Installed] [Browse] │
├─────────────────────────────────────────────────────────────────┤
│ Search: figma_                              Category: [All ▼]   │
│                                                                  │
│  BROWSE REGISTRY                            ★ Stars   Transport │
│  ─────────────────────────────────────────────────────────────  │
│  ▸ Figma                    verified        ★ 4.2k    HTTP      │
│    Design-to-code from Figma frames                              │
│                                                                  │
│    figma-mcp               community        ★ 892     stdio     │
│    Alternative Figma integration                                 │
│                                                                  │
│    figma-variables         community        ★ 234     stdio     │
│    Export Figma variables to code                                │
│                                                                  │
│  ─────────────────────────────────────────────────────────────  │
│  [Enter] Install  [i] Info  [Tab] Installed  [/] Search  [q] Quit│
└─────────────────────────────────────────────────────────────────┘
```

### 6.2 TUI Commands

```bash
agentctl ui                    # Open TUI (default: Installed view)
agentctl ui --browse           # Open TUI in Browse view
agentctl browse                # Alias for `agentctl ui --browse`
```

---

## 7. Update System

### 7.1 Check on Install

```bash
agentctl install postgres
# Output:
# ✓ Installed postgres
#
# 💡 2 updates available:
#    • github: v1.2.3 → v1.3.0 (security fix)
#    • filesystem: v1.0.0 → v1.1.0
#    Run 'agentctl update' to upgrade
```

### 7.2 Update Commands

```bash
agentctl update                # Interactive: show updates, confirm
agentctl update --all          # Update all without prompting
agentctl update github         # Update specific MCP
agentctl update --check        # Just check, don't update
```

---

## 8. Enhanced CLI Commands

### 8.1 Command Reference

```bash
# Installation
agentctl install <mcp>           # Install with auto-sync
agentctl install <mcp> --local   # Force local variant
agentctl install <mcp> --remote  # Force remote variant
agentctl install <mcp> --no-sync # Don't auto-sync
agentctl install <mcp> --target cursor  # Sync to specific tool

# Listing & Search
agentctl list                    # List installed MCPs
agentctl search <query>          # Search registry
agentctl info <mcp>              # Detailed MCP info

# Syncing
agentctl sync                    # Sync to all tools (detects project config)
agentctl sync --tool cursor      # Sync to specific tool
agentctl sync --dry-run          # Preview changes

# Profiles
agentctl profile list            # List profiles
agentctl profile create <name>   # Create profile
agentctl profile switch <name>   # Switch active profile
agentctl profile show            # Show current profile details
agentctl profile delete <name>   # Delete profile

# Authentication
agentctl auth <mcp>              # Authenticate/re-authenticate
agentctl secret list             # List stored credentials
agentctl secret set <name>       # Store a secret
agentctl secret delete <name>    # Remove a secret

# Updates
agentctl update                  # Update all MCPs
agentctl update <mcp>            # Update specific MCP
agentctl update --check          # Check for updates only

# Management
agentctl remove <mcp>            # Uninstall MCP
agentctl disable <mcp>           # Disable without removing
agentctl enable <mcp>            # Re-enable disabled MCP
agentctl test <mcp>              # Test MCP connection

# TUI
agentctl ui                      # Open TUI
agentctl browse                  # Open TUI in browse mode

# Aliases
agentctl alias list              # List available aliases
agentctl alias add <name> <url>  # Add custom alias
agentctl alias remove <name>     # Remove custom alias
```

---

## 9. Implementation Phases

Since this is a "big bang" v2.0 release, all features should be completed before release:

### Phase 1: Core UX (Foundation)
- [ ] Auto-sync on install (default behavior)
- [ ] Rich/interactive output with spinners
- [ ] Transport fallback suggestions
- [ ] Post-install test & open prompts
- [ ] Update check on install

### Phase 2: Registry Expansion
- [ ] Enhanced registry schema with variants
- [ ] GitHub extended registry infrastructure
- [ ] npm/pypi fallback resolution
- [ ] 100+ essential MCPs in embedded registry
- [ ] Search command with registry fetch

### Phase 3: Authentication
- [ ] Hybrid OAuth detection from metadata
- [ ] agentctl OAuth flow (browser + callback)
- [ ] Keychain integration improvements
- [ ] Token refresh for expiring credentials

### Phase 4: Profiles & Local Config
- [ ] Profile create/switch/delete commands
- [ ] Additive profile model implementation
- [ ] Project .agentctl.json detection
- [ ] Merge strategy (override same-name, add others)
- [ ] Profile-specific tool sync

### Phase 5: TUI
- [ ] Two-view interface (Installed + Browse)
- [ ] Search and filtering
- [ ] Enable/disable/update from TUI
- [ ] Install from browse view

### Phase 6: Polish
- [ ] Comprehensive error messages
- [ ] Help text and examples
- [ ] Shell completions
- [ ] Documentation site

---

## 10. Success Metrics

- Install time for popular MCPs < 5 seconds (remote) / < 30 seconds (local)
- Zero-config setup for top 20 MCPs (one command, no manual steps)
- TUI discovery leads to 3+ MCP installs per session (average)
- Profile switching < 2 seconds
- 95%+ of users never need to manually edit JSON config

---

## Sources Referenced

- [Notion MCP Documentation](https://developers.notion.com/docs/mcp)
- [Linear MCP Server](https://linear.app/docs/mcp)
- [PlanetScale MCP Support](https://planetscale.com/docs/vitess/connecting/mcp)
- [GitHub MCP Server](https://github.com/github/github-mcp-server)
- [Figma MCP Server](https://developers.figma.com/docs/figma-mcp-server/remote-server-installation/)
- [Sentry MCP Documentation](https://docs.sentry.io/product/sentry-mcp/)
- [MCP.so Registry](https://mcp.so)
