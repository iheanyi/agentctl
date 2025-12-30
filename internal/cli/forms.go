package cli

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"
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

// formTheme returns the standard theme for all agentctl forms
func formTheme() *huh.Theme {
	return huh.ThemeDracula()
}

// newStyledForm creates a new form with consistent styling
func newStyledForm(groups ...*huh.Group) *huh.Form {
	return huh.NewForm(groups...).WithTheme(formTheme())
}
