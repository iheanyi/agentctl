package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/output"
	"github.com/iheanyi/agentctl/pkg/safeio"
	"github.com/iheanyi/agentctl/pkg/skill"
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

var skillCommandCmd = &cobra.Command{
	Use:   "command <skill-name> <command-name>",
	Short: "Add a new subcommand to an existing skill",
	Long: `Add a new subcommand to an existing skill.

This creates a new .md file in the skill directory that can be invoked
as skill-name:command-name.

Examples:
  agentctl skill command my-skill review       # Add review subcommand
  agentctl skill command my-skill test         # Add test subcommand`,
	Args: cobra.ExactArgs(2),
	RunE: runSkillCommand,
}

var skillCopyCmd = &cobra.Command{
	Use:   "copy <name> --to <scope>",
	Short: "Copy a skill between global and local scopes",
	Long: `Copy a skill from one scope to another.

This allows you to:
- Customize a global skill for a specific project (global → local)
- Promote a project skill to be available everywhere (local → global)

Examples:
  agentctl skill copy my-skill --to local    # Copy global skill to project
  agentctl skill copy my-skill --to global   # Copy local skill to global`,
	Args: cobra.ExactArgs(1),
	RunE: runSkillCopy,
}

var skillCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check installed skills for updates",
	Long: `Check installed skills for updates using source tracking metadata.

Only skills installed from tracked GitHub sources can be checked automatically.`,
	RunE: runSkillCheck,
}

var skillUpdateCmd = &cobra.Command{
	Use:   "update [skill-name...]",
	Short: "Update tracked skills from their sources",
	Long: `Update one or more tracked skills from their recorded source metadata.

If no skill names are provided, all tracked skills are considered.`,
	Args: cobra.ArbitraryArgs,
	RunE: runSkillUpdate,
}

var skillFindCmd = &cobra.Command{
	Use:   "find [query]",
	Short: "Find installed skills by name or description",
	Long: `Find installed skills across local/global scope.

If a query is provided, results are filtered by skill name, description,
and command names.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSkillFind,
}

var skillScope string
var skillCopyTo string
var skillAddYes bool
var skillAddList bool
var skillAddFilter []string
var skillUpdateYes bool

const skillSourceMetadataFile = ".agentctl-source.json"

type skillSourceMetadata struct {
	SourceType  string `json:"sourceType"` // local, github
	Source      string `json:"source"`
	RepoURL     string `json:"repoUrl,omitempty"`
	RepoSubPath string `json:"repoSubPath,omitempty"`
	SkillPath   string `json:"skillPath,omitempty"` // Path relative to repository root
	Commit      string `json:"commit,omitempty"`
	InstalledAt string `json:"installedAt"`
}

type installCandidate struct {
	Skill        *skill.Skill
	SourceDir    string // Absolute directory for this skill source
	RelativePath string // Path relative to repository/local root
}

type skillUpdateStatus struct {
	Skill        *skill.Skill
	Metadata     *skillSourceMetadata
	LatestCommit string
	Status       string
	Err          error
}

func init() {
	// Add scope flag to skill commands
	skillCmd.PersistentFlags().StringVarP(&skillScope, "scope", "s", "", "Config scope: local, global (default: global)")

	// Add --to flag for copy command
	skillCopyCmd.Flags().StringVar(&skillCopyTo, "to", "", "Target scope: local or global (required)")
	skillCopyCmd.MarkFlagRequired("to")

	// Add install/update behavior flags.
	skillAddCmd.Flags().BoolVarP(&skillAddYes, "yes", "y", false, "Skip confirmation prompts")
	skillAddCmd.Flags().BoolVarP(&skillAddList, "list", "l", false, "List available skills without installing")
	skillAddCmd.Flags().StringArrayVarP(&skillAddFilter, "skill", "k", nil, "Install only selected skill name(s)")
	skillUpdateCmd.Flags().BoolVarP(&skillUpdateYes, "yes", "y", false, "Skip confirmation prompts")

	// Add subcommands
	skillCmd.AddCommand(skillShowCmd)
	skillCmd.AddCommand(skillEditCmd)
	skillCmd.AddCommand(skillRemoveCmd)
	skillCmd.AddCommand(skillAddCmd)
	skillCmd.AddCommand(skillCommandCmd)
	skillCmd.AddCommand(skillCopyCmd)
	skillCmd.AddCommand(skillCheckCmd)
	skillCmd.AddCommand(skillUpdateCmd)
	skillCmd.AddCommand(skillFindCmd)

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

	// Print skill header with scope indicator
	scopeIndicator := "[G]"
	if s.Scope == "local" {
		scopeIndicator = "[L]"
	}
	out.Println("%s Skill: %s", scopeIndicator, s.Name)
	out.Println("")

	if s.Description != "" {
		out.Println("Description: %s", s.Description)
	}

	out.Println("Scope:       %s", scopeLabel(s.Scope))
	out.Println("Path:        %s", s.Path)

	// Show available commands/invocations
	out.Println("")
	out.Println("Invocations:")
	if s.Content != "" {
		out.Println("  /%s              (default)", s.Name)
	}
	for _, c := range s.Commands {
		desc := ""
		if c.Description != "" {
			desc = " - " + c.Description
		}
		out.Println("  /%s:%s%s", s.Name, c.Name, desc)
	}

	// Print default command content
	if s.Content != "" {
		out.Println("")
		out.Println("Default Command (SKILL.md):")
		out.Println("─────────────────────────────────────────")
		out.Println("%s", s.Content)
		out.Println("─────────────────────────────────────────")
	}

	// Print subcommands
	if len(s.Commands) > 0 {
		out.Println("")
		out.Println("Subcommands:")
		for _, c := range s.Commands {
			out.Println("")
			out.Println("  %s:%s (%s)", s.Name, c.Name, c.FileName)
			if c.Description != "" {
				out.Println("  Description: %s", c.Description)
			}
			out.Println("  ─────────────────────────────────────")
			// Indent the content
			lines := strings.Split(c.Content, "\n")
			for _, line := range lines {
				out.Println("  %s", line)
			}
			out.Println("  ─────────────────────────────────────")
		}
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

	var (
		candidates []installCandidate
		metaBase   skillSourceMetadata
	)

	if isGitHubPath(sourcePath) {
		repoURL, subPath, err := parseGitHubPath(sourcePath)
		if err != nil {
			return err
		}

		out.Info("Fetching skills from %s", sourcePath)
		tmpDir, err := os.MkdirTemp("", "agentctl-skill-*")
		if err != nil {
			return fmt.Errorf("failed to create temp directory: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		cloneCmd := exec.Command("git", "clone", "--depth", "1", repoURL, tmpDir)
		cloneCmd.Stderr = os.Stderr
		if err := cloneCmd.Run(); err != nil {
			return fmt.Errorf("failed to clone repository: %w", err)
		}

		candidates, err = discoverInstallCandidates(tmpDir, subPath)
		if err != nil {
			return err
		}

		commit, err := resolveLocalGitHead(tmpDir)
		if err != nil {
			return err
		}

		metaBase = skillSourceMetadata{
			SourceType:  "github",
			Source:      sourcePath,
			RepoURL:     repoURL,
			RepoSubPath: subPath,
			Commit:      commit,
		}
	} else {
		absPath, err := filepath.Abs(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to resolve path: %w", err)
		}

		candidates, err = discoverInstallCandidates(absPath, "")
		if err != nil {
			return err
		}

		metaBase = skillSourceMetadata{
			SourceType: "local",
			Source:     absPath,
		}
	}

	candidates, err := filterInstallCandidates(candidates, skillAddFilter)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		out.Warning("No skills matched your filter")
		return nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Skill.Name < candidates[j].Skill.Name
	})

	if skillAddList {
		out.Println("Available skills:")
		printSkillCandidates(candidates, out)
		return nil
	}

	if !skillAddYes {
		ok, err := confirmSkillInstall(candidates)
		if err != nil {
			return err
		}
		if !ok {
			out.Info("Cancelled.")
			return nil
		}
	}

	for _, candidate := range candidates {
		meta := metaBase
		meta.SkillPath = candidate.RelativePath
		meta.InstalledAt = time.Now().UTC().Format(time.RFC3339)
		if err := installSkillCandidate(candidate, &meta, out); err != nil {
			return err
		}
	}

	out.Println("")
	out.Success("Installed %d skill(s)", len(candidates))
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

func resolveLocalGitHead(repoDir string) (string, error) {
	cmd := exec.Command("git", "-C", repoDir, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get repository commit: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func resolveRemoteGitHead(repoURL string) (string, error) {
	cmd := exec.Command("git", "ls-remote", repoURL, "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to resolve remote head for %s: %w", repoURL, err)
	}

	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return "", fmt.Errorf("unexpected git ls-remote output for %s", repoURL)
	}
	return fields[0], nil
}

func discoverInstallCandidates(rootPath, subPath string) ([]installCandidate, error) {
	basePath := rootPath
	if strings.TrimSpace(subPath) != "" {
		basePath = filepath.Join(rootPath, subPath)
	}

	info, err := os.Stat(basePath)
	if err != nil {
		return nil, fmt.Errorf("source path not found: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("source path must be a directory")
	}

	// Direct skill directory (single-skill install).
	if s, err := skill.Load(basePath); err == nil {
		relPath, _ := filepath.Rel(rootPath, basePath)
		return []installCandidate{{
			Skill:        s,
			SourceDir:    basePath,
			RelativePath: normalizeRelativePath(relPath),
		}}, nil
	}

	var searchDirs []string
	searchDirs = append(searchDirs, basePath)
	skillsDir := filepath.Join(basePath, "skills")
	if dirInfo, err := os.Stat(skillsDir); err == nil && dirInfo.IsDir() {
		searchDirs = append(searchDirs, skillsDir)
	}

	seen := make(map[string]bool)
	var candidates []installCandidate
	for _, searchDir := range searchDirs {
		entries, err := os.ReadDir(searchDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			candidateDir := filepath.Join(searchDir, entry.Name())
			s, err := skill.Load(candidateDir)
			if err != nil {
				continue
			}
			if seen[s.Name] {
				continue
			}
			seen[s.Name] = true
			relPath, _ := filepath.Rel(rootPath, candidateDir)
			candidates = append(candidates, installCandidate{
				Skill:        s,
				SourceDir:    candidateDir,
				RelativePath: normalizeRelativePath(relPath),
			})
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no valid skills found at %s", basePath)
	}
	return candidates, nil
}

func normalizeRelativePath(relPath string) string {
	normalized := strings.TrimSpace(relPath)
	normalized = strings.ReplaceAll(normalized, "\\", "/")
	normalized = filepath.ToSlash(normalized)
	if normalized == "." {
		return ""
	}
	return normalized
}

func filterInstallCandidates(candidates []installCandidate, wanted []string) ([]installCandidate, error) {
	if len(wanted) == 0 {
		return candidates, nil
	}

	wantAll := false
	wantedSet := make(map[string]bool)
	for _, item := range wanted {
		item = strings.TrimSpace(strings.ToLower(item))
		if item == "" {
			continue
		}
		if item == "*" {
			wantAll = true
			continue
		}
		wantedSet[item] = false
	}
	if wantAll {
		return candidates, nil
	}

	var filtered []installCandidate
	for _, candidate := range candidates {
		name := strings.ToLower(candidate.Skill.Name)
		if _, ok := wantedSet[name]; ok {
			wantedSet[name] = true
			filtered = append(filtered, candidate)
		}
	}

	var missing []string
	for name, found := range wantedSet {
		if !found {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("requested skill(s) not found: %s", strings.Join(missing, ", "))
	}
	return filtered, nil
}

func printSkillCandidates(candidates []installCandidate, out *output.Writer) {
	table := output.NewTable("Name", "Description", "Source Path")
	for _, candidate := range candidates {
		sourcePath := candidate.RelativePath
		if sourcePath == "" {
			sourcePath = "."
		}
		table.AddRow(candidate.Skill.Name, candidate.Skill.Description, sourcePath)
	}
	table.Render()
}

func confirmSkillInstall(candidates []installCandidate) (bool, error) {
	if !isInteractive() {
		return false, fmt.Errorf("non-interactive mode requires --yes")
	}

	fmt.Println("The following skill(s) will be installed:")
	for _, candidate := range candidates {
		fmt.Printf("  - %s\n", candidate.Skill.Name)
	}
	fmt.Printf("\nContinue? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("failed to read confirmation: %w", err)
	}

	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

func installSkillCandidate(candidate installCandidate, metadata *skillSourceMetadata, out *output.Writer) error {
	targetDir, scopeStr, err := getSkillTargetDir(candidate.Skill.Name)
	if err != nil {
		return err
	}

	if _, err := os.Stat(targetDir); err == nil {
		return fmt.Errorf("skill %q already exists at %s", candidate.Skill.Name, targetDir)
	}

	if err := copyDir(candidate.SourceDir, targetDir); err != nil {
		return fmt.Errorf("failed to copy skill %q: %w", candidate.Skill.Name, err)
	}
	if err := writeSkillSourceMetadata(targetDir, metadata); err != nil {
		return fmt.Errorf("failed to write source metadata for %q: %w", candidate.Skill.Name, err)
	}

	scopeIndicator := "[G]"
	if scopeStr == "local" {
		scopeIndicator = "[L]"
	}

	out.Success("Installed skill %q %s", candidate.Skill.Name, scopeIndicator)
	out.Info("Location: %s", targetDir)
	if metadata.Source != "" {
		out.Info("Source: %s", metadata.Source)
	}
	out.Println("Invocations:")
	out.Println("  /%s", candidate.Skill.Name)
	for _, c := range candidate.Skill.Commands {
		out.Println("  /%s:%s", candidate.Skill.Name, c.Name)
	}
	out.Println("")

	return nil
}

func writeSkillSourceMetadata(skillDir string, metadata *skillSourceMetadata) error {
	if metadata == nil {
		return nil
	}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal source metadata: %w", err)
	}

	path := filepath.Join(skillDir, skillSourceMetadataFile)
	return safeio.SafeWriteFileWithLock(path, data, 0644, safeio.DefaultBackupCount)
}

func readSkillSourceMetadata(skillDir string) (*skillSourceMetadata, error) {
	path := filepath.Join(skillDir, skillSourceMetadataFile)
	data, err := safeio.SafeReadFile(path)
	if err != nil {
		return nil, err
	}

	var metadata skillSourceMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("parse source metadata: %w", err)
	}
	return &metadata, nil
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

func runSkillCommand(cmd *cobra.Command, args []string) error {
	skillName := args[0]
	commandName := args[1]
	out := output.DefaultWriter()

	s, err := findSkill(skillName)
	if err != nil {
		return err
	}

	// Check if command already exists
	if s.GetCommand(commandName) != nil {
		return fmt.Errorf("command %q already exists in skill %q", commandName, skillName)
	}

	// Create the new command
	newCmd := &skill.Command{
		Name:        commandName,
		Description: fmt.Sprintf("Description for %s command", commandName),
		FileName:    commandName + ".md",
	}

	// Add and save the command
	if err := s.AddCommand(newCmd); err != nil {
		return err
	}

	if err := s.SaveCommand(newCmd); err != nil {
		return fmt.Errorf("failed to save command: %w", err)
	}

	// Show scope indicator
	scopeIndicator := "[G]"
	if s.Scope == "local" {
		scopeIndicator = "[L]"
	}

	out.Success("Added command %q to skill %q %s", commandName, skillName, scopeIndicator)
	out.Info("File: %s/%s", s.Path, newCmd.FileName)
	out.Println("")
	out.Println("Edit the command file to customize the prompt.")
	out.Println("Invoke with: /%s:%s", skillName, commandName)

	return nil
}

// runSkillCopy copies a skill between global and local scopes
func runSkillCopy(cmd *cobra.Command, args []string) error {
	out := output.DefaultWriter()
	name := args[0]

	// Validate target scope
	targetScope, err := config.ParseScope(skillCopyTo)
	if err != nil {
		return fmt.Errorf("invalid --to value: %s (must be 'local' or 'global')", skillCopyTo)
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
	s, err := findSkill(name)
	if err != nil {
		return err
	}

	// Check if skill is already in target scope
	if s.Scope == string(targetScope) {
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
	if err := copyDir(s.Path, targetDir); err != nil {
		return fmt.Errorf("failed to copy skill: %w", err)
	}

	// Show result
	fromIndicator := "[G]"
	toIndicator := "[L]"
	if s.Scope == "local" {
		fromIndicator = "[L]"
		toIndicator = "[G]"
	}

	out.Success("Copied skill %q from %s to %s", name, fromIndicator, toIndicator)
	out.Info("Source: %s", s.Path)
	out.Info("Target: %s", targetDir)
	out.Println("")
	out.Println("The copied skill is independent - changes to one won't affect the other.")

	return nil
}

func runSkillCheck(cmd *cobra.Command, args []string) error {
	out := output.DefaultWriter()

	statuses, err := collectSkillUpdateStatuses()
	if err != nil {
		return err
	}
	if len(statuses) == 0 {
		out.Info("No installed skills found.")
		return nil
	}

	table := output.NewTable("Skill", "Scope", "Status", "Current", "Latest")
	updateCount := 0
	for _, status := range statuses {
		current := "-"
		latest := "-"
		if status.Metadata != nil && status.Metadata.Commit != "" {
			current = shortCommit(status.Metadata.Commit)
		}
		if status.LatestCommit != "" {
			latest = shortCommit(status.LatestCommit)
		}
		if status.Status == "update-available" {
			updateCount++
		}
		table.AddRow(status.Skill.Name, scopeLabel(status.Skill.Scope), status.Status, current, latest)
	}
	table.Render()

	for _, status := range statuses {
		if status.Err != nil {
			out.Warning("%s: %v", status.Skill.Name, status.Err)
		}
	}

	if updateCount == 0 {
		out.Success("All tracked skills are up to date.")
	} else {
		out.Warning("%d skill(s) have updates available.", updateCount)
		out.Info("Run 'agentctl skill update --yes' to apply updates.")
	}

	return nil
}

func runSkillUpdate(cmd *cobra.Command, args []string) error {
	out := output.DefaultWriter()

	statuses, err := collectSkillUpdateStatuses()
	if err != nil {
		return err
	}
	if len(statuses) == 0 {
		out.Info("No installed skills found.")
		return nil
	}

	selected, err := selectSkillStatusesForUpdate(statuses, args)
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		out.Info("No skills need updates.")
		return nil
	}

	if !skillUpdateYes {
		if !isInteractive() {
			return fmt.Errorf("non-interactive mode requires --yes")
		}
		fmt.Println("The following skills will be updated:")
		for _, status := range selected {
			fmt.Printf("  - %s\n", status.Skill.Name)
		}
		fmt.Printf("\nContinue? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}
		answer := strings.ToLower(strings.TrimSpace(line))
		if answer != "y" && answer != "yes" {
			out.Info("Cancelled.")
			return nil
		}
	}

	updated := 0
	for _, status := range selected {
		if err := updateTrackedSkill(status, out); err != nil {
			out.Warning("Failed to update %s: %v", status.Skill.Name, err)
			continue
		}
		updated++
	}

	out.Println("")
	out.Success("Updated %d skill(s)", updated)
	out.Println("Run 'agentctl sync' to sync changes to your tools.")
	return nil
}

func runSkillFind(cmd *cobra.Command, args []string) error {
	out := output.DefaultWriter()
	query := ""
	if len(args) > 0 {
		query = strings.ToLower(strings.TrimSpace(args[0]))
	}

	cfg, err := config.LoadWithProject()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	scope := config.ScopeAll
	if skillScope != "" {
		scope, err = config.ParseScope(skillScope)
		if err != nil {
			return err
		}
	}

	skills := cfg.SkillsForScope(scope)
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})

	table := output.NewTable("Name", "Description", "Scope", "Path")
	matchCount := 0
	for _, s := range skills {
		if query != "" && !skillMatchesQuery(s, query) {
			continue
		}
		table.AddRow(s.Name, s.Description, scopeLabel(s.Scope), s.Path)
		matchCount++
	}

	if matchCount == 0 {
		if query == "" {
			out.Info("No skills found.")
		} else {
			out.Info("No skills matched query %q.", query)
		}
		return nil
	}

	table.Render()
	return nil
}

func collectSkillUpdateStatuses() ([]skillUpdateStatus, error) {
	cfg, err := config.LoadWithProject()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	scope := config.ScopeAll
	if skillScope != "" {
		scope, err = config.ParseScope(skillScope)
		if err != nil {
			return nil, err
		}
	}

	skills := cfg.SkillsForScope(scope)
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})

	statuses := make([]skillUpdateStatus, 0, len(skills))
	for _, s := range skills {
		status := skillUpdateStatus{Skill: s}
		metadata, err := readSkillSourceMetadata(s.Path)
		if err != nil {
			status.Status = "untracked"
			statuses = append(statuses, status)
			continue
		}
		status.Metadata = metadata

		if metadata.SourceType != "github" || metadata.RepoURL == "" {
			status.Status = "tracked-local"
			statuses = append(statuses, status)
			continue
		}

		latest, err := resolveRemoteGitHead(metadata.RepoURL)
		if err != nil {
			status.Status = "check-failed"
			status.Err = err
			statuses = append(statuses, status)
			continue
		}

		status.LatestCommit = latest
		if metadata.Commit == "" {
			status.Status = "unknown-current"
		} else if metadata.Commit == latest {
			status.Status = "up-to-date"
		} else {
			status.Status = "update-available"
		}
		statuses = append(statuses, status)
	}

	return statuses, nil
}

func selectSkillStatusesForUpdate(statuses []skillUpdateStatus, names []string) ([]skillUpdateStatus, error) {
	if len(names) == 0 {
		var updates []skillUpdateStatus
		for _, status := range statuses {
			if status.Status == "update-available" || status.Status == "unknown-current" {
				updates = append(updates, status)
			}
		}
		return updates, nil
	}

	lookup := make(map[string]skillUpdateStatus)
	for _, status := range statuses {
		lookup[strings.ToLower(status.Skill.Name)] = status
	}

	var selected []skillUpdateStatus
	for _, name := range names {
		key := strings.ToLower(strings.TrimSpace(name))
		status, ok := lookup[key]
		if !ok {
			return nil, fmt.Errorf("skill %q not found", name)
		}
		if status.Metadata == nil {
			return nil, fmt.Errorf("skill %q is not source-tracked; reinstall with 'agentctl skill add' first", status.Skill.Name)
		}
		if status.Metadata.SourceType != "github" {
			return nil, fmt.Errorf("skill %q is tracked as %s and cannot be auto-updated", status.Skill.Name, status.Metadata.SourceType)
		}
		selected = append(selected, status)
	}
	return selected, nil
}

func updateTrackedSkill(status skillUpdateStatus, out *output.Writer) error {
	if status.Metadata == nil {
		return fmt.Errorf("missing source metadata")
	}

	tmpDir, err := os.MkdirTemp("", "agentctl-skill-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	cloneCmd := exec.Command("git", "clone", "--depth", "1", status.Metadata.RepoURL, tmpDir)
	cloneCmd.Stderr = os.Stderr
	if err := cloneCmd.Run(); err != nil {
		return fmt.Errorf("failed to clone source repository: %w", err)
	}

	commit, err := resolveLocalGitHead(tmpDir)
	if err != nil {
		return err
	}

	skillPath := status.Metadata.SkillPath
	if strings.TrimSpace(skillPath) == "" {
		candidates, err := discoverInstallCandidates(tmpDir, status.Metadata.RepoSubPath)
		if err != nil {
			return fmt.Errorf("could not locate skill in source: %w", err)
		}
		found := false
		for _, candidate := range candidates {
			if candidate.Skill.Name == status.Skill.Name {
				skillPath = candidate.RelativePath
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("skill %q not found in source repository", status.Skill.Name)
		}
	}

	sourceSkillDir := filepath.Join(tmpDir, filepath.FromSlash(skillPath))
	if _, err := os.Stat(sourceSkillDir); err != nil {
		return fmt.Errorf("skill source path not found: %s", skillPath)
	}

	replacementDir := status.Skill.Path + ".tmp-update"
	backupDir := status.Skill.Path + ".bak-update"
	_ = os.RemoveAll(replacementDir)
	_ = os.RemoveAll(backupDir)

	if err := copyDir(sourceSkillDir, replacementDir); err != nil {
		return fmt.Errorf("failed to stage updated skill: %w", err)
	}

	if err := os.Rename(status.Skill.Path, backupDir); err != nil {
		_ = os.RemoveAll(replacementDir)
		return fmt.Errorf("failed to backup current skill: %w", err)
	}
	if err := os.Rename(replacementDir, status.Skill.Path); err != nil {
		_ = os.Rename(backupDir, status.Skill.Path)
		return fmt.Errorf("failed to apply skill update: %w", err)
	}
	_ = os.RemoveAll(backupDir)

	metadata := *status.Metadata
	metadata.Commit = commit
	metadata.SkillPath = normalizeRelativePath(skillPath)
	metadata.InstalledAt = time.Now().UTC().Format(time.RFC3339)
	if err := writeSkillSourceMetadata(status.Skill.Path, &metadata); err != nil {
		return err
	}

	out.Success("Updated skill %q to %s", status.Skill.Name, shortCommit(commit))
	return nil
}

func shortCommit(commit string) string {
	trimmed := strings.TrimSpace(commit)
	if len(trimmed) <= 8 {
		return trimmed
	}
	return trimmed[:8]
}

func skillMatchesQuery(s *skill.Skill, query string) bool {
	if strings.Contains(strings.ToLower(s.Name), query) {
		return true
	}
	if strings.Contains(strings.ToLower(s.Description), query) {
		return true
	}
	for _, cmd := range s.Commands {
		if strings.Contains(strings.ToLower(cmd.Name), query) {
			return true
		}
		if strings.Contains(strings.ToLower(cmd.Description), query) {
			return true
		}
	}
	return false
}
