# Interactive Forms for CLI Commands

## Overview

When CLI commands are invoked with no arguments, they should launch an interactive form using the `huh` library to guide users through the required inputs. This provides a friendlier UX while keeping the CLI flags available for scripting.

## Design Principles

1. **Consistent UX** - All forms use the same `huh` form style
2. **Progressive disclosure** - Ask only what's needed, in logical order
3. **Graceful cancellation** - Confirm cancel, then show hint for non-interactive usage
4. **Non-blocking** - Forms never block users who know what they want (flags still work)

## Commands to Implement

### 1. `agentctl add` (DONE)

Already implemented. Shows form for:
- Server name
- Transport type (stdio/http/sse)
- Transport-specific fields (command+args OR url)

### 2. `agentctl init`

**Trigger:** `agentctl init` with no flags

**Flow:**
1. Welcome message
2. Detect existing tool configs (Claude, Cursor, etc.)
3. If configs found:
   - Show list of detected tools with server counts
   - Ask which to import (multi-select)
   - For each conflict (same server name in multiple tools):
     - Show both configs side-by-side
     - Ask user to pick one
4. If no configs found:
   - Ask if user wants to add first server (launches `add` form)
5. Create config file
6. Show summary of what was imported

**Form fields:**
- Multi-select: Tools to import from
- Per-conflict: Radio select which config to keep

### 3. `agentctl new`

**Trigger:** `agentctl new` with no subcommand

**Flow:**
1. Detect context (is user in a project dir with .agentctl.json?)
2. Smart default: Suggest most likely resource type based on context
3. Show picker for resource type: command, rule, prompt, skill
4. Based on selection, ask for:
   - **command**: name, description, command template
   - **rule**: name, description, rule content
   - **prompt**: name, description, prompt template
   - **skill**: name, description, trigger patterns
5. Create file in appropriate directory

**Form fields:**
- Select: Resource type
- Input: Name
- Text: Description
- Text area: Content/template

### 4. `agentctl profile create`

**Trigger:** `agentctl profile create` with no name arg

**Flow:**
1. Ask for profile name
2. Ask for optional description
3. Show list of currently installed servers
4. Multi-select: Which servers to include in this profile
5. Create profile

**Form fields:**
- Input: Profile name (validated for uniqueness)
- Input: Description (optional)
- Multi-select: Servers to include

### 5. `agentctl secret set`

**Trigger:** `agentctl secret set` with no name arg

**Flow:**
1. Ask for secret name
2. Show password input for secret value (masked)
3. Confirm storage location (keychain)
4. Store secret

**Form fields:**
- Input: Secret name
- Password: Secret value (masked input)

### 6. `agentctl alias add`

**Trigger:** `agentctl alias add` with no args

**Flow:**
1. Ask for alias name
2. Ask for transport type (stdio/http/sse)
3. Based on transport:
   - **stdio**: Ask for runtime (node/python), package name
   - **http/sse**: Ask for URL
4. Ask for optional description
5. Save alias

**Form fields:**
- Input: Alias name
- Select: Transport type
- Conditional inputs based on transport
- Input: Description (optional)

## Cancellation Behavior

When user presses Ctrl+C or Esc during any form:

1. Show confirmation: "Cancel setup? (y/N)"
2. If confirmed:
   - Exit with message: "Cancelled. Run `agentctl <cmd> --help` for non-interactive usage."
3. If not confirmed:
   - Return to form

## Implementation Notes

### TTY Detection

Before launching any interactive form, check if stdin is a TTY:

```go
import "golang.org/x/term"

func isInteractive() bool {
    return term.IsTerminal(int(os.Stdin.Fd()))
}
```

If not interactive (piped input, CI, etc.):
- Show error: "Interactive mode requires a terminal. Use flags instead: agentctl <cmd> --help"
- Exit with non-zero status

### Shared Form Utilities

Create `internal/cli/forms.go` with:
- `isInteractive()` - Check if stdin is a TTY
- `requireInteractive(cmd string)` - Exit with helpful message if not TTY
- `confirmCancel()` - Standard cancel confirmation
- `showCancelHint(cmd string)` - Standard cancel message
- Common validation functions
- Consistent styling setup

### Form Styling

Use consistent huh theme:
```go
form := huh.NewForm(groups...).
    WithTheme(huh.ThemeDracula())  // or custom theme
```

## Implementation Order

Implement all simultaneously using parallel agents:
1. Agent 1: `init` with import wizard
2. Agent 2: `new` with smart defaults
3. Agent 3: `profile create` with server selection
4. Agent 4: `secret set` and `alias add`

## Files to Modify

- `internal/cli/forms.go` - NEW: Shared form utilities (TTY detection, cancel handling, styling)
- `internal/cli/init.go` - Add interactive import wizard
- `internal/cli/new.go` - Add interactive form with smart defaults
- `internal/cli/profile.go` - Add interactive form for create subcommand
- `internal/cli/secret.go` - Add interactive form for set subcommand
- `internal/cli/alias.go` - Add interactive form for add subcommand

## Dependencies

```bash
go get golang.org/x/term  # For TTY detection
# huh already added for the add command
```

## Success Criteria

1. All listed commands work with no args (launch form)
2. All commands still work with flags (skip form)
3. Consistent look and feel across all forms
4. Graceful cancellation with helpful hints
5. Forms validate input before submission
