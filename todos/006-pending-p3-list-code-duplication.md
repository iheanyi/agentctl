---
status: pending
priority: p3
issue_id: "006"
tags: [code-review, duplication, simplification]
dependencies: ["002"]
---

# Massive Code Duplication in list.go

## Problem Statement

The `runList` and `runListJSON` functions contain nearly identical logic repeated 6 times for each resource type (servers, commands, rules, skills, plugins, agents). This results in ~400 lines of duplicated code.

## Findings

**Location**: `/Users/iheanyi/development/mcp-pkg/internal/cli/list.go`

**Duplicate pattern (repeated 6x for each resource type):**
```go
if listType == "" || listType == "commands" {
    commands := cfg.CommandsForScope(scope)
    var nativeCommands []*discovery.NativeResource
    if listNative {
        // ... discovery logic
    }
    if len(commands) > 0 || len(nativeCommands) > 0 {
        fmt.Println("Commands:")
        w := tabwriter.NewWriter(...)
        // ... table formatting
    }
}
```

**Lines of duplication:**
- Text output: lines 99-371 (~270 lines)
- JSON output: lines 374-593 (~220 lines)
- Total: ~490 lines that could be ~100 lines

## Proposed Solutions

### Solution 1: Table-driven rendering (Recommended)
**Pros:** 80% code reduction, easier to add new resource types
**Cons:** Requires refactoring
**Effort:** Medium (2 hours)
**Risk:** Low

```go
type listSection struct {
    name      string
    typeKey   string
    getItems  func(cfg *config.Config, scope config.Scope) []listItem
    getNative func(resources []*discovery.NativeResource, scope config.Scope) []listItem
}

var sections = []listSection{
    {
        name:      "Commands",
        typeKey:   "commands",
        getItems:  func(cfg, scope) []listItem { return commandsToItems(cfg.CommandsForScope(scope)) },
        getNative: func(res, scope) []listItem { return nativeToItems(res, "command", scope) },
    },
    // ... other resource types
}

func runList(cmd *cobra.Command, args []string) error {
    // ... setup
    for _, sec := range sections {
        if listType == "" || listType == sec.typeKey {
            items := sec.getItems(cfg, scope)
            if listNative {
                items = append(items, sec.getNative(nativeResources, scope)...)
            }
            if len(items) > 0 {
                renderTable(sec.name, items)
            }
        }
    }
}
```

## Recommended Action

Implement Solution 1 after fixing #002 (repeated discovery calls).

## Technical Details

**Affected files:**
- `internal/cli/list.go`

**Helper types needed:**
```go
type listItem struct {
    Name        string
    Scope       string
    Source      string
    Description string
    Extra       map[string]string
}
```

## Acceptance Criteria

- [ ] Single loop handles all resource types
- [ ] Adding new resource type requires only new section entry
- [ ] Text and JSON output both use table-driven approach
- [ ] All existing list functionality preserved
- [ ] 50%+ reduction in lines of code

## Work Log

| Date | Action | Notes |
|------|--------|-------|
| 2026-01-21 | Created | Found by code-simplicity-reviewer agent |

## Resources

- PR: feat/native-discovery-tui-enhancements
- Depends on: #002 (single discovery call)
