---
status: pending
priority: p3
issue_id: "004"
tags: [code-review, agent-native, cli]
dependencies: []
---

# Missing JSON Output for Mutating CLI Commands

## Problem Statement

The `new`, `copy`, and `import` commands lack JSON output support, making it difficult for agents to programmatically verify what was created or get paths to new resources.

## Findings

**Location**: `/Users/iheanyi/development/mcp-pkg/internal/cli/`

**Affected commands:**
- `new command/rule/skill/prompt/agent` - no JSON output
- `copy command/rule/skill` - no JSON output
- `import` - no JSON output
- `skill show <name>` - no JSON output

**Current pattern (new.go):**
```go
func runNewAgent(cmd *cobra.Command, args []string) error {
    // ... creates agent ...
    out.Success("Created agent %q%s", name, scopeLabel)  // Text only
    return nil
}
```

**Impact:** Agents cannot programmatically verify success or get paths to created resources.

## Proposed Solutions

### Solution 1: Add JSONOutput check to mutating commands (Recommended)
**Pros:** Follows existing pattern from list.go
**Cons:** Requires changes in multiple files
**Effort:** Medium (2 hours)
**Risk:** Low

```go
func runNewAgent(cmd *cobra.Command, args []string) error {
    // ... creates agent ...

    if JSONOutput {
        jw := output.NewJSONWriter()
        return jw.WriteSuccess(output.NewResourceResult{
            Type:  "agent",
            Name:  name,
            Scope: string(scope),
            Path:  agentPath,
        })
    }

    out.Success("Created agent %q%s", name, scopeLabel)
    return nil
}
```

## Recommended Action

Implement Solution 1 for all mutating commands.

## Technical Details

**Affected files:**
- `internal/cli/new.go` (all runNew* functions)
- `internal/cli/copy.go` (all runCopy* functions)
- `internal/cli/import.go` (runImport, runImportAll)
- `internal/cli/skill.go` (runSkillShow)

**New types needed in `pkg/output/json.go`:**
```go
type NewResourceResult struct {
    Type  string `json:"type"`
    Name  string `json:"name"`
    Scope string `json:"scope"`
    Path  string `json:"path"`
}

type CopyResult struct {
    Type       string `json:"type"`
    Name       string `json:"name"`
    FromScope  string `json:"fromScope"`
    ToScope    string `json:"toScope"`
    TargetPath string `json:"targetPath"`
}
```

## Acceptance Criteria

- [ ] `new` commands return JSON with type, name, scope, path
- [ ] `copy` commands return JSON with source and target info
- [ ] `import` command returns JSON listing imported resources
- [ ] `skill show` returns JSON with skill details
- [ ] Text output unchanged when `--json` not specified

## Work Log

| Date | Action | Notes |
|------|--------|-------|
| 2026-01-21 | Created | Found by agent-native-reviewer agent |

## Resources

- PR: feat/native-discovery-tui-enhancements
- Pattern reference: `internal/cli/list.go:374` (runListJSON)
