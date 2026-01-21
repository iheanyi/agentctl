package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// CLIOutput represents a structured output for machine-parseable JSON responses
type CLIOutput struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// JSONWriter handles JSON output for CLI commands
type JSONWriter struct {
	Out io.Writer
}

// NewJSONWriter creates a new JSON writer that outputs to stdout
func NewJSONWriter() *JSONWriter {
	return &JSONWriter{Out: os.Stdout}
}

// Write outputs a CLIOutput as JSON
func (w *JSONWriter) Write(output CLIOutput) error {
	encoder := json.NewEncoder(w.Out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// WriteSuccess outputs a successful result as JSON
func (w *JSONWriter) WriteSuccess(data interface{}) error {
	return w.Write(CLIOutput{
		Success: true,
		Data:    data,
	})
}

// WriteError outputs an error as JSON
func (w *JSONWriter) WriteError(err error) error {
	return w.Write(CLIOutput{
		Success: false,
		Error:   err.Error(),
	})
}

// WriteErrorString outputs an error string as JSON
func (w *JSONWriter) WriteErrorString(errMsg string) error {
	return w.Write(CLIOutput{
		Success: false,
		Error:   errMsg,
	})
}

// JSON output types for specific commands

// ListOutput represents the JSON output for the list command
type ListOutput struct {
	ProjectPath string        `json:"projectPath,omitempty"`
	Servers     []ServerInfo  `json:"servers,omitempty"`
	Commands    []CommandInfo `json:"commands,omitempty"`
	Rules       []RuleInfo    `json:"rules,omitempty"`
	Skills      []SkillInfo   `json:"skills,omitempty"`
}

// ServerInfo represents server information in JSON output
type ServerInfo struct {
	Name      string `json:"name"`
	Scope     string `json:"scope"`
	Source    string `json:"source"`
	Status    string `json:"status"`
	Command   string `json:"command,omitempty"`
	URL       string `json:"url,omitempty"`
	Transport string `json:"transport,omitempty"`
}

// CommandInfo represents command information in JSON output
type CommandInfo struct {
	Name        string `json:"name"`
	Scope       string `json:"scope"`
	Tool        string `json:"tool,omitempty"`
	Description string `json:"description,omitempty"`
}

// RuleInfo represents rule information in JSON output
type RuleInfo struct {
	Name   string `json:"name"`
	Scope  string `json:"scope"`
	Tool   string `json:"tool,omitempty"`
	Path   string `json:"path,omitempty"`
}

// SkillInfo represents skill information in JSON output
type SkillInfo struct {
	Name        string `json:"name"`
	Scope       string `json:"scope"`
	Tool        string `json:"tool,omitempty"`
	Description string `json:"description,omitempty"`
}

// SyncOutput represents the JSON output for the sync command
type SyncOutput struct {
	DryRun      bool             `json:"dryRun"`
	ProjectPath string           `json:"projectPath,omitempty"`
	ToolResults []SyncToolResult `json:"toolResults"`
	Summary     SyncSummary      `json:"summary"`
}

// SyncToolResult represents the sync result for a single tool
type SyncToolResult struct {
	Tool           string       `json:"tool"`
	ConfigPath     string       `json:"configPath"`
	Success        bool         `json:"success"`
	Error          string       `json:"error,omitempty"`
	ServersAdded   int          `json:"serversAdded,omitempty"`
	ServersUpdated int          `json:"serversUpdated,omitempty"`
	ServersRemoved int          `json:"serversRemoved,omitempty"`
	CommandsSynced int          `json:"commandsSynced,omitempty"`
	RulesSynced    int          `json:"rulesSynced,omitempty"`
	Changes        []SyncChange `json:"changes,omitempty"`
}

// SyncChange represents a single change during sync
type SyncChange struct {
	Type     string `json:"type"`     // "add", "update", "remove", "preserve"
	Resource string `json:"resource"` // "server", "command", "rule"
	Name     string `json:"name"`
}

// SyncSummary represents the summary of a sync operation
type SyncSummary struct {
	ToolsSucceeded int `json:"toolsSucceeded"`
	ToolsFailed    int `json:"toolsFailed"`
	TotalServers   int `json:"totalServers"`
	TotalCommands  int `json:"totalCommands"`
	TotalRules     int `json:"totalRules"`
}

// DoctorOutput represents the JSON output for the doctor command
type DoctorOutput struct {
	Config     DoctorConfigResult    `json:"config"`
	Runtimes   []DoctorRuntimeResult `json:"runtimes"`
	Tools      []DoctorToolResult    `json:"tools"`
	Servers    []DoctorServerResult  `json:"servers,omitempty"`
	SyncState  DoctorSyncState       `json:"syncState"`
	System     DoctorSystemInfo      `json:"system"`
	IssueCount int                   `json:"issueCount"`
}

// DoctorConfigResult represents the config check result
type DoctorConfigResult struct {
	Path        string `json:"path"`
	Valid       bool   `json:"valid"`
	Error       string `json:"error,omitempty"`
	ServerCount int    `json:"serverCount"`
}

// DoctorRuntimeResult represents a runtime check result
type DoctorRuntimeResult struct {
	Name     string `json:"name"`
	Command  string `json:"command"`
	Version  string `json:"version,omitempty"`
	Found    bool   `json:"found"`
	Required bool   `json:"required"`
}

// DoctorToolResult represents a tool check result
type DoctorToolResult struct {
	Name        string `json:"name"`
	ConfigPath  string `json:"configPath"`
	Detected    bool   `json:"detected"`
	Valid       bool   `json:"valid"`
	Error       string `json:"error,omitempty"`
	ServerCount int    `json:"serverCount"`
}

// DoctorServerResult represents an MCP server check result
type DoctorServerResult struct {
	Name      string `json:"name"`
	Type      string `json:"type"` // "stdio" or "http"
	Disabled  bool   `json:"disabled"`
	Available bool   `json:"available"`
	Error     string `json:"error,omitempty"`
}

// DoctorSyncState represents the sync state check result
type DoctorSyncState struct {
	Path           string `json:"path"`
	Valid          bool   `json:"valid"`
	Error          string `json:"error,omitempty"`
	ManagedServers int    `json:"managedServers"`
	AdapterCount   int    `json:"adapterCount"`
}

// DoctorSystemInfo represents system information
type DoctorSystemInfo struct {
	OS              string `json:"os"`
	Arch            string `json:"arch"`
	AgentctlVersion string `json:"agentctlVersion"`
	AgentctlCommit  string `json:"agentctlCommit"`
}

// ValidateOutput represents the JSON output for the validate command
type ValidateOutput struct {
	Results []ValidateToolResult `json:"results"`
	Summary ValidateSummary      `json:"summary"`
}

// ValidateToolResult represents the validation result for a single tool
type ValidateToolResult struct {
	Tool        string   `json:"tool"`
	ConfigPath  string   `json:"configPath"`
	Valid       bool     `json:"valid"`
	Errors      []string `json:"errors,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
	ServerCount int      `json:"serverCount"`
}

// ValidateSummary represents the summary of validation
type ValidateSummary struct {
	TotalTools   int `json:"totalTools"`
	ValidTools   int `json:"validTools"`
	InvalidTools int `json:"invalidTools"`
}

// PrintJSON is a helper function to print any data as JSON
func PrintJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// MustPrintJSON prints JSON and panics on error (for simple cases)
func MustPrintJSON(data interface{}) {
	if err := PrintJSON(data); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
	}
}
