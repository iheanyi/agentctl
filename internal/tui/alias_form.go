package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/iheanyi/agentctl/pkg/aliases"
)

// AliasFormResult contains the result of an alias form
type AliasFormResult struct {
	Name  string
	Alias aliases.Alias
}

// RunAliasAddForm runs an interactive form to add a new alias
func RunAliasAddForm() (*AliasFormResult, error) {
	return runAliasFormInternal("", aliases.Alias{})
}

// RunAliasEditForm runs an interactive form to edit an existing alias
func RunAliasEditForm(name string, existing aliases.Alias) (*AliasFormResult, error) {
	return runAliasFormInternal(name, existing)
}

func runAliasFormInternal(existingName string, existing aliases.Alias) (*AliasFormResult, error) {
	isEdit := existingName != ""
	title := "Add New Alias"
	if isEdit {
		title = fmt.Sprintf("Edit Alias: %s", existingName)
	}

	var (
		name        = existingName
		description = existing.Description
		configType  = determineConfigType(existing)
		transport   = existing.Transport
		packageName = existing.Package
		mcpURL      = existing.MCPURL
		gitURL      = existing.URL
		runtime     = existing.Runtime
	)

	// Default values
	if transport == "" {
		transport = "stdio"
	}
	if runtime == "" {
		runtime = "node"
	}
	if configType == "" {
		configType = "simple"
	}

	// Step 1: Basic info
	basicForm := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().Title(title),
			huh.NewInput().
				Title("Name").
				Description("Short name for the alias").
				Placeholder("e.g., my-server").
				Value(&name).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("name is required")
					}
					if strings.ContainsAny(s, " \t\n") {
						return fmt.Errorf("name cannot contain whitespace")
					}
					return nil
				}),
			huh.NewInput().
				Title("Description").
				Description("Brief description of what this server does").
				Placeholder("e.g., Database access server").
				Value(&description),
		),
	)

	if err := basicForm.Run(); err != nil {
		return nil, err
	}

	// Step 2: Configuration type
	typeForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Configuration Type").
				Description("How is this MCP server distributed?").
				Options(
					huh.NewOption("Simple - Single package or URL", "simple"),
					huh.NewOption("Variants - Multiple options (local + remote)", "variants"),
				).
				Value(&configType),
		),
	)

	if err := typeForm.Run(); err != nil {
		return nil, err
	}

	var result aliases.Alias
	result.Description = description

	if configType == "simple" {
		// Simple configuration
		simpleForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Transport").
					Description("How does the server communicate?").
					Options(
						huh.NewOption("stdio - Local command (npx, uvx)", "stdio"),
						huh.NewOption("http - Remote HTTP endpoint", "http"),
						huh.NewOption("sse - Remote Server-Sent Events", "sse"),
					).
					Value(&transport),
			),
		)

		if err := simpleForm.Run(); err != nil {
			return nil, err
		}

		result.Transport = transport

		if transport == "stdio" {
			// Stdio config
			stdioForm := huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title("Runtime").
						Description("Which runtime to use?").
						Options(
							huh.NewOption("node - Node.js (npx)", "node"),
							huh.NewOption("python - Python (uvx)", "python"),
							huh.NewOption("go - Go", "go"),
							huh.NewOption("docker - Docker", "docker"),
						).
						Value(&runtime),
					huh.NewInput().
						Title("Package").
						Description("npm/PyPI package name").
						Placeholder("e.g., @org/mcp-server").
						Value(&packageName).
						Validate(func(s string) error {
							if s == "" {
								return fmt.Errorf("package is required for stdio transport")
							}
							return nil
						}),
				),
			)

			if err := stdioForm.Run(); err != nil {
				return nil, err
			}

			result.Runtime = runtime
			result.Package = packageName
		} else {
			// HTTP/SSE config
			urlForm := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("URL").
						Description("Remote MCP server URL").
						Placeholder("https://mcp.example.com/mcp").
						Value(&mcpURL).
						Validate(func(s string) error {
							if s == "" {
								return fmt.Errorf("URL is required")
							}
							if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
								return fmt.Errorf("URL must start with http:// or https://")
							}
							return nil
						}),
				),
			)

			if err := urlForm.Run(); err != nil {
				return nil, err
			}

			result.MCPURL = mcpURL
		}
	} else {
		// Variants configuration
		var hasLocal, hasRemote bool
		var localPackage, localRuntime string
		var remoteURL, remoteTransport string
		var defaultVariant string

		variantSelectForm := huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Available Variants").
					Description("Which variants should this alias support?").
					Options(
						huh.NewOption("local - Run locally via npx/uvx", "local"),
						huh.NewOption("remote - Connect to remote server", "remote"),
					).
					Value(&[]string{}). // Will be set by filterfunc
					Validate(func(s []string) error {
						if len(s) == 0 {
							return fmt.Errorf("select at least one variant")
						}
						hasLocal = contains(s, "local")
						hasRemote = contains(s, "remote")
						return nil
					}),
			),
		)

		// Run variant selection with a simpler approach
		variantChoices := []string{}
		variantForm := huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Available Variants").
					Description("Which variants should this alias support?").
					Options(
						huh.NewOption("local - Run locally via npx/uvx", "local"),
						huh.NewOption("remote - Connect to remote server", "remote"),
					).
					Value(&variantChoices),
			),
		)

		if err := variantForm.Run(); err != nil {
			return nil, err
		}

		hasLocal = contains(variantChoices, "local")
		hasRemote = contains(variantChoices, "remote")
		_ = variantSelectForm // unused but needed for the logic above

		if !hasLocal && !hasRemote {
			return nil, fmt.Errorf("at least one variant required")
		}

		result.Variants = make(map[string]aliases.Variant)

		if hasLocal {
			localForm := huh.NewForm(
				huh.NewGroup(
					huh.NewNote().Title("Local Variant Configuration"),
					huh.NewSelect[string]().
						Title("Runtime").
						Options(
							huh.NewOption("node - Node.js (npx)", "node"),
							huh.NewOption("python - Python (uvx)", "python"),
						).
						Value(&localRuntime),
					huh.NewInput().
						Title("Package").
						Placeholder("@org/mcp-server").
						Value(&localPackage).
						Validate(func(s string) error {
							if s == "" {
								return fmt.Errorf("package is required")
							}
							return nil
						}),
				),
			)

			if err := localForm.Run(); err != nil {
				return nil, err
			}

			result.Variants["local"] = aliases.Variant{
				Transport: "stdio",
				Runtime:   localRuntime,
				Package:   localPackage,
			}
		}

		if hasRemote {
			remoteTransport = "http"
			remoteForm := huh.NewForm(
				huh.NewGroup(
					huh.NewNote().Title("Remote Variant Configuration"),
					huh.NewSelect[string]().
						Title("Transport").
						Options(
							huh.NewOption("http", "http"),
							huh.NewOption("sse", "sse"),
						).
						Value(&remoteTransport),
					huh.NewInput().
						Title("URL").
						Placeholder("https://mcp.example.com/mcp").
						Value(&remoteURL).
						Validate(func(s string) error {
							if s == "" {
								return fmt.Errorf("URL is required")
							}
							return nil
						}),
				),
			)

			if err := remoteForm.Run(); err != nil {
				return nil, err
			}

			result.Variants["remote"] = aliases.Variant{
				Transport: remoteTransport,
				MCPURL:    remoteURL,
			}
		}

		// Default variant
		var defaultOptions []huh.Option[string]
		if hasLocal {
			defaultOptions = append(defaultOptions, huh.NewOption("local", "local"))
		}
		if hasRemote {
			defaultOptions = append(defaultOptions, huh.NewOption("remote", "remote"))
		}

		if len(defaultOptions) > 1 {
			defaultForm := huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title("Default Variant").
						Description("Which variant should be used by default?").
						Options(defaultOptions...).
						Value(&defaultVariant),
				),
			)

			if err := defaultForm.Run(); err != nil {
				return nil, err
			}

			result.DefaultVariant = defaultVariant
		} else if hasLocal {
			result.DefaultVariant = "local"
		} else {
			result.DefaultVariant = "remote"
		}
	}

	// Optional: Git URL
	var wantGitURL bool
	gitURLForm := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Add Git URL?").
				Description("Link to the source repository (optional)").
				Value(&wantGitURL),
		),
	)

	if err := gitURLForm.Run(); err != nil {
		return nil, err
	}

	if wantGitURL {
		gitInputForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Git URL").
					Placeholder("github.com/org/repo").
					Value(&gitURL),
			),
		)

		if err := gitInputForm.Run(); err != nil {
			return nil, err
		}

		result.URL = gitURL
	}

	return &AliasFormResult{
		Name:  name,
		Alias: result,
	}, nil
}

func determineConfigType(alias aliases.Alias) string {
	if len(alias.Variants) > 0 {
		return "variants"
	}
	return "simple"
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
