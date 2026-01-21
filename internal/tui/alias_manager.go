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
	Name       string
	Alias      aliases.Alias
	IsBundled  bool
	IsExpanded bool
}

// AliasManagerModel is the Bubble Tea model for managing bundled aliases
type AliasManagerModel struct {
	// Data
	entries     []AliasEntry
	filtered    []AliasEntry
	bundledPath string

	// State
	cursor      int
	searchQuery string
	searchInput textinput.Model

	// UI state
	width         int
	height        int
	showHelp      bool
	quitting      bool
	statusMsg     string
	statusIsError bool

	// Mode
	mode          aliasManagerMode
	confirmDelete string

	// Wizard state
	wizardStep            int  // 0=basic, 1=type, 2=config, 3=giturl
	wizardIsNew           bool // true for add, false for edit
	wizardFocus           int  // current focused field within step
	wizardName            textinput.Model
	wizardDesc            textinput.Model
	wizardConfigType      int // 0=simple, 1=variants
	wizardTransport       int // 0=stdio, 1=http, 2=sse
	wizardRuntime         int // 0=node, 1=python, 2=go, 3=docker
	wizardPackage         textinput.Model
	wizardURL             textinput.Model
	wizardHasLocal        bool
	wizardHasRemote       bool
	wizardLocalRuntime    int // 0=node, 1=python
	wizardLocalPackage    textinput.Model
	wizardRemoteTransport int // 0=http, 1=sse
	wizardRemoteURL       textinput.Model
	wizardDefaultVariant  int // 0=local, 1=remote
	wizardWantGitURL      bool
	wizardGitURL          textinput.Model
	wizardExistingName    string // for editing - the original name
}

type aliasManagerMode int

const (
	modeList aliasManagerMode = iota
	modeSearch
	modeConfirmDelete
	modeWizard
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

	// Wizard textinputs
	wizardName := textinput.New()
	wizardName.Placeholder = "my-alias"
	wizardName.CharLimit = 50

	wizardDesc := textinput.New()
	wizardDesc.Placeholder = "Description of what this server does"
	wizardDesc.CharLimit = 200

	wizardPackage := textinput.New()
	wizardPackage.Placeholder = "@org/mcp-server"
	wizardPackage.CharLimit = 200

	wizardURL := textinput.New()
	wizardURL.Placeholder = "https://mcp.example.com/mcp"
	wizardURL.CharLimit = 300

	wizardLocalPackage := textinput.New()
	wizardLocalPackage.Placeholder = "@org/mcp-server"
	wizardLocalPackage.CharLimit = 200

	wizardRemoteURL := textinput.New()
	wizardRemoteURL.Placeholder = "https://mcp.example.com/mcp"
	wizardRemoteURL.CharLimit = 300

	wizardGitURL := textinput.New()
	wizardGitURL.Placeholder = "github.com/org/repo"
	wizardGitURL.CharLimit = 200

	m := &AliasManagerModel{
		searchInput:        ti,
		bundledPath:        getBundledAliasesPath(),
		wizardName:         wizardName,
		wizardDesc:         wizardDesc,
		wizardPackage:      wizardPackage,
		wizardURL:          wizardURL,
		wizardLocalPackage: wizardLocalPackage,
		wizardRemoteURL:    wizardRemoteURL,
		wizardGitURL:       wizardGitURL,
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
		case modeWizard:
			return m.updateWizard(msg)
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
			m.openWizard(true, nil)
			return m, textinput.Blink

		case key.Matches(msg, aliasManagerKeys.Edit):
			if len(m.filtered) > 0 {
				entry := m.filtered[m.cursor]
				m.openWizard(false, &entry)
				return m, textinput.Blink
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
		m.mode = modeList // Close wizard
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
		m.mode = modeList // Close wizard
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

	// Render wizard modal if active
	if m.mode == modeWizard {
		return m.renderWizard()
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

// Commands

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

// Wizard functions

// openWizard initializes the wizard for add or edit mode
func (m *AliasManagerModel) openWizard(isNew bool, entry *AliasEntry) {
	m.mode = modeWizard
	m.wizardIsNew = isNew
	m.wizardStep = 0
	m.wizardFocus = 0

	// Reset all fields
	m.wizardName.SetValue("")
	m.wizardDesc.SetValue("")
	m.wizardConfigType = 0
	m.wizardTransport = 0
	m.wizardRuntime = 0
	m.wizardPackage.SetValue("")
	m.wizardURL.SetValue("")
	m.wizardHasLocal = false
	m.wizardHasRemote = false
	m.wizardLocalRuntime = 0
	m.wizardLocalPackage.SetValue("")
	m.wizardRemoteTransport = 0
	m.wizardRemoteURL.SetValue("")
	m.wizardDefaultVariant = 0
	m.wizardWantGitURL = false
	m.wizardGitURL.SetValue("")
	m.wizardExistingName = ""

	// Blur all inputs
	m.wizardName.Blur()
	m.wizardDesc.Blur()
	m.wizardPackage.Blur()
	m.wizardURL.Blur()
	m.wizardLocalPackage.Blur()
	m.wizardRemoteURL.Blur()
	m.wizardGitURL.Blur()

	if !isNew && entry != nil {
		// Editing existing alias
		m.wizardExistingName = entry.Name
		m.wizardName.SetValue(entry.Name)
		m.wizardDesc.SetValue(entry.Alias.Description)
		m.wizardGitURL.SetValue(entry.Alias.URL)
		m.wizardWantGitURL = entry.Alias.URL != ""

		// Determine config type
		if len(entry.Alias.Variants) > 0 {
			m.wizardConfigType = 1 // variants
			for name, variant := range entry.Alias.Variants {
				if name == "local" {
					m.wizardHasLocal = true
					m.wizardLocalPackage.SetValue(variant.Package)
					if variant.Runtime == "python" {
						m.wizardLocalRuntime = 1
					}
				} else if name == "remote" {
					m.wizardHasRemote = true
					m.wizardRemoteURL.SetValue(variant.MCPURL)
					if variant.Transport == "sse" {
						m.wizardRemoteTransport = 1
					}
				}
			}
			if entry.Alias.DefaultVariant == "remote" {
				m.wizardDefaultVariant = 1
			}
		} else {
			m.wizardConfigType = 0 // simple
			m.wizardPackage.SetValue(entry.Alias.Package)
			m.wizardURL.SetValue(entry.Alias.MCPURL)

			// Determine transport
			switch entry.Alias.Transport {
			case "http":
				m.wizardTransport = 1
			case "sse":
				m.wizardTransport = 2
			default:
				m.wizardTransport = 0 // stdio
			}

			// Determine runtime
			switch entry.Alias.Runtime {
			case "python":
				m.wizardRuntime = 1
			case "go":
				m.wizardRuntime = 2
			case "docker":
				m.wizardRuntime = 3
			default:
				m.wizardRuntime = 0 // node
			}
		}
	}

	// Focus the first field
	m.wizardName.Focus()
}

// updateWizard handles keyboard input in wizard mode
func (m *AliasManagerModel) updateWizard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "esc":
		m.mode = modeList
		m.statusMsg = "Cancelled"
		m.statusIsError = false
		return m, nil

	case "ctrl+s":
		// Save the alias
		return m, m.saveWizardAlias()

	case "tab", "down":
		cmd = m.wizardNextField()
		return m, cmd

	case "shift+tab", "up":
		cmd = m.wizardPrevField()
		return m, cmd

	case "enter":
		// On step 0 (basic info), move to next step
		// On other steps, either toggle checkboxes or move to next step
		if m.wizardStep == 0 {
			if m.wizardFocus == 0 {
				// Validate name
				if strings.TrimSpace(m.wizardName.Value()) == "" {
					m.statusMsg = "Name is required"
					m.statusIsError = true
					return m, nil
				}
			}
			m.wizardStep = 1
			m.wizardFocus = 0
			return m, nil
		} else if m.wizardStep == 1 {
			m.wizardStep = 2
			m.wizardFocus = 0
			cmd = m.wizardFocusCurrentField()
			return m, cmd
		} else if m.wizardStep == 2 {
			// If on variants step and focus is on hasLocal/hasRemote toggles
			if m.wizardConfigType == 1 {
				if m.wizardFocus == 0 {
					m.wizardHasLocal = !m.wizardHasLocal
					return m, nil
				} else if m.wizardFocus == 1 {
					m.wizardHasRemote = !m.wizardHasRemote
					return m, nil
				}
			}
			m.wizardStep = 3
			m.wizardFocus = 0
			return m, nil
		} else if m.wizardStep == 3 {
			if m.wizardFocus == 0 {
				m.wizardWantGitURL = !m.wizardWantGitURL
				if m.wizardWantGitURL {
					m.wizardFocus = 1
					m.wizardGitURL.Focus()
					return m, textinput.Blink
				}
			}
		}
		return m, nil

	case "left", "h":
		// Cycle selection left
		return m, m.wizardCycleLeft()

	case "right", "l":
		// Cycle selection right
		return m, m.wizardCycleRight()

	case "backspace":
		// Go back a step
		if m.wizardStep > 0 {
			m.wizardStep--
			m.wizardFocus = 0
			cmd = m.wizardFocusCurrentField()
		}
		return m, cmd

	default:
		// Pass to focused input
		cmd = m.wizardUpdateInput(msg)
		return m, cmd
	}
}

// wizardNextField moves to the next field in the current step
func (m *AliasManagerModel) wizardNextField() tea.Cmd {
	maxFields := m.wizardMaxFields()
	if m.wizardFocus < maxFields-1 {
		m.wizardFocus++
	}
	return m.wizardFocusCurrentField()
}

// wizardPrevField moves to the previous field in the current step
func (m *AliasManagerModel) wizardPrevField() tea.Cmd {
	if m.wizardFocus > 0 {
		m.wizardFocus--
	}
	return m.wizardFocusCurrentField()
}

// wizardMaxFields returns the number of fields in the current step
func (m *AliasManagerModel) wizardMaxFields() int {
	switch m.wizardStep {
	case 0: // Basic info
		return 2 // name, desc
	case 1: // Config type
		return 1 // simple/variants toggle
	case 2: // Config details
		if m.wizardConfigType == 0 { // simple
			if m.wizardTransport == 0 { // stdio
				return 3 // transport, runtime, package
			}
			return 2 // transport, url
		}
		// variants
		fieldsCount := 2 // hasLocal, hasRemote toggles
		if m.wizardHasLocal {
			fieldsCount += 2 // runtime, package
		}
		if m.wizardHasRemote {
			fieldsCount += 2 // transport, url
		}
		if m.wizardHasLocal && m.wizardHasRemote {
			fieldsCount++ // default variant
		}
		return fieldsCount
	case 3: // Git URL
		return 2 // wantGitURL toggle, gitURL input
	}
	return 1
}

// wizardFocusCurrentField focuses the appropriate input for the current field
func (m *AliasManagerModel) wizardFocusCurrentField() tea.Cmd {
	// Blur all inputs first
	m.wizardName.Blur()
	m.wizardDesc.Blur()
	m.wizardPackage.Blur()
	m.wizardURL.Blur()
	m.wizardLocalPackage.Blur()
	m.wizardRemoteURL.Blur()
	m.wizardGitURL.Blur()

	switch m.wizardStep {
	case 0:
		if m.wizardFocus == 0 {
			m.wizardName.Focus()
		} else {
			m.wizardDesc.Focus()
		}
	case 2:
		if m.wizardConfigType == 0 { // simple
			if m.wizardTransport == 0 { // stdio
				if m.wizardFocus == 2 {
					m.wizardPackage.Focus()
				}
			} else {
				if m.wizardFocus == 1 {
					m.wizardURL.Focus()
				}
			}
		} else { // variants
			// Calculate which field is focused based on toggles
			fieldIdx := m.wizardFocus
			if fieldIdx >= 2 && m.wizardHasLocal {
				localFieldOffset := fieldIdx - 2
				if localFieldOffset == 1 {
					m.wizardLocalPackage.Focus()
				}
			}
			if m.wizardHasLocal && m.wizardHasRemote {
				remoteStart := 4
				if fieldIdx >= remoteStart && fieldIdx < remoteStart+2 {
					remoteFieldOffset := fieldIdx - remoteStart
					if remoteFieldOffset == 1 {
						m.wizardRemoteURL.Focus()
					}
				}
			} else if !m.wizardHasLocal && m.wizardHasRemote {
				remoteStart := 2
				if fieldIdx >= remoteStart && fieldIdx < remoteStart+2 {
					remoteFieldOffset := fieldIdx - remoteStart
					if remoteFieldOffset == 1 {
						m.wizardRemoteURL.Focus()
					}
				}
			}
		}
	case 3:
		if m.wizardFocus == 1 && m.wizardWantGitURL {
			m.wizardGitURL.Focus()
		}
	}

	return textinput.Blink
}

// wizardCycleLeft cycles selector left
func (m *AliasManagerModel) wizardCycleLeft() tea.Cmd {
	switch m.wizardStep {
	case 1:
		if m.wizardConfigType > 0 {
			m.wizardConfigType--
		}
	case 2:
		if m.wizardConfigType == 0 { // simple
			if m.wizardFocus == 0 { // transport
				if m.wizardTransport > 0 {
					m.wizardTransport--
				}
			} else if m.wizardFocus == 1 && m.wizardTransport == 0 { // runtime (only for stdio)
				if m.wizardRuntime > 0 {
					m.wizardRuntime--
				}
			}
		} else { // variants
			m.wizardCycleVariantFieldLeft()
		}
	}
	return nil
}

// wizardCycleRight cycles selector right
func (m *AliasManagerModel) wizardCycleRight() tea.Cmd {
	switch m.wizardStep {
	case 1:
		if m.wizardConfigType < 1 {
			m.wizardConfigType++
		}
	case 2:
		if m.wizardConfigType == 0 { // simple
			if m.wizardFocus == 0 { // transport
				if m.wizardTransport < 2 {
					m.wizardTransport++
				}
			} else if m.wizardFocus == 1 && m.wizardTransport == 0 { // runtime (only for stdio)
				if m.wizardRuntime < 3 {
					m.wizardRuntime++
				}
			}
		} else { // variants
			m.wizardCycleVariantFieldRight()
		}
	}
	return nil
}

func (m *AliasManagerModel) wizardCycleVariantFieldLeft() {
	fieldIdx := m.wizardFocus
	if m.wizardHasLocal {
		if fieldIdx == 2 { // local runtime
			if m.wizardLocalRuntime > 0 {
				m.wizardLocalRuntime--
			}
		}
	}
	if m.wizardHasRemote {
		remoteStart := 2
		if m.wizardHasLocal {
			remoteStart = 4
		}
		if fieldIdx == remoteStart { // remote transport
			if m.wizardRemoteTransport > 0 {
				m.wizardRemoteTransport--
			}
		}
	}
	if m.wizardHasLocal && m.wizardHasRemote {
		defaultStart := 6
		if fieldIdx == defaultStart { // default variant
			if m.wizardDefaultVariant > 0 {
				m.wizardDefaultVariant--
			}
		}
	}
}

func (m *AliasManagerModel) wizardCycleVariantFieldRight() {
	fieldIdx := m.wizardFocus
	if m.wizardHasLocal {
		if fieldIdx == 2 { // local runtime
			if m.wizardLocalRuntime < 1 {
				m.wizardLocalRuntime++
			}
		}
	}
	if m.wizardHasRemote {
		remoteStart := 2
		if m.wizardHasLocal {
			remoteStart = 4
		}
		if fieldIdx == remoteStart { // remote transport
			if m.wizardRemoteTransport < 1 {
				m.wizardRemoteTransport++
			}
		}
	}
	if m.wizardHasLocal && m.wizardHasRemote {
		defaultStart := 6
		if fieldIdx == defaultStart { // default variant
			if m.wizardDefaultVariant < 1 {
				m.wizardDefaultVariant++
			}
		}
	}
}

// wizardUpdateInput passes key events to the focused input
func (m *AliasManagerModel) wizardUpdateInput(msg tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd
	switch m.wizardStep {
	case 0:
		if m.wizardFocus == 0 {
			m.wizardName, cmd = m.wizardName.Update(msg)
		} else {
			m.wizardDesc, cmd = m.wizardDesc.Update(msg)
		}
	case 2:
		if m.wizardConfigType == 0 { // simple
			if m.wizardTransport == 0 { // stdio
				if m.wizardFocus == 2 {
					m.wizardPackage, cmd = m.wizardPackage.Update(msg)
				}
			} else {
				if m.wizardFocus == 1 {
					m.wizardURL, cmd = m.wizardURL.Update(msg)
				}
			}
		} else { // variants
			if m.wizardHasLocal {
				if m.wizardFocus == 3 {
					m.wizardLocalPackage, cmd = m.wizardLocalPackage.Update(msg)
				}
			}
			remoteURLField := 3
			if m.wizardHasLocal {
				remoteURLField = 5
			}
			if m.wizardHasRemote && m.wizardFocus == remoteURLField {
				m.wizardRemoteURL, cmd = m.wizardRemoteURL.Update(msg)
			}
		}
	case 3:
		if m.wizardFocus == 1 {
			m.wizardGitURL, cmd = m.wizardGitURL.Update(msg)
		}
	}
	return cmd
}

// renderWizard renders the wizard modal
func (m *AliasManagerModel) renderWizard() string {
	var b strings.Builder

	// Title
	title := "Add New Alias"
	if !m.wizardIsNew {
		title = fmt.Sprintf("Edit Alias: %s", m.wizardExistingName)
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6")).
		MarginBottom(1)
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	// Step indicator
	steps := []string{"Basic", "Type", "Config", "Git URL"}
	stepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	activeStepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	for i, step := range steps {
		if i == m.wizardStep {
			b.WriteString(activeStepStyle.Render(fmt.Sprintf("[%s]", step)))
		} else if i < m.wizardStep {
			b.WriteString(stepStyle.Render(fmt.Sprintf("✓%s", step)))
		} else {
			b.WriteString(stepStyle.Render(fmt.Sprintf(" %s ", step)))
		}
		if i < len(steps)-1 {
			b.WriteString(" → ")
		}
	}
	b.WriteString("\n\n")

	// Current step content
	switch m.wizardStep {
	case 0:
		b.WriteString(m.renderWizardStep0())
	case 1:
		b.WriteString(m.renderWizardStep1())
	case 2:
		b.WriteString(m.renderWizardStep2())
	case 3:
		b.WriteString(m.renderWizardStep3())
	}

	// Status message
	if m.statusMsg != "" {
		b.WriteString("\n")
		var statusStyle lipgloss.Style
		if m.statusIsError {
			statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
		} else {
			statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
		}
		b.WriteString(statusStyle.Render(m.statusMsg))
	}

	// Help
	b.WriteString("\n\n")
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	b.WriteString(helpStyle.Render("Tab:next  Shift+Tab:prev  ←/→:select  Enter:confirm  Ctrl+S:save  Esc:cancel  Backspace:back"))

	// Center the content
	content := b.String()
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("6")).
		Padding(1, 2).
		Width(70)
	modal := modalStyle.Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
}

func (m *AliasManagerModel) renderWizardStep0() string {
	var b strings.Builder

	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Width(15)
	focusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))

	// Name field
	nameLabel := "Name:"
	if m.wizardFocus == 0 {
		nameLabel = focusStyle.Render("▸ Name:")
	}
	b.WriteString(labelStyle.Render(nameLabel))
	b.WriteString(m.wizardName.View())
	b.WriteString("\n")

	// Description field
	descLabel := "Description:"
	if m.wizardFocus == 1 {
		descLabel = focusStyle.Render("▸ Description:")
	}
	b.WriteString(labelStyle.Render(descLabel))
	b.WriteString(m.wizardDesc.View())

	return b.String()
}

func (m *AliasManagerModel) renderWizardStep1() string {
	var b strings.Builder

	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	b.WriteString(labelStyle.Render("Configuration Type:"))
	b.WriteString("\n\n")

	options := []struct {
		name string
		desc string
	}{
		{"Simple", "Single package or URL"},
		{"Variants", "Multiple options (local + remote)"},
	}

	for i, opt := range options {
		prefix := "  "
		style := normalStyle
		if i == m.wizardConfigType {
			prefix = "▸ "
			style = selectedStyle
		}
		b.WriteString(style.Render(fmt.Sprintf("%s%s - %s", prefix, opt.name, opt.desc)))
		b.WriteString("\n")
	}

	return b.String()
}

func (m *AliasManagerModel) renderWizardStep2() string {
	if m.wizardConfigType == 0 {
		return m.renderWizardStep2Simple()
	}
	return m.renderWizardStep2Variants()
}

func (m *AliasManagerModel) renderWizardStep2Simple() string {
	var b strings.Builder

	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Width(15)
	focusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	// Transport selector
	transportLabel := "Transport:"
	if m.wizardFocus == 0 {
		transportLabel = focusStyle.Render("▸ Transport:")
	}
	b.WriteString(labelStyle.Render(transportLabel))
	transports := []string{"stdio", "http", "sse"}
	for i, t := range transports {
		style := normalStyle
		if i == m.wizardTransport {
			style = selectedStyle
		}
		b.WriteString(style.Render(fmt.Sprintf(" [%s] ", t)))
	}
	b.WriteString("\n")

	if m.wizardTransport == 0 { // stdio
		// Runtime selector
		runtimeLabel := "Runtime:"
		if m.wizardFocus == 1 {
			runtimeLabel = focusStyle.Render("▸ Runtime:")
		}
		b.WriteString(labelStyle.Render(runtimeLabel))
		runtimes := []string{"node", "python", "go", "docker"}
		for i, r := range runtimes {
			style := normalStyle
			if i == m.wizardRuntime {
				style = selectedStyle
			}
			b.WriteString(style.Render(fmt.Sprintf(" [%s] ", r)))
		}
		b.WriteString("\n")

		// Package input
		pkgLabel := "Package:"
		if m.wizardFocus == 2 {
			pkgLabel = focusStyle.Render("▸ Package:")
		}
		b.WriteString(labelStyle.Render(pkgLabel))
		b.WriteString(m.wizardPackage.View())
	} else {
		// URL input
		urlLabel := "URL:"
		if m.wizardFocus == 1 {
			urlLabel = focusStyle.Render("▸ URL:")
		}
		b.WriteString(labelStyle.Render(urlLabel))
		b.WriteString(m.wizardURL.View())
	}

	return b.String()
}

func (m *AliasManagerModel) renderWizardStep2Variants() string {
	var b strings.Builder

	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Width(18)
	focusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	checkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))

	fieldIdx := 0

	// Has local toggle
	localLabel := "Include Local:"
	if m.wizardFocus == fieldIdx {
		localLabel = focusStyle.Render("▸ Include Local:")
	}
	localCheck := "[ ]"
	if m.wizardHasLocal {
		localCheck = checkStyle.Render("[✓]")
	}
	b.WriteString(labelStyle.Render(localLabel))
	b.WriteString(localCheck)
	b.WriteString("\n")
	fieldIdx++

	// Has remote toggle
	remoteLabel := "Include Remote:"
	if m.wizardFocus == fieldIdx {
		remoteLabel = focusStyle.Render("▸ Include Remote:")
	}
	remoteCheck := "[ ]"
	if m.wizardHasRemote {
		remoteCheck = checkStyle.Render("[✓]")
	}
	b.WriteString(labelStyle.Render(remoteLabel))
	b.WriteString(remoteCheck)
	b.WriteString("\n\n")
	fieldIdx++

	// Local config if enabled
	if m.wizardHasLocal {
		b.WriteString(lipgloss.NewStyle().Bold(true).Render("Local Variant:"))
		b.WriteString("\n")

		// Runtime selector
		runtimeLabel := "  Runtime:"
		if m.wizardFocus == fieldIdx {
			runtimeLabel = focusStyle.Render("▸ Runtime:")
		}
		b.WriteString(labelStyle.Render(runtimeLabel))
		runtimes := []string{"node", "python"}
		for i, r := range runtimes {
			style := normalStyle
			if i == m.wizardLocalRuntime {
				style = selectedStyle
			}
			b.WriteString(style.Render(fmt.Sprintf(" [%s] ", r)))
		}
		b.WriteString("\n")
		fieldIdx++

		// Package input
		pkgLabel := "  Package:"
		if m.wizardFocus == fieldIdx {
			pkgLabel = focusStyle.Render("▸ Package:")
		}
		b.WriteString(labelStyle.Render(pkgLabel))
		b.WriteString(m.wizardLocalPackage.View())
		b.WriteString("\n\n")
		fieldIdx++
	}

	// Remote config if enabled
	if m.wizardHasRemote {
		b.WriteString(lipgloss.NewStyle().Bold(true).Render("Remote Variant:"))
		b.WriteString("\n")

		// Transport selector
		transportLabel := "  Transport:"
		if m.wizardFocus == fieldIdx {
			transportLabel = focusStyle.Render("▸ Transport:")
		}
		b.WriteString(labelStyle.Render(transportLabel))
		transports := []string{"http", "sse"}
		for i, t := range transports {
			style := normalStyle
			if i == m.wizardRemoteTransport {
				style = selectedStyle
			}
			b.WriteString(style.Render(fmt.Sprintf(" [%s] ", t)))
		}
		b.WriteString("\n")
		fieldIdx++

		// URL input
		urlLabel := "  URL:"
		if m.wizardFocus == fieldIdx {
			urlLabel = focusStyle.Render("▸ URL:")
		}
		b.WriteString(labelStyle.Render(urlLabel))
		b.WriteString(m.wizardRemoteURL.View())
		b.WriteString("\n\n")
		fieldIdx++
	}

	// Default variant if both enabled
	if m.wizardHasLocal && m.wizardHasRemote {
		defaultLabel := "Default Variant:"
		if m.wizardFocus == fieldIdx {
			defaultLabel = focusStyle.Render("▸ Default Variant:")
		}
		b.WriteString(labelStyle.Render(defaultLabel))
		variants := []string{"local", "remote"}
		for i, v := range variants {
			style := normalStyle
			if i == m.wizardDefaultVariant {
				style = selectedStyle
			}
			b.WriteString(style.Render(fmt.Sprintf(" [%s] ", v)))
		}
	}

	return b.String()
}

func (m *AliasManagerModel) renderWizardStep3() string {
	var b strings.Builder

	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Width(15)
	focusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	checkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))

	// Want git URL toggle
	gitLabel := "Add Git URL:"
	if m.wizardFocus == 0 {
		gitLabel = focusStyle.Render("▸ Add Git URL:")
	}
	gitCheck := "[ ]"
	if m.wizardWantGitURL {
		gitCheck = checkStyle.Render("[✓]")
	}
	b.WriteString(labelStyle.Render(gitLabel))
	b.WriteString(gitCheck)
	b.WriteString("  (optional, link to source repo)")
	b.WriteString("\n\n")

	if m.wizardWantGitURL {
		urlLabel := "Git URL:"
		if m.wizardFocus == 1 {
			urlLabel = focusStyle.Render("▸ Git URL:")
		}
		b.WriteString(labelStyle.Render(urlLabel))
		b.WriteString(m.wizardGitURL.View())
	}

	return b.String()
}

// saveWizardAlias saves the alias from wizard data
func (m *AliasManagerModel) saveWizardAlias() tea.Cmd {
	return func() tea.Msg {
		name := strings.TrimSpace(m.wizardName.Value())
		if name == "" {
			m.statusMsg = "Name is required"
			m.statusIsError = true
			return nil
		}

		var alias aliases.Alias
		alias.Description = strings.TrimSpace(m.wizardDesc.Value())

		if m.wizardConfigType == 0 { // simple
			transports := []string{"stdio", "http", "sse"}
			alias.Transport = transports[m.wizardTransport]

			if m.wizardTransport == 0 { // stdio
				runtimes := []string{"node", "python", "go", "docker"}
				alias.Runtime = runtimes[m.wizardRuntime]
				alias.Package = strings.TrimSpace(m.wizardPackage.Value())
			} else {
				alias.MCPURL = strings.TrimSpace(m.wizardURL.Value())
			}
		} else { // variants
			alias.Variants = make(map[string]aliases.Variant)

			if m.wizardHasLocal {
				runtimes := []string{"node", "python"}
				alias.Variants["local"] = aliases.Variant{
					Transport: "stdio",
					Runtime:   runtimes[m.wizardLocalRuntime],
					Package:   strings.TrimSpace(m.wizardLocalPackage.Value()),
				}
			}

			if m.wizardHasRemote {
				transports := []string{"http", "sse"}
				alias.Variants["remote"] = aliases.Variant{
					Transport: transports[m.wizardRemoteTransport],
					MCPURL:    strings.TrimSpace(m.wizardRemoteURL.Value()),
				}
			}

			if m.wizardHasLocal && m.wizardHasRemote {
				variants := []string{"local", "remote"}
				alias.DefaultVariant = variants[m.wizardDefaultVariant]
			} else if m.wizardHasLocal {
				alias.DefaultVariant = "local"
			} else {
				alias.DefaultVariant = "remote"
			}
		}

		if m.wizardWantGitURL {
			alias.URL = strings.TrimSpace(m.wizardGitURL.Value())
		}

		if m.wizardIsNew {
			return addAliasMsg{name: name, alias: alias}
		}
		return editAliasMsg{oldName: m.wizardExistingName, name: name, alias: alias}
	}
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
