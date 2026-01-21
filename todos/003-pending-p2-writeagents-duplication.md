---
status: pending
priority: p2
issue_id: "003"
tags: [code-review, duplication, refactor]
dependencies: []
---

# WriteAgents Duplication Across Adapters

## Problem Statement

Three adapters (Claude, Cursor, OpenCode) have nearly identical `WriteAgents` implementations. This violates DRY and makes maintenance harder - any fix needs to be applied in 3 places.

## Findings

**Locations:**
- `/Users/iheanyi/development/mcp-pkg/pkg/sync/claude.go` (lines 612-633)
- `/Users/iheanyi/development/mcp-pkg/pkg/sync/cursor.go` (lines 359-379)
- `/Users/iheanyi/development/mcp-pkg/pkg/sync/opencode.go` (lines 390-410)

**Duplicate pattern:**
```go
func (a *XxxAdapter) WriteAgents(agents []*agent.Agent) error {
    agentsDir := a.agentsDir()
    if err := os.MkdirAll(agentsDir, 0755); err != nil { return err }
    for _, ag := range agents {
        if err := SanitizeName(ag.Name); err != nil {
            return fmt.Errorf("invalid agent name: %w", err)
        }
        if err := ag.Save(agentsDir); err != nil { return err }
    }
    return nil
}
```

**Exception:** Copilot adapter (line 266) correctly differs by using `.agent.md` extension.

## Proposed Solutions

### Solution 1: Add WriteAgentsToDir helper (Recommended)
**Pros:** Follows existing helpers.go pattern
**Cons:** Minor refactor
**Effort:** Small (30 min)
**Risk:** Low

```go
// pkg/sync/helpers.go
func WriteAgentsToDir(agentsDir string, agents []*agent.Agent) error {
    if err := os.MkdirAll(agentsDir, 0755); err != nil {
        return err
    }
    for _, ag := range agents {
        if err := SanitizeName(ag.Name); err != nil {
            return fmt.Errorf("invalid agent name: %w", err)
        }
        if err := ag.Save(agentsDir); err != nil {
            return err
        }
    }
    return nil
}

// In adapters:
func (a *ClaudeAdapter) WriteAgents(agents []*agent.Agent) error {
    return WriteAgentsToDir(a.agentsDir(), agents)
}
```

### Solution 2: Extension callback for Copilot
**Pros:** Handles Copilot's `.agent.md` edge case
**Cons:** Slightly more complex
**Effort:** Small
**Risk:** Low

```go
func WriteAgentsToDir(agentsDir string, agents []*agent.Agent, saveFunc func(*agent.Agent, string) error) error
```

## Recommended Action

Implement Solution 1 with Solution 2's flexibility for Copilot.

## Technical Details

**Affected files:**
- `pkg/sync/helpers.go` (add helper)
- `pkg/sync/claude.go` (simplify)
- `pkg/sync/cursor.go` (simplify)
- `pkg/sync/opencode.go` (simplify)

## Acceptance Criteria

- [ ] `WriteAgentsToDir` helper added to helpers.go
- [ ] Claude, Cursor, OpenCode adapters use helper
- [ ] Copilot adapter remains unchanged (different extension)
- [ ] All adapter tests pass

## Work Log

| Date | Action | Notes |
|------|--------|-------|
| 2026-01-21 | Created | Found by pattern-recognition-specialist review agent |

## Resources

- PR: feat/native-discovery-tui-enhancements
- Similar pattern: `WriteCommandsToDir` in helpers.go
