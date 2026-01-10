# Tasuku Decisions

_Auto-synced from .tasuku/context/decisions.md_

## tui-input-components (2026-01-09)

**Chose**: bubbles components (textinput, textarea) for TUI input forms

**Over**: huh forms within TUI, manual string manipulation for input handling

**Because**: huh uses tea.Exec which suspends Bubble Tea and causes jarring context switches. bubbles components stay within the Update/View cycle, providing smooth in-place editing with consistent styling. Users experience seamless modal interactions instead of being taken to a separate full-screen program.

