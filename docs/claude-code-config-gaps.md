# Universal Configuration Management Vision

agentctl manages MCP servers today. This document proposes expanding to **universal configuration management** across all supported tools—not just MCP servers, but plugins, permissions, hooks, rules, and tool-specific settings.

## Strategic Vision

The AI coding tool ecosystem is converging:

| Concept | Claude Code | Cursor | Zed | Others |
|---------|-------------|--------|-----|--------|
| MCP Servers | ✅ | ✅ | ✅ | Expanding |
| Plugins/Extensions | plugins | VS Code ext | extensions | Coming |
| Permissions | permissions | - | - | Coming |
| Hooks | hooks | - | - | Coming |
| Rules/Instructions | CLAUDE.md | .cursorrules | - | Converging |

**Today:** agentctl syncs MCP servers across tools.
**Tomorrow:** agentctl becomes the universal abstraction layer that maps equivalent concepts across all tools.

### Architectural Model

```
agentctl.json (universal config)
       │
       ▼
   Adapters translate to tool-specific formats
       │
       ├── Claude Code: settings.json + CLAUDE.md + plugins/
       ├── Cursor: mcp.json + .cursorrules + extensions
       ├── Zed: settings.json + context_servers + extensions
       ├── Codex: config.json
       ├── Copilot CLI: config files
       └── etc.
```

Tools that don't support a resource type return `nil`—same as how Cursor filters HTTP transport servers today.

### Adapter Interface Evolution

```go
type Adapter interface {
    // Existing
    Name() string
    Detect() (bool, error)
    ConfigPath() string
    SupportedResources() []ResourceType
    ReadServers() ([]*mcp.Server, error)
    WriteServers([]*mcp.Server) error

    // New resource types
    ReadPlugins() ([]*Plugin, error)
    WritePlugins([]*Plugin) error

    ReadPermissions() ([]*Permission, error)
    WritePermissions([]*Permission) error

    ReadHooks() ([]*Hook, error)
    WriteHooks([]*Hook) error

    ReadRules() (string, error)  // CLAUDE.md, .cursorrules, etc.
    WriteRules(string) error
}
```

---

## Rules Systems Comparison

Different tools have different "rules" concepts that need proper mapping:

| Tool | Location | Format | Purpose |
|------|----------|--------|---------|
| **Claude Code** | `.claude/rules/*.md`, `~/.claude/rules/*.md` | Markdown + YAML frontmatter | Coding instructions |
| **Cursor** | `.cursorrules`, `~/.cursorrules` | Plain markdown | Coding instructions |
| **Windsurf** | `.windsurfrules` | Plain markdown | Coding instructions |
| **Codex** | `~/.codex/rules/*.rules` | Starlark DSL | Command permissions |

### Claude Code Rules Format

```yaml
---
paths:
  - "src/api/**/*.ts"
---
# API Development Rules
- All API endpoints must include input validation
- Use the standard error response format
```

The `paths:` frontmatter makes rules conditional - only applied when working with matching files.

### Codex Rules Format (Different!)

Codex "rules" are **not** coding instructions - they control command execution:

```starlark
prefix_rule(
    pattern=["git", "push"],
    decision="prompt",
    justification="Pushing requires explicit approval"
)
```

This maps to Claude Code's **permissions** concept, not `.claude/rules/`.

---

## Phase 1: Claude Code Full Support

Starting with Claude Code since it has the richest configuration surface. Other tools will follow as they add equivalent features.

### Current State

agentctl handles MCP servers for Claude Code. However, Claude Code has additional configuration that users need to manage separately in `~/.claude/`:

## Claude-Specific Config Not Currently Managed

### 1. `CLAUDE.md` - Global Instructions

**File:** `~/.claude/CLAUDE.md`

**What it does:** Global instructions that apply to all Claude Code sessions (like a system prompt). Users put coding standards, project conventions, safety rules here.

**Example content:**
```markdown
## REQUIRED
- Never use `rm -rf` - use `trash` instead
- Always run tests before committing
- Use TypeScript strict mode
```

**Suggestion:** `agentctl` could manage this with:
```bash
agentctl claude instructions edit    # Open in $EDITOR
agentctl claude instructions show    # Display current
agentctl claude instructions sync    # Sync from dotfiles
```

---

### 2. Plugins & Marketplaces

**Files:**
- `~/.claude/settings.json` → `enabledPlugins` section
- `~/.claude/plugins/known_marketplaces.json`
- `~/.claude/plugins/installed_plugins.json`

**What it does:** Claude Code has a plugin system with:
- **Marketplaces** - Git repos containing plugins (e.g., `anthropics/claude-plugins-official`, `obra/superpowers-marketplace`)
- **Plugins** - Skills, commands, hooks bundled together

**Current workflow:**
```bash
/plugin marketplace add anthropics/skills
/plugin install document-skills@anthropic-agent-skills
```

**Suggestion:** agentctl could track desired plugins:
```jsonc
// agentctl.json
{
  "claude": {
    "marketplaces": [
      "anthropics/claude-plugins-official",
      "anthropics/skills",
      "obra/superpowers-marketplace"
    ],
    "plugins": [
      "superpowers@claude-plugins-official",
      "document-skills@anthropic-agent-skills",
      "sentry@claude-plugins-official"
    ]
  }
}
```

```bash
agentctl claude plugins sync    # Register marketplaces & install plugins
agentctl claude plugins list    # Show installed
agentctl claude plugins add superpowers@claude-plugins-official
```

---

### 3. Status Line

**File:** `~/.claude/settings.json` → `statusLine` section

**What it does:** Custom command that runs to display info in Claude Code's status bar.

**Current config:**
```json
{
  "statusLine": {
    "command": "/bin/bash ~/.claude/statusline-command.sh",
    "type": "command"
  }
}
```

**Suggestion:** Could be part of Claude-specific settings in agentctl.json

---

### 4. Permissions

**File:** `~/.claude/settings.json` → `permissions` section

**What it does:** Pre-approved tool permissions so Claude doesn't ask every time.

**Example:**
```json
{
  "permissions": {
    "allow": [
      "Skill(dev-browser:dev-browser)",
      "Bash(npx tsx:*)",
      "Bash(git:*)"
    ]
  }
}
```

**Suggestion:** Track in agentctl for consistency across machines.

---

### 5. Hooks (Global)

**File:** `~/.claude/settings.json` → `hooks` section

**What it does:** Shell commands that run on Claude Code events (SessionStart, PreToolUse, Stop, etc.)

**Note:** These can also be project-local in `.claude/settings.json`, but global hooks are useful for things like:
- Session logging
- Automatic context priming
- Integration with task management (tasuku)

---

## Proposed agentctl.json Schema Addition

```jsonc
{
  "servers": { /* existing MCP servers */ },
  "settings": { /* existing tool settings */ },

  // NEW: Claude Code specific
  "claude": {
    "instructions": "path/to/CLAUDE.md",  // or inline string
    "marketplaces": [
      "anthropics/claude-plugins-official",
      "obra/superpowers-marketplace"
    ],
    "plugins": [
      "superpowers@claude-plugins-official",
      "document-skills@anthropic-agent-skills"
    ],
    "permissions": {
      "allow": ["Bash(git:*)"]
    },
    "statusLine": {
      "command": "~/.config/agentctl/statusline.sh"
    },
    "hooks": {
      "SessionStart": [
        { "command": "tk hooks session", "type": "command" }
      ]
    },
    "alwaysThinkingEnabled": true
  }
}
```

## Benefits

1. **Single source of truth** - All config in one place
2. **Easy machine bootstrap** - `agentctl sync` sets up everything
3. **Version controlled** - agentctl.json in dotfiles
4. **Consistent mental model** - Same patterns across all resource types
5. **Future-proof** - As tools add features, agentctl can map them

---

## Implementation Phases

### Phase 1: Claude Code Full Support
- [ ] Add `claude` section to agentctl.json schema
- [ ] Implement CLAUDE.md management (instructions)
- [ ] Implement plugins/marketplaces sync
- [ ] Implement permissions sync
- [ ] Implement hooks sync
- [ ] Implement statusLine sync

### Phase 2: Cross-Tool Rules
- [ ] Fix Claude adapter to support `.claude/rules/` directory (currently returns nil)
- [ ] Map `.claude/rules/*.md` ↔ `.cursorrules` ↔ `.windsurfrules`
- [ ] Support YAML frontmatter `paths:` for conditional rules
- [ ] Universal rules format with tool-specific output

**Note on Codex "rules"**: Codex uses Starlark DSL files (`~/.codex/rules/*.rules`) for **command execution permissions**, not coding instructions. These map to Claude Code's `permissions` concept, not to `.claude/rules/`. See [Codex Rules Docs](https://developers.openai.com/codex/rules/).

### Phase 3: Universal Extensions
- [ ] Abstract plugin/extension concept
- [ ] Map equivalent functionality across tools where possible

---

## Implementation Notes

- Plugins/marketplaces would need to shell out to Claude Code CLI or manipulate the JSON files directly
- CLAUDE.md could be symlinked or copied on sync
- Would need to handle the merge of agentctl-managed settings with any local/dynamic settings Claude adds
- Need to audit current tool adapters against latest versions to ensure compatibility
