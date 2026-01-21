---
status: pending
priority: p3
issue_id: "005"
tags: [code-review, architecture, duplication]
dependencies: []
---

# Config Path Duplication Between Discovery and Sync

## Problem Statement

Sync adapters and discovery scanners both define paths to tool config directories independently. A tool path change requires updates in two places, increasing maintenance burden and risk of inconsistency.

## Findings

**Locations:**

Discovery (claude.go):
```go
func (s *ClaudeScanner) ScanAgents(dir string) ([]*agent.Agent, error) {
    agentsDir := filepath.Join(dir, ".claude", "agents")  // Hardcoded path
```

Sync (claude.go):
```go
func (a *ClaudeAdapter) agentsDir() string {
    return filepath.Join(a.configDir(), "agents")  // Duplicate path logic
}
```

**Impact:**
- Path changes require 2 updates
- Risk of divergence between read (discovery) and write (sync) paths
- Same pattern affects all tools: Cursor, OpenCode, Copilot, etc.

## Proposed Solutions

### Solution 1: Shared tool metadata registry (Recommended)
**Pros:** Single source of truth, both packages consume it
**Cons:** New package needed
**Effort:** Medium (3 hours)
**Risk:** Low

```go
// pkg/tools/metadata.go
type ToolMetadata struct {
    Name            string
    LocalConfigDir  string   // e.g., ".claude"
    GlobalConfigDir string   // e.g., "~/.claude"
    AgentsDir       string   // e.g., "agents"
    RulesDir        string   // e.g., "rules"
    CommandsDir     string
    SkillsDir       string
    ConfigFileName  string   // e.g., "settings.json"
}

var Tools = map[string]ToolMetadata{
    "claude": {
        Name:            "claude",
        LocalConfigDir:  ".claude",
        GlobalConfigDir: "~/.claude",
        AgentsDir:       "agents",
        RulesDir:        "rules",
        CommandsDir:     "commands",
        SkillsDir:       "skills",
        ConfigFileName:  "settings.json",
    },
    // ... other tools
}
```

### Solution 2: Discovery package owns paths, sync imports
**Pros:** Less new code
**Cons:** Couples sync to discovery
**Effort:** Small
**Risk:** Medium (coupling)

## Recommended Action

Implement Solution 1 for future maintainability, but this is P3 and can be deferred.

## Technical Details

**Would affect:**
- `pkg/discovery/claude.go`
- `pkg/discovery/gemini.go`
- `pkg/discovery/directory_scanner.go`
- `pkg/discovery/scanners.go`
- `pkg/sync/claude.go`
- `pkg/sync/cursor.go`
- `pkg/sync/copilot.go`
- `pkg/sync/opencode.go`

## Acceptance Criteria

- [ ] Tool paths defined in single location
- [ ] Both discovery and sync use shared metadata
- [ ] Adding new tool requires single config entry
- [ ] All existing tests pass

## Work Log

| Date | Action | Notes |
|------|--------|-------|
| 2026-01-21 | Created | Found by architecture-strategist review agent |

## Resources

- PR: feat/native-discovery-tui-enhancements
