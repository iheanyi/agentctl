package discovery

// This file registers all DirectoryScanner-based tool scanners.
// Each tool has a simple config - no complex logic needed.

func init() {
	// Cursor Scanner
	// Detects: .cursor/ directory or .cursorrules file
	// Resources: rules in .cursor/rules/
	Register(NewDirectoryScanner(ScannerConfig{
		Name:        "cursor",
		LocalDirs:   []string{".cursor"},
		DetectFiles: []string{".cursorrules"},
		RulesDirs:   []string{"rules"},
		FileExts:    []string{".md", ".mdc"},
	}))

	// Codex Scanner
	// Detects: .codex/ directory or AGENTS.md file
	// Resources: skills in .codex/skills/
	Register(NewDirectoryScanner(ScannerConfig{
		Name:        "codex",
		LocalDirs:   []string{".codex"},
		DetectFiles: []string{"AGENTS.md"},
		SkillsDirs:  []string{"skills"},
	}))

	// OpenCode Scanner
	// Detects: .opencode/ directory
	// Resources: rules, commands, skills
	Register(NewDirectoryScanner(ScannerConfig{
		Name:         "opencode",
		LocalDirs:    []string{".opencode"},
		RulesDirs:    []string{"rules"},
		CommandsDirs: []string{"commands"},
		SkillsDirs:   []string{"skills"},
	}))

	// Copilot Scanner
	// Detects: .github-copilot/ directory
	// Resources: commands
	Register(NewDirectoryScanner(ScannerConfig{
		Name:         "copilot",
		LocalDirs:    []string{".github-copilot"},
		CommandsDirs: []string{"commands"},
	}))

	// TopLevel Scanner
	// Detects: skills/ directory or CLAUDE.md/AGENTS.md at project root
	// Resources: skills from skills/, treats CLAUDE.md/AGENTS.md as rules
	Register(NewDirectoryScanner(ScannerConfig{
		Name:        "toplevel",
		LocalDirs:   []string{"skills"},
		DetectFiles: []string{"CLAUDE.md", "AGENTS.md"},
		SkillsDirs:  []string{""}, // skills/ itself is the skills dir
	}))
}
