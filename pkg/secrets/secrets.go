package secrets

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Store provides secret storage using the system keychain
type Store struct {
	Service string // Service name for keychain entries
}

// NewStore creates a new secret store
func NewStore() *Store {
	return &Store{Service: "agentctl"}
}

// Set stores a secret in the system keychain
func (s *Store) Set(name, value string) error {
	switch runtime.GOOS {
	case "darwin":
		return s.setMacOS(name, value)
	case "linux":
		return s.setLinux(name, value)
	case "windows":
		return s.setWindows(name, value)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// Get retrieves a secret from the system keychain
func (s *Store) Get(name string) (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return s.getMacOS(name)
	case "linux":
		return s.getLinux(name)
	case "windows":
		return s.getWindows(name)
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// Delete removes a secret from the system keychain
func (s *Store) Delete(name string) error {
	switch runtime.GOOS {
	case "darwin":
		return s.deleteMacOS(name)
	case "linux":
		return s.deleteLinux(name)
	case "windows":
		return s.deleteWindows(name)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// List returns all secret names stored for this service
func (s *Store) List() ([]string, error) {
	switch runtime.GOOS {
	case "darwin":
		return s.listMacOS()
	case "linux":
		return s.listLinux()
	case "windows":
		return s.listWindows()
	default:
		return nil, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// macOS implementation using security command
func (s *Store) setMacOS(name, value string) error {
	// First try to delete existing (ignore error if not found)
	s.deleteMacOS(name)

	cmd := exec.Command("security", "add-generic-password",
		"-a", name,
		"-s", s.Service,
		"-w", value,
		"-U", // Update if exists
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to store secret: %w", err)
	}
	return nil
}

func (s *Store) getMacOS(name string) (string, error) {
	cmd := exec.Command("security", "find-generic-password",
		"-a", name,
		"-s", s.Service,
		"-w",
	)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 44 {
			return "", fmt.Errorf("secret %q not found", name)
		}
		return "", fmt.Errorf("failed to retrieve secret: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func (s *Store) deleteMacOS(name string) error {
	cmd := exec.Command("security", "delete-generic-password",
		"-a", name,
		"-s", s.Service,
	)
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 44 {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to delete secret: %w", err)
	}
	return nil
}

func (s *Store) listMacOS() ([]string, error) {
	cmd := exec.Command("security", "dump-keychain")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	var secrets []string
	lines := strings.Split(string(output), "\n")
	inAgentctl := false

	for _, line := range lines {
		if strings.Contains(line, `svce"<blob>="agentctl"`) || strings.Contains(line, `"svce"<blob>="`+s.Service) {
			inAgentctl = true
			continue
		}
		if inAgentctl && strings.Contains(line, `"acct"<blob>="`) {
			// Extract account name
			start := strings.Index(line, `"acct"<blob>="`) + len(`"acct"<blob>="`)
			end := strings.LastIndex(line, `"`)
			if start < end {
				secrets = append(secrets, line[start:end])
			}
			inAgentctl = false
		}
	}

	return secrets, nil
}

// Linux implementation using secret-tool (libsecret)
func (s *Store) setLinux(name, value string) error {
	cmd := exec.Command("secret-tool", "store",
		"--label", fmt.Sprintf("%s/%s", s.Service, name),
		"service", s.Service,
		"account", name,
	)
	cmd.Stdin = strings.NewReader(value)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to store secret (is secret-tool installed?): %w", err)
	}
	return nil
}

func (s *Store) getLinux(name string) (string, error) {
	cmd := exec.Command("secret-tool", "lookup",
		"service", s.Service,
		"account", name,
	)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("secret %q not found", name)
	}
	return strings.TrimSpace(string(output)), nil
}

func (s *Store) deleteLinux(name string) error {
	cmd := exec.Command("secret-tool", "clear",
		"service", s.Service,
		"account", name,
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}
	return nil
}

func (s *Store) listLinux() ([]string, error) {
	// secret-tool doesn't have a list command, so we'd need to track names elsewhere
	// For now, return empty list with note
	return nil, fmt.Errorf("listing secrets not supported on Linux (use secret-tool manually)")
}

// Windows implementation using cmdkey
func (s *Store) setWindows(name, value string) error {
	target := fmt.Sprintf("%s/%s", s.Service, name)
	cmd := exec.Command("cmdkey", "/generic:"+target, "/user:"+name, "/pass:"+value)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to store secret: %w", err)
	}
	return nil
}

func (s *Store) getWindows(name string) (string, error) {
	// Windows cmdkey doesn't provide a way to retrieve passwords programmatically
	// We'd need to use the Windows Credential Manager API directly
	return "", fmt.Errorf("retrieving secrets on Windows requires native API (not implemented)")
}

func (s *Store) deleteWindows(name string) error {
	target := fmt.Sprintf("%s/%s", s.Service, name)
	cmd := exec.Command("cmdkey", "/delete:"+target)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}
	return nil
}

func (s *Store) listWindows() ([]string, error) {
	cmd := exec.Command("cmdkey", "/list")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	var secrets []string
	prefix := fmt.Sprintf("%s/", s.Service)
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Target:") {
			target := strings.TrimPrefix(line, "Target:")
			target = strings.TrimSpace(target)
			if strings.HasPrefix(target, prefix) {
				secrets = append(secrets, strings.TrimPrefix(target, prefix))
			}
		}
	}

	return secrets, nil
}

// Resolve resolves a value that may be a secret reference
// Supports: $ENV_VAR, keychain:name
func Resolve(value string) (string, error) {
	// Environment variable
	if strings.HasPrefix(value, "$") {
		envName := strings.TrimPrefix(value, "$")
		envValue := os.Getenv(envName)
		if envValue == "" {
			return "", fmt.Errorf("environment variable %s not set", envName)
		}
		return envValue, nil
	}

	// Keychain reference
	if strings.HasPrefix(value, "keychain:") {
		name := strings.TrimPrefix(value, "keychain:")
		store := NewStore()
		return store.Get(name)
	}

	// Plain value
	return value, nil
}

// ResolveEnv resolves all values in an env map
func ResolveEnv(env map[string]string) (map[string]string, error) {
	resolved := make(map[string]string)
	for k, v := range env {
		val, err := Resolve(v)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve %s: %w", k, err)
		}
		resolved[k] = val
	}
	return resolved, nil
}
