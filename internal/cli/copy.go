package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/iheanyi/agentctl/pkg/command"
	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/output"
	"github.com/iheanyi/agentctl/pkg/rule"
	"github.com/iheanyi/agentctl/pkg/skill"
)

var copyCmd = &cobra.Command{
	Use:   "copy",
	Short: "Copy resources between global and local scopes",
	Long: `Copy resources between global and local scopes.

This allows you to:
- Customize a global resource for a specific project (global -> local)
- Promote a project resource to be available everywhere (local -> global)

Examples:
  agentctl copy command my-cmd --to local    # Copy global command to project
  agentctl copy rule my-rule --to global     # Copy local rule to global
  agentctl copy skill my-skill --to local    # Copy global skill to project`,
}

var copyTo string

var copyCommandCmd = &cobra.Command{
	Use:   "command <name> --to <scope>",
	Short: "Copy a command between global and local scopes",
	Args:  cobra.ExactArgs(1),
	RunE:  runCopyCommand,
}

var copyRuleCmd = &cobra.Command{
	Use:   "rule <name> --to <scope>",
	Short: "Copy a rule between global and local scopes",
	Args:  cobra.ExactArgs(1),
	RunE:  runCopyRule,
}

var copySkillCmd = &cobra.Command{
	Use:   "skill <name> --to <scope>",
	Short: "Copy a skill between global and local scopes",
	Args:  cobra.ExactArgs(1),
	RunE:  runCopySkill,
}

func init() {
	// Add --to flag to all copy subcommands
	copyCommandCmd.Flags().StringVar(&copyTo, "to", "", "Target scope: local or global (required)")
	copyCommandCmd.MarkFlagRequired("to")

	copyRuleCmd.Flags().StringVar(&copyTo, "to", "", "Target scope: local or global (required)")
	copyRuleCmd.MarkFlagRequired("to")

	copySkillCmd.Flags().StringVar(&copyTo, "to", "", "Target scope: local or global (required)")
	copySkillCmd.MarkFlagRequired("to")

	// Add subcommands
	copyCmd.AddCommand(copyCommandCmd)
	copyCmd.AddCommand(copyRuleCmd)
	copyCmd.AddCommand(copySkillCmd)

	// Register with root
	rootCmd.AddCommand(copyCmd)
}

// runCopyCommand copies a command between global and local scopes
func runCopyCommand(cmd *cobra.Command, args []string) error {
	out := output.DefaultWriter()
	name := args[0]

	// Validate target scope
	targetScope, err := config.ParseScope(copyTo)
	if err != nil {
		return fmt.Errorf("invalid --to value: %s (must be 'local' or 'global')", copyTo)
	}

	// Load config
	cfg, err := config.LoadWithProject()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if we're in a project for local scope
	if targetScope == config.ScopeLocal && cfg.ProjectPath == "" {
		return fmt.Errorf("cannot copy to local scope: not in a project (no .agentctl.json found)")
	}

	// Find the command
	var foundCmd *command.Command
	for _, c := range cfg.CommandsForScope(config.ScopeAll) {
		if c.Name == name {
			foundCmd = c
			break
		}
	}
	if foundCmd == nil {
		return fmt.Errorf("command %q not found", name)
	}

	// Check if command is already in target scope
	if foundCmd.Scope == string(targetScope) {
		return fmt.Errorf("command %q is already in %s scope", name, targetScope)
	}

	// Determine target path
	var targetDir string
	if targetScope == config.ScopeLocal {
		targetDir = filepath.Join(filepath.Dir(cfg.ProjectPath), ".agentctl", "commands")
	} else {
		targetDir = filepath.Join(cfg.ConfigDir, "commands")
	}
	targetPath := filepath.Join(targetDir, name+".json")

	// Check if command already exists in target
	if _, err := os.Stat(targetPath); err == nil {
		return fmt.Errorf("command %q already exists in %s scope at %s", name, targetScope, targetPath)
	}

	// Ensure target directory exists
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Copy the file
	if err := copyFile(foundCmd.Path, targetPath); err != nil {
		return fmt.Errorf("failed to copy command: %w", err)
	}

	if JSONOutput {
		jw := output.NewJSONWriter()
		return jw.WriteSuccess(output.CopyResourceResult{
			Type:       "command",
			Name:       name,
			FromScope:  foundCmd.Scope,
			ToScope:    string(targetScope),
			SourcePath: foundCmd.Path,
			TargetPath: targetPath,
		})
	}

	// Show result
	fromIndicator := "[G]"
	toIndicator := "[L]"
	if foundCmd.Scope == "local" {
		fromIndicator = "[L]"
		toIndicator = "[G]"
	}

	out.Success("Copied command %q from %s to %s", name, fromIndicator, toIndicator)
	out.Info("Source: %s", foundCmd.Path)
	out.Info("Target: %s", targetPath)
	out.Println("")
	out.Println("The copied command is independent - changes to one won't affect the other.")

	return nil
}

// runCopyRule copies a rule between global and local scopes
func runCopyRule(cmd *cobra.Command, args []string) error {
	out := output.DefaultWriter()
	name := args[0]

	// Validate target scope
	targetScope, err := config.ParseScope(copyTo)
	if err != nil {
		return fmt.Errorf("invalid --to value: %s (must be 'local' or 'global')", copyTo)
	}

	// Load config
	cfg, err := config.LoadWithProject()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if we're in a project for local scope
	if targetScope == config.ScopeLocal && cfg.ProjectPath == "" {
		return fmt.Errorf("cannot copy to local scope: not in a project (no .agentctl.json found)")
	}

	// Find the rule
	var foundRule *rule.Rule
	for _, r := range cfg.RulesForScope(config.ScopeAll) {
		if r.Name == name {
			foundRule = r
			break
		}
	}
	if foundRule == nil {
		return fmt.Errorf("rule %q not found", name)
	}

	// Check if rule is already in target scope
	if foundRule.Scope == string(targetScope) {
		return fmt.Errorf("rule %q is already in %s scope", name, targetScope)
	}

	// Determine target path
	var targetDir string
	if targetScope == config.ScopeLocal {
		targetDir = filepath.Join(filepath.Dir(cfg.ProjectPath), ".agentctl", "rules")
	} else {
		targetDir = filepath.Join(cfg.ConfigDir, "rules")
	}
	targetPath := filepath.Join(targetDir, name+".md")

	// Check if rule already exists in target
	if _, err := os.Stat(targetPath); err == nil {
		return fmt.Errorf("rule %q already exists in %s scope at %s", name, targetScope, targetPath)
	}

	// Ensure target directory exists
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Copy the file
	if err := copyFile(foundRule.Path, targetPath); err != nil {
		return fmt.Errorf("failed to copy rule: %w", err)
	}

	if JSONOutput {
		jw := output.NewJSONWriter()
		return jw.WriteSuccess(output.CopyResourceResult{
			Type:       "rule",
			Name:       name,
			FromScope:  foundRule.Scope,
			ToScope:    string(targetScope),
			SourcePath: foundRule.Path,
			TargetPath: targetPath,
		})
	}

	// Show result
	fromIndicator := "[G]"
	toIndicator := "[L]"
	if foundRule.Scope == "local" {
		fromIndicator = "[L]"
		toIndicator = "[G]"
	}

	out.Success("Copied rule %q from %s to %s", name, fromIndicator, toIndicator)
	out.Info("Source: %s", foundRule.Path)
	out.Info("Target: %s", targetPath)
	out.Println("")
	out.Println("The copied rule is independent - changes to one won't affect the other.")

	return nil
}

// runCopySkill copies a skill between global and local scopes
func runCopySkill(cmd *cobra.Command, args []string) error {
	out := output.DefaultWriter()
	name := args[0]

	// Validate target scope
	targetScope, err := config.ParseScope(copyTo)
	if err != nil {
		return fmt.Errorf("invalid --to value: %s (must be 'local' or 'global')", copyTo)
	}

	// Load config
	cfg, err := config.LoadWithProject()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if we're in a project for local scope
	if targetScope == config.ScopeLocal && cfg.ProjectPath == "" {
		return fmt.Errorf("cannot copy to local scope: not in a project (no .agentctl.json found)")
	}

	// Find the skill
	var foundSkill *skill.Skill
	for _, s := range cfg.SkillsForScope(config.ScopeAll) {
		if s.Name == name {
			foundSkill = s
			break
		}
	}
	if foundSkill == nil {
		return fmt.Errorf("skill %q not found", name)
	}

	// Check if skill is already in target scope
	if foundSkill.Scope == string(targetScope) {
		return fmt.Errorf("skill %q is already in %s scope", name, targetScope)
	}

	// Determine target directory
	var targetDir string
	if targetScope == config.ScopeLocal {
		targetDir = filepath.Join(filepath.Dir(cfg.ProjectPath), ".agentctl", "skills", name)
	} else {
		targetDir = filepath.Join(cfg.ConfigDir, "skills", name)
	}

	// Check if skill already exists in target
	if _, err := os.Stat(targetDir); err == nil {
		return fmt.Errorf("skill %q already exists in %s scope at %s", name, targetScope, targetDir)
	}

	// Copy the skill directory
	if err := copyDir(foundSkill.Path, targetDir); err != nil {
		return fmt.Errorf("failed to copy skill: %w", err)
	}

	if JSONOutput {
		jw := output.NewJSONWriter()
		return jw.WriteSuccess(output.CopyResourceResult{
			Type:       "skill",
			Name:       name,
			FromScope:  foundSkill.Scope,
			ToScope:    string(targetScope),
			SourcePath: foundSkill.Path,
			TargetPath: targetDir,
		})
	}

	// Show result
	fromIndicator := "[G]"
	toIndicator := "[L]"
	if foundSkill.Scope == "local" {
		fromIndicator = "[L]"
		toIndicator = "[G]"
	}

	out.Success("Copied skill %q from %s to %s", name, fromIndicator, toIndicator)
	out.Info("Source: %s", foundSkill.Path)
	out.Info("Target: %s", targetDir)
	out.Println("")
	out.Println("The copied skill is independent - changes to one won't affect the other.")

	return nil
}
