package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/iheanyi/agentctl/pkg/output"
	"github.com/iheanyi/agentctl/pkg/sync"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration syntax and schema",
	Long: `Validate the syntax and schema of all tool configuration files.

This checks that each tool's config file:
- Is valid JSON
- Has the expected structure for MCP servers
- Contains no obvious errors

Examples:
  agentctl validate                # Validate all detected tool configs
  agentctl validate --tool claude  # Validate only Claude config`,
	RunE: runValidate,
}

var validateTool string

func init() {
	validateCmd.Flags().StringVarP(&validateTool, "tool", "t", "", "Validate specific tool only")
}

// ValidationResult represents the result of validating a single tool's config
type ValidationResult struct {
	Tool        string
	ConfigPath  string
	Valid       bool
	Errors      []string
	Warnings    []string
	ServerCount int
}

func runValidate(cmd *cobra.Command, args []string) error {
	if !JSONOutput {
		fmt.Println("Validating configurations...")
		fmt.Println()
	}

	// Get adapters to validate
	var adapters []sync.Adapter
	if validateTool != "" {
		adapter, ok := sync.Get(validateTool)
		if !ok {
			err := fmt.Errorf("unknown tool %q", validateTool)
			if JSONOutput {
				jw := output.NewJSONWriter()
				return jw.WriteError(err)
			}
			return err
		}
		adapters = []sync.Adapter{adapter}
	} else {
		adapters = sync.Detected()
	}

	if len(adapters) == 0 {
		if JSONOutput {
			jw := output.NewJSONWriter()
			return jw.WriteSuccess(output.ValidateOutput{
				Results: []output.ValidateToolResult{},
				Summary: output.ValidateSummary{
					TotalTools:   0,
					ValidTools:   0,
					InvalidTools: 0,
				},
			})
		}
		fmt.Println("No supported tools detected.")
		return nil
	}

	var results []ValidationResult
	var hasErrors bool

	for _, adapter := range adapters {
		result := validateAdapter(adapter)
		results = append(results, result)
		if !result.Valid {
			hasErrors = true
		}
	}

	// JSON output
	if JSONOutput {
		jw := output.NewJSONWriter()
		validateOutput := output.ValidateOutput{
			Results: make([]output.ValidateToolResult, len(results)),
		}

		validCount := 0
		for i, r := range results {
			validateOutput.Results[i] = output.ValidateToolResult{
				Tool:        r.Tool,
				ConfigPath:  r.ConfigPath,
				Valid:       r.Valid,
				Errors:      r.Errors,
				Warnings:    r.Warnings,
				ServerCount: r.ServerCount,
			}
			if r.Valid {
				validCount++
			}
		}

		validateOutput.Summary = output.ValidateSummary{
			TotalTools:   len(results),
			ValidTools:   validCount,
			InvalidTools: len(results) - validCount,
		}

		return jw.Write(output.CLIOutput{
			Success: !hasErrors,
			Data:    validateOutput,
			Error: func() string {
				if hasErrors {
					return "validation failed"
				}
				return ""
			}(),
		})
	}

	// Print results
	for _, result := range results {
		printValidationResult(result)
	}

	// Summary
	fmt.Println()
	validCount := 0
	for _, r := range results {
		if r.Valid {
			validCount++
		}
	}

	if hasErrors {
		fmt.Printf("Validated %d tool(s): %d valid, %d with errors\n", len(results), validCount, len(results)-validCount)
		return fmt.Errorf("validation failed")
	}

	fmt.Printf("All %d configuration(s) valid.\n", len(results))
	return nil
}

func validateAdapter(adapter sync.Adapter) ValidationResult {
	result := ValidationResult{
		Tool:       adapter.Name(),
		ConfigPath: adapter.ConfigPath(),
		Valid:      true,
	}

	// Check if config file exists
	data, err := os.ReadFile(result.ConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			result.Warnings = append(result.Warnings, "Config file does not exist (will be created on first sync)")
			return result
		}
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("Cannot read config: %v", err))
		return result
	}

	// Parse JSON
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("Invalid JSON: %v", err))
		return result
	}

	// Check for expected MCP server structure
	serverKey := getServerKey(adapter.Name())
	servers, hasServers := raw[serverKey]

	if !hasServers {
		result.Warnings = append(result.Warnings, fmt.Sprintf("No %s section found", serverKey))
	} else {
		// Validate servers section
		serversMap, ok := servers.(map[string]interface{})
		if !ok {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("%s should be an object", serverKey))
		} else {
			result.ServerCount = len(serversMap)

			// Validate each server
			for name, serverData := range serversMap {
				serverErrors := validateServer(adapter.Name(), name, serverData)
				result.Errors = append(result.Errors, serverErrors...)
				if len(serverErrors) > 0 {
					result.Valid = false
				}
			}
		}
	}

	return result
}

func getServerKey(adapterName string) string {
	switch adapterName {
	case "zed":
		return "context_servers"
	case "opencode":
		return "mcp"
	default:
		return "mcpServers"
	}
}

func validateServer(adapterName, serverName string, serverData interface{}) []string {
	var errors []string

	server, ok := serverData.(map[string]interface{})
	if !ok {
		return []string{fmt.Sprintf("Server %q: should be an object", serverName)}
	}

	// OpenCode has different structure
	if adapterName == "opencode" {
		// Check for type field
		serverType, hasType := server["type"].(string)
		if !hasType {
			errors = append(errors, fmt.Sprintf("Server %q: missing 'type' field", serverName))
		} else {
			switch serverType {
			case "local":
				if _, hasCmd := server["command"]; !hasCmd {
					errors = append(errors, fmt.Sprintf("Server %q: local server missing 'command' array", serverName))
				}
			case "remote":
				if _, hasURL := server["url"]; !hasURL {
					errors = append(errors, fmt.Sprintf("Server %q: remote server missing 'url' field", serverName))
				}
			default:
				errors = append(errors, fmt.Sprintf("Server %q: invalid type %q (expected 'local' or 'remote')", serverName, serverType))
			}
		}
	} else {
		// Standard format (Claude, Cursor, etc.)
		hasCommand := false
		hasURL := false

		if _, ok := server["command"]; ok {
			hasCommand = true
		}
		if _, ok := server["url"]; ok {
			hasURL = true
		}

		if !hasCommand && !hasURL {
			errors = append(errors, fmt.Sprintf("Server %q: missing 'command' or 'url' field", serverName))
		}
		if hasCommand && hasURL {
			errors = append(errors, fmt.Sprintf("Server %q: has both 'command' and 'url' (should have only one)", serverName))
		}
	}

	return errors
}

func printValidationResult(result ValidationResult) {
	// Tool name and status
	if result.Valid {
		fmt.Printf("%s:\n", result.Tool)
		fmt.Printf("  ✓ Config syntax valid\n")
		if result.ServerCount > 0 {
			fmt.Printf("  ✓ %d server(s) configured\n", result.ServerCount)
		}
	} else {
		fmt.Printf("%s:\n", result.Tool)
		fmt.Printf("  ✗ Validation failed\n")
	}

	// Errors
	for _, err := range result.Errors {
		fmt.Printf("    Error: %s\n", err)
	}

	// Warnings
	for _, warn := range result.Warnings {
		fmt.Printf("    Warning: %s\n", warn)
	}

	// Config path
	if result.ConfigPath != "" {
		// Shorten home directory
		home, _ := os.UserHomeDir()
		path := result.ConfigPath
		if strings.HasPrefix(path, home) {
			path = "~" + path[len(home):]
		}
		fmt.Printf("  Config: %s\n", path)
	}

	fmt.Println()
}
