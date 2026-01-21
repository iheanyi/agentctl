---
status: pending
priority: p2
issue_id: "002"
tags: [code-review, performance, duplication]
dependencies: []
---

# Repeated Discovery Calls in list.go

## Problem Statement

The `runList` function calls `discovery.DiscoverBoth(cwd)` up to 4 times when `--native` flag is set. Each call triggers 40-60 `os.Stat()` operations, resulting in ~240 redundant filesystem calls for a single `list --native` command.

## Findings

**Location**: `/Users/iheanyi/development/mcp-pkg/internal/cli/list.go`

**Redundant calls:**
- Line 133: First call for commands filtering
- Line 190: Second call for rules filtering
- Line 236: Third call for skills filtering
- Line 385: Fourth call in JSON mode

**Current pattern (repeated 4x):**
```go
if listNative {
    cwd, _ := os.Getwd()
    for _, res := range discovery.DiscoverBoth(cwd) {
        if res.Type == "command" {
            // ...
        }
    }
}
```

**Impact:**
- ~240 stat calls per `list --native` invocation
- Linear degradation with more tools/resources
- Noticeable delay in large projects

## Proposed Solutions

### Solution 1: Single discovery call with filtering (Recommended)
**Pros:** 75% reduction in discovery calls, simple refactor
**Cons:** Slightly more memory for cached results
**Effort:** Small (1 hour)
**Risk:** Low

```go
func runList(cmd *cobra.Command, args []string) error {
    // Single discovery call, reused throughout
    var nativeResources []*discovery.NativeResource
    if listNative {
        cwd, _ := os.Getwd()
        nativeResources = discovery.DiscoverBoth(cwd)
    }

    // Filter by type when needed
    nativeCommands := filterByType(nativeResources, "command", scope)
    nativeRules := filterByType(nativeResources, "rule", scope)
    // ...
}

func filterByType(resources []*discovery.NativeResource, resType string, scope config.Scope) []*discovery.NativeResource {
    var filtered []*discovery.NativeResource
    for _, res := range resources {
        if res.Type == resType && (scope == config.ScopeAll || string(scope) == res.Scope) {
            filtered = append(filtered, res)
        }
    }
    return filtered
}
```

### Solution 2: Discovery caching layer
**Pros:** Benefits all callers, not just list
**Cons:** More complex, needs cache invalidation
**Effort:** Medium (3 hours)
**Risk:** Medium

## Recommended Action

Implement Solution 1 - single discovery call with filtering helper.

## Technical Details

**Affected files:**
- `internal/cli/list.go`

**Also fix:**
- Repeated `os.Getwd()` calls (lines 132, 189, 234, 297, 327, 384)

## Acceptance Criteria

- [ ] `discovery.DiscoverBoth()` called at most once per `list` invocation
- [ ] `os.Getwd()` called at most once
- [ ] Benchmark shows 50%+ reduction in execution time
- [ ] All existing list functionality preserved

## Work Log

| Date | Action | Notes |
|------|--------|-------|
| 2026-01-21 | Created | Found by performance-oracle review agent |

## Resources

- PR: feat/native-discovery-tui-enhancements
