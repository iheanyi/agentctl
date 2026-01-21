package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/jsonutil"
	"github.com/iheanyi/agentctl/pkg/output"
	"github.com/iheanyi/agentctl/pkg/pathutil"
)

var newCmd = &cobra.Command{
	Use:   "new [type] [name]",
	Short: "Create a new resource from template",
	Long: `Create a new command, rule, prompt, or skill from a template.

If no arguments are provided, launches an interactive form.

Scope:
  By default, creates resources locally (.agentctl/) if .agentctl.json exists,
  otherwise globally (~/.config/agentctl/). Use --scope to override.

Examples:
  agentctl new                      # Interactive mode
  agentctl new command explain      # Create a new slash command
  agentctl new rule coding-style    # Create a new rule
  agentctl new rule project-rules --scope local   # Create local rule
  agentctl new prompt review        # Create a new prompt template
  agentctl new skill my-skill       # Create a new skill`,
	RunE: runNew,
}

var newScope string

var newCommandCmd = &cobra.Command{
	Use:   "command <name>",
	Short: "Create a new slash command",
	Args:  cobra.ExactArgs(1),
	RunE:  runNewCommand,
}

var newRuleCmd = &cobra.Command{
	Use:   "rule <name>",
	Short: "Create a new rule",
	Args:  cobra.ExactArgs(1),
	RunE:  runNewRule,
}

var newPromptCmd = &cobra.Command{
	Use:   "prompt <name>",
	Short: "Create a new prompt template",
	Args:  cobra.ExactArgs(1),
	RunE:  runNewPrompt,
}

var newSkillCmd = &cobra.Command{
	Use:   "skill <name>",
	Short: "Create a new skill",
	Args:  cobra.ExactArgs(1),
	RunE:  runNewSkill,
}

var newAgentCmd = &cobra.Command{
	Use:   "agent <name>",
	Short: "Create a new custom agent/subagent",
	Args:  cobra.ExactArgs(1),
	RunE:  runNewAgent,
}

func init() {
	newCmd.AddCommand(newCommandCmd)
	newCmd.AddCommand(newRuleCmd)
	newCmd.AddCommand(newPromptCmd)
	newCmd.AddCommand(newSkillCmd)
	newCmd.AddCommand(newAgentCmd)

	// Add scope flag to all new commands
	newCmd.PersistentFlags().StringVarP(&newScope, "scope", "s", "", "Config scope: local, global (default: local if .agentctl.json exists)")
}

// getResourceDir returns the directory for creating resources based on scope
func getResourceDir(resourceType string) (string, config.Scope, error) {
	// Determine scope
	var scope config.Scope
	if newScope != "" {
		var err error
		scope, err = config.ParseScope(newScope)
		if err != nil {
			return "", "", err
		}
	} else {
		// Smart default: local if project config exists, otherwise global
		if config.HasProjectConfig() {
			scope = config.ScopeLocal
		} else {
			scope = config.ScopeGlobal
		}
	}

	if scope == config.ScopeLocal {
		// Get current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return "", "", fmt.Errorf("failed to get working directory: %w", err)
		}
		return filepath.Join(cwd, ".agentctl", resourceType), scope, nil
	}

	// Global scope
	cfg, err := config.Load()
	if err != nil {
		return "", "", fmt.Errorf("failed to load config: %w", err)
	}
	return filepath.Join(cfg.ConfigDir, resourceType), scope, nil
}

func runNewCommand(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Validate name to prevent path traversal
	if err := pathutil.SanitizeName(name); err != nil {
		return fmt.Errorf("invalid command name: %w", err)
	}

	commandsDir, scope, err := getResourceDir("commands")
	if err != nil {
		return err
	}

	out := output.DefaultWriter()

	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		return fmt.Errorf("failed to create commands directory: %w", err)
	}

	commandPath := filepath.Join(commandsDir, name+".json")
	if _, err := os.Stat(commandPath); err == nil {
		return fmt.Errorf("command %q already exists", name)
	}

	template := map[string]interface{}{
		"name":        name,
		"description": "Description of what this command does",
		"prompt":      "Your prompt template here. Use {{variable}} for placeholders.",
		"args": map[string]interface{}{
			"example": map[string]interface{}{
				"type":        "string",
				"description": "Example argument",
				"required":    false,
			},
		},
		"allowedTools":    []string{},
		"disallowedTools": []string{},
	}

	data, err := jsonutil.MarshalIndent(template, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(commandPath, data, 0644); err != nil {
		return fmt.Errorf("failed to create command: %w", err)
	}

	if JSONOutput {
		jw := output.NewJSONWriter()
		return jw.WriteSuccess(output.NewResourceResult{
			Type:  "command",
			Name:  name,
			Scope: string(scope),
			Path:  commandPath,
		})
	}

	scopeLabel := ""
	if scope == config.ScopeLocal {
		scopeLabel = " (local)"
	} else {
		scopeLabel = " (global)"
	}
	out.Success("Created command %q%s", name, scopeLabel)
	out.Info("Edit %s to customize", commandPath)
	out.Println("")
	out.Println("Run 'agentctl sync' to sync to your tools.")

	return nil
}

func runNewRule(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Validate name to prevent path traversal
	if err := pathutil.SanitizeName(name); err != nil {
		return fmt.Errorf("invalid rule name: %w", err)
	}

	rulesDir, scope, err := getResourceDir("rules")
	if err != nil {
		return err
	}

	out := output.DefaultWriter()

	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		return fmt.Errorf("failed to create rules directory: %w", err)
	}

	rulePath := filepath.Join(rulesDir, name+".md")
	if _, err := os.Stat(rulePath); err == nil {
		return fmt.Errorf("rule %q already exists", name)
	}

	template := `---
priority: 1
tools: []
applies: "*"
---

# ` + name + `

Add your rules and guidelines here.

## Guidelines

- Guideline 1
- Guideline 2

## Examples

` + "```" + `
// Good example
` + "```" + `

` + "```" + `
// Bad example
` + "```" + `
`

	if err := os.WriteFile(rulePath, []byte(template), 0644); err != nil {
		return fmt.Errorf("failed to create rule: %w", err)
	}

	if JSONOutput {
		jw := output.NewJSONWriter()
		return jw.WriteSuccess(output.NewResourceResult{
			Type:  "rule",
			Name:  name,
			Scope: string(scope),
			Path:  rulePath,
		})
	}

	scopeLabel := ""
	if scope == config.ScopeLocal {
		scopeLabel = " (local)"
	} else {
		scopeLabel = " (global)"
	}
	out.Success("Created rule %q%s", name, scopeLabel)
	out.Info("Edit %s to customize", rulePath)
	out.Println("")
	out.Println("Run 'agentctl sync' to sync to your tools.")

	return nil
}

func runNewPrompt(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Validate name to prevent path traversal
	if err := pathutil.SanitizeName(name); err != nil {
		return fmt.Errorf("invalid prompt name: %w", err)
	}

	promptsDir, scope, err := getResourceDir("prompts")
	if err != nil {
		return err
	}

	out := output.DefaultWriter()

	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		return fmt.Errorf("failed to create prompts directory: %w", err)
	}

	promptPath := filepath.Join(promptsDir, name+".json")
	if _, err := os.Stat(promptPath); err == nil {
		return fmt.Errorf("prompt %q already exists", name)
	}

	template := map[string]interface{}{
		"name":        name,
		"description": "Description of this prompt template",
		"template":    "Your prompt template here.\n\nUse {{variable}} for placeholders.\n\n{{input}}",
		"variables":   []string{"input"},
	}

	data, err := jsonutil.MarshalIndent(template, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(promptPath, data, 0644); err != nil {
		return fmt.Errorf("failed to create prompt: %w", err)
	}

	if JSONOutput {
		jw := output.NewJSONWriter()
		return jw.WriteSuccess(output.NewResourceResult{
			Type:  "prompt",
			Name:  name,
			Scope: string(scope),
			Path:  promptPath,
		})
	}

	scopeLabel := ""
	if scope == config.ScopeLocal {
		scopeLabel = " (local)"
	} else {
		scopeLabel = " (global)"
	}
	out.Success("Created prompt %q%s", name, scopeLabel)
	out.Info("Edit %s to customize", promptPath)

	return nil
}

func runNewSkill(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Validate name to prevent path traversal
	if err := pathutil.SanitizeName(name); err != nil {
		return fmt.Errorf("invalid skill name: %w", err)
	}

	skillsDir, scope, err := getResourceDir("skills")
	if err != nil {
		return err
	}

	out := output.DefaultWriter()
	skillDir := filepath.Join(skillsDir, name)

	if _, err := os.Stat(skillDir); err == nil {
		return fmt.Errorf("skill %q already exists", name)
	}

	// Create skill directory
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("failed to create skill directory: %w", err)
	}

	// Create SKILL.md (Claude Code format)
	skillMd := `---
name: ` + name + `
description: Description of this skill
---

# ` + name + `

This is your skill prompt. It will be sent to the AI when users invoke this skill.

## Instructions

Add your instructions here. Be specific about:
- What this skill should do
- How it should behave
- What output format to use

## Usage

Use $ARGUMENTS to reference any arguments the user provides when invoking this skill.

Example: "Analyze $ARGUMENTS and provide feedback."
`

	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMd), 0644); err != nil {
		return fmt.Errorf("failed to create SKILL.md: %w", err)
	}

	if JSONOutput {
		jw := output.NewJSONWriter()
		return jw.WriteSuccess(output.NewResourceResult{
			Type:  "skill",
			Name:  name,
			Scope: string(scope),
			Path:  skillDir,
		})
	}

	scopeLabel := ""
	if scope == config.ScopeLocal {
		scopeLabel = " (local)"
	} else {
		scopeLabel = " (global)"
	}
	out.Success("Created skill %q%s", name, scopeLabel)
	out.Info("Edit %s to customize", filepath.Join(skillDir, "SKILL.md"))
	out.Println("")
	out.Println("Run 'agentctl sync' to sync to your tools.")

	return nil
}

func runNewAgent(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Validate name to prevent path traversal
	if err := pathutil.SanitizeName(name); err != nil {
		return fmt.Errorf("invalid agent name: %w", err)
	}

	agentsDir, scope, err := getResourceDir("agents")
	if err != nil {
		return err
	}

	out := output.DefaultWriter()

	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create agents directory: %w", err)
	}

	agentPath := filepath.Join(agentsDir, name+".md")
	if _, err := os.Stat(agentPath); err == nil {
		return fmt.Errorf("agent %q already exists", name)
	}

	template := `---
name: ` + name + `
description: Description of what this agent does
model: sonnet
tools:
  - Read
  - Grep
  - Glob
---

You are a specialized AI agent.

## Purpose

Describe what this agent is designed to do.

## Instructions

- Instruction 1
- Instruction 2
- Instruction 3

## Constraints

- Only perform actions within your designated scope
- Ask for clarification when requirements are unclear
`

	if err := os.WriteFile(agentPath, []byte(template), 0644); err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	if JSONOutput {
		jw := output.NewJSONWriter()
		return jw.WriteSuccess(output.NewResourceResult{
			Type:  "agent",
			Name:  name,
			Scope: string(scope),
			Path:  agentPath,
		})
	}

	scopeLabel := ""
	if scope == config.ScopeLocal {
		scopeLabel = " (local)"
	} else {
		scopeLabel = " (global)"
	}
	out.Success("Created agent %q%s", name, scopeLabel)
	out.Info("Edit %s to customize", agentPath)
	out.Println("")
	out.Println("Run 'agentctl sync' to sync to your tools.")

	return nil
}

// runNew handles the `agentctl new` command without subcommand
// It launches an interactive form to create a new resource
func runNew(cmd *cobra.Command, args []string) error {
	// If we have args, show help to guide user to subcommands
	if len(args) > 0 {
		return cmd.Help()
	}

	// Check if we're in an interactive terminal
	if err := requireInteractive("new"); err != nil {
		return err
	}

	return runInteractiveNew()
}

// resourceType represents the type of resource to create
type resourceType string

const (
	resourceCommand resourceType = "command"
	resourceRule    resourceType = "rule"
	resourcePrompt  resourceType = "prompt"
	resourceSkill   resourceType = "skill"
	resourceAgent   resourceType = "agent"
)

// runInteractiveNew launches the interactive form to create a new resource
func runInteractiveNew() error {
	// Detect context - is user in a project dir with .agentctl.json?
	inProject := config.HasProjectConfig()

	// Determine smart default based on context
	defaultResource := suggestResourceType(inProject)

	var selectedType string

	// Step 1: Select resource type
	typeForm := newStyledForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("What would you like to create?").
				Description(getResourceTypeDescription(inProject)).
				Options(
					huh.NewOption("Command - A slash command for AI tools", string(resourceCommand)),
					huh.NewOption("Rule - Guidelines for AI behavior", string(resourceRule)),
					huh.NewOption("Prompt - A reusable prompt template", string(resourcePrompt)),
					huh.NewOption("Skill - A complete skill package", string(resourceSkill)),
					huh.NewOption("Agent - A custom AI agent/subagent", string(resourceAgent)),
				).
				Value(&selectedType),
		),
	)

	// Set default selection
	selectedType = string(defaultResource)

	if ok, err := runFormWithCancel(typeForm, "new"); err != nil {
		return err
	} else if !ok {
		return nil
	}

	// Step 2: Based on selection, ask for relevant fields
	switch resourceType(selectedType) {
	case resourceCommand:
		return runInteractiveNewCommand()
	case resourceRule:
		return runInteractiveNewRule()
	case resourcePrompt:
		return runInteractiveNewPrompt()
	case resourceSkill:
		return runInteractiveNewSkill()
	case resourceAgent:
		return runInteractiveNewAgent()
	default:
		return fmt.Errorf("unknown resource type: %s", selectedType)
	}
}

// suggestResourceType returns the most likely resource type based on context
func suggestResourceType(inProject bool) resourceType {
	// If in a project, commands are most common for project-specific workflows
	if inProject {
		return resourceCommand
	}
	// For global config, rules are often the first thing users want
	return resourceRule
}

// getResourceTypeDescription returns a description based on context
func getResourceTypeDescription(inProject bool) string {
	if inProject {
		return "Creating in project directory"
	}
	return "Creating in global config (~/.config/agentctl)"
}

// runInteractiveNewCommand creates a new command via interactive form
func runInteractiveNewCommand() error {
	commandsDir, scope, err := getResourceDir("commands")
	if err != nil {
		return err
	}

	out := output.DefaultWriter()

	var name, description string

	// Get command details
	form := newStyledForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Command name").
				Description("The name users will type to invoke this command (e.g., 'explain', 'review')").
				Placeholder("my-command").
				Value(&name).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("name is required")
					}
					// Check if command already exists
					commandPath := filepath.Join(commandsDir, s+".json")
					if _, err := os.Stat(commandPath); err == nil {
						return fmt.Errorf("command %q already exists", s)
					}
					return nil
				}),
			huh.NewInput().
				Title("Description").
				Description("What does this command do?").
				Placeholder("Description of what this command does").
				Value(&description),
		),
	)

	if ok, err := runFormWithCancel(form, "new command"); err != nil {
		return err
	} else if !ok {
		return nil
	}

	// Create the command
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		return fmt.Errorf("failed to create commands directory: %w", err)
	}

	commandPath := filepath.Join(commandsDir, name+".json")

	// Use default description if not provided
	if description == "" {
		description = "Description of what this command does"
	}

	template := map[string]interface{}{
		"name":        name,
		"description": description,
		"prompt":      "Your prompt template here. Use {{variable}} for placeholders.",
		"args": map[string]interface{}{
			"example": map[string]interface{}{
				"type":        "string",
				"description": "Example argument",
				"required":    false,
			},
		},
		"allowedTools":    []string{},
		"disallowedTools": []string{},
	}

	data, err := jsonutil.MarshalIndent(template, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(commandPath, data, 0644); err != nil {
		return fmt.Errorf("failed to create command: %w", err)
	}

	scopeLabel := ""
	if scope == config.ScopeLocal {
		scopeLabel = " (local)"
	} else {
		scopeLabel = " (global)"
	}
	out.Println("")
	out.Success("Created command %q%s", name, scopeLabel)
	out.Info("Edit %s to customize", commandPath)
	out.Println("")
	out.Println("Run 'agentctl sync' to sync to your tools.")

	return nil
}

// runInteractiveNewRule creates a new rule via interactive form
func runInteractiveNewRule() error {
	rulesDir, scope, err := getResourceDir("rules")
	if err != nil {
		return err
	}

	out := output.DefaultWriter()

	var name, description string

	// Get rule details
	form := newStyledForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Rule name").
				Description("A short identifier for this rule (e.g., 'coding-style', 'security')").
				Placeholder("my-rule").
				Value(&name).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("name is required")
					}
					// Check if rule already exists
					rulePath := filepath.Join(rulesDir, s+".md")
					if _, err := os.Stat(rulePath); err == nil {
						return fmt.Errorf("rule %q already exists", s)
					}
					return nil
				}),
			huh.NewInput().
				Title("Description").
				Description("Brief description of what guidelines this rule contains").
				Placeholder("Guidelines for...").
				Value(&description),
		),
	)

	if ok, err := runFormWithCancel(form, "new rule"); err != nil {
		return err
	} else if !ok {
		return nil
	}

	// Create the rule
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		return fmt.Errorf("failed to create rules directory: %w", err)
	}

	rulePath := filepath.Join(rulesDir, name+".md")

	// Build the rule template
	ruleDescription := description
	if ruleDescription == "" {
		ruleDescription = "Add your rules and guidelines here."
	}

	template := `---
priority: 1
tools: []
applies: "*"
---

# ` + name + `

` + ruleDescription + `

## Guidelines

- Guideline 1
- Guideline 2

## Examples

` + "```" + `
// Good example
` + "```" + `

` + "```" + `
// Bad example
` + "```" + `
`

	if err := os.WriteFile(rulePath, []byte(template), 0644); err != nil {
		return fmt.Errorf("failed to create rule: %w", err)
	}

	scopeLabel := ""
	if scope == config.ScopeLocal {
		scopeLabel = " (local)"
	} else {
		scopeLabel = " (global)"
	}
	out.Println("")
	out.Success("Created rule %q%s", name, scopeLabel)
	out.Info("Edit %s to customize", rulePath)
	out.Println("")
	out.Println("Run 'agentctl sync' to sync to your tools.")

	return nil
}

// runInteractiveNewPrompt creates a new prompt via interactive form
func runInteractiveNewPrompt() error {
	promptsDir, scope, err := getResourceDir("prompts")
	if err != nil {
		return err
	}

	out := output.DefaultWriter()

	var name, description string

	// Get prompt details
	form := newStyledForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Prompt name").
				Description("A short identifier for this prompt template (e.g., 'review', 'explain')").
				Placeholder("my-prompt").
				Value(&name).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("name is required")
					}
					// Check if prompt already exists
					promptPath := filepath.Join(promptsDir, s+".json")
					if _, err := os.Stat(promptPath); err == nil {
						return fmt.Errorf("prompt %q already exists", s)
					}
					return nil
				}),
			huh.NewInput().
				Title("Description").
				Description("What is this prompt template used for?").
				Placeholder("Description of this prompt template").
				Value(&description),
		),
	)

	if ok, err := runFormWithCancel(form, "new prompt"); err != nil {
		return err
	} else if !ok {
		return nil
	}

	// Create the prompt
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		return fmt.Errorf("failed to create prompts directory: %w", err)
	}

	promptPath := filepath.Join(promptsDir, name+".json")

	// Use default description if not provided
	if description == "" {
		description = "Description of this prompt template"
	}

	template := map[string]interface{}{
		"name":        name,
		"description": description,
		"template":    "Your prompt template here.\n\nUse {{variable}} for placeholders.\n\n{{input}}",
		"variables":   []string{"input"},
	}

	data, err := jsonutil.MarshalIndent(template, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(promptPath, data, 0644); err != nil {
		return fmt.Errorf("failed to create prompt: %w", err)
	}

	scopeLabel := ""
	if scope == config.ScopeLocal {
		scopeLabel = " (local)"
	} else {
		scopeLabel = " (global)"
	}
	out.Println("")
	out.Success("Created prompt %q%s", name, scopeLabel)
	out.Info("Edit %s to customize", promptPath)

	return nil
}

// runInteractiveNewSkill creates a new skill via interactive form
func runInteractiveNewSkill() error {
	skillsDir, scope, err := getResourceDir("skills")
	if err != nil {
		return err
	}

	out := output.DefaultWriter()

	var name, description string

	// Get skill details
	form := newStyledForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Skill name").
				Description("A short identifier for this skill (e.g., 'code-review', 'testing')").
				Placeholder("my-skill").
				Value(&name).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("name is required")
					}
					// Check if skill already exists
					skillDir := filepath.Join(skillsDir, s)
					if _, err := os.Stat(skillDir); err == nil {
						return fmt.Errorf("skill %q already exists", s)
					}
					return nil
				}),
			huh.NewInput().
				Title("Description").
				Description("What does this skill do?").
				Placeholder("Description of this skill").
				Value(&description),
		),
	)

	if ok, err := runFormWithCancel(form, "new skill"); err != nil {
		return err
	} else if !ok {
		return nil
	}

	// Create the skill directory
	skillDir := filepath.Join(skillsDir, name)

	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("failed to create skill directory: %w", err)
	}

	// Use default description if not provided
	if description == "" {
		description = "Description of this skill"
	}

	// Create SKILL.md (Claude Code format)
	skillMd := `---
name: ` + name + `
description: ` + description + `
---

# ` + name + `

This is your skill prompt. It will be sent to the AI when users invoke this skill.

## Instructions

Add your instructions here. Be specific about:
- What this skill should do
- How it should behave
- What output format to use

## Usage

Use $ARGUMENTS to reference any arguments the user provides when invoking this skill.

Example: "Analyze $ARGUMENTS and provide feedback."
`

	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMd), 0644); err != nil {
		return fmt.Errorf("failed to create SKILL.md: %w", err)
	}

	scopeLabel := ""
	if scope == config.ScopeLocal {
		scopeLabel = " (local)"
	} else {
		scopeLabel = " (global)"
	}
	out.Println("")
	out.Success("Created skill %q%s", name, scopeLabel)
	out.Info("Edit %s to customize", filepath.Join(skillDir, "SKILL.md"))
	out.Println("")
	out.Println("Run 'agentctl sync' to sync to your tools.")

	return nil
}

// runInteractiveNewAgent creates a new agent via interactive form
func runInteractiveNewAgent() error {
	agentsDir, scope, err := getResourceDir("agents")
	if err != nil {
		return err
	}

	out := output.DefaultWriter()

	var name, description string

	// Get agent details
	form := newStyledForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Agent name").
				Description("A short identifier for this agent (e.g., 'code-reviewer', 'security-auditor')").
				Placeholder("my-agent").
				Value(&name).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("name is required")
					}
					// Check if agent already exists
					agentPath := filepath.Join(agentsDir, s+".md")
					if _, err := os.Stat(agentPath); err == nil {
						return fmt.Errorf("agent %q already exists", s)
					}
					return nil
				}),
			huh.NewInput().
				Title("Description").
				Description("What does this agent do?").
				Placeholder("Description of this agent").
				Value(&description),
		),
	)

	if ok, err := runFormWithCancel(form, "new agent"); err != nil {
		return err
	} else if !ok {
		return nil
	}

	// Create the agent
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create agents directory: %w", err)
	}

	agentPath := filepath.Join(agentsDir, name+".md")

	// Use default description if not provided
	if description == "" {
		description = "Description of what this agent does"
	}

	template := `---
name: ` + name + `
description: ` + description + `
model: sonnet
tools:
  - Read
  - Grep
  - Glob
---

You are a specialized AI agent.

## Purpose

Describe what this agent is designed to do.

## Instructions

- Instruction 1
- Instruction 2
- Instruction 3

## Constraints

- Only perform actions within your designated scope
- Ask for clarification when requirements are unclear
`

	if err := os.WriteFile(agentPath, []byte(template), 0644); err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	scopeLabel := ""
	if scope == config.ScopeLocal {
		scopeLabel = " (local)"
	} else {
		scopeLabel = " (global)"
	}
	out.Println("")
	out.Success("Created agent %q%s", name, scopeLabel)
	out.Info("Edit %s to customize", agentPath)
	out.Println("")
	out.Println("Run 'agentctl sync' to sync to your tools.")

	return nil
}
