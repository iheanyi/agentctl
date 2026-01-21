# feat: Native Resource Discovery & TUI Enhancements

## Enhancement Summary

**Deepened on:** 2026-01-20
**Sections enhanced:** All major sections
**Research agents used:** architecture-strategist, performance-oracle, security-sentinel, code-simplicity-reviewer, pattern-recognition-specialist, agent-native-reviewer, best-practices-researcher, framework-docs-researcher

### Key Improvements
1. **DirectoryScanner pattern** - Config-driven scanner with frozen config struct (no DSL creep)
2. **Security hardening** - File size limits with SafeReader helper (skip symlink complexity)
3. **Inspectable interface** - Each resource type renders itself, eliminating `interface{}` smell
4. **CLI enhancement** - Enhance `list --native --json` with full details (no separate inspect commands)

### New Considerations Discovered
- Keep single Scanner interface (3 interfaces is premature)
- DiscoveredResource type (not "DiscoveredResource" - confusing in Go context)
- Single inspector.go file with Inspectable interface pattern
- Defer parallel discovery until performance problem is proven

---

## Overview

Add comprehensive native resource discovery from tool directories (`.claude/`, `.gemini/`, `.cursor/`, etc.), read-only inspection views in the TUI, CLI integration with source attribution, and parallel discovery for performance improvements.

## Problem Statement

When running `agentctl list` or opening the TUI in a repo with existing `.claude/rules/` or `.codex/skills/`, some resources don't appear because:
1. Only Claude and Gemini scanners exist; Cursor, Codex, OpenCode, and Copilot scanners are missing
2. CLI `list` command doesn't use the discovery package - only shows agentctl-managed resources
3. TUI lacks read-only inspection modals for Rules, Commands, Hooks
4. Discovery runs sequentially, missing performance opportunities from parallelization
5. Claude plugins from `settings.json` are not discovered

## Proposed Solution

### Phase 1: Complete Scanner Coverage

#### DirectoryScanner Pattern (Config-Driven)

Use a config-driven `DirectoryScanner` with a **frozen config struct** (no future field additions without strong justification):

```go
// pkg/discovery/directory_scanner.go
type ScannerConfig struct {
    Name           string   // e.g., "cursor"
    LocalDirs      []string // e.g., [".cursor"]
    GlobalDirs     []string // e.g., ["~/.cursor/rules"]
    DetectFiles    []string // e.g., [".cursorrules"]
    RulesDirs      []string // relative to tool dir
    SkillsDirs     []string
    CommandsDirs   []string
    FileExtensions []string // e.g., [".md", ".mdc"]
}

func NewDirectoryScanner(cfg ScannerConfig) *DirectoryScanner
```

**⚠️ WARNING: Keep config struct frozen.** The moment someone suggests adding `ValidationFunc`, `CustomDetectors`, or `IgnorePatterns`, consider whether 5 simple files would be clearer. Config-driven is good; a DSL is not.

**Scanner configurations:**

| Scanner | Config |
|---------|--------|
| **Cursor** | `LocalDirs: [".cursor"], DetectFiles: [".cursorrules"], RulesDirs: ["rules"]` |
| **Codex** | `LocalDirs: [".codex"], DetectFiles: ["AGENTS.md"], SkillsDirs: ["skills"]` |
| **OpenCode** | `LocalDirs: [".opencode"], RulesDirs: ["rules"], CommandsDirs: ["commands"]` |
| **Copilot** | `LocalDirs: [".github-copilot"], CommandsDirs: ["commands"]` |
| **TopLevel** | `LocalDirs: ["skills"], DetectFiles: ["CLAUDE.md", "AGENTS.md"]` |

### Phase 2: Plugin Discovery (Claude-specific)

Add plugin discovery from Claude's `settings.json`:

```go
// pkg/plugin/plugin.go
type Plugin struct {
    Name    string `json:"name"`
    Path    string `json:"path"`
    Enabled bool   `json:"enabled"`
    Scope   string `json:"-"` // "local" or "global"
    Tool    string `json:"-"` // "claude"
}

// ClaudeScanner.ScanPlugins(dir string) ([]*plugin.Plugin, error)
// ClaudeScanner.ScanGlobalPlugins() ([]*plugin.Plugin, error)
```

### Phase 3: Read-Only Inspector Modals

Add inspector modals for viewing resource details without editing:

| Resource | Inspector Shows | Key Binding |
|----------|-----------------|-------------|
| **Rule** | Name, Tool, Scope, Path, Frontmatter, Content | Enter on rule |
| **Command** | Name, Tool, Scope, Description, Prompt, Model | Enter on command |
| **Hook** | Name, Tool, Scope, Type, Matcher, Command | Enter on hook |
| **Server** | Name, Transport, Command/URL, Env vars, Health | Enter on server (or 'i') |
| **Plugin** | Name, Path, Enabled status | Enter on plugin |

#### Inspectable Interface Pattern

**Avoid `interface{}` + `kind string` (code smell).** Use a sealed interface where each type renders itself:

```go
// pkg/inspectable/inspectable.go
type Inspectable interface {
    InspectTitle() string   // Display name for modal header
    InspectContent() string // Formatted content for viewport
}

// Each type implements Inspectable - rendering belongs with the type
func (r *rule.Rule) InspectTitle() string { return fmt.Sprintf("Rule: %s", r.Name) }
func (r *rule.Rule) InspectContent() string {
    var b strings.Builder
    b.WriteString(fmt.Sprintf("Tool:  %s\n", r.Tool))
    b.WriteString(fmt.Sprintf("Scope: %s\n", r.Scope))
    b.WriteString(fmt.Sprintf("Path:  %s\n\n", r.Path))
    if len(r.Frontmatter) > 0 {
        b.WriteString("Frontmatter:\n")
        // ... format frontmatter
    }
    b.WriteString("Content:\n")
    b.WriteString(r.Content)
    return b.String()
}
```

**Inspector stays focused on modal mechanics:**
```go
// internal/tui/inspector.go
type InspectorModel struct {
    viewport viewport.Model
    resource Inspectable  // Type-safe, no switches needed
}

func NewInspector(resource Inspectable, width, height int) InspectorModel {
    vp := viewport.New(width-4, height-6)
    vp.SetContent(resource.InspectContent())  // Each type renders itself
    return InspectorModel{viewport: vp, resource: resource}
}

func (m InspectorModel) Title() string {
    return m.resource.InspectTitle()
}
```

**Modal centering with lipgloss + horizontal overflow handling:**
```go
func (m Model) viewInspector() string {
    title := m.inspector.Title()
    content := m.inspector.View()

    // Handle horizontal overflow with word wrap
    wrapped := lipgloss.NewStyle().Width(m.width - 8).Render(content)

    bordered := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        Padding(1).
        Render(fmt.Sprintf("%s\n\n%s", title, wrapped))
    return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, bordered)
}
```

### Phase 4: CLI List Enhancement

Add `--native` flag and source attribution:

```bash
# Current behavior (agentctl-managed only)
agentctl list

# New: Show tool-native resources with source markers
agentctl list --native

# Output example:
Rules:
  NAME           SCOPE    SOURCE      PATH
  development    [L]      [claude]    .claude/rules/development.md
  testing        [G]      [claude]    ~/.claude/rules/testing.md
  cursor-rules   [L]      [cursor]    .cursor/rules/main.mdc
  my-rule        [L]      [agentctl]  ~/.config/agentctl/rules/my-rule.md
```

#### Enhanced `list --native --json` (No Separate inspect Commands)

**Don't add 6 new commands.** Enhance `list` with full resource details in JSON output:

```bash
# Human-readable output with source column
agentctl list --native
# Output:
#   NAME           SCOPE    SOURCE      PATH
#   development    [L]      [claude]    .claude/rules/development.md

# Machine-readable with FULL details (content, frontmatter, etc.)
agentctl list --native --json
```

**JSON output includes everything an agent needs:**
```json
{
  "rules": [
    {
      "name": "development",
      "tool": "claude",
      "scope": "local",
      "path": ".claude/rules/development.md",
      "frontmatter": {"paths": ["src/**"]},
      "content": "# Development Rules\n..."
    }
  ],
  "errors": [
    {"scanner": "codex", "path": ".codex/broken.md", "error": "parse error"}
  ]
}
```

**Why NOT separate `inspect` commands:**
- `list --json | jq '.rules[] | select(.name=="development")'` does the same thing
- 6 new commands is "command explosion" - harder to discover, document, test
- Agents parsing JSON don't need command-per-resource-type

### Phase 5: Parallel Discovery

Use `errgroup` with bounded concurrency:

#### Research Insight: I/O-Bound Concurrency

**Performance recommendation:** Use `SetLimit(32)` not `GOMAXPROCS` for I/O-bound work:

```go
func ParallelDiscoverAll(ctx context.Context, dir string) (*DiscoveryResult, error) {
    g, ctx := errgroup.WithContext(ctx)
    g.SetLimit(32)  // I/O-bound, not CPU-bound - can have more goroutines than cores

    var mu sync.Mutex
    result := &DiscoveryResult{
        Resources: make([]*DiscoveredResource, 0),
        Errors:    make([]ScanError, 0),  // Collect partial errors
    }

    for _, scanner := range registry {
        scanner := scanner
        g.Go(func() error {
            if !scanner.Detect(dir) {
                return nil
            }
            resources, errs := scanToolWithErrors(ctx, scanner, dir)

            mu.Lock()
            result.Resources = append(result.Resources, resources...)
            result.Errors = append(result.Errors, errs...)
            mu.Unlock()

            return nil  // Don't fail fast - collect all results
        })
    }

    if err := g.Wait(); err != nil {
        return result, err  // Return partial results even on error
    }
    return result, nil
}

// DiscoveryResult enables partial success reporting
type DiscoveryResult struct {
    Resources []*DiscoveredResource
    Errors    []ScanError
}

type ScanError struct {
    Scanner string
    Path    string
    Err     error
}
```

**Additional performance optimizations:**
- Use `filepath.WalkDir` over `filepath.Walk` (2-10x faster, avoids extra stat calls)
- Lazy content loading: Only read file content when inspector is opened
- Skip directories early: `.git`, `node_modules`, `vendor`, `.venv`

## Technical Considerations

### Architecture

#### Keep Single Scanner Interface

**Don't create 3 interfaces for 7 scanners.** The existing pattern works:

```go
// pkg/discovery/discovery.go

// Scanner - one interface, simple contract
type Scanner interface {
    Name() string
    Detect(dir string) bool
    ScanRules(dir string) ([]*rule.Rule, error)
    ScanSkills(dir string) ([]*skill.Skill, error)
    ScanCommands(dir string) ([]*command.Command, error)
    ScanHooks(dir string) ([]*hook.Hook, error)
    ScanPrompts(dir string) ([]*prompt.Prompt, error)
    ScanServers(dir string) ([]*mcp.Server, error)
}

// GlobalScanner - optional extension (existing pattern)
type GlobalScanner interface {
    Scanner
    ScanGlobalRules() ([]*rule.Rule, error)
    ScanGlobalSkills() ([]*skill.Skill, error)
    ScanGlobalCommands() ([]*command.Command, error)
    ScanGlobalHooks() ([]*hook.Hook, error)
}

// For plugins: Just add methods to ClaudeScanner directly.
// Don't create an interface for a single implementation.
```

**File structure:**
```
pkg/discovery/
├── discovery.go          # Scanner + GlobalScanner interfaces, DiscoveryResult
├── directory_scanner.go  # DirectoryScanner (config-driven)
├── claude.go             # ClaudeScanner (existing + plugin methods)
├── gemini.go             # GeminiScanner (existing)
├── plugin.go             # Plugin type (NOT in separate pkg/plugin - avoids Go stdlib conflict)
├── safe.go               # SafeReader helper
└── parallel.go           # ParallelDiscoverAll (defer until needed)

pkg/inspectable/
└── inspectable.go        # Inspectable interface

internal/tui/
└── inspector.go          # InspectorModel (uses Inspectable)

internal/cli/
├── list.go               # Add --native flag, enhanced --json output
└── import.go             # Add --all, --from, --force flags
```

**Modified Files:**
- `pkg/discovery/discovery.go` - Add DiscoveryResult type
- `pkg/discovery/claude.go` - Add ScanPlugins, ScanGlobalPlugins methods
- `internal/tui/tui.go` - Add inspector modal state, key bindings
- `internal/cli/list.go` - Add --native flag, source attribution, full JSON output
- `internal/cli/import.go` - Add --all, --from, --force flags

### Performance Implications

- **Parallel scanning**: 2-4x faster on multi-core systems with multiple tool directories
- **Bounded concurrency**: Prevents file descriptor exhaustion
- **Directory skipping**: Skip `.git`, `node_modules`, `vendor` early

### Security Considerations

#### SafeReader Pattern (Avoid Threading projectRoot Everywhere)

**Problem with `SafeReadFile(path, projectRoot)`:** Every caller needs to know projectRoot. This will be forgotten or passed incorrectly.

**Use a closure/method pattern instead:**

```go
// pkg/discovery/safe.go
const MaxFileSize = 1 << 20  // 1MB

type SafeReader struct {
    MaxSize int64
}

func NewSafeReader() *SafeReader {
    return &SafeReader{MaxSize: MaxFileSize}
}

func (r *SafeReader) ReadFile(path string) ([]byte, error) {
    // Check file size before reading (prevents memory exhaustion)
    info, err := os.Stat(path)
    if err != nil {
        return nil, err
    }
    if info.Size() > r.MaxSize {
        return nil, fmt.Errorf("file too large: %d bytes (max %d)", info.Size(), r.MaxSize)
    }
    return os.ReadFile(path)
}
```

**Note on symlinks:** Skip symlink validation complexity. These are markdown files in user's own project directories - the user already controls them. File size check is the real protection against resource exhaustion.

**Security checklist:**
- [x] File size limits (1MB default) to prevent memory exhaustion (SafeReader implemented in pkg/discovery/safe.go)
- [x] Permission errors logged, not propagated (graceful degradation)
- [x] No execution of discovered content (read-only inspection)

## Acceptance Criteria

### Functional Requirements

- [x] All 7 scanners implemented (Claude, Gemini, Cursor, Codex, OpenCode, Copilot, TopLevel)
- [x] Plugin discovery works for Claude (local + global settings.json)
- [x] Inspector modals show full resource details read-only
- [x] `agentctl list --native` shows tool-native resources with source markers
- [x] `agentctl list --native --json` includes full resource details (content, frontmatter)
- [x] `agentctl import --from <tool>` imports resources from specific tool
- [x] `agentctl import --all` imports all discovered native resources
- [x] `agentctl import --force` overwrites existing resources

### Error Handling Requirements

- [x] When resource doesn't exist, return non-zero exit code with clear error message
- [x] When multiple tools have same-named resource, show all with source disambiguation (via --native flag)
- [ ] `--json` output includes `"errors"` array for partial failures
- [x] Exit code is 0 with partial errors (warnings don't fail the command)
- [x] File size > 1MB is logged as warning, file skipped, discovery continues (SafeReader returns error, discovery continues)

### Non-Functional Requirements

- [x] Inspector modals support keyboard navigation (j/k, up/down, PgUp/PgDn)
- [x] Inspector modals close with Esc or q
- [x] Inspector handles horizontal overflow (word wrap for long paths/content)
- [x] Discovery completes in <2s for typical projects
- [x] Graceful degradation on permission errors

### Quality Gates

- [x] Unit tests for each scanner using `fstest.MapFS`
- [ ] Golden tests for CLI list output with --native flag
- [ ] TUI golden tests for inspector modals
- [x] Tests with `-race` flag in CI
- [x] No regressions in existing functionality

## Implementation Phases

### Phase 1: Scanner Implementation (Foundation)

**Depends on:** Nothing (start here)

#### DirectoryScanner Approach

**Tasks:**
1. Create `pkg/discovery/generic.go` - GenericScanner with ScannerConfig
2. Create `pkg/discovery/safe.go` - SafeReadFile and security helpers
3. Register scanner configs for Cursor, Codex, OpenCode, Copilot, TopLevel
4. Add unit tests using `fstest.MapFS` for isolation
5. Update existing Claude/Gemini scanners to use SafeReadFile

**Implementation pattern:**
```go
// pkg/discovery/generic.go
func init() {
    Register(NewGenericScanner(ScannerConfig{
        Name:       "cursor",
        LocalDirs:  []string{".cursor"},
        DetectFiles: []string{".cursorrules"},
        RulesDirs:  []string{"rules"},
        FileExts:   []string{".md", ".mdc"},
    }))

    Register(NewGenericScanner(ScannerConfig{
        Name:       "codex",
        LocalDirs:  []string{".codex"},
        DetectFiles: []string{"AGENTS.md"},
        SkillsDirs: []string{"skills"},
    }))

    // ... similar for opencode, copilot, toplevel
}
```

**Testing with fstest.MapFS:**
```go
func TestCursorScanner(t *testing.T) {
    fs := fstest.MapFS{
        ".cursor/rules/main.mdc": &fstest.MapFile{
            Data: []byte("# Cursor rules"),
        },
    }
    scanner := NewGenericScanner(cursorConfig)
    // Test against in-memory filesystem
}
```

**Success criteria:** `go test ./pkg/discovery/...` passes

### Phase 2: Plugin Discovery

**Depends on:** Phase 1 (extends ClaudeScanner)

**Tasks:**
1. Create `pkg/discovery/plugin.go` - Plugin type (NOT separate pkg/plugin - avoids Go stdlib conflict)
2. Add `ScanPlugins(dir string)` method to ClaudeScanner
3. Add `ScanGlobalPlugins()` method to ClaudeScanner
4. Add `InspectTitle()` and `InspectContent()` to Plugin type
5. Update TUI to display plugins (new tab or under Servers)

**Success criteria:** TUI shows Claude plugins from settings.json

### Phase 3: Inspector Modals

**Depends on:** Phase 1 (needs Inspectable implementations on resource types)

#### Inspectable Pattern Implementation

**Tasks:**
1. Create `pkg/inspectable/inspectable.go` - Define Inspectable interface
2. Add `InspectTitle()` and `InspectContent()` to rule.Rule, command.Command, hook.Hook, mcp.Server, Plugin
3. Create `internal/tui/inspector.go` - InspectorModel using Inspectable
4. Add `showInspector bool` and `inspector InspectorModel` to main Model
5. Add key bindings: Enter to open (on any list), Esc/q to close
6. Use `lipgloss.Place()` for centering, word wrap for horizontal overflow
7. Add golden tests for inspector views

**State management pattern:**
```go
// internal/tui/tui.go
type Model struct {
    // ... existing fields
    showInspector bool
    inspector     InspectorModel
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // Inspector takes priority when shown
    if m.showInspector {
        switch msg := msg.(type) {
        case tea.KeyMsg:
            switch msg.String() {
            case "esc", "q":
                m.showInspector = false
                return m, nil
            }
        }
        var cmd tea.Cmd
        m.inspector, cmd = m.inspector.Update(msg)
        return m, cmd
    }
    // ... normal handling
}

func (m Model) View() string {
    if m.showInspector {
        return m.viewInspector()  // Full overlay
    }
    return m.viewNormal()
}
```

**Success criteria:** All resource types have read-only inspection

### Phase 4: CLI Enhancement

**Depends on:** Phase 1 (needs scanners to discover resources)

**Tasks:**
1. Add `--native` flag to `list.go`
2. Add `Source` (tool) field to output structs
3. Update tabwriter output to show source column
4. Enhance `--json` output to include full resource details (content, frontmatter)
5. Include `"errors"` array in JSON output for partial failures
6. Add `--all` and `--from` flags to `import.go`
7. Add `--force` flag to import (overwrite existing)
8. Add `--dry-run` flag to import

**Success criteria:**
- `agentctl list --native` shows tool sources
- `agentctl list --native --json` includes full resource details
- Exit code is 0 with partial errors (warnings don't fail)

### Phase 5: Parallel Discovery (DEFER UNTIL NEEDED)

**⚠️ This phase is premature optimization.** Most repos have 1-2 tool directories. Sequential scanning takes <100ms. Only implement if users report performance issues.

**When to revisit:**
- Users file issues about slow discovery (>2s)
- Repos with 10+ tool directories become common
- Profiling shows discovery is the bottleneck

**If implemented later:**
```go
func ParallelDiscoverAll(ctx context.Context, dir string) (*DiscoveryResult, error) {
    g, ctx := errgroup.WithContext(ctx)
    g.SetLimit(32)  // I/O-bound, not CPU-bound

    // ... collect results with mutex
    // ... aggregate errors

    return result, g.Wait()
}
```

**For now:** Keep the existing sequential `DiscoverAll()` - it's simple and fast enough.

## Duplicate Handling Strategy

When same resource exists in multiple tools:
1. **List**: Show all instances with different source markers
2. **Import**: Skip if resource with same name already exists in agentctl
3. **Sync**: Only sync agentctl-managed resources (unchanged behavior)

## Error Handling

| Error Type | Behavior |
|------------|----------|
| Permission denied | Log warning, skip directory, continue |
| Malformed file | Log warning, skip file, continue |
| Directory missing | Return empty results, no error |
| Timeout | Cancel with context, return partial results |

## References

### Internal References
- `pkg/discovery/discovery.go:29-53` - Scanner interface
- `pkg/discovery/claude.go:1-214` - ClaudeScanner implementation pattern
- `internal/tui/tui.go:3787-3881` - Existing skill detail modal pattern
- `pkg/sync/adapter.go:222-290` - Existing parallel sync pattern

### External References
- [errgroup package](https://pkg.go.dev/golang.org/x/sync/errgroup) - Bounded concurrency
- [Bubble Tea viewport](https://github.com/charmbracelet/bubbles) - Scrollable content
- [mise config discovery](https://mise.jdx.dev/dev-tools/) - Directory walk patterns

### Related Work
- Discovery package already exists with Claude and Gemini scanners
- TUI has skill detail modal as pattern to follow
- Hook loading with project support as pattern for local/global

## Research Findings Summary

### Post-Review Decisions

**DHH Review:**
- ✅ Keep DirectoryScanner but freeze config struct (no DSL creep)
- ✅ One Scanner interface is enough (dropped 3-interface split)
- ✅ Remove `inspect` command family (enhance `list --json` instead)
- ✅ Defer parallel discovery (Phase 5) until performance problem is proven

**Kieran Review:**
- ✅ Renamed `GenericScanner` → `DirectoryScanner`
- ✅ Use `DiscoveredResource` instead of `DiscoveredResource`
- ✅ Use `Inspectable` interface instead of `interface{}` + `kind string`
- ✅ Move plugin.go into `pkg/discovery/` (not separate `pkg/plugin`)
- ✅ Add explicit phase dependencies
- ✅ Add missing acceptance criteria (error cases, `--force` flag)
- ✅ Use `SafeReader` struct instead of threading `projectRoot` everywhere

### Retained from Initial Research

**Architecture:**
- DiscoveryResult type with both resources AND errors for partial failure handling
- init() registration pattern - appropriate for plugin architecture

**Security:**
- File size limits (1MB) to prevent memory exhaustion
- SafeReader pattern for centralized checks
- Symlink validation deferred (not a real threat for local files)

**TUI Patterns:**
- Viewport component for scrollable inspector content
- `lipgloss.Place()` for modal centering
- Word wrap for horizontal overflow handling
- `ansi.Truncate` for string truncation (handles multi-byte UTF-8)

**Testing:**
- `fstest.MapFS` for unit tests
- `-race` flag in CI
- Golden tests for CLI and TUI output
