# TUI Redesign: Lazy.nvim-Style Interface

## Overview

Redesign `agentctl ui` to provide a Lazy.nvim-inspired experience for managing MCP servers. The new interface will be a unified, keyboard-driven dashboard with real-time status, inline operations, and a persistent log panel.

## Layout

```
┌─────────────────────────────────────────────────────────────────────┐
│  agentctl                    default (5 servers)    5 ● 2 ○ 1 ◌    │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ● filesystem          stdio · npx @modelcontextprotocol/server... │
│  ● playwright          stdio · npx playwriter@latest               │
│  ● figma               http  · https://mcp.figma.com/mcp       ✓   │
│  ◌ github              stdio · npx @modelcontextprotocol/server... │
│  ○ sqlite              Available · Python database server          │
│  ○ puppeteer           Available · Browser automation              │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│  ✓ Installed figma                                            3.2s │
│  ↻ Checking health...                                              │
│  ✓ filesystem: healthy                                             │
├─────────────────────────────────────────────────────────────────────┤
│  j/k:navigate  i:install  d:delete  e:edit  s:sync  /:search  ?:help│
└─────────────────────────────────────────────────────────────────────┘
```

### Components

1. **Header Bar** (1 line)
   - App name: `agentctl`
   - Current profile: `default (5 servers)`
   - Status counts: `5 ● 2 ○ 1 ◌` (installed, available, disabled)

2. **Server List** (main area)
   - Unified list showing ALL servers (installed + available)
   - Status badges: `●` installed, `○` available, `◌` disabled
   - Health indicators: `✓` healthy, `✗` unhealthy, `?` unknown, spinner checking

3. **Log Panel** (3-4 lines, toggleable to half-screen with `L`)
   - Shows operation output in real-time
   - Scrollable history when expanded
   - Timestamps on the right

4. **Key Hints Bar** (1 line)
   - Essential keys always visible
   - Full reference with `?` overlay

## Status Badges

| Badge | Meaning |
|-------|---------|
| `●` | Installed and enabled |
| `◌` | Installed but disabled |
| `○` | Available (not installed) |
| `✓` | Health check passed |
| `✗` | Health check failed |
| `?` | Health unknown/not checked |
| `↻` | Operation in progress (spinner) |

## Keybindings (Vim-style)

### Navigation
| Key | Action |
|-----|--------|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `g` | Go to first item |
| `G` | Go to last item |
| `Ctrl+d` | Page down |
| `Ctrl+u` | Page up |
| `/` | Search/filter by name |
| `Esc` | Clear filter/search, deselect |

### Selection (for bulk operations)
| Key | Action |
|-----|--------|
| `Space` | Toggle selection on current item |
| `v` | Enter visual/selection mode |
| `V` | Select all visible |
| `Ctrl+a` | Select all |

### Operations
| Key | Action |
|-----|--------|
| `i` | Install selected/current |
| `d` | Delete/remove selected/current |
| `e` | Edit config (opens modal) |
| `Enter` | Toggle enable/disable |
| `u` | Update to latest version |
| `s` | Sync to tools |
| `S` | Sync all |
| `t` | Test/health check selected |
| `T` | Health check all |
| `r` | Refresh list |

### Filters
| Key | Action |
|-----|--------|
| `f` | Cycle filter: All → Installed → Available → Disabled |
| `1` | Show all |
| `2` | Show installed only |
| `3` | Show available only |
| `4` | Show disabled only |

### Profiles
| Key | Action |
|-----|--------|
| `P` | Open profile quick-switcher |
| `Ctrl+p` | Add selected to current profile |

### UI
| Key | Action |
|-----|--------|
| `L` | Toggle log panel size |
| `?` | Toggle help overlay |
| `q` | Quit |
| `Ctrl+c` | Quit |

## Profiles

### Default Profile

- All servers belong to `default` profile implicitly
- User-created profiles are explicit subsets
- When no profile is selected, shows all servers

### Profile Switching

- `P` opens a quick-switcher popup
- Selecting a profile filters the server list
- Header shows current profile: `work (3 servers)`
- Operations (sync, etc.) apply to filtered servers only

### Profile Quick-Switcher

```
┌─ Switch Profile ─────────────────┐
│                                  │
│  > default (5 servers)           │
│    work (3 servers)              │
│    personal (2 servers)          │
│                                  │
│  Enter:select  n:new  Esc:cancel │
└──────────────────────────────────┘
```

## Modal Dialogs

### Edit Server Modal

Triggered by `e` on a selected server:

```
┌─ Edit: filesystem ───────────────────────────────────┐
│                                                      │
│  Transport   [stdio ▼]                               │
│                                                      │
│  Command     [npx                              ]     │
│  Args        [-y @modelcontextprotocol/server-f]     │
│                                                      │
│  Environment                                         │
│  [HOME=/Users/foo                              ]     │
│  [+ Add variable]                                    │
│                                                      │
│           [Cancel]  [Save]                           │
└──────────────────────────────────────────────────────┘
```

### Help Overlay

Triggered by `?`:

```
┌─ Keyboard Shortcuts ─────────────────────────────────┐
│                                                      │
│  Navigation           Operations                     │
│  ──────────           ──────────                     │
│  j/k    up/down       i      install                 │
│  g/G    top/bottom    d      delete                  │
│  /      search        e      edit                    │
│  Esc    clear         Enter  enable/disable          │
│                       s      sync                    │
│  Selection            t      test health             │
│  ─────────                                           │
│  Space  toggle        Filters                        │
│  V      select all    ───────                        │
│                       f      cycle filter            │
│  Profiles             1-4    quick filter            │
│  ────────                                            │
│  P      switch        UI                             │
│                       ──                             │
│                       L      toggle logs             │
│                       ?      this help               │
│                       q      quit                    │
│                                                      │
│                          Press any key to close      │
└──────────────────────────────────────────────────────┘
```

## Health Checking

### Background Health Checks

1. On UI launch, immediately render the list
2. Spawn goroutines to health-check installed servers
3. Update list items as results come in (non-blocking)
4. Show spinner `↻` while checking
5. Cache results for session (re-check with `t` or `T`)

### Health Check Logic

For each server type:
- **stdio**: Spawn process, send `initialize`, expect valid response
- **http/sse**: HTTP HEAD or GET to URL, check for MCP headers
- Timeout: 5 seconds per server

## Log Panel

### Minimal Mode (default, 3-4 lines)

Shows most recent operations:
```
✓ Installed figma                                            3.2s
↻ Checking health...
✓ filesystem: healthy
```

### Expanded Mode (toggle with `L`)

Shows scrollable history, half screen height:
```
─────────────────────────────────────────────────────────────────
12:34:21  ✓ Installed figma                                  3.2s
12:34:18  ↻ Installing figma from https://mcp.figma.com...
12:34:15  ✓ Synced to claude, cursor
12:34:10  ✗ playwright: connection refused
12:34:05  ✓ filesystem: healthy
─────────────────────────────────────────────────────────────────
j/k:scroll  q:close panel
```

## Bulk Operations

1. Use `Space` to toggle selection on items
2. Selected items show `▶` indicator
3. Press operation key (e.g., `d`) to apply to ALL selected
4. Confirmation prompt for destructive operations
5. Progress shown in log panel

Example with 3 selected:
```
│  ▶ ● filesystem        stdio · npx @modelcontextprotocol/...   │
│    ● playwright        stdio · npx playwriter@latest           │
│  ▶ ● figma             http  · https://mcp.figma.com/mcp       │
│  ▶ ○ sqlite            Available · Python database server      │
```

Pressing `i` would install sqlite, pressing `d` would prompt:
```
Delete 2 servers? (filesystem, figma) [y/N]
```

## Implementation Plan

### Phase 1: Core Layout
1. Unified list model with status badges
2. Header with profile and counts
3. Basic key navigation (j/k/g/G)
4. Log panel (minimal mode)
5. Key hints bar

### Phase 2: Operations
1. Install/remove with log output
2. Enable/disable toggle
3. Edit modal (reuse huh forms)
4. Sync operation

### Phase 3: Advanced Features
1. Multi-select and bulk operations
2. Background health checking
3. Filter modes
4. Profile quick-switcher

### Phase 4: Polish
1. Toggleable log panel
2. Help overlay
3. Animations/spinners
4. Error handling and edge cases

## Files to Modify

- `internal/tui/tui.go` - Complete rewrite
- `internal/tui/model.go` - New: Model and state management
- `internal/tui/view.go` - New: View rendering
- `internal/tui/keys.go` - New: Keybinding definitions
- `internal/tui/commands.go` - New: Async operations (health, install, etc.)
- `internal/tui/components/` - New directory for reusable components:
  - `header.go` - Header bar
  - `list.go` - Server list
  - `logpanel.go` - Log panel
  - `modal.go` - Modal dialogs
  - `help.go` - Help overlay

## Dependencies

Already have:
- `github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/bubbles`
- `github.com/charmbracelet/lipgloss`
- `github.com/charmbracelet/huh`

May need:
- `github.com/charmbracelet/bubbles/spinner` - For loading indicators
- `github.com/charmbracelet/bubbles/viewport` - For scrollable log panel

## Success Criteria

1. Single unified view shows all servers with clear status
2. All operations work without leaving the UI
3. Health status updates in background without blocking
4. Bulk operations work intuitively with multi-select
5. Profile switching is fast and obvious
6. Keyboard-only workflow is smooth (no mouse needed)
7. Matches Poimandres color scheme throughout
