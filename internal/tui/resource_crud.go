package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/iheanyi/agentctl/pkg/command"
	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/jsonutil"
	"github.com/iheanyi/agentctl/pkg/mcp"
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

// CreateSkill creates a new skill via interactive wizard
// The wizard allows defining the skill and adding multiple commands before saving
func (r *ResourceCRUD) CreateSkill() (*skill.Skill, error) {
	var name, description, version, author, scopeChoice string

	// Determine available scope options
	hasProjectConfig := r.cfg.ProjectPath != ""
	scopeOptions := []huh.Option[string]{
		huh.NewOption("[G] Global (user-wide)", "global"),
	}
	if hasProjectConfig {
		scopeOptions = append(scopeOptions, huh.NewOption("[L] Local (project-only)", "local"))
	}

	// Build form groups for skill metadata
	var skillContent string
	groups := []*huh.Group{
		huh.NewGroup(
			huh.NewNote().Title("Create New Skill").Description("Step 1: Define the skill"),
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
					return nil
				}),
			huh.NewInput().
				Title("Description").
				Description("What does this skill do? (shown in help)").
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
		huh.NewGroup(
			huh.NewText().
				Title("Default Prompt").
				Description("The main prompt for this skill (invoked as /skill-name). Use $ARGUMENTS for user input.").
				Placeholder("Write the default prompt for this skill...\n\nExample:\nYou are an expert at...\n\n$ARGUMENTS").
				Value(&skillContent).
				CharLimit(8000).
				Lines(12),
		),
	}

	// Add scope selector if project config exists
	if hasProjectConfig {
		groups = append(groups, huh.NewGroup(
			huh.NewSelect[string]().
				Title("Scope").
				Description("Where should this skill be saved?").
				Options(scopeOptions...).
				Value(&scopeChoice),
		))
	} else {
		scopeChoice = "global"
	}

	form := huh.NewForm(groups...)

	if err := form.Run(); err != nil {
		return nil, err
	}

	// Determine skills directory based on scope
	var skillsDir string
	var scope string
	if scopeChoice == "local" && r.cfg.ProjectPath != "" {
		skillsDir = filepath.Join(filepath.Dir(r.cfg.ProjectPath), ".agentctl", "skills")
		scope = "local"
	} else {
		skillsDir = filepath.Join(r.cfg.ConfigDir, "skills")
		scope = "global"
	}

	// Check if skill already exists
	skillDir := filepath.Join(skillsDir, name)
	if _, err := os.Stat(skillDir); err == nil {
		return nil, fmt.Errorf("skill %q already exists in %s scope", name, scope)
	}

	if description == "" {
		description = "No description"
	}
	if version == "" {
		version = "1.0.0"
	}

	// Create skill object
	s := &skill.Skill{
		Name:        name,
		Description: description,
		Version:     version,
		Author:      author,
		Scope:       scope,
		Content:     skillContent,
	}

	// Step 2: Add commands wizard
	var commands []*skill.Command
	addMore := true

	for addMore {
		var wantCommand bool
		cmdCountLabel := "Add a command to this skill?"
		if len(commands) > 0 {
			cmdCountLabel = fmt.Sprintf("Add another command? (%d added so far)", len(commands))
		}

		confirmForm := huh.NewForm(
			huh.NewGroup(
				huh.NewNote().Title("Step 2: Add Commands").
					Description("Commands are subcommands invoked as /skill-name:command-name"),
				huh.NewConfirm().
					Title(cmdCountLabel).
					Value(&wantCommand),
			),
		)

		if err := confirmForm.Run(); err != nil {
			return nil, err
		}

		if !wantCommand {
			addMore = false
			continue
		}

		// Get command details
		var cmdName, cmdDesc, cmdContent string
		existingNames := make(map[string]bool)
		for _, c := range commands {
			existingNames[c.Name] = true
		}

		cmdForm := huh.NewForm(
			huh.NewGroup(
				huh.NewNote().Title(fmt.Sprintf("Add Command to %q", name)),
				huh.NewInput().
					Title("Command Name").
					Description("Short identifier (e.g., 'review', 'test', 'lint')").
					Placeholder("command-name").
					Value(&cmdName).
					Validate(func(s string) error {
						if s == "" {
							return fmt.Errorf("command name is required")
						}
						if strings.ContainsAny(s, " \t\n/\\:") {
							return fmt.Errorf("name cannot contain spaces, colons, or path separators")
						}
						if existingNames[s] {
							return fmt.Errorf("command %q already added", s)
						}
						return nil
					}),
				huh.NewInput().
					Title("Description").
					Description("What does this command do? (shown in help)").
					Placeholder("Description of this command").
					Value(&cmdDesc),
			),
			huh.NewGroup(
				huh.NewText().
					Title("Command Prompt").
					Description("The instructions for this command. Use $ARGUMENTS for user input.").
					Placeholder("Write the prompt for this command...\n\nExample:\nReview the following code for best practices:\n\n$ARGUMENTS").
					Value(&cmdContent).
					CharLimit(8000).
					Lines(12),
			),
		)

		if err := cmdForm.Run(); err != nil {
			return nil, err
		}

		if cmdDesc == "" {
			cmdDesc = fmt.Sprintf("Description for %s command", cmdName)
		}

		commands = append(commands, &skill.Command{
			Name:        cmdName,
			Description: cmdDesc,
			Content:     cmdContent,
			FileName:    cmdName + ".md",
		})
	}

	// Save the skill and all commands
	if err := s.Save(skillDir); err != nil {
		return nil, fmt.Errorf("failed to create skill: %w", err)
	}
	s.Path = skillDir

	// Save each command
	for _, cmd := range commands {
		if err := s.AddCommand(cmd); err != nil {
			return nil, fmt.Errorf("failed to add command %q: %w", cmd.Name, err)
		}
		if err := s.SaveCommand(cmd); err != nil {
			return nil, fmt.Errorf("failed to save command %q: %w", cmd.Name, err)
		}
	}

	return s, nil
}

// EditSkill opens the skill's SKILL.md file in the user's editor
func (r *ResourceCRUD) EditSkill(s *skill.Skill) error {
	// Open the SKILL.md file
	skillMdPath := filepath.Join(s.Path, skill.SkillFileName)
	return r.openInEditor(skillMdPath)
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
				Description("Registry alias, remote URL, or local path").
				Placeholder("e.g. figma, https://api.example.com, ./my-server").
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
