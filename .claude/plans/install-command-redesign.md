# Install Command Redesign Spec

## Status: IMPLEMENTED

## Problem Statement

The original `agentctl install` command was over-engineered. MCP server configurations are fundamentally simple:

**stdio transport:**
```json
{
  "command": "npx",
  "args": ["playwriter@latest"]
}
```

**http/sse transport:**
```json
{
  "url": "https://mcp.figma.com/mcp",
  "type": "http"
}
```

The experience should reflect this simplicity.

## Implementation Summary

We renamed `install` to `add` (with `install` kept as alias for backwards compatibility) and made the command:

1. **Transparent** - Always shows the exact config that will be added
2. **Simple** - Supports explicit `--command`/`--url` flags for direct config
3. **Flexible** - Works with registry aliases, URLs, git repos, or explicit config

## Final Command Design

### Primary Command: `agentctl add`

```bash
# Add from registry
agentctl add figma
agentctl add filesystem

# Add with explicit URL (http or sse transport)
agentctl add figma --url https://mcp.figma.com/mcp
agentctl add my-api --url https://api.example.com/mcp/sse --type sse

# Add with explicit command (stdio transport)
agentctl add playwright --command npx --args "playwriter@latest"
agentctl add fs --command npx --args "-y,@modelcontextprotocol/server-filesystem"

# Add from git URL
agentctl add github.com/org/mcp-server

# Preview without adding
agentctl add figma --dry-run
```

### Flags

```
--command       Command to run (e.g., npx, uvx)
--args          Command arguments (comma-separated)
--url           Remote MCP URL
--type          Transport type: stdio, http, or sse
--header, -H    HTTP headers for remote servers (Key: Value)
--no-sync       Don't sync to tools after adding
--target        Sync to specific tool only
--dry-run       Preview config without adding
--local         Force local variant (npx/uvx)
--remote        Force remote variant
--namespace     Namespace prefix for tool names
```

## Output Format

Always shows the resulting config in clean, copyable JSON:

```
Config to be added:
{
  "figma": {
    "url": "https://mcp.figma.com/mcp",
    "type": "http"
  }
}

✓ Added figma

Syncing to tools...
  + claude (~/.claude/mcp_servers.json)
  + claude-desktop (~/Library/.../claude_desktop_config.json)
  - cursor (no HTTP/SSE support)

✓ Synced to 2 tool(s)
```

## Files Changed

- `internal/cli/install.go` - Renamed command, added explicit config flags, simplified output
- `pkg/mcp/server.go` - Added Headers field for http/sse transport
- `README.md` - Updated documentation

## Backwards Compatibility

- `agentctl install` works as alias for `agentctl add`
- Existing config files unchanged
- Registry resolution still works

## Success Metrics (Achieved)

1. ✓ User can add any MCP with a single command
2. ✓ User always sees exactly what config will be added
3. ✓ Config structure in output matches MCP format
4. ✓ `add` is opposite of `remove` (intuitive)
