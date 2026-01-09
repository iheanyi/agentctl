package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/output"
	"github.com/iheanyi/agentctl/pkg/skill"
	"github.com/spf13/cobra"
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage skills",
	Long: `Manage skills - reusable AI capability packages.

Skills are directories containing a SKILL.md file with YAML frontmatter
that defines the skill's name, description, and prompt content.

Examples:
  agentctl skill list                  # List all skills
  agentctl skill show my-skill         # Show skill details
  agentctl skill edit my-skill         # Edit skill in $EDITOR
  agentctl skill remove my-skill       # Remove a skill`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var skillShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show detailed information about a skill",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillShow,
}

var skillEditCmd = &cobra.Command{
	Use:   "edit <name>",
	Short: "Edit a skill in your default editor",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillEdit,
}

var skillRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"rm", "delete"},
	Short:   "Remove an installed skill",
	Args:    cobra.ExactArgs(1),
	RunE:    runSkillRemove,
}

var skillAddCmd = &cobra.Command{
	Use:   "add <path-or-url>",
	Short: "Install a skill from a local path or GitHub",
	Long: `Install a skill from a local directory or GitHub repository.

The path can be:
- A local directory containing a SKILL.md file
- A GitHub path: github.com/owner/repo/path/to/skill
- A full GitHub URL: https://github.com/owner/repo/tree/main/path/to/skill

Examples:
  agentctl skill add ./my-skill                           # Install from local directory
  agentctl skill add /path/to/skill                       # Install from absolute path
  agentctl skill add github.com/user/repo/skills/review   # Install from GitHub
  agentctl skill add ./skill --scope local                # Install to project`,
	Args: cobra.ExactArgs(1),
	RunE: runSkillAdd,
}

var skillScope string

func init() {
	// Add scope flag to skill commands
	skillCmd.PersistentFlags().StringVarP(&skillScope, "scope", "s", "", "Config scope: local, global (default: global)")

	// Add subcommands
	skillCmd.AddCommand(skillShowCmd)
	skillCmd.AddCommand(skillEditCmd)
	skillCmd.AddCommand(skillRemoveCmd)
	skillCmd.AddCommand(skillAddCmd)

	// Register with root
	rootCmd.AddCommand(skillCmd)
}

// findSkill finds a skill by name across local and global scopes
func findSkill(name string) (*skill.Skill, error) {
	cfg, err := config.LoadWithProject()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Determine scope filter
	var scope config.Scope
	if skillScope != "" {
		scope, err = config.ParseScope(skillScope)
		if err != nil {
			return nil, err
		}
	} else {
		scope = config.ScopeAll
	}

	// Search for skill
	skills := cfg.SkillsForScope(scope)
	for _, s := range skills {
		if s.Name == name {
			return s, nil
		}
	}

	return nil, fmt.Errorf("skill %q not found", name)
}

func runSkillShow(cmd *cobra.Command, args []string) error {
	name := args[0]
	out := output.DefaultWriter()

	s, err := findSkill(name)
	if err != nil {
		return err
	}

	// Print skill information
	out.Println("Skill: %s", s.Name)
	out.Println("")

	if s.Description != "" {
		out.Println("Description: %s", s.Description)
		out.Println("")
	}

	out.Println("Scope: %s", scopeLabel(s.Scope))
	out.Println("Path: %s", s.Path)
	out.Println("")

	// Print SKILL.md content
	if s.Content != "" {
		out.Println("Content:")
		out.Println("─────────────────────────────────────────")
		out.Println("%s", s.Content)
		out.Println("─────────────────────────────────────────")
	}

	return nil
}

func runSkillEdit(cmd *cobra.Command, args []string) error {
	name := args[0]
	out := output.DefaultWriter()

	s, err := findSkill(name)
	if err != nil {
		return err
	}

	// Find the SKILL.md file
	skillMdPath := filepath.Join(s.Path, skill.SkillFileName)
	if _, err := os.Stat(skillMdPath); os.IsNotExist(err) {
		// Fall back to skill.json for legacy skills
		skillMdPath = filepath.Join(s.Path, skill.LegacySkillFileName)
	}

	// Get editor from environment
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}

	// Open in editor
	editorCmd := exec.Command(editor, skillMdPath)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr

	if err := editorCmd.Run(); err != nil {
		return fmt.Errorf("failed to open editor: %w", err)
	}

	out.Success("Edited skill %q", name)
	out.Println("Run 'agentctl sync' to sync changes to your tools.")

	return nil
}

func runSkillRemove(cmd *cobra.Command, args []string) error {
	name := args[0]
	out := output.DefaultWriter()

	s, err := findSkill(name)
	if err != nil {
		return err
	}

	// Confirm removal
	out.Println("This will remove the skill directory: %s", s.Path)
	out.Println("")

	// Remove the skill directory
	if err := os.RemoveAll(s.Path); err != nil {
		return fmt.Errorf("failed to remove skill: %w", err)
	}

	out.Success("Removed skill %q", name)
	out.Println("Run 'agentctl sync' to sync changes to your tools.")

	return nil
}

// scopeLabel returns a human-readable scope label
func scopeLabel(scope string) string {
	switch scope {
	case string(config.ScopeLocal):
		return "local (project)"
	case string(config.ScopeGlobal):
		return "global"
	default:
		return "global"
	}
}

func runSkillAdd(cmd *cobra.Command, args []string) error {
	sourcePath := args[0]
	out := output.DefaultWriter()

	// Check if this is a GitHub path
	if isGitHubPath(sourcePath) {
		return runSkillAddGitHub(sourcePath, out)
	}

	// Handle local path
	return runSkillAddLocal(sourcePath, out)
}

// runSkillAddLocal installs a skill from a local path
func runSkillAddLocal(sourcePath string, out *output.Writer) error {
	// Resolve to absolute path
	absPath, err := filepath.Abs(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Verify source exists and is a directory
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("source path not found: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("source path must be a directory")
	}

	// Try to load the skill to validate it
	s, err := skill.Load(absPath)
	if err != nil {
		return fmt.Errorf("invalid skill: %w", err)
	}

	// Get target directory
	targetDir, scopeStr, err := getSkillTargetDir(s.Name)
	if err != nil {
		return err
	}

	// Check if skill already exists
	if _, err := os.Stat(targetDir); err == nil {
		return fmt.Errorf("skill %q already exists at %s", s.Name, targetDir)
	}

	// Copy the skill directory
	if err := copyDir(absPath, targetDir); err != nil {
		return fmt.Errorf("failed to copy skill: %w", err)
	}

	out.Success("Installed skill %q (%s)", s.Name, scopeStr)
	out.Info("Location: %s", targetDir)
	out.Println("")
	out.Println("Run 'agentctl sync' to sync to your tools.")

	return nil
}

// runSkillAddGitHub installs a skill from a GitHub path
func runSkillAddGitHub(ghPath string, out *output.Writer) error {
	// Parse the GitHub path
	repoURL, subPath, err := parseGitHubPath(ghPath)
	if err != nil {
		return err
	}

	out.Info("Fetching skill from %s", ghPath)

	// Create a temporary directory for cloning
	tmpDir, err := os.MkdirTemp("", "agentctl-skill-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Clone the repository (shallow clone)
	cloneCmd := exec.Command("git", "clone", "--depth", "1", repoURL, tmpDir)
	cloneCmd.Stderr = os.Stderr
	if err := cloneCmd.Run(); err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	// Find the skill directory
	skillSourceDir := filepath.Join(tmpDir, subPath)
	if _, err := os.Stat(skillSourceDir); err != nil {
		return fmt.Errorf("skill path not found in repository: %s", subPath)
	}

	// Load and validate the skill
	s, err := skill.Load(skillSourceDir)
	if err != nil {
		return fmt.Errorf("invalid skill at %s: %w", subPath, err)
	}

	// Get target directory
	targetDir, scopeStr, err := getSkillTargetDir(s.Name)
	if err != nil {
		return err
	}

	// Check if skill already exists
	if _, err := os.Stat(targetDir); err == nil {
		return fmt.Errorf("skill %q already exists at %s", s.Name, targetDir)
	}

	// Copy the skill directory
	if err := copyDir(skillSourceDir, targetDir); err != nil {
		return fmt.Errorf("failed to copy skill: %w", err)
	}

	out.Success("Installed skill %q (%s)", s.Name, scopeStr)
	out.Info("Location: %s", targetDir)
	out.Info("Source: %s", ghPath)
	out.Println("")
	out.Println("Run 'agentctl sync' to sync to your tools.")

	return nil
}

// isGitHubPath checks if a path looks like a GitHub path
func isGitHubPath(path string) bool {
	return strings.HasPrefix(path, "github.com/") ||
		strings.HasPrefix(path, "https://github.com/") ||
		strings.HasPrefix(path, "http://github.com/")
}

// parseGitHubPath parses a GitHub path into repo URL and subpath
// Supported formats:
//   - github.com/owner/repo/path/to/skill
//   - https://github.com/owner/repo/tree/main/path/to/skill
//   - https://github.com/owner/repo/blob/main/path/to/skill
func parseGitHubPath(ghPath string) (repoURL string, subPath string, err error) {
	// Normalize the path
	path := strings.TrimPrefix(ghPath, "https://")
	path = strings.TrimPrefix(path, "http://")
	path = strings.TrimPrefix(path, "github.com/")

	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid GitHub path: need at least owner/repo")
	}

	owner := parts[0]
	repo := parts[1]
	repoURL = fmt.Sprintf("https://github.com/%s/%s", owner, repo)

	// Handle tree/blob format: owner/repo/tree/main/path or owner/repo/blob/main/path
	if len(parts) > 3 && (parts[2] == "tree" || parts[2] == "blob") {
		// Skip tree/blob and branch name
		if len(parts) > 4 {
			subPath = strings.Join(parts[4:], "/")
		}
	} else if len(parts) > 2 {
		// Simple format: owner/repo/path/to/skill
		subPath = strings.Join(parts[2:], "/")
	}

	return repoURL, subPath, nil
}

// getSkillTargetDir returns the target directory for a skill based on scope
func getSkillTargetDir(skillName string) (targetDir string, scopeStr string, err error) {
	if skillScope != "" {
		scope, err := config.ParseScope(skillScope)
		if err != nil {
			return "", "", err
		}
		if scope == config.ScopeLocal {
			cwd, err := os.Getwd()
			if err != nil {
				return "", "", fmt.Errorf("failed to get working directory: %w", err)
			}
			return filepath.Join(cwd, ".agentctl", "skills", skillName), "local", nil
		}
	}

	// Default to global
	cfg, err := config.Load()
	if err != nil {
		return "", "", fmt.Errorf("failed to load config: %w", err)
	}
	return filepath.Join(cfg.ConfigDir, "skills", skillName), "global", nil
}

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	// Create destination directory
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	// Preserve file permissions
	info, err := srcFile.Stat()
	if err != nil {
		return err
	}
	return os.Chmod(dst, info.Mode())
}
