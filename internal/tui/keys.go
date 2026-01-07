package tui

import "github.com/charmbracelet/bubbles/key"

// keyMap defines all keybindings for the TUI.
type keyMap struct {
	// Navigation
	Up       key.Binding
	Down     key.Binding
	Top      key.Binding
	Bottom   key.Binding
	PageDown key.Binding
	PageUp   key.Binding
	Search   key.Binding
	Escape   key.Binding

	// Selection (for bulk operations)
	Select    key.Binding
	SelectAll key.Binding

	// Operations
	Install  key.Binding
	Add      key.Binding
	Delete   key.Binding
	Edit     key.Binding
	Toggle   key.Binding
	Update   key.Binding
	Sync     key.Binding
	SyncAll  key.Binding
	Test     key.Binding
	TestAll  key.Binding
	Refresh  key.Binding
	ExecTool key.Binding

	// Filters
	CycleFilter     key.Binding
	FilterAll       key.Binding
	FilterInstalled key.Binding
	FilterAvailable key.Binding
	FilterDisabled  key.Binding

	// Profiles
	ProfileSwitch key.Binding

	// UI
	ToggleLogs key.Binding
	Help       key.Binding
	Quit       key.Binding

	// Tabs
	NextTab key.Binding
	PrevTab key.Binding
	Tab1    key.Binding
	Tab2    key.Binding
	Tab3    key.Binding
	Tab4    key.Binding
	Tab5    key.Binding
	Tab6    key.Binding
}

// newKeyMap creates a new keyMap with all keybindings configured.
func newKeyMap() keyMap {
	return keyMap{
		// Navigation
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Top: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "bottom"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("ctrl+d", "page down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("ctrl+u"),
			key.WithHelp("ctrl+u", "page up"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "clear/back"),
		),

		// Selection
		Select: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "select"),
		),
		SelectAll: key.NewBinding(
			key.WithKeys("V"),
			key.WithHelp("V", "select all"),
		),

		// Operations
		Install: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "install"),
		),
		Add: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit"),
		),
		Toggle: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "toggle"),
		),
		Update: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "update"),
		),
		Sync: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "sync"),
		),
		SyncAll: key.NewBinding(
			key.WithKeys("S"),
			key.WithHelp("S", "sync all"),
		),
		Test: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "test"),
		),
		TestAll: key.NewBinding(
			key.WithKeys("T"),
			key.WithHelp("T", "test all"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		ExecTool: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "tools"),
		),

		// Filters
		CycleFilter: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "cycle filter"),
		),
		FilterAll: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "all"),
		),
		FilterInstalled: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "installed"),
		),
		FilterAvailable: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "available"),
		),
		FilterDisabled: key.NewBinding(
			key.WithKeys("4"),
			key.WithHelp("4", "disabled"),
		),

		// Profiles
		ProfileSwitch: key.NewBinding(
			key.WithKeys("P"),
			key.WithHelp("P", "switch profile"),
		),

		// UI
		ToggleLogs: key.NewBinding(
			key.WithKeys("L"),
			key.WithHelp("L", "toggle logs"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),

		// Tabs
		NextTab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("Tab", "next tab"),
		),
		PrevTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("Shift+Tab", "prev tab"),
		),
		Tab1: key.NewBinding(
			key.WithKeys("F1"),
			key.WithHelp("F1", "servers"),
		),
		Tab2: key.NewBinding(
			key.WithKeys("F2"),
			key.WithHelp("F2", "commands"),
		),
		Tab3: key.NewBinding(
			key.WithKeys("F3"),
			key.WithHelp("F3", "rules"),
		),
		Tab4: key.NewBinding(
			key.WithKeys("F4"),
			key.WithHelp("F4", "skills"),
		),
		Tab5: key.NewBinding(
			key.WithKeys("F5"),
			key.WithHelp("F5", "prompts"),
		),
		Tab6: key.NewBinding(
			key.WithKeys("F6"),
			key.WithHelp("F6", "hooks"),
		),
	}
}

// ShortHelp returns the key hints for the bottom bar.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Up,
		k.Down,
		k.Search,
		k.Install,
		k.Delete,
		k.Toggle,
		k.Sync,
		k.Help,
		k.Quit,
	}
}

// FullHelp returns the full list of keybindings for the help screen.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		// Navigation column
		{
			k.Up,
			k.Down,
			k.Top,
			k.Bottom,
			k.PageUp,
			k.PageDown,
			k.Search,
			k.Escape,
		},
		// Selection & Operations column
		{
			k.Select,
			k.SelectAll,
			k.Install,
			k.Delete,
			k.Edit,
			k.Toggle,
			k.Update,
		},
		// Sync & Test column
		{
			k.Sync,
			k.SyncAll,
			k.Test,
			k.TestAll,
			k.Refresh,
		},
		// Filters column
		{
			k.CycleFilter,
			k.FilterAll,
			k.FilterInstalled,
			k.FilterAvailable,
			k.FilterDisabled,
		},
		// UI column
		{
			k.ProfileSwitch,
			k.ToggleLogs,
			k.Help,
			k.Quit,
		},
	}
}
