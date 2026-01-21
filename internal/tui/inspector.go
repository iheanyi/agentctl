package tui

import (
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"

	"github.com/iheanyi/agentctl/pkg/inspectable"
)

// InspectorModel handles the display of resource details in a modal.
// It uses the Inspectable interface so each resource type renders itself,
// avoiding type switches and interface{} code smells.
type InspectorModel struct {
	viewport viewport.Model
	resource inspectable.Inspectable
	title    string
	width    int
	height   int
}

// NewInspector creates a new InspectorModel for displaying the given resource.
func NewInspector(resource inspectable.Inspectable, width, height int) InspectorModel {
	// Calculate viewport dimensions (account for modal border and padding)
	vpWidth := width - 8
	vpHeight := height - 10

	// Ensure minimum dimensions
	if vpWidth < 40 {
		vpWidth = 40
	}
	if vpHeight < 10 {
		vpHeight = 10
	}

	vp := viewport.New(vpWidth, vpHeight)
	vp.Style = lipgloss.NewStyle().
		Foreground(colorFg)

	// Word-wrap the content to fit the viewport width
	content := resource.InspectContent()
	wrappedContent := lipgloss.NewStyle().
		Width(vpWidth - 2).
		Render(content)

	vp.SetContent(wrappedContent)

	return InspectorModel{
		viewport: vp,
		resource: resource,
		title:    resource.InspectTitle(),
		width:    width,
		height:   height,
	}
}

// Title returns the modal title from the resource
func (m InspectorModel) Title() string {
	return m.title
}

// View renders the inspector viewport content
func (m InspectorModel) View() string {
	return m.viewport.View()
}

// SetSize updates the viewport dimensions
func (m *InspectorModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	vpWidth := width - 8
	vpHeight := height - 10

	if vpWidth < 40 {
		vpWidth = 40
	}
	if vpHeight < 10 {
		vpHeight = 10
	}

	m.viewport.Width = vpWidth
	m.viewport.Height = vpHeight

	// Re-wrap content for new width
	if m.resource != nil {
		content := m.resource.InspectContent()
		wrappedContent := lipgloss.NewStyle().
			Width(vpWidth - 2).
			Render(content)
		m.viewport.SetContent(wrappedContent)
	}
}

// ScrollUp scrolls the viewport up
func (m *InspectorModel) ScrollUp() {
	m.viewport.ScrollUp(1)
}

// ScrollDown scrolls the viewport down
func (m *InspectorModel) ScrollDown() {
	m.viewport.ScrollDown(1)
}

// PageUp scrolls the viewport up by a page
func (m *InspectorModel) PageUp() {
	m.viewport.PageUp()
}

// PageDown scrolls the viewport down by a page
func (m *InspectorModel) PageDown() {
	m.viewport.PageDown()
}

// ScrollToTop scrolls to the top of the content
func (m *InspectorModel) ScrollToTop() {
	m.viewport.GotoTop()
}

// ScrollToBottom scrolls to the bottom of the content
func (m *InspectorModel) ScrollToBottom() {
	m.viewport.GotoBottom()
}

// AtTop returns true if the viewport is scrolled to the top
func (m InspectorModel) AtTop() bool {
	return m.viewport.AtTop()
}

// AtBottom returns true if the viewport is scrolled to the bottom
func (m InspectorModel) AtBottom() bool {
	return m.viewport.AtBottom()
}

// ScrollPercent returns the scroll position as a percentage
func (m InspectorModel) ScrollPercent() float64 {
	return m.viewport.ScrollPercent()
}
