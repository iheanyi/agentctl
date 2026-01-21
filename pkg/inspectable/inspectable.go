package inspectable

// Inspectable represents a resource that can be displayed in an inspector modal.
// Each resource type implements this interface to render itself, avoiding
// type switches and interface{} code smells in the TUI.
type Inspectable interface {
	// InspectTitle returns the display name for the modal header
	InspectTitle() string

	// InspectContent returns the formatted content for the viewport
	InspectContent() string
}
