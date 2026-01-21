package tui

import "github.com/charmbracelet/lipgloss"

// Poimandres color palette
// Reference: https://github.com/drcmda/poimandres-theme
var (
	// Base colors
	colorBg       = lipgloss.Color("#1b1e28")
	colorFg       = lipgloss.Color("#a6accd")
	colorFgMuted  = lipgloss.Color("#767c9d")
	colorFgSubtle = lipgloss.Color("#506477")

	// Accent colors
	colorTeal   = lipgloss.Color("#5DE4c7")
	colorCyan   = lipgloss.Color("#89ddff")
	colorBlue   = lipgloss.Color("#ADD7FF")
	colorPink   = lipgloss.Color("#f087bd")
	colorYellow = lipgloss.Color("#fffac2")

	// Derived colors for specific UI states
	_          = colorTeal   // colorSuccess - reserved for future use
	_          = colorYellow // colorWarning - reserved for future use
	colorError = colorPink
	_          = colorCyan // colorInfo - reserved for future use
)

// Status badge symbols
const (
	StatusInstalled = "●"
	StatusAvailable = "○"
	StatusDisabled  = "◌"
)

// Health indicator symbols
const (
	HealthHealthy   = "✓"
	HealthUnhealthy = "✗"
	HealthUnknown   = "?"
	HealthChecking  = "◐" // Can be animated with spinner
)

// Scope indicator symbols
const (
	ScopeLocalIndicator  = "[L]"
	ScopeGlobalIndicator = "[G]"
)

// App-level styles
var (
	// Main application container style
	AppStyle = lipgloss.NewStyle().
			Background(colorBg).
			Foreground(colorFg)

	// Header style for the top bar
	HeaderStyle = lipgloss.NewStyle().
			Background(colorBg).
			Foreground(colorFg).
			Bold(true).
			Padding(0, 1)

	// Title style within header
	HeaderTitleStyle = lipgloss.NewStyle().
				Foreground(colorTeal).
				Bold(true)

	// Subtitle/version style
	HeaderSubtitleStyle = lipgloss.NewStyle().
				Foreground(colorFgMuted)
)

// Status badge styles
var (
	// Installed status badge (●) - teal/green
	StatusInstalledStyle = lipgloss.NewStyle().
				Foreground(colorTeal).
				Bold(true)

	// Available status badge (○) - muted
	StatusAvailableStyle = lipgloss.NewStyle().
				Foreground(colorFgMuted)

	// Disabled status badge (◌) - subtle/dim
	StatusDisabledStyle = lipgloss.NewStyle().
				Foreground(colorFgSubtle)
)

// Health indicator styles
var (
	// Healthy indicator (✓) - teal/green
	HealthHealthyStyle = lipgloss.NewStyle().
				Foreground(colorTeal).
				Bold(true)

	// Unhealthy indicator (✗) - pink/error
	HealthUnhealthyStyle = lipgloss.NewStyle().
				Foreground(colorPink).
				Bold(true)

	// Unknown indicator (?) - yellow/warning
	HealthUnknownStyle = lipgloss.NewStyle().
				Foreground(colorYellow)

	// Checking indicator (spinner) - cyan
	HealthCheckingStyle = lipgloss.NewStyle().
				Foreground(colorCyan)
)

// Scope indicator styles
var (
	// Local scope indicator [L] - teal (project-specific)
	ScopeLocalStyle = lipgloss.NewStyle().
			Foreground(colorTeal)

	// Global scope indicator [G] - muted (user-wide)
	ScopeGlobalStyle = lipgloss.NewStyle().
				Foreground(colorFgMuted)
)

// List item styles
var (
	// Normal list item
	ListItemNormalStyle = lipgloss.NewStyle().
				Foreground(colorFg).
				Padding(0, 1)

	// Selected list item
	ListItemSelectedStyle = lipgloss.NewStyle().
				Foreground(colorCyan).
				Background(lipgloss.Color("#252836")).
				Bold(true).
				Padding(0, 1)

	// Dimmed/inactive list item
	ListItemDimmedStyle = lipgloss.NewStyle().
				Foreground(colorFgSubtle).
				Padding(0, 1)

	// List item name (primary text)
	ListItemNameStyle = lipgloss.NewStyle().
				Foreground(colorFg)

	// List item name when selected
	ListItemNameSelectedStyle = lipgloss.NewStyle().
					Foreground(colorCyan).
					Bold(true)

	// List item description (secondary text)
	ListItemDescStyle = lipgloss.NewStyle().
				Foreground(colorFgMuted)

	// List item description when selected
	ListItemDescSelectedStyle = lipgloss.NewStyle().
					Foreground(colorBlue)

	// Cursor/selection indicator
	ListCursorStyle = lipgloss.NewStyle().
			Foreground(colorTeal).
			Bold(true)
)

// Log panel styles
var (
	// Log panel container
	LogPanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorFgSubtle).
			Padding(0, 1)

	// Log panel title
	LogPanelTitleStyle = lipgloss.NewStyle().
				Foreground(colorBlue).
				Bold(true).
				Padding(0, 1)

	// Log entry styles by level
	LogEntryInfoStyle = lipgloss.NewStyle().
				Foreground(colorFg)

	LogEntryDebugStyle = lipgloss.NewStyle().
				Foreground(colorFgMuted)

	LogEntryWarnStyle = lipgloss.NewStyle().
				Foreground(colorYellow)

	LogEntryErrorStyle = lipgloss.NewStyle().
				Foreground(colorPink)

	// Log timestamp
	LogTimestampStyle = lipgloss.NewStyle().
				Foreground(colorFgSubtle)

	// Log source/component
	LogSourceStyle = lipgloss.NewStyle().
			Foreground(colorCyan)
)

// Key hints bar styles
var (
	// Key hints bar container (bottom bar)
	KeyHintsBarStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#252836")).
				Padding(0, 1)

	// Key binding (e.g., "j/k")
	KeyStyle = lipgloss.NewStyle().
			Foreground(colorTeal).
			Bold(true)

	// Key description (e.g., "up/down")
	KeyDescStyle = lipgloss.NewStyle().
			Foreground(colorFgMuted)

	// Separator between key hints
	KeyHintSeparatorStyle = lipgloss.NewStyle().
				Foreground(colorFgSubtle)
)

// Modal/overlay styles
var (
	// Modal backdrop (dimmed background)
	ModalBackdropStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#1b1e28")).
				Foreground(colorFgSubtle)

	// Modal container
	ModalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorCyan).
			Background(colorBg).
			Padding(1, 2)

	// Modal title
	ModalTitleStyle = lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true).
			Align(lipgloss.Center).
			Padding(0, 0, 1, 0)

	// Modal body text
	ModalBodyStyle = lipgloss.NewStyle().
			Foreground(colorFg)

	// Modal button (normal)
	ModalButtonStyle = lipgloss.NewStyle().
				Foreground(colorFgMuted).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorFgSubtle).
				Padding(0, 2)

	// Modal button (focused/selected)
	ModalButtonActiveStyle = lipgloss.NewStyle().
				Foreground(colorBg).
				Background(colorTeal).
				Bold(true).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorTeal).
				Padding(0, 2)

	// Modal button (danger/destructive)
	ModalButtonDangerStyle = lipgloss.NewStyle().
				Foreground(colorBg).
				Background(colorPink).
				Bold(true).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorPink).
				Padding(0, 2)
)

// Tab styles (for switching between views)
var (
	// Tab bar container
	TabBarStyle = lipgloss.NewStyle().
			BorderBottom(true).
			BorderForeground(colorFgSubtle).
			Padding(0, 1)

	// Inactive tab
	TabStyle = lipgloss.NewStyle().
			Foreground(colorFgMuted).
			Padding(0, 2)

	// Active tab
	TabActiveStyle = lipgloss.NewStyle().
			Foreground(colorTeal).
			Bold(true).
			BorderBottom(true).
			BorderForeground(colorTeal).
			Padding(0, 2)
)

// Search/filter styles
var (
	// Search input container
	SearchStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorCyan).
			Padding(0, 1)

	// Search prompt (e.g., "/")
	SearchPromptStyle = lipgloss.NewStyle().
				Foreground(colorTeal).
				Bold(true)

	// Search input text
	SearchInputStyle = lipgloss.NewStyle().
				Foreground(colorFg)

	// Search placeholder
	SearchPlaceholderStyle = lipgloss.NewStyle().
				Foreground(colorFgSubtle)

	// Match highlight in search results
	SearchMatchStyle = lipgloss.NewStyle().
				Foreground(colorYellow).
				Bold(true)
)

// Progress/loading styles
var (
	// Progress bar track
	ProgressTrackStyle = lipgloss.NewStyle().
				Foreground(colorFgSubtle)

	// Progress bar fill
	ProgressFillStyle = lipgloss.NewStyle().
				Foreground(colorTeal)

	// Spinner style
	SpinnerStyle = lipgloss.NewStyle().
			Foreground(colorCyan)

	// Loading text
	LoadingTextStyle = lipgloss.NewStyle().
				Foreground(colorFgMuted).
				Italic(true)
)

// Notification/message styles
var (
	// Success message
	SuccessStyle = lipgloss.NewStyle().
			Foreground(colorTeal).
			Bold(true)

	// Warning message
	WarningStyle = lipgloss.NewStyle().
			Foreground(colorYellow).
			Bold(true)

	// Error message
	ErrorStyle = lipgloss.NewStyle().
			Foreground(colorPink).
			Bold(true)

	// Info message
	InfoStyle = lipgloss.NewStyle().
			Foreground(colorCyan)
)

// Detail pane styles (right panel showing server details)
var (
	// Detail pane container
	DetailPaneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorFgSubtle).
			Padding(1, 2)

	// Detail pane title (server name)
	DetailTitleStyle = lipgloss.NewStyle().
				Foreground(colorCyan).
				Bold(true)

	// Detail section header
	DetailSectionStyle = lipgloss.NewStyle().
				Foreground(colorBlue).
				Bold(true).
				MarginTop(1)

	// Detail label (e.g., "Version:", "Author:")
	DetailLabelStyle = lipgloss.NewStyle().
				Foreground(colorFgMuted)

	// Detail value
	DetailValueStyle = lipgloss.NewStyle().
				Foreground(colorFg)

	// Detail URL/link
	DetailLinkStyle = lipgloss.NewStyle().
			Foreground(colorCyan).
			Underline(true)
)

// Helper functions for common style operations

// RenderStatusBadge returns a styled status badge
func RenderStatusBadge(installed, enabled bool) string {
	if !installed {
		return StatusAvailableStyle.Render(StatusAvailable)
	}
	if !enabled {
		return StatusDisabledStyle.Render(StatusDisabled)
	}
	return StatusInstalledStyle.Render(StatusInstalled)
}

// RenderHealthIndicator returns a styled health indicator
func RenderHealthIndicator(healthy, checking bool) string {
	if checking {
		return HealthCheckingStyle.Render(HealthChecking)
	}
	if healthy {
		return HealthHealthyStyle.Render(HealthHealthy)
	}
	return HealthUnhealthyStyle.Render(HealthUnhealthy)
}

// RenderKeyHint returns a formatted key hint (e.g., "j/k up/down")
func RenderKeyHint(key, desc string) string {
	return KeyStyle.Render(key) + " " + KeyDescStyle.Render(desc)
}

// RenderKeyHintsBar renders a complete key hints bar with multiple hints
func RenderKeyHintsBar(hints []struct{ Key, Desc string }) string {
	var rendered []string
	for _, h := range hints {
		rendered = append(rendered, RenderKeyHint(h.Key, h.Desc))
	}

	separator := KeyHintSeparatorStyle.Render(" │ ")
	result := ""
	for i, r := range rendered {
		if i > 0 {
			result += separator
		}
		result += r
	}

	return KeyHintsBarStyle.Render(result)
}

// RenderScopeIndicator returns a styled scope indicator [L] or [G]
func RenderScopeIndicator(scope string) string {
	if scope == "local" {
		return ScopeLocalStyle.Render(ScopeLocalIndicator)
	}
	return ScopeGlobalStyle.Render(ScopeGlobalIndicator)
}
