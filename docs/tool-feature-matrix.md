# Tool Feature Matrix

Comprehensive comparison of configuration features across all supported AI coding tools.

## Feature Support Matrix

| Feature | Claude Code | Codex CLI | OpenCode | Cursor | Copilot CLI |
|---------|-------------|-----------|----------|--------|-------------|
| **MCP Servers** | `mcpServers` | `[mcp_servers]` (TOML) | `mcp` (different schema!) | `mcpServers` | `servers` |
| **Instructions/Rules** | `CLAUDE.md` + `.claude/rules/` | `AGENTS.md` | `AGENTS.md`/`CLAUDE.md` | `.cursorrules` + `.cursor/rules/` | `.github/copilot-instructions.md` |
| **Commands** | `.claude/commands/` | `~/.codex/prompts/` | `~/.config/opencode/command/` | `.cursor/commands/` | - |
| **Skills** | `.claude/skills/` | `~/.codex/skills/` | `~/.config/opencode/skill/` | - | `.github/skills/` |
| **Agents/Subagents** | `.claude/agents/` | - | `~/.config/opencode/agent/` | - | `.github/agents/` |
| **Hooks** | `hooks` in settings.json | `[hooks]` in config.toml | Plugins (JS/TS) | Enterprise only | `.github/hooks/hooks.json` |
| **Plugins** | `~/.claude/plugins/` | - | `~/.config/opencode/plugin/` | VS Code extensions | - |
| **Permissions** | `permissions` in settings.json | `~/.codex/rules/*.rules` (Starlark) | `permission` in config | - | `trusted_folders`, `allowed_urls` |
| **Themes** | - | - | `~/.config/opencode/themes/` | VS Code themes | - |
| **Output Styles** | `.claude/output-styles/` | - | - | - | - |
| **LSP Servers** | `.claude/lspServers/` | - | Built-in (30+) | VS Code LSP | - |
| **Formatters** | - | - | Built-in (20+) | VS Code formatters | - |
| **Ignore Files** | - | - | `watcher.ignore` | `.cursorignore` | - |

## Config File Locations

### Claude Code
```
~/.claude/
├── settings.json          # Main config (mcpServers, permissions, hooks, plugins, statusLine)
├── CLAUDE.md              # Global instructions
├── rules/                 # Scoped rules with paths: frontmatter
├── commands/              # Slash commands (.md)
├── skills/                # Skills (SKILL.md in subdirs)
├── agents/                # Subagents (.md)
├── output-styles/         # Output personalities (.md)
├── plugins/
│   ├── installed_plugins.json
│   ├── known_marketplaces.json
│   └── config.json
└── lspServers/            # Language server configs
```

### Codex CLI
```
~/.codex/
├── config.toml            # Main config (TOML format!)
├── config.json            # Legacy MCP config
├── AGENTS.md              # Global instructions
├── prompts/               # Custom prompts/commands (.md)
├── skills/                # Skills (SKILL.md in subdirs)
├── rules/                 # Starlark execution rules (.rules)
├── hooks/                 # Notification hooks (.sh)
└── sessions/              # Session logs (.jsonl)
```

### OpenCode
```
~/.config/opencode/
├── opencode.json          # Main config (mcp key, not mcpServers!)
├── AGENTS.md              # Global instructions
├── agent/                 # Agent definitions (.md)
├── command/               # Commands (.md)
├── skill/                 # Skills (SKILL.md in subdirs)
├── plugin/                # JS/TS plugins
├── themes/                # Custom themes (.json)
├── tools/                 # Custom tools (.ts)
└── dcp.jsonc              # Context pruning config
```

### Cursor
```
~/.cursor/
├── mcp.json               # MCP servers
└── (settings via VS Code mechanism)

Project:
├── .cursor/
│   ├── mcp.json           # Project MCP servers
│   ├── rules/             # Rules (.mdc files with frontmatter)
│   └── commands/          # Commands (.md)
├── .cursorrules           # Legacy rules (deprecated)
└── .cursorignore          # Ignore patterns
```

### Copilot CLI
```
~/.copilot/
├── config.json            # Main config (trusted_folders, allowed_urls)
├── mcp-config.json        # MCP servers
└── skills/                # Personal skills

Project (.github/):
├── copilot-instructions.md    # Project instructions
├── instructions/              # Path-specific instructions (.instructions.md)
├── agents/                    # Custom agents (.md)
├── skills/                    # Project skills (SKILL.md in subdirs)
└── hooks/
    └── hooks.json             # Lifecycle hooks
```

## Key Schema Differences

### MCP Servers

**Claude Code / Cursor / Copilot:**
```json
{
  "mcpServers": {
    "name": { "command": "...", "args": [...] }
  }
}
```

**Codex CLI (TOML):**
```toml
[mcp_servers.name]
command = "..."
args = [...]
```

**OpenCode:**
```json
{
  "mcp": {
    "name": {
      "type": "local",
      "command": ["cmd", "arg1", "arg2"]
    }
  }
}
```
Note: OpenCode uses `"command": [array]` not `"command": "string"` + `"args": [...]`

### Instructions/Rules

**Claude Code:** Markdown + YAML frontmatter with `paths:` for scoping
```yaml
---
paths:
  - "src/api/**/*.ts"
---
# Instructions here
```

**Codex CLI:** Plain markdown (`AGENTS.md`)

**OpenCode:** Plain markdown (`AGENTS.md` or `CLAUDE.md`)

**Cursor:** `.mdc` files with `globs:` and `alwaysApply:` frontmatter
```yaml
---
description: Rule description
globs: ["*.py", "src/**/*.js"]
alwaysApply: false
---
```

**Copilot CLI:** Markdown with `applyTo:` frontmatter
```yaml
---
applyTo: "app/models/**/*.rb,src/**/*.ts"
---
```

### Permissions

**Claude Code:** Tool-based allow/deny
```json
{
  "permissions": {
    "allow": ["Bash(git:*)", "Skill(dev-browser:*)"],
    "deny": ["Bash(rm -rf:*)"]
  }
}
```

**Codex CLI:** Starlark DSL for command execution
```starlark
prefix_rule(
    pattern = ["git", "push"],
    decision = "prompt",
    justification = "Pushing requires approval"
)
```

**OpenCode:** Tool-based with glob patterns
```json
{
  "permission": {
    "*": "ask",
    "bash": "allow",
    "edit": { "git *": "allow", "rm *": "deny" }
  }
}
```

**Copilot CLI:** Path and URL allowlists
```json
{
  "trusted_folders": ["/path/to/project"],
  "allowed_urls": ["*.github.com"],
  "denied_urls": ["*.malware.com"]
}
```

### Skills Format

All tools that support skills use similar format:
```
skill-name/
└── SKILL.md
```

With YAML frontmatter:
```yaml
---
name: skill-name
description: What the skill does
---
# Skill instructions
```

### Hooks

**Claude Code:** JSON in settings.json
```json
{
  "hooks": {
    "SessionStart": [{ "type": "command", "command": "..." }],
    "PreToolUse": [{ "matcher": "Bash", "hooks": [...] }]
  }
}
```

**Codex CLI:** TOML
```toml
[hooks]
agent-turn-complete = ["/path/to/script.sh"]
```

**OpenCode:** JavaScript/TypeScript plugins with event handlers

**Copilot CLI:** JSON
```json
{
  "hooks": {
    "sessionStart": [{ "type": "command", "bash": "..." }]
  }
}
```

## agentctl Support Status

| Feature | Current Status | Priority |
|---------|----------------|----------|
| MCP Servers | Supported (all tools) | - |
| Claude rules (`.claude/rules/`) | **NOT SUPPORTED** (returns nil) | High |
| Cursor rules (`.cursor/rules/`) | Partial (legacy .cursorrules only) | High |
| Commands | Not supported | Medium |
| Skills | Not supported | Medium |
| Agents | Not supported | Medium |
| Hooks | Not supported | Medium |
| Plugins | Not supported | High |
| Permissions | Not supported | Medium |
| OpenCode MCP schema | **DIFFERENT SCHEMA** - needs audit | High |

## Sources

- [Claude Code Settings](https://code.claude.com/docs/en/settings)
- [Claude Code Plugins](https://code.claude.com/docs/en/plugins-reference)
- [Codex CLI Config Reference](https://developers.openai.com/codex/config-reference/)
- [Codex CLI Skills](https://developers.openai.com/codex/skills/)
- [Codex CLI Rules](https://developers.openai.com/codex/rules/)
- [OpenCode Documentation](https://opencode.ai/docs/)
- [Cursor Rules](https://cursor.com/docs/context/rules)
- [Copilot CLI](https://docs.github.com/en/copilot/how-tos/use-copilot-agents/use-copilot-cli)
- [Copilot Custom Agents](https://docs.github.com/en/copilot/how-tos/use-copilot-agents/coding-agent/create-custom-agents)
