package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/iheanyi/agentctl/pkg/aliases"
)

// AliasEntry represents an alias in the list
type AliasEntry struct {
	Name        string
	Alias       aliases.Alias
	IsBundled   bool
	IsExpanded  bool
}

// AliasManagerModel is the Bubble Tea model for managing bundled aliases
type AliasManagerModel struct {
	// Data
	entries       []AliasEntry
	filtered      []AliasEntry
	bundledPath   string

	// State
	cursor        int
	searchQuery   string
	searchInput   textinput.Model
	searching     bool

	// UI state
	width         int
	height        int
	showHelp      bool
	quitting      bool
	statusMsg     string
	statusIsError bool

	// Mode
	mode          aliasManagerMode
	editEntry     *AliasEntry
	confirmDelete string
}

type aliasManagerMode int

const (
	modeList aliasManagerMode = iota
	modeSearch
	modeConfirmDelete
)

// AliasManagerKeyMap defines key bindings
type AliasManagerKeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Add      key.Binding
	Edit     key.Binding
	Delete   key.Binding
	Test     key.Binding
	Validate key.Binding
	Search   key.Binding
	Escape   key.Binding
	Enter    key.Binding
	Help     key.Binding
	Quit     key.Binding
	Yes      key.Binding
	No       key.Binding
}

var aliasManagerKeys = AliasManagerKeyMap{
	Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("k/up", "up")),
	Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("j/down", "down")),
	Add:      key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add")),
	Edit:     key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
	Delete:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
	Test:     key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "test")),
	Validate: key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "validate")),
	Search:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	Escape:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	Enter:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
	Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Yes:      key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "yes")),
	No:       key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "no")),
}

// NewAliasManager creates a new alias manager model
func NewAliasManager() *AliasManagerModel {
	ti := textinput.New()
	ti.Placeholder = "Search aliases..."
	ti.CharLimit = 50

	m := &AliasManagerModel{
		searchInput: ti,
		bundledPath: getBundledAliasesPath(),
	}
	m.loadAliases()
	m.applyFilter()
	return m
}

func getBundledAliasesPath() string {
	// Get the path to the bundled aliases.json
	// This is in the source tree at pkg/aliases/aliases.json
	// For development, we'll use a relative path or env var
	if path := os.Getenv("AGENTCTL_ALIASES_PATH"); path != "" {
		return path
	}

	// Try to find it relative to current working directory
	cwd, _ := os.Getwd()
	candidates := []string{
		filepath.Join(cwd, "pkg", "aliases", "aliases.json"),
		filepath.Join(cwd, "..", "pkg", "aliases", "aliases.json"),
		filepath.Join(cwd, "..", "..", "pkg", "aliases", "aliases.json"),
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return "pkg/aliases/aliases.json"
}

func (m *AliasManagerModel) loadAliases() {
	m.entries = nil

	// Load from file directly (not embedded) so we can edit
	data, err := os.ReadFile(m.bundledPath)
	if err != nil {
		m.statusMsg = fmt.Sprintf("Error loading aliases: %v", err)
		m.statusIsError = true
		return
	}

	var raw map[string]aliases.Alias
	if err := json.Unmarshal(data, &raw); err != nil {
		m.statusMsg = fmt.Sprintf("Error parsing aliases: %v", err)
		m.statusIsError = true
		return
	}

	for name, alias := range raw {
		m.entries = append(m.entries, AliasEntry{
			Name:      name,
			Alias:     alias,
			IsBundled: true,
		})
	}

	// Sort by name
	sort.Slice(m.entries, func(i, j int) bool {
		return m.entries[i].Name < m.entries[j].Name
	})
}

func (m *AliasManagerModel) saveAliases() error {
	raw := make(map[string]aliases.Alias)
	for _, entry := range m.entries {
		raw[entry.Name] = entry.Alias
	}

	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.bundledPath, data, 0644)
}

func (m *AliasManagerModel) applyFilter() {
	if m.searchQuery == "" {
		m.filtered = m.entries
		return
	}

	query := strings.ToLower(m.searchQuery)
	m.filtered = nil
	for _, entry := range m.entries {
		if strings.Contains(strings.ToLower(entry.Name), query) ||
			strings.Contains(strings.ToLower(entry.Alias.Description), query) {
			m.filtered = append(m.filtered, entry)
		}
	}

	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

func (m *AliasManagerModel) Init() tea.Cmd {
	return nil
}

func (m *AliasManagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		// Handle mode-specific keys
		switch m.mode {
		case modeSearch:
			return m.updateSearch(msg)
		case modeConfirmDelete:
			return m.updateConfirmDelete(msg)
		}

		// List mode keys
		switch {
		case key.Matches(msg, aliasManagerKeys.Quit):
			m.quitting = true
			return m, tea.Quit

		case key.Matches(msg, aliasManagerKeys.Up):
			if m.cursor > 0 {
				m.cursor--
			}

		case key.Matches(msg, aliasManagerKeys.Down):
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}

		case key.Matches(msg, aliasManagerKeys.Search):
			m.mode = modeSearch
			m.searchInput.Focus()
			return m, textinput.Blink

		case key.Matches(msg, aliasManagerKeys.Add):
			return m, m.runAddForm()

		case key.Matches(msg, aliasManagerKeys.Edit):
			if len(m.filtered) > 0 {
				return m, m.runEditForm()
			}

		case key.Matches(msg, aliasManagerKeys.Delete):
			if len(m.filtered) > 0 {
				m.confirmDelete = m.filtered[m.cursor].Name
				m.mode = modeConfirmDelete
			}

		case key.Matches(msg, aliasManagerKeys.Test):
			if len(m.filtered) > 0 {
				return m, m.runTest()
			}

		case key.Matches(msg, aliasManagerKeys.Validate):
			return m, m.runValidateAll()

		case key.Matches(msg, aliasManagerKeys.Help):
			m.showHelp = !m.showHelp

		case key.Matches(msg, aliasManagerKeys.Enter):
			if len(m.filtered) > 0 {
				// Toggle expanded view
				m.filtered[m.cursor].IsExpanded = !m.filtered[m.cursor].IsExpanded
			}
		}

	case addAliasMsg:
		m.entries = append(m.entries, AliasEntry{
			Name:      msg.name,
			Alias:     msg.alias,
			IsBundled: true,
		})
		sort.Slice(m.entries, func(i, j int) bool {
			return m.entries[i].Name < m.entries[j].Name
		})
		m.applyFilter()
		if err := m.saveAliases(); err != nil {
			m.statusMsg = fmt.Sprintf("Error saving: %v", err)
			m.statusIsError = true
		} else {
			m.statusMsg = fmt.Sprintf("Added %q", msg.name)
			m.statusIsError = false
		}

	case editAliasMsg:
		for i, entry := range m.entries {
			if entry.Name == msg.oldName {
				if msg.oldName != msg.name {
					// Name changed, remove old and add new
					m.entries = append(m.entries[:i], m.entries[i+1:]...)
					m.entries = append(m.entries, AliasEntry{
						Name:      msg.name,
						Alias:     msg.alias,
						IsBundled: true,
					})
				} else {
					m.entries[i].Alias = msg.alias
				}
				break
			}
		}
		sort.Slice(m.entries, func(i, j int) bool {
			return m.entries[i].Name < m.entries[j].Name
		})
		m.applyFilter()
		if err := m.saveAliases(); err != nil {
			m.statusMsg = fmt.Sprintf("Error saving: %v", err)
			m.statusIsError = true
		} else {
			m.statusMsg = fmt.Sprintf("Updated %q", msg.name)
			m.statusIsError = false
		}

	case testResultMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Test failed: %v", msg.err)
			m.statusIsError = true
		} else {
			m.statusMsg = fmt.Sprintf("Test passed: %s", msg.name)
			m.statusIsError = false
		}

	case validateResultMsg:
		if len(msg.errors) > 0 {
			m.statusMsg = fmt.Sprintf("Validation: %d errors found", len(msg.errors))
			m.statusIsError = true
		} else {
			m.statusMsg = fmt.Sprintf("Validation passed: %d aliases OK", msg.count)
			m.statusIsError = false
		}

	case formCancelledMsg:
		m.statusMsg = "Cancelled"
		m.statusIsError = false
	}

	return m, tea.Batch(cmds...)
}

func (m *AliasManagerModel) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, aliasManagerKeys.Escape):
		m.mode = modeList
		m.searchQuery = ""
		m.searchInput.SetValue("")
		m.applyFilter()
		return m, nil

	case key.Matches(msg, aliasManagerKeys.Enter):
		m.mode = modeList
		m.searchQuery = m.searchInput.Value()
		m.applyFilter()
		return m, nil
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	m.searchQuery = m.searchInput.Value()
	m.applyFilter()
	return m, cmd
}

func (m *AliasManagerModel) updateConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, aliasManagerKeys.Yes):
		// Delete the alias
		for i, entry := range m.entries {
			if entry.Name == m.confirmDelete {
				m.entries = append(m.entries[:i], m.entries[i+1:]...)
				break
			}
		}
		m.applyFilter()
		if m.cursor >= len(m.filtered) {
			m.cursor = max(0, len(m.filtered)-1)
		}
		if err := m.saveAliases(); err != nil {
			m.statusMsg = fmt.Sprintf("Error saving: %v", err)
			m.statusIsError = true
		} else {
			m.statusMsg = fmt.Sprintf("Deleted %q", m.confirmDelete)
			m.statusIsError = false
		}
		m.confirmDelete = ""
		m.mode = modeList

	case key.Matches(msg, aliasManagerKeys.No), key.Matches(msg, aliasManagerKeys.Escape):
		m.confirmDelete = ""
		m.mode = modeList
	}

	return m, nil
}

func (m *AliasManagerModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6")).
		MarginBottom(1)
	b.WriteString(headerStyle.Render("Bundled Aliases Manager"))
	b.WriteString("\n")

	// Search bar
	if m.mode == modeSearch {
		b.WriteString(m.searchInput.View())
		b.WriteString("\n")
	} else if m.searchQuery != "" {
		filterStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		b.WriteString(filterStyle.Render(fmt.Sprintf("Filter: %s", m.searchQuery)))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// List
	listHeight := m.height - 10
	if listHeight < 5 {
		listHeight = 5
	}

	if len(m.filtered) == 0 {
		emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
		b.WriteString(emptyStyle.Render("  No aliases found"))
		b.WriteString("\n")
	} else {
		// Calculate visible range
		start := 0
		if m.cursor >= listHeight {
			start = m.cursor - listHeight + 1
		}
		end := start + listHeight
		if end > len(m.filtered) {
			end = len(m.filtered)
		}

		for i := start; i < end; i++ {
			entry := m.filtered[i]
			line := m.renderEntry(entry, i == m.cursor)
			b.WriteString(line)
			b.WriteString("\n")

			// Show expanded details
			if entry.IsExpanded && i == m.cursor {
				details := m.renderDetails(entry)
				b.WriteString(details)
			}
		}
	}

	// Confirm delete dialog
	if m.mode == modeConfirmDelete {
		b.WriteString("\n")
		confirmStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			Bold(true)
		b.WriteString(confirmStyle.Render(fmt.Sprintf("Delete %q? (y/n)", m.confirmDelete)))
		b.WriteString("\n")
	}

	// Status
	b.WriteString("\n")
	if m.statusMsg != "" {
		var statusStyle lipgloss.Style
		if m.statusIsError {
			statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
		} else {
			statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
		}
		b.WriteString(statusStyle.Render(m.statusMsg))
		b.WriteString("\n")
	}

	// Help
	b.WriteString("\n")
	if m.showHelp {
		b.WriteString(m.renderFullHelp())
	} else {
		helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		b.WriteString(helpStyle.Render("a:add e:edit d:delete t:test v:validate /:search ?:help q:quit"))
	}

	// File path
	b.WriteString("\n")
	pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
	b.WriteString(pathStyle.Render(fmt.Sprintf("File: %s", m.bundledPath)))

	return b.String()
}

func (m *AliasManagerModel) renderEntry(entry AliasEntry, selected bool) string {
	var style lipgloss.Style
	if selected {
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("6")).
			Bold(true)
	} else {
		style = lipgloss.NewStyle()
	}

	// Determine transport indicator
	transport := "stdio"
	if entry.Alias.Transport != "" {
		transport = entry.Alias.Transport
	} else if entry.Alias.MCPURL != "" {
		transport = "http"
	} else if len(entry.Alias.Variants) > 0 {
		// Check default variant
		if entry.Alias.DefaultVariant != "" {
			if v, ok := entry.Alias.Variants[entry.Alias.DefaultVariant]; ok && v.Transport != "" {
				transport = v.Transport
			}
		}
	}

	cursor := "  "
	if selected {
		cursor = "> "
	}

	// Truncate description
	desc := entry.Alias.Description
	maxDesc := 40
	if len(desc) > maxDesc {
		desc = desc[:maxDesc-3] + "..."
	}

	line := fmt.Sprintf("%s%-16s %-6s %s", cursor, entry.Name, transport, desc)
	return style.Render(line)
}

func (m *AliasManagerModel) renderDetails(entry AliasEntry) string {
	detailStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		MarginLeft(4)

	var lines []string

	if entry.Alias.Description != "" {
		lines = append(lines, fmt.Sprintf("Description: %s", entry.Alias.Description))
	}
	if entry.Alias.Package != "" {
		lines = append(lines, fmt.Sprintf("Package: %s", entry.Alias.Package))
	}
	if entry.Alias.Runtime != "" {
		lines = append(lines, fmt.Sprintf("Runtime: %s", entry.Alias.Runtime))
	}
	if entry.Alias.MCPURL != "" {
		lines = append(lines, fmt.Sprintf("URL: %s", entry.Alias.MCPURL))
	}
	if entry.Alias.URL != "" {
		lines = append(lines, fmt.Sprintf("Git URL: %s", entry.Alias.URL))
	}
	if len(entry.Alias.Variants) > 0 {
		var variants []string
		for name := range entry.Alias.Variants {
			if name == entry.Alias.DefaultVariant {
				variants = append(variants, name+"*")
			} else {
				variants = append(variants, name)
			}
		}
		lines = append(lines, fmt.Sprintf("Variants: %s", strings.Join(variants, ", ")))
	}

	var result strings.Builder
	for _, line := range lines {
		result.WriteString(detailStyle.Render(line))
		result.WriteString("\n")
	}
	return result.String()
}

func (m *AliasManagerModel) renderFullHelp() string {
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	help := `Keyboard Shortcuts:
  j/k, up/down  Navigate list
  Enter         Toggle details
  a             Add new alias
  e             Edit selected alias
  d             Delete selected alias
  t             Test selected alias
  v             Validate all aliases
  /             Search/filter
  ?             Toggle help
  q             Quit`

	return helpStyle.Render(help)
}

// Messages

type addAliasMsg struct {
	name  string
	alias aliases.Alias
}

type editAliasMsg struct {
	oldName string
	name    string
	alias   aliases.Alias
}

type testResultMsg struct {
	name string
	err  error
}

type validateResultMsg struct {
	count  int
	errors []string
}

type formCancelledMsg struct{}

// Commands

func (m *AliasManagerModel) runAddForm() tea.Cmd {
	return func() tea.Msg {
		name, alias, err := runAliasForm("", aliases.Alias{})
		if err != nil {
			return formCancelledMsg{}
		}
		return addAliasMsg{name: name, alias: alias}
	}
}

func (m *AliasManagerModel) runEditForm() tea.Cmd {
	entry := m.filtered[m.cursor]
	return func() tea.Msg {
		name, alias, err := runAliasForm(entry.Name, entry.Alias)
		if err != nil {
			return formCancelledMsg{}
		}
		return editAliasMsg{oldName: entry.Name, name: name, alias: alias}
	}
}

func (m *AliasManagerModel) runTest() tea.Cmd {
	entry := m.filtered[m.cursor]
	return func() tea.Msg {
		err := testAlias(entry.Name, entry.Alias)
		return testResultMsg{name: entry.Name, err: err}
	}
}

func (m *AliasManagerModel) runValidateAll() tea.Cmd {
	entries := m.entries
	return func() tea.Msg {
		var errors []string
		for _, entry := range entries {
			if err := validateAlias(entry.Name, entry.Alias); err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", entry.Name, err))
			}
		}
		return validateResultMsg{count: len(entries), errors: errors}
	}
}

// Helper functions

func testAlias(name string, alias aliases.Alias) error {
	// For now, just validate the structure
	// In the future, we could actually try to spawn the server
	return validateAlias(name, alias)
}

func validateAlias(name string, alias aliases.Alias) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}

	hasPackage := alias.Package != ""
	hasURL := alias.MCPURL != ""
	hasGitURL := alias.URL != ""
	hasVariants := len(alias.Variants) > 0

	if !hasPackage && !hasURL && !hasGitURL && !hasVariants {
		return fmt.Errorf("must have package, url, git url, or variants")
	}

	// Validate variants
	for vname, variant := range alias.Variants {
		if variant.Transport == "" && variant.Package == "" && variant.MCPURL == "" {
			return fmt.Errorf("variant %q is empty", vname)
		}
	}

	return nil
}

// runAliasForm runs a huh form for adding/editing an alias
// This is called in a tea.Cmd so it runs outside the main tea loop
func runAliasForm(existingName string, existing aliases.Alias) (string, aliases.Alias, error) {
	var result *AliasFormResult
	var err error

	if existingName == "" {
		result, err = RunAliasAddForm()
	} else {
		result, err = RunAliasEditForm(existingName, existing)
	}

	if err != nil {
		return "", aliases.Alias{}, err
	}

	return result.Name, result.Alias, nil
}

// RunAliasManager runs the alias manager TUI
func RunAliasManager() error {
	// Check if we can find the aliases file
	path := getBundledAliasesPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("bundled aliases file not found at %s\nRun from the agentctl source directory or set AGENTCTL_ALIASES_PATH", path)
	}

	m := NewAliasManager()
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

