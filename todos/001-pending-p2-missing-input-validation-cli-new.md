---
status: pending
priority: p2
issue_id: "001"
tags: [code-review, security, validation]
dependencies: []
---

# Missing Input Validation in CLI `new` Commands

## Problem Statement

The `agentctl new` command accepts user-provided names directly without validation through `SanitizeName()`. While interactive forms have basic validation, the non-interactive paths don't validate names for malicious characters before using `filepath.Join()`.

This creates potential path traversal vulnerabilities where a malicious user could create files outside the intended directory.

## Findings

**Location**: `/Users/iheanyi/development/mcp-pkg/internal/cli/new.go`

**Vulnerable functions:**
- `runNewCommand` (line ~122) - uses `args[0]` directly
- `runNewRule` (line ~179)
- `runNewPrompt` (line ~241)
- `runNewSkill` (line ~288)
- `runNewAgent` (line ~364)

**Example vulnerable pattern:**
```go
func runNewCommand(cmd *cobra.Command, args []string) error {
    name := args[0]  // User-provided name - NO VALIDATION
    // ...
    commandPath := filepath.Join(commandsDir, name+".json")  // Direct use
```

**Attack vector:** Names like `../../../etc/passwd` could potentially create files outside intended directories.

## Proposed Solutions

### Solution 1: Add SanitizeName() validation (Recommended)
**Pros:** Consistent with sync adapters, reuses existing security function
**Cons:** Small code change in 5 places
**Effort:** Small (1 hour)
**Risk:** Low

Add at the start of each `runNew*` function:
```go
func runNewCommand(cmd *cobra.Command, args []string) error {
    name := args[0]
    if err := pathutil.SanitizeName(name); err != nil {
        return fmt.Errorf("invalid command name: %w", err)
    }
    // ... rest of function
}
```

### Solution 2: Centralized validation helper
**Pros:** Single validation point
**Cons:** Over-engineering for 5 call sites
**Effort:** Medium
**Risk:** Low

## Recommended Action

Implement Solution 1 - add `pathutil.SanitizeName()` validation to all 5 `runNew*` functions.

## Technical Details

**Affected files:**
- `internal/cli/new.go`

**Related code:**
- `pkg/pathutil/sanitize.go` - existing `SanitizeName()` function
- `pkg/sync/helpers.go` - uses `SanitizeName()` correctly

## Acceptance Criteria

- [ ] All `runNew*` functions validate names via `SanitizeName()`
- [ ] Tests verify invalid names are rejected
- [ ] Names with `..`, `/`, `\` are rejected with clear error messages

## Work Log

| Date | Action | Notes |
|------|--------|-------|
| 2026-01-21 | Created | Found by security-sentinel review agent |

## Resources

- PR: feat/native-discovery-tui-enhancements
- Similar pattern: `pkg/sync/helpers.go:314-316`
