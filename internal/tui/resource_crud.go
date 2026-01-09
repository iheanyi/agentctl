package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/iheanyi/agentctl/pkg/command"
	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/jsonutil"
	"github.com/iheanyi/agentctl/pkg/mcp"
	"github.com/iheanyi/agentctl/pkg/prompt"
	"github.com/iheanyi/agentctl/pkg/rule"
	"github.com/iheanyi/agentctl/pkg/skill"
)

// ResourceCRUD handles create, read, update, delete operations for resources
type ResourceCRUD struct {
	cfg *config.Config
}

// NewResourceCRUD creates a new resource CRUD handler
func NewResourceCRUD(cfg *config.Config) *ResourceCRUD {
	return &ResourceCRUD{cfg: cfg}
}

// CreateCommand creates a new command via interactive form
func (r *ResourceCRUD) CreateCommand() (*command.Command, error) {
	commandsDir := filepath.Join(r.cfg.ConfigDir, "commands")

	var name, description, argumentHint, model, promptText string
	var allowedTools, disallowedTools []string
	var hasArgs bool

	toolOptions := []huh.Option[string]{
		huh.NewOption("Read", "Read"),
		huh.NewOption("Write", "Write"),
		huh.NewOption("Edit", "Edit"),
		huh.NewOption("Bash", "Bash"),
		huh.NewOption("Glob", "Glob"),
		huh.NewOption("Grep", "Grep"),
		huh.NewOption("WebFetch", "WebFetch"),
		huh.NewOption("WebSearch", "WebSearch"),
		huh.NewOption("Task", "Task"),
		huh.NewOption("TodoWrite", "TodoWrite"),
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().Title("Create New Command"),
			huh.NewInput().
				Title("Name").
				Description("The slash command name (e.g., 'review', 'explain')").
				Placeholder("my-command").
				Value(&name).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("name is required")
					}
					if strings.ContainsAny(s, " \t\n/") {
						return fmt.Errorf("name cannot contain spaces or slashes")
					}
					commandPath := filepath.Join(commandsDir, s+".json")
					if _, err := os.Stat(commandPath); err == nil {
						return fmt.Errorf("command %q already exists", s)
					}
					return nil
				}),
			huh.NewInput().
				Title("Description").
				Description("What does this command do? (shown in help)").
				Placeholder("Describe the command").
				Value(&description),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Argument Hint").
				Description("Hint shown to user for expected arguments (optional)").
				Placeholder("[file.md or feature description]").
				Value(&argumentHint),
			huh.NewSelect[string]().
				Title("Preferred Model").
				Description("Which model should this command use? (optional)").
				Options(
					huh.NewOption("Default (no preference)", ""),
					huh.NewOption("Opus (most capable)", "opus"),
					huh.NewOption("Sonnet (balanced)", "sonnet"),
					huh.NewOption("Haiku (fast)", "haiku"),
				).
				Value(&model),
		),
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Allowed Tools (optional)").
				Description("Restrict command to only use these tools. Leave empty for all.").
				Options(toolOptions...).
				Value(&allowedTools),
		),
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Disallowed Tools (optional)").
				Description("Prevent command from using these tools.").
				Options(toolOptions...).
				Value(&disallowedTools),
		),
		huh.NewGroup(
			huh.NewText().
				Title("Prompt").
				Description("The prompt template. Use $ARGUMENTS for user input, {{var}} for args").
				Placeholder("Review this code for...\n\n$ARGUMENTS").
				Value(&promptText).
				CharLimit(4000),
			huh.NewConfirm().
				Title("Add structured argument definitions?").
				Description("For typed args with validation (can edit in JSON later)").
				Value(&hasArgs),
		),
	)

	if err := form.Run(); err != nil {
		return nil, err
	}

	// Create the command
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create commands directory: %w", err)
	}

	if description == "" {
		description = "No description"
	}
	if promptText == "" {
		promptText = "$ARGUMENTS"
	}

	cmd := &command.Command{
		Name:            name,
		Description:     description,
		ArgumentHint:    argumentHint,
		Model:           model,
		Prompt:          promptText,
		AllowedTools:    allowedTools,
		DisallowedTools: disallowedTools,
	}

	// If user wants args, add example structure
	if hasArgs {
		cmd.Args = map[string]command.Arg{
			"target": {
				Type:        "string",
				Description: "The target to operate on",
				Required:    true,
			},
			"verbose": {
				Type:        "boolean",
				Description: "Enable verbose output",
				Default:     false,
			},
		}
	}

	commandPath := filepath.Join(commandsDir, name+".json")
	data, err := jsonutil.MarshalIndent(cmd, "", "  ")
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(commandPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to create command: %w", err)
	}

	return cmd, nil
}

// EditCommand opens the command in the user's editor
func (r *ResourceCRUD) EditCommand(cmd *command.Command) error {
	commandPath := filepath.Join(r.cfg.ConfigDir, "commands", cmd.Name+".json")
	return r.openInEditor(commandPath)
}

// DeleteCommand deletes a command
func (r *ResourceCRUD) DeleteCommand(cmd *command.Command) error {
	commandPath := filepath.Join(r.cfg.ConfigDir, "commands", cmd.Name+".json")
	return os.Remove(commandPath)
}

// CreateRule creates a new rule via interactive form
func (r *ResourceCRUD) CreateRule() (*rule.Rule, error) {
	rulesDir := filepath.Join(r.cfg.ConfigDir, "rules")

	var name, description, appliesPattern string
	var priority int = 1
	var selectedTools []string

	toolOptions := []huh.Option[string]{
		huh.NewOption("All tools", "*"),
		huh.NewOption("Claude Code", "claude-code"),
		huh.NewOption("Cursor", "cursor"),
		huh.NewOption("Windsurf", "windsurf"),
		huh.NewOption("Cline", "cline"),
		huh.NewOption("Continue", "continue"),
		huh.NewOption("Zed", "zed"),
		huh.NewOption("OpenCode", "opencode"),
		huh.NewOption("Codex", "codex"),
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().Title("Create New Rule"),
			huh.NewInput().
				Title("Name").
				Description("A short identifier for this rule (becomes filename)").
				Placeholder("my-rule").
				Value(&name).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("name is required")
					}
					if strings.ContainsAny(s, " \t\n/\\") {
						return fmt.Errorf("name cannot contain spaces or path separators")
					}
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
		huh.NewGroup(
			huh.NewSelect[int]().
				Title("Priority").
				Description("Higher priority rules are applied first (1-10)").
				Options(
					huh.NewOption("1 - Low", 1),
					huh.NewOption("3 - Normal", 3),
					huh.NewOption("5 - Medium", 5),
					huh.NewOption("7 - High", 7),
					huh.NewOption("10 - Critical", 10),
				).
				Value(&priority),
			huh.NewMultiSelect[string]().
				Title("Target Tools").
				Description("Which tools should this rule apply to?").
				Options(toolOptions...).
				Value(&selectedTools),
			huh.NewInput().
				Title("Applies Pattern (optional)").
				Description("File glob pattern this rule applies to (e.g., '*.ts', 'src/**/*.go')").
				Placeholder("*").
				Value(&appliesPattern),
		),
	)

	if err := form.Run(); err != nil {
		return nil, err
	}

	// Create the rule
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create rules directory: %w", err)
	}

	if description == "" {
		description = "Add your guidelines here."
	}
	if appliesPattern == "" {
		appliesPattern = "*"
	}

	// Build tools array for frontmatter
	toolsStr := "[]"
	if len(selectedTools) > 0 && !(len(selectedTools) == 1 && selectedTools[0] == "*") {
		filtered := []string{}
		for _, t := range selectedTools {
			if t != "*" {
				filtered = append(filtered, t)
			}
		}
		if len(filtered) > 0 {
			toolsStr = "[" + strings.Join(filtered, ", ") + "]"
		}
	}

	template := fmt.Sprintf(`---
priority: %d
tools: %s
applies: "%s"
---

# %s

%s

## Guidelines

- Guideline 1
- Guideline 2

## Examples

`+"```"+`
// Good example
`+"```"+`

`+"```"+`
// Bad example
`+"```"+`
`, priority, toolsStr, appliesPattern, name, description)

	rulePath := filepath.Join(rulesDir, name+".md")
	if err := os.WriteFile(rulePath, []byte(template), 0644); err != nil {
		return nil, fmt.Errorf("failed to create rule: %w", err)
	}

	return &rule.Rule{
		Name: name,
		Path: rulePath,
		Frontmatter: &rule.Frontmatter{
			Priority: priority,
			Tools:    selectedTools,
			Applies:  appliesPattern,
		},
	}, nil
}

// EditRule opens the rule in the user's editor
func (r *ResourceCRUD) EditRule(rl *rule.Rule) error {
	return r.openInEditor(rl.Path)
}

// DeleteRule deletes a rule
func (r *ResourceCRUD) DeleteRule(rl *rule.Rule) error {
	return os.Remove(rl.Path)
}

// CreatePrompt creates a new prompt via interactive form
func (r *ResourceCRUD) CreatePrompt() (*prompt.Prompt, error) {
	promptsDir := filepath.Join(r.cfg.ConfigDir, "prompts")

	var name, description, templateText string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().Title("Create New Prompt Template"),
			huh.NewInput().
				Title("Name").
				Description("A short identifier for this prompt template").
				Placeholder("my-prompt").
				Value(&name).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("name is required")
					}
					if strings.ContainsAny(s, " \t\n/\\") {
						return fmt.Errorf("name cannot contain spaces or path separators")
					}
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
			huh.NewText().
				Title("Template").
				Description("Use {{variable}} for placeholders. Variables will be auto-extracted.").
				Placeholder("You are a {{role}} expert.\n\nAnalyze the following:\n{{input}}").
				Value(&templateText).
				CharLimit(4000),
		),
	)

	if err := form.Run(); err != nil {
		return nil, err
	}

	// Create the prompt
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create prompts directory: %w", err)
	}

	if description == "" {
		description = "No description"
	}
	if templateText == "" {
		templateText = "{{input}}"
	}

	// Auto-extract variables from template
	variables := extractVariables(templateText)

	p := &prompt.Prompt{
		Name:        name,
		Description: description,
		Template:    templateText,
		Variables:   variables,
	}

	promptPath := filepath.Join(promptsDir, name+".json")
	data, err := jsonutil.MarshalIndent(p, "", "  ")
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(promptPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to create prompt: %w", err)
	}

	return p, nil
}

// extractVariables extracts {{variable}} patterns from a template string
func extractVariables(template string) []string {
	re := regexp.MustCompile(`\{\{(\w+)\}\}`)
	matches := re.FindAllStringSubmatch(template, -1)

	seen := make(map[string]bool)
	var variables []string
	for _, match := range matches {
		if len(match) > 1 && !seen[match[1]] {
			seen[match[1]] = true
			variables = append(variables, match[1])
		}
	}
	return variables
}

// EditPrompt opens the prompt in the user's editor
func (r *ResourceCRUD) EditPrompt(p *prompt.Prompt) error {
	promptPath := filepath.Join(r.cfg.ConfigDir, "prompts", p.Name+".json")
	return r.openInEditor(promptPath)
}

// DeletePrompt deletes a prompt
func (r *ResourceCRUD) DeletePrompt(p *prompt.Prompt) error {
	promptPath := filepath.Join(r.cfg.ConfigDir, "prompts", p.Name+".json")
	return os.Remove(promptPath)
}

// CreateSkill creates a new skill via interactive form
func (r *ResourceCRUD) CreateSkill() (*skill.Skill, error) {
	skillsDir := filepath.Join(r.cfg.ConfigDir, "skills")

	var name, description, version, author string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().Title("Create New Skill"),
			huh.NewInput().
				Title("Name").
				Description("A short identifier for this skill (becomes directory name)").
				Placeholder("my-skill").
				Value(&name).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("name is required")
					}
					if strings.ContainsAny(s, " \t\n/\\") {
						return fmt.Errorf("name cannot contain spaces or path separators")
					}
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
		huh.NewGroup(
			huh.NewInput().
				Title("Version").
				Description("Semantic version (optional)").
				Placeholder("1.0.0").
				Value(&version),
			huh.NewInput().
				Title("Author").
				Description("Your name or handle (optional)").
				Placeholder("Your Name").
				Value(&author),
		),
	)

	if err := form.Run(); err != nil {
		return nil, err
	}

	// Create the skill directory structure
	skillDir := filepath.Join(skillsDir, name)

	dirs := []string{
		skillDir,
		filepath.Join(skillDir, "prompts"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}
	}

	if description == "" {
		description = "No description"
	}
	if version == "" {
		version = "1.0.0"
	}

	// Create skill.json
	skillJSON := map[string]interface{}{
		"name":        name,
		"description": description,
		"version":     version,
	}
	if author != "" {
		skillJSON["author"] = author
	}

	data, err := jsonutil.MarshalIndent(skillJSON, "", "  ")
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(filepath.Join(skillDir, "skill.json"), data, 0644); err != nil {
		return nil, fmt.Errorf("failed to create skill.json: %w", err)
	}

	// Create main prompt with better template
	mainPrompt := fmt.Sprintf(`# %s

%s

## Overview

Describe what this skill does and when it should be used.

## Instructions

1. Step one
2. Step two
3. Step three

## Guidelines

- Follow best practices
- Be thorough and careful
- Ask for clarification if needed

## Examples

### Example 1: Basic Usage

`+"```"+`
// Show example input/output here
`+"```"+`

### Example 2: Advanced Usage

`+"```"+`
// Show more complex example
`+"```"+`
`, name, description)

	if err := os.WriteFile(filepath.Join(skillDir, "prompts", "main.md"), []byte(mainPrompt), 0644); err != nil {
		return nil, fmt.Errorf("failed to create main prompt: %w", err)
	}

	return &skill.Skill{
		Name:        name,
		Description: description,
		Version:     version,
		Author:      author,
		Path:        skillDir,
	}, nil
}

// EditSkill opens the skill directory in the user's editor
func (r *ResourceCRUD) EditSkill(s *skill.Skill) error {
	// Open the main prompt file
	mainPromptPath := filepath.Join(s.Path, "prompts", "main.md")
	return r.openInEditor(mainPromptPath)
}

// DeleteSkill deletes a skill
func (r *ResourceCRUD) DeleteSkill(s *skill.Skill) error {
	return os.RemoveAll(s.Path)
}

// openInEditor opens a file in the user's preferred editor
func (r *ResourceCRUD) openInEditor(path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		// Try common editors
		for _, e := range []string{"code", "vim", "nano", "vi"} {
			if _, err := exec.LookPath(e); err == nil {
				editor = e
				break
			}
		}
	}
	if editor == "" {
		return fmt.Errorf("no editor found - set $EDITOR environment variable")
	}

	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ConfirmDelete shows a confirmation dialog for deletion
func ConfirmDelete(resourceType, name string) (bool, error) {
	var confirm bool

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Delete %s %q?", resourceType, name)).
				Description("This action cannot be undone.").
				Affirmative("Delete").
				Negative("Cancel").
				Value(&confirm),
		),
	)

	if err := form.Run(); err != nil {
		return false, err
	}

	return confirm, nil
}

// CreateServer creates a new server via interactive form
func (r *ResourceCRUD) CreateServer() (*mcp.Server, error) {
	var name, input string
	var transport string
	var commandStr, argsStr string

	// Step 1: Get server name and input (URL or Alias)
	form1 := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().Title("Add New Server"),
			huh.NewInput().
				Title("Name").
				Description("Unique identifier for this server").
				Placeholder("my-server").
				Value(&name).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("name is required")
					}
					if _, exists := r.cfg.Servers[s]; exists {
						return fmt.Errorf("server %q already exists", s)
					}
					return nil
				}),
			huh.NewInput().
				Title("Source").
				Description("Registry alias, GitHub URL, or local path").
				Placeholder("e.g. figma, github.com/user/repo, ./my-server").
				Value(&input).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("source is required")
					}
					return nil
				}),
		),
	)

	if err := form1.Run(); err != nil {
		return nil, err
	}

	// Analyze input to determine next steps
	var server *mcp.Server

	if strings.HasPrefix(input, "./") || strings.HasPrefix(input, "/") { // Local path
		server = &mcp.Server{
			Name: name,
			Source: mcp.Source{
				Type: "local",
				URL:  input,
			},
			Transport: mcp.TransportStdio,
		}
		// Prompt for command if local
		formCmd := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Command").
					Description("Command to run").
					Placeholder("npx").
					Value(&commandStr),
				huh.NewInput().
					Title("Arguments").
					Description("Command arguments").
					Placeholder("-y package").
					Value(&argsStr),
			),
		)
		if err := formCmd.Run(); err != nil {
			return nil, err
		}
		server.Command = commandStr
		if argsStr != "" {
			server.Args = strings.Fields(argsStr)
		}
	} else if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") { // Remote HTTP/SSE
		// Prompt for transport type
		formTransport := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Transport").
					Options(
						huh.NewOption("HTTP", "http"),
						huh.NewOption("SSE", "sse"),
					).
					Value(&transport),
			),
		)
		if err := formTransport.Run(); err != nil {
			return nil, err
		}
		server = &mcp.Server{
			Name: name,
			Source: mcp.Source{
				Type: "remote",
				URL:  input,
			},
			URL:       input,
			Transport: mcp.Transport(transport),
		}
	} else {
		// Assume alias or manual stdio
		// Check if it looks like an alias (no spaces, etc)
		// For simplicity, we'll offer a choice: Registry/Alias or Manual Command
		var sourceType string
		formSource := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Source Type").
					Options(
						huh.NewOption("Registry Alias (e.g. figma)", "alias"),
						huh.NewOption("Manual Command (stdio)", "manual"),
					).
					Value(&sourceType),
			),
		)
		if err := formSource.Run(); err != nil {
			return nil, err
		}

		if sourceType == "alias" {
			// treat input as alias
			server = &mcp.Server{
				Name: name,
				Source: mcp.Source{
					Type:  "alias",
					Alias: input,
				},
			}
			// We'll let the builder/sync logic resolve it later or failed if invalid
		} else {
			// Manual command
			formCmd := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Command").
						Description("Command to run").
						Placeholder("npx").
						Value(&commandStr),
					huh.NewInput().
						Title("Arguments").
						Description("Command arguments").
						Placeholder("-y package").
						Value(&argsStr),
				),
			)
			if err := formCmd.Run(); err != nil {
				return nil, err
			}
			server = &mcp.Server{
				Name: name,
				Source: mcp.Source{
					Type: "manual",
				},
				Transport: mcp.TransportStdio,
				Command:   commandStr,
			}
			if argsStr != "" {
				server.Args = strings.Fields(argsStr)
			}
		}
	}

	// Add to config
	if r.cfg.Servers == nil {
		r.cfg.Servers = make(map[string]*mcp.Server)
	}
	r.cfg.Servers[name] = server

	if err := r.cfg.Save(); err != nil {
		return nil, err
	}

	return server, nil
}
