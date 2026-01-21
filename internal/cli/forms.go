package cli

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// Poimandres color palette
// https://github.com/drcmda/poimandres-theme
var (
	// Base colors
	poimandresBg       = lipgloss.Color("#1b1e28")
	poimandresFg       = lipgloss.Color("#a6accd")
	poimandresFgMuted  = lipgloss.Color("#767c9d")
	poimandresFgSubtle = lipgloss.Color("#506477")
	poimandresPanel    = lipgloss.Color("#303340")

	// Accent colors
	poimandresTeal = lipgloss.Color("#5DE4c7")
	_              = lipgloss.Color("#89ddff") // poimandresCyan - reserved for future use
	poimandresPink = lipgloss.Color("#f087bd")
)

// isInteractive checks if stdin is a TTY (interactive terminal)
func isInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// requireInteractive checks if we're in an interactive terminal.
// If not, it prints a helpful message and returns an error.
func requireInteractive(cmdName string) error {
	if !isInteractive() {
		return fmt.Errorf("interactive mode requires a terminal\nUse flags instead: agentctl %s --help", cmdName)
	}
	return nil
}

// confirmCancel shows a confirmation prompt when the user tries to cancel.
// Returns true if the user confirms they want to cancel.
func confirmCancel() bool {
	var confirm bool
	huh.NewConfirm().
		Title("Cancel setup?").
		Affirmative("Yes").
		Negative("No").
		Value(&confirm).
		Run()
	return confirm
}

// showCancelHint prints a helpful message after cancellation
func showCancelHint(cmdName string) {
	fmt.Printf("Cancelled. Run 'agentctl %s --help' for non-interactive usage.\n", cmdName)
}

// runFormWithCancel runs a form and handles cancellation gracefully.
// Returns true if the form completed successfully, false if cancelled.
func runFormWithCancel(form *huh.Form, cmdName string) (bool, error) {
	err := form.Run()
	if err != nil {
		// Check if it's a user cancel (Ctrl+C / Esc)
		if err == huh.ErrUserAborted {
			if confirmCancel() {
				showCancelHint(cmdName)
				return false, nil
			}
			// User chose not to cancel, but we can't resume the form
			// So we'll need to return and let caller restart
			return false, fmt.Errorf("form interrupted - please try again")
		}
		return false, err
	}
	return true, nil
}

// formTheme returns a Poimandres-inspired theme for all agentctl forms
func formTheme() *huh.Theme {
	t := huh.ThemeBase()

	// Focused styles
	t.Focused.Base = t.Focused.Base.BorderForeground(poimandresTeal)
	t.Focused.Title = t.Focused.Title.Foreground(poimandresTeal)
	t.Focused.Description = t.Focused.Description.Foreground(poimandresFgMuted)
	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(poimandresPink)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(poimandresPink)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(poimandresTeal)
	t.Focused.Option = t.Focused.Option.Foreground(poimandresFg)
	t.Focused.MultiSelectSelector = t.Focused.MultiSelectSelector.Foreground(poimandresTeal)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(poimandresTeal)
	t.Focused.SelectedPrefix = t.Focused.SelectedPrefix.Foreground(poimandresTeal)
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.Foreground(poimandresFgMuted)
	t.Focused.UnselectedPrefix = t.Focused.UnselectedPrefix.Foreground(poimandresFgSubtle)
	t.Focused.FocusedButton = t.Focused.FocusedButton.Foreground(poimandresBg).Background(poimandresTeal)
	t.Focused.BlurredButton = t.Focused.BlurredButton.Foreground(poimandresFg).Background(poimandresPanel)
	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(poimandresTeal)
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(poimandresFgSubtle)
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(poimandresTeal)

	// Blurred styles
	t.Blurred.Base = t.Blurred.Base.BorderForeground(poimandresFgSubtle)
	t.Blurred.Title = t.Blurred.Title.Foreground(poimandresFgMuted)
	t.Blurred.Description = t.Blurred.Description.Foreground(poimandresFgSubtle)
	t.Blurred.TextInput.Placeholder = t.Blurred.TextInput.Placeholder.Foreground(poimandresFgSubtle)
	t.Blurred.TextInput.Prompt = t.Blurred.TextInput.Prompt.Foreground(poimandresFgMuted)

	return t
}

// newStyledForm creates a new form with consistent styling
func newStyledForm(groups ...*huh.Group) *huh.Form {
	return huh.NewForm(groups...).WithTheme(formTheme())
}
