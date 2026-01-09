---
paths: internal/tui/**
---

# Tasuku Learnings

_Auto-synced from .tasuku/context/learnings.md_

## Rules

- Never use hardcoded numbers for enum cycling (e.g., `% 4` or `% 6`). Instead, define a corresponding Names slice (e.g., `TabNames`, `FilterModeNames`) and use `% EnumType(len(NamesSlice))`. This ensures cycling remains correct when new enum values are added.
- Always use `ansi.Truncate(s, n, "...")` from `github.com/charmbracelet/x/ansi` for string truncation in TUIs. Never use slice operations like `s[:n]` which break on multi-byte UTF-8 characters and ANSI escape sequences.
- In multi-tab TUIs, navigation bounds (Down, PageDown, Bottom keys) must check against the active tab's list length, not a single list. Create a helper like `currentTabLength()` that switches on the active tab and returns the appropriate slice length.
- Never use huh forms within Bubble Tea TUIs. huh uses tea.Exec which suspends the program and causes jarring context switches. Instead, use bubbles components (textinput, textarea, list) that stay within the Update/View cycle for smooth in-place editing. Reserve huh for CLI-only interactive flows where full-screen forms are expected.
- Never use huh forms within Bubble Tea TUIs. huh uses tea.Exec which suspends the program and causes jarring context switches. Instead, use bubbles components (textinput, textarea, list) that stay within the Update/View cycle for smooth in-place editing. Reserve huh for CLI-only interactive flows where full-screen forms are expected.

## Insights

- Use `bubbles/textinput` for text input instead of manual string manipulation. textinput provides proper cursor handling, Focus/Blur state, and automatic Update/View integration with Bubble Tea.
- Use `lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)` to center modals and overlays in the terminal. This is cleaner than manual padding calculations.

