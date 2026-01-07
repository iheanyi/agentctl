package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/jsonutil"
	"github.com/iheanyi/agentctl/pkg/output"
	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:   "new [type] [name]",
	Short: "Create a new resource from template",
	Long: `Create a new command, rule, prompt, or skill from a template.

If no arguments are provided, launches an interactive form.

Examples:
  agentctl new                      # Interactive mode
  agentctl new command explain      # Create a new slash command
  agentctl new rule coding-style    # Create a new rule
  agentctl new prompt review        # Create a new prompt template
  agentctl new skill my-skill       # Create a new skill`,
	RunE: runNew,
}

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

func init() {
	newCmd.AddCommand(newCommandCmd)
	newCmd.AddCommand(newRuleCmd)
	newCmd.AddCommand(newPromptCmd)
	newCmd.AddCommand(newSkillCmd)
}

func runNewCommand(cmd *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	out := output.DefaultWriter()
	commandsDir := filepath.Join(cfg.ConfigDir, "commands")

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

	out.Success("Created command %q", name)
	out.Info("Edit %s to customize", commandPath)
	out.Println("")
	out.Println("Run 'agentctl sync' to sync to your tools.")

	return nil
}

func runNewRule(cmd *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	out := output.DefaultWriter()
	rulesDir := filepath.Join(cfg.ConfigDir, "rules")

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

	out.Success("Created rule %q", name)
	out.Info("Edit %s to customize", rulePath)
	out.Println("")
	out.Println("Run 'agentctl sync' to sync to your tools.")

	return nil
}

func runNewPrompt(cmd *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	out := output.DefaultWriter()
	promptsDir := filepath.Join(cfg.ConfigDir, "prompts")

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

	out.Success("Created prompt %q", name)
	out.Info("Edit %s to customize", promptPath)

	return nil
}

func runNewSkill(cmd *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	out := output.DefaultWriter()
	skillsDir := filepath.Join(cfg.ConfigDir, "skills")
	skillDir := filepath.Join(skillsDir, name)

	if _, err := os.Stat(skillDir); err == nil {
		return fmt.Errorf("skill %q already exists", name)
	}

	// Create skill directory structure
	dirs := []string{
		skillDir,
		filepath.Join(skillDir, "prompts"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// Create skill.json
	skillJSON := map[string]interface{}{
		"name":        name,
		"description": "Description of this skill",
		"version":     "1.0.0",
		"author":      "",
	}

	data, err := jsonutil.MarshalIndent(skillJSON, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(skillDir, "skill.json"), data, 0644); err != nil {
		return fmt.Errorf("failed to create skill.json: %w", err)
	}

	// Create main prompt
	mainPrompt := `# ` + name + `

This is the main prompt for your skill.

## What This Skill Does

Describe what this skill does here.

## How to Use

Explain how to use this skill.

## Examples

Provide usage examples.
`

	if err := os.WriteFile(filepath.Join(skillDir, "prompts", "main.md"), []byte(mainPrompt), 0644); err != nil {
		return fmt.Errorf("failed to create main prompt: %w", err)
	}

	out.Success("Created skill %q", name)
	out.Info("Skill directory: %s", skillDir)
	out.Println("")
	out.Println("Files created:")
	out.List([]string{
		"skill.json - Skill metadata",
		"prompts/main.md - Main skill prompt",
	})
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
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	out := output.DefaultWriter()
	commandsDir := filepath.Join(cfg.ConfigDir, "commands")

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

	out.Println("")
	out.Success("Created command %q", name)
	out.Info("Edit %s to customize", commandPath)
	out.Println("")
	out.Println("Run 'agentctl sync' to sync to your tools.")

	return nil
}

// runInteractiveNewRule creates a new rule via interactive form
func runInteractiveNewRule() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	out := output.DefaultWriter()
	rulesDir := filepath.Join(cfg.ConfigDir, "rules")

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

	out.Println("")
	out.Success("Created rule %q", name)
	out.Info("Edit %s to customize", rulePath)
	out.Println("")
	out.Println("Run 'agentctl sync' to sync to your tools.")

	return nil
}

// runInteractiveNewPrompt creates a new prompt via interactive form
func runInteractiveNewPrompt() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	out := output.DefaultWriter()
	promptsDir := filepath.Join(cfg.ConfigDir, "prompts")

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

	out.Println("")
	out.Success("Created prompt %q", name)
	out.Info("Edit %s to customize", promptPath)

	return nil
}

// runInteractiveNewSkill creates a new skill via interactive form
func runInteractiveNewSkill() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	out := output.DefaultWriter()
	skillsDir := filepath.Join(cfg.ConfigDir, "skills")

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

	// Create the skill
	skillDir := filepath.Join(skillsDir, name)

	// Create skill directory structure
	dirs := []string{
		skillDir,
		filepath.Join(skillDir, "prompts"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// Use default description if not provided
	if description == "" {
		description = "Description of this skill"
	}

	// Create skill.json
	skillJSON := map[string]interface{}{
		"name":        name,
		"description": description,
		"version":     "1.0.0",
		"author":      "",
	}

	data, err := jsonutil.MarshalIndent(skillJSON, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(skillDir, "skill.json"), data, 0644); err != nil {
		return fmt.Errorf("failed to create skill.json: %w", err)
	}

	// Create main prompt
	mainPrompt := `# ` + name + `

` + description + `

## What This Skill Does

Describe what this skill does here.

## How to Use

Explain how to use this skill.

## Examples

Provide usage examples.
`

	if err := os.WriteFile(filepath.Join(skillDir, "prompts", "main.md"), []byte(mainPrompt), 0644); err != nil {
		return fmt.Errorf("failed to create main prompt: %w", err)
	}

	out.Println("")
	out.Success("Created skill %q", name)
	out.Info("Skill directory: %s", skillDir)
	out.Println("")
	out.Println("Files created:")
	out.List([]string{
		"skill.json - Skill metadata",
		"prompts/main.md - Main skill prompt",
	})
	out.Println("")
	out.Println("Run 'agentctl sync' to sync to your tools.")

	return nil
}
