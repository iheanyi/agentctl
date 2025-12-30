package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/output"
	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:   "new <type> <name>",
	Short: "Create a new resource from template",
	Long: `Create a new command, rule, prompt, or skill from a template.

Examples:
  agentctl new command explain      # Create a new slash command
  agentctl new rule coding-style    # Create a new rule
  agentctl new prompt review        # Create a new prompt template
  agentctl new skill my-skill       # Create a new skill`,
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

	data, err := json.MarshalIndent(template, "", "  ")
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

	data, err := json.MarshalIndent(template, "", "  ")
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

	data, err := json.MarshalIndent(skillJSON, "", "  ")
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
