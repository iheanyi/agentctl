package aliases

import (
	"embed"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed aliases.json
var embeddedAliases embed.FS

// Alias maps a short name to an MCP server configuration
type Alias struct {
	URL            string             `json:"url"`                      // Git URL for local servers
	Description    string             `json:"description,omitempty"`
	Runtime        string             `json:"runtime,omitempty"`        // node, python, go, docker
	Transport      string             `json:"transport,omitempty"`      // stdio (default), http, sse
	MCPURL         string             `json:"mcpUrl,omitempty"`         // Remote MCP URL (for http/sse transport)
	OAuth          *OAuth             `json:"oauth,omitempty"`          // OAuth configuration if required
	Package        string             `json:"package,omitempty"`        // npm/pypi package name (e.g., "@figma/mcp-local")
	Variants       map[string]Variant `json:"variants,omitempty"`       // Available variants (local, remote)
	DefaultVariant string             `json:"defaultVariant,omitempty"` // Preferred variant (local or remote)
}

// Variant represents a specific distribution variant of an MCP server
type Variant struct {
	Transport string `json:"transport,omitempty"` // stdio, http, sse
	Package   string `json:"package,omitempty"`   // npm/pypi package name
	MCPURL    string `json:"mcpUrl,omitempty"`    // Remote MCP URL
	Runtime   string `json:"runtime,omitempty"`   // node, python, etc.
}

// OAuth represents OAuth configuration for remote MCP servers
type OAuth struct {
	AuthURL     string   `json:"authUrl"`              // OAuth authorization endpoint
	TokenURL    string   `json:"tokenUrl"`             // OAuth token endpoint
	Scopes      []string `json:"scopes,omitempty"`     // Required OAuth scopes
	ClientID    string   `json:"clientId,omitempty"`   // Public client ID (if applicable)
	PKCEEnabled bool     `json:"pkceEnabled,omitempty"` // Use PKCE for security
}

// Store manages alias lookups from bundled and user-defined sources
type Store struct {
	bundled map[string]Alias
	user    map[string]Alias
	mu      sync.RWMutex
	userDir string
}

var defaultStore *Store
var once sync.Once

// Default returns the default alias store
func Default() *Store {
	once.Do(func() {
		defaultStore = NewStore("")
	})
	return defaultStore
}

// NewStore creates a new alias store with optional user directory
func NewStore(userDir string) *Store {
	s := &Store{
		bundled: make(map[string]Alias),
		user:    make(map[string]Alias),
		userDir: userDir,
	}
	s.loadBundled()
	s.loadUser()
	return s
}

// loadBundled loads the bundled aliases from embedded file
func (s *Store) loadBundled() {
	data, err := embeddedAliases.ReadFile("aliases.json")
	if err != nil {
		return
	}

	var aliases map[string]Alias
	if err := json.Unmarshal(data, &aliases); err != nil {
		return
	}

	s.mu.Lock()
	s.bundled = aliases
	s.mu.Unlock()
}

// loadUser loads user-defined aliases from config directory
func (s *Store) loadUser() {
	if s.userDir == "" {
		// Use default config dir
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return
		}
		s.userDir = filepath.Join(homeDir, ".config", "agentctl")
	}

	path := filepath.Join(s.userDir, "aliases.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var aliases map[string]Alias
	if err := json.Unmarshal(data, &aliases); err != nil {
		return
	}

	s.mu.Lock()
	s.user = aliases
	s.mu.Unlock()
}

// Resolve looks up an alias by name, checking user aliases first then bundled
func (s *Store) Resolve(name string) (Alias, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// User aliases take precedence
	if alias, ok := s.user[name]; ok {
		return alias, true
	}

	// Fall back to bundled
	if alias, ok := s.bundled[name]; ok {
		return alias, true
	}

	return Alias{}, false
}

// Add adds or updates a user alias
func (s *Store) Add(name string, alias Alias) error {
	s.mu.Lock()
	s.user[name] = alias
	s.mu.Unlock()

	return s.saveUser()
}

// Remove removes a user alias
func (s *Store) Remove(name string) error {
	s.mu.Lock()
	delete(s.user, name)
	s.mu.Unlock()

	return s.saveUser()
}

// saveUser saves user aliases to disk
func (s *Store) saveUser() error {
	if s.userDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		s.userDir = filepath.Join(homeDir, ".config", "agentctl")
	}

	if err := os.MkdirAll(s.userDir, 0755); err != nil {
		return err
	}

	s.mu.RLock()
	data, err := json.MarshalIndent(s.user, "", "  ")
	s.mu.RUnlock()

	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(s.userDir, "aliases.json"), data, 0644)
}

// List returns all aliases (bundled and user)
func (s *Store) List() map[string]Alias {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]Alias)

	// Add bundled first
	for name, alias := range s.bundled {
		result[name] = alias
	}

	// User aliases override bundled
	for name, alias := range s.user {
		result[name] = alias
	}

	return result
}

// ListBundled returns only bundled aliases
func (s *Store) ListBundled() map[string]Alias {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]Alias)
	for name, alias := range s.bundled {
		result[name] = alias
	}
	return result
}

// ListUser returns only user-defined aliases
func (s *Store) ListUser() map[string]Alias {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]Alias)
	for name, alias := range s.user {
		result[name] = alias
	}
	return result
}

// IsBundled checks if an alias is from the bundled set
func (s *Store) IsBundled(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.bundled[name]
	return ok
}

// IsUser checks if an alias is user-defined
func (s *Store) IsUser(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.user[name]
	return ok
}

// Global convenience functions

// Resolve looks up an alias using the default store
func Resolve(name string) (Alias, bool) {
	return Default().Resolve(name)
}

// Add adds a user alias using the default store
func Add(name string, alias Alias) error {
	return Default().Add(name, alias)
}

// Remove removes a user alias using the default store
func Remove(name string) error {
	return Default().Remove(name)
}

// List returns all aliases using the default store
func List() map[string]Alias {
	return Default().List()
}

// AliasInfo contains alias info for display
type AliasInfo struct {
	Name        string
	Description string
	Runtime     string
	URL         string
}

// Search searches for aliases matching the query
func (s *Store) Search(query string) []AliasInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query = strings.ToLower(query)
	var results []AliasInfo

	// Search bundled
	for name, alias := range s.bundled {
		if strings.Contains(strings.ToLower(name), query) ||
			strings.Contains(strings.ToLower(alias.Description), query) {
			results = append(results, AliasInfo{
				Name:        name,
				Description: alias.Description,
				Runtime:     alias.Runtime,
				URL:         alias.URL,
			})
		}
	}

	// Search user
	for name, alias := range s.user {
		if strings.Contains(strings.ToLower(name), query) ||
			strings.Contains(strings.ToLower(alias.Description), query) {
			results = append(results, AliasInfo{
				Name:        name,
				Description: alias.Description,
				Runtime:     alias.Runtime,
				URL:         alias.URL,
			})
		}
	}

	return results
}

// Search searches aliases using the default store
func Search(query string) []AliasInfo {
	return Default().Search(query)
}

// ResolveVariant resolves an alias with a specific variant preference
// variantPref can be "local", "remote", or empty (use default)
func (s *Store) ResolveVariant(name string, variantPref string) (Alias, string, bool) {
	alias, ok := s.Resolve(name)
	if !ok {
		return Alias{}, "", false
	}

	// If no variants defined, return the alias as-is
	if len(alias.Variants) == 0 {
		return alias, "", true
	}

	// Determine which variant to use
	variantName := variantPref
	if variantName == "" {
		variantName = alias.DefaultVariant
	}
	if variantName == "" {
		// Default to "local" if available, otherwise "remote"
		if _, ok := alias.Variants["local"]; ok {
			variantName = "local"
		} else if _, ok := alias.Variants["remote"]; ok {
			variantName = "remote"
		}
	}

	variant, ok := alias.Variants[variantName]
	if !ok {
		// Variant not found, return base alias
		return alias, "", true
	}

	// Merge variant into alias
	resolved := alias
	if variant.Transport != "" {
		resolved.Transport = variant.Transport
	}
	if variant.Package != "" {
		resolved.Package = variant.Package
	}
	if variant.MCPURL != "" {
		resolved.MCPURL = variant.MCPURL
	}
	if variant.Runtime != "" {
		resolved.Runtime = variant.Runtime
	}

	return resolved, variantName, true
}

// ResolveVariant resolves an alias with a specific variant using the default store
func ResolveVariant(name string, variantPref string) (Alias, string, bool) {
	return Default().ResolveVariant(name, variantPref)
}

// HasVariants checks if an alias has multiple variants
func (a *Alias) HasVariants() bool {
	return len(a.Variants) > 0
}

// GetVariantNames returns the names of available variants
func (a *Alias) GetVariantNames() []string {
	var names []string
	for name := range a.Variants {
		names = append(names, name)
	}
	return names
}

// HasLocalVariant checks if a local variant exists
func (a *Alias) HasLocalVariant() bool {
	_, ok := a.Variants["local"]
	return ok
}

// HasRemoteVariant checks if a remote variant exists
func (a *Alias) HasRemoteVariant() bool {
	_, ok := a.Variants["remote"]
	return ok
}
