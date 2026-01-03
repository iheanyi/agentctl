# Sync Configuration Testing System

## Overview

A comprehensive testing system for agentctl's sync adapters that ensures configuration files are correctly generated for all supported tools (Claude, Cursor, OpenCode, Zed, Continue, Cline, Windsurf, Codex, Claude Desktop).

## Goals

1. **Regression Prevention**: Catch format/schema issues before release
2. **Real Tool Validation**: Verify tools actually accept generated configs
3. **Config Preservation**: Ensure user's manual entries and unknown fields survive sync
4. **Cross-Platform Confidence**: Validate behavior across macOS, Linux, Windows

---

## Architecture

### Three-Tier Testing Approach

```
Tier 1: Golden File Tests (Always Run)
├── Fast, deterministic, no external deps
├── Per-adapter golden files
├── Compare output against known-good snapshots
└── Run on every `go test`

Tier 2: Schema Validation (Always Run)
├── Validate JSON structure matches tool's expected format
├── Check required fields, types, nesting
└── Uses JSON Schema where available

Tier 3: Real Tool Validation (Opt-In)
├── Enabled via AGENTCTL_TEST_REAL_TOOLS=1
├── Verify tools can load the generated config
├── Skip tools that aren't installed
└── Primarily for CI environment
```

---

## Test Categories

### 1. Per-Adapter Golden File Tests

Each adapter gets its own golden file directory:

```
pkg/sync/testdata/
├── golden/
│   ├── claude/
│   │   ├── basic.input.json      # Input config (existing state)
│   │   ├── basic.golden.json     # Expected output after sync
│   │   ├── preserve_fields.input.json
│   │   └── preserve_fields.golden.json
│   ├── opencode/
│   │   ├── basic.input.json
│   │   ├── basic.golden.json
│   │   ├── strict_schema.input.json  # Tests $schema, hook, plugin preservation
│   │   └── strict_schema.golden.json
│   ├── cursor/
│   │   ├── basic.input.json
│   │   ├── basic.golden.json
│   │   ├── http_filtered.input.json  # Verifies HTTP servers are filtered
│   │   └── http_filtered.golden.json
│   ├── continue/
│   │   ├── legacy.input.json         # experimental.modelContextProtocolServers
│   │   ├── legacy.golden.json
│   │   ├── modern.input.json         # mcpServers
│   │   └── modern.golden.json
│   └── ... (other adapters)
├── fixtures/
│   ├── servers_minimal.json      # Simple test servers
│   ├── servers_realistic.json    # Real-world servers (context7, sentry, etc.)
│   └── servers_all_transports.json  # stdio, HTTP, SSE servers
└── state/
    └── sync-state.golden.json    # Expected state file output
```

### 2. Config Preservation Tests

Test that unknown/user fields survive sync:

```go
// Test cases (predefined, not fuzzed)
var preservationCases = []struct {
    name   string
    fields map[string]interface{}
}{
    {"schema_field", map[string]interface{}{"$schema": "https://..."}},
    {"hook_config", map[string]interface{}{"hook": map[string]interface{}{"session-start": []string{"cmd"}}}},
    {"plugin_array", map[string]interface{}{"plugin": []string{"plugin1", "plugin2"}}},
    {"nested_custom", map[string]interface{}{"custom": map[string]interface{}{"deeply": map[string]interface{}{"nested": "value"}}}},
    {"numeric_keys", map[string]interface{}{"settings": map[string]interface{}{"timeout": 30}}},
}
```

### 3. Transport Filtering Tests

Verify transport-specific behavior:

```go
func TestCursorFiltersHTTPServers(t *testing.T) {
    servers := []*mcp.Server{
        {Name: "stdio-server", Command: "npx", Args: []string{"server"}},
        {Name: "http-server", URL: "https://mcp.example.com", Transport: mcp.TransportHTTP},
    }

    // Cursor should only include stdio-server
    adapter := &CursorAdapter{}
    // ... sync and verify
}
```

### 4. State File Lifecycle Tests

```go
func TestStateFileLifecycle(t *testing.T) {
    // 1. Initial sync - state file created
    // 2. Add new servers - state updated
    // 3. Remove servers from config - state tracks removal
    // 4. Re-sync - stale entries cleaned up
}
```

### 5. Merge Conflict Tests

```go
func TestMergeConflict_SameNameDifferentConfig(t *testing.T) {
    // Existing: server "foo" with command "old-cmd"
    // Incoming: server "foo" with command "new-cmd"
    // Result: Updated to "new-cmd" (managed entry replaced)
}

func TestMergeConflict_UnmanagedNotOverwritten(t *testing.T) {
    // Existing: unmanaged server "foo"
    // Incoming: server "foo" from agentctl
    // Result: ??? (need to decide behavior - probably warn and skip)
}
```

---

## Test Fixtures

### Minimal Fixtures (Unit Tests)

```json
{
  "servers": [
    {
      "name": "test-server",
      "command": "echo",
      "args": ["hello"]
    }
  ]
}
```

### Realistic Fixtures (Golden Tests)

```json
{
  "servers": [
    {
      "name": "context7",
      "command": "npx",
      "args": ["-y", "@context7/mcp-server"],
      "transport": "stdio"
    },
    {
      "name": "sentry",
      "url": "https://mcp.sentry.io",
      "transport": "http",
      "headers": {"Authorization": "Bearer ${SENTRY_TOKEN}"}
    },
    {
      "name": "filesystem",
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"],
      "env": {"DEBUG": "true"}
    }
  ]
}
```

---

## CLI Commands

### `agentctl validate`

Validates config syntax and schema compliance:

```bash
$ agentctl validate
Validating configurations...

claude:
  ✓ Config syntax valid
  ✓ Schema compliance OK
  ✓ 7 servers configured

cursor:
  ✓ Config syntax valid
  ✓ Schema compliance OK
  ✓ 5 servers configured (2 HTTP servers filtered)

opencode:
  ✓ Config syntax valid
  ✓ Schema compliance OK
  ✓ 7 servers configured

All configurations valid.
```

### `agentctl doctor`

Full health check including tool installation and connectivity:

```bash
$ agentctl doctor
Running health checks...

Tools:
  ✓ claude (v2.0.76) - installed
  ✓ cursor - installed (config valid)
  ✓ opencode (v1.0.217) - installed
  ✗ zed - not installed
  ✓ continue - installed (config valid)

Configurations:
  ✓ claude: ~/.claude/settings.json (valid)
  ✓ cursor: ~/.cursor/mcp.json (valid)
  ✓ opencode: ~/.config/opencode/opencode.json (valid)

MCP Servers:
  ✓ context7 - responding (stdio)
  ✓ sentry - responding (http)
  ✗ filesystem - not responding (connection refused)

State:
  ✓ sync-state.json exists
  ✓ Tracking 7 managed servers across 5 tools

Issues found: 2
  - zed not installed
  - filesystem server not responding
```

---

## Golden File Update Workflow

### Interactive Diff Mode (Default)

```bash
$ go test ./pkg/sync/... -v
--- FAIL: TestClaudeGoldenFiles
    Golden file mismatch for 'basic'

    --- Expected (golden)
    +++ Actual (output)
    @@ -5,7 +5,7 @@
       "mcpServers": {
         "context7": {
           "command": "npx",
    -      "args": ["-y", "@context7/mcp"],
    +      "args": ["-y", "@context7/mcp-server"],
           "_managedBy": "agentctl"
         }
       }

    Accept this change? [y/n/d(iff)/q(uit)]:
```

### Auto-Update Mode

```bash
$ UPDATE_GOLDENS=1 go test ./pkg/sync/...
Updated 3 golden files:
  - pkg/sync/testdata/golden/claude/basic.golden.json
  - pkg/sync/testdata/golden/opencode/basic.golden.json
  - pkg/sync/testdata/golden/cursor/basic.golden.json
```

### TUI Diff Viewer (--interactive)

```bash
$ go test ./pkg/sync/... --interactive
# Opens TUI with side-by-side diff, keyboard navigation
# [a]ccept [r]eject [n]ext [p]rev [q]uit
```

---

## Implementation Plan

### Phase 1: Test Infrastructure
- [ ] Create `pkg/sync/testdata/` directory structure
- [ ] Implement golden file comparison utilities
- [ ] Add `TestMain` with update flag handling
- [ ] Create minimal and realistic fixture files

### Phase 2: Per-Adapter Golden Tests
- [ ] Claude adapter golden tests
- [ ] OpenCode adapter golden tests (with strict schema cases)
- [ ] Cursor adapter golden tests (with HTTP filtering)
- [ ] Continue adapter golden tests (legacy + modern)
- [ ] Remaining adapters (Zed, Cline, Windsurf, Codex, Claude Desktop)

### Phase 3: Edge Case Tests
- [ ] Config preservation tests (predefined risky patterns)
- [ ] Transport filtering tests
- [ ] Merge conflict tests
- [ ] State file lifecycle tests

### Phase 4: CLI Commands
- [ ] Implement `agentctl validate` command
- [ ] Implement `agentctl doctor` command
- [ ] Add tool version detection
- [ ] Add MCP server connectivity checks

### Phase 5: Interactive Tooling
- [ ] Implement interactive diff mode for golden updates
- [ ] Add TUI diff viewer (bubbletea-based)
- [ ] Add `--interactive` flag support

### Phase 6: CI Integration
- [ ] Add `AGENTCTL_TEST_REAL_TOOLS` env var support
- [ ] Create CI workflow that installs tools
- [ ] Add test coverage reporting

---

## Test Execution

### Local Development (Fast)
```bash
# Run all sync tests (golden files only)
go test ./pkg/sync/...

# Run with verbose output
go test ./pkg/sync/... -v

# Update golden files interactively
go test ./pkg/sync/... -v  # then accept changes

# Auto-update all golden files
UPDATE_GOLDENS=1 go test ./pkg/sync/...
```

### CI (Full Validation)
```bash
# Enable real tool validation
AGENTCTL_TEST_REAL_TOOLS=1 go test ./pkg/sync/...
```

### User Validation
```bash
# Check config syntax
agentctl validate

# Full health check
agentctl doctor

# Verbose health check
agentctl doctor -v
```

---

## Success Criteria

1. **All adapters have golden file coverage** for basic sync operations
2. **Config preservation tests pass** for all known risky patterns
3. **Transport filtering verified** for Cursor (and any future filtered adapters)
4. **State file lifecycle tested** including creation, update, and cleanup
5. **CLI commands work** - `validate` and `doctor` provide useful output
6. **CI runs full validation** with real tools installed
7. **Interactive golden updates** work smoothly for developers
