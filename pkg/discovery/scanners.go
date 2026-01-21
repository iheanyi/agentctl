package discovery

// This file registers all DirectoryScanner-based tool scanners.
// Each tool has a simple config - no complex logic needed.

func init() {
	// Cursor Scanner
	// Detects: .cursor/ directory or .cursorrules file
	// Resources: rules in .cursor/rules/, agents in .cursor/agents/
	Register(NewDirectoryScanner(ScannerConfig{
		Name:        "cursor",
		LocalDirs:   []string{".cursor"},
		GlobalDirs:  []string{"~/.cursor"},
		DetectFiles: []string{".cursorrules"},
		RulesDirs:   []string{"rules"},
		AgentsDirs:  []string{"agents"},
		FileExts:    []string{".md", ".mdc"},
	}))

	// Codex Scanner
	// Detects: .codex/ directory or AGENTS.md file
	// Resources: skills in .codex/skills/, custom prompts in prompts/
	// Note: Codex custom prompts are markdown files with description/argument-hint frontmatter
	// They function as slash commands (e.g., ~/.codex/prompts/test.md -> /test)
	Register(NewDirectoryScanner(ScannerConfig{
		Name:         "codex",
		LocalDirs:    []string{".codex"},
		GlobalDirs:   []string{"~/.codex"},
		DetectFiles:  []string{"AGENTS.md"},
		CommandsDirs: []string{"prompts"}, // Custom prompts as slash commands
		SkillsDirs:   []string{"skills"},
	}))

	// OpenCode Scanner
	// Detects: .opencode/ directory
	// Resources: rules, commands, skills, agents
	Register(NewDirectoryScanner(ScannerConfig{
		Name:         "opencode",
		LocalDirs:    []string{".opencode"},
		GlobalDirs:   []string{"~/.config/opencode"},
		RulesDirs:    []string{"rules"},
		CommandsDirs: []string{"commands"},
		SkillsDirs:   []string{"skills"},
		AgentsDirs:   []string{"agent", "agents"}, // OpenCode uses "agent" directory
	}))

	// Copilot Scanner
	// Detects: .github/ directory with agents/ subdirectory
	// Resources: agents in .github/agents/
	Register(NewDirectoryScanner(ScannerConfig{
		Name:         "copilot",
		LocalDirs:    []string{".github"},
		DetectFiles:  []string{".github/agents"},
		CommandsDirs: []string{"commands"},
		AgentsDirs:   []string{"agents"},
		FileExts:     []string{".md", ".agent.md"},
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
