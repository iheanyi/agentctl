# Native Resource Discovery Design

## Overview

Add automatic discovery of native tool configurations (`.claude/`, `.cursor/`, `.codex/`, etc.) so `agentctl list` shows all resources in a project without requiring explicit setup.

## Problem

When running `agentctl list` in a repo with existing `.claude/rules/` or `.codex/skills/`, nothing appears because agentctl only looks at its own `.agentctl/` directory. Users must explicitly import resources before agentctl recognizes them.

## Solution

Add a discovery layer that scans native tool directories and presents them alongside agentctl-managed resources with clear source attribution.

## Native Directories to Scan

| Tool | Project Directories | Resource Types |
|------|---------------------|----------------|
| **Claude Code** | `.claude/rules/`, `.claude/skills/`, `.claude/commands/`, `.claude/prompts/`, `.claude/settings.json` | rules, skills, commands, prompts, hooks, plugins |
| **Cursor** | `.cursor/rules/`, `.cursorrules` | rules |
| **OpenCode** | `.opencode/rules/`, `.opencode/commands/`, `.opencode/skills/` | rules, commands, skills |
| **Copilot CLI** | `.github-copilot/commands/`, `.github-copilot/skills/` | commands, skills |
| **Codex** | `.codex/skills/`, `.codex/prompts/`, `.codex/rules/` (exec policies), `AGENTS.md` | skills, prompts, exec-rules, instructions |
| **Top-level** | `skills/`, `CLAUDE.md`, `AGENTS.md`, `.cursorrules` | skills, rules/instructions |

## Architecture

### New Package: `pkg/discovery/`

```go
// pkg/discovery/discovery.go

type NativeResource struct {
    Type     ResourceType  // rule, skill, command, prompt, etc.
    Name     string
    Source   string        // "claude", "cursor", "codex", etc.
    Path     string        // Full path to the resource
    Format   string        // "md", "mdc", "yaml", etc.
}

type Scanner interface {
    Name() string
    Scan(projectDir string) ([]NativeResource, error)
}

func ScanProject(projectDir string) ([]NativeResource, error)
```

### Scanners

- `ClaudeScanner` - `.claude/rules/`, `.claude/skills/`, `.claude/commands/`, `.claude/prompts/`
- `CursorScanner` - `.cursor/rules/`, `.cursorrules`
- `CodexScanner` - `.codex/skills/`, `.codex/prompts/`, `AGENTS.md`
- `OpenCodeScanner` - `.opencode/` directories
- `CopilotScanner` - `.github-copilot/` directories
- `TopLevelScanner` - `skills/`, `CLAUDE.md`, `AGENTS.md`

## CLI Integration

### `agentctl list`

Shows native resources with source markers:

```
Rules:
  NAME           SOURCE      PATH
  development    [claude]    .claude/rules/development.md
  testing        [claude]    .claude/rules/testing.md
  cursor-rules   [cursor]    .cursor/rules/main.mdc
  my-rule        [agentctl]  .agentctl/rules/my-rule.md

Skills:
  NAME           SOURCE      PATH
  tk-context     [native]    skills/tk-context/
  my-skill       [agentctl]  .agentctl/skills/my-skill/
```

### `agentctl init`

Prompts to import when native resources are found:

```
Found 5 existing resources:
  - 2 rules from .claude/
  - 3 skills from skills/

Import these into agentctl? [Y/n]
```

### `agentctl import`

New flags for native import:

```bash
agentctl import --all              # Import everything found
agentctl import --from claude      # Import from specific tool
agentctl import --from codex
agentctl import --rules            # Import specific resource types
agentctl import --skills
agentctl import --all --dry-run    # Preview what would be imported
```

## Import Behavior

**Copy (not move):** Native files stay in place, agentctl creates copies in `.agentctl/`. Users can delete originals manually if desired.

```
.claude/rules/development.md   →   .agentctl/rules/development.md
.claude/skills/my-skill/       →   .agentctl/skills/my-skill/
skills/tk-context/             →   .agentctl/skills/tk-context/
```

## JSON Output

```json
{
  "rules": [
    {"name": "development", "source": "claude", "path": ".claude/rules/development.md", "managed": false},
    {"name": "my-rule", "source": "agentctl", "path": ".agentctl/rules/my-rule.md", "managed": true}
  ],
  "skills": [
    {"name": "tk-context", "source": "native", "path": "skills/tk-context/", "managed": false}
  ]
}
```

## Files to Create

```
pkg/discovery/
├── discovery.go       # Scanner interface, ScanProject(), NativeResource type
├── discovery_test.go  # Tests
├── claude.go          # ClaudeScanner
├── cursor.go          # CursorScanner
├── codex.go           # CodexScanner
├── opencode.go        # OpenCodeScanner
├── copilot.go         # CopilotScanner
└── toplevel.go        # TopLevelScanner
```

## Files to Modify

```
internal/cli/list.go   # Add native resource display
internal/cli/init.go   # Add import prompt on init
internal/cli/import.go # Add --all, --from flags
internal/tui/tui.go    # Show native resources in list views
```

## Behavior Summary

| Command | Behavior |
|---------|----------|
| `agentctl list` | Shows native resources with `[claude]`, `[cursor]` markers |
| `agentctl init` | Prompts "Found X resources, import?" |
| `agentctl import --all` | Copies all native resources to `.agentctl/` |
| `agentctl import --from claude` | Copies only Claude resources |
| `agentctl sync` | Only syncs agentctl-managed resources (unchanged) |
