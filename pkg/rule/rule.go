package rule

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/iheanyi/agentctl/pkg/pathutil"
)

// Frontmatter represents the optional YAML frontmatter in a rule file
type Frontmatter struct {
	Priority int      `yaml:"priority,omitempty"` // Rule priority (higher = more important)
	Tools    []string `yaml:"tools,omitempty"`    // Which tools this rule applies to
	Applies  string   `yaml:"applies,omitempty"`  // File pattern this rule applies to (e.g., "*.ts") - legacy
	Paths    []string `yaml:"paths,omitempty"`    // File patterns for conditional rules (Claude Code style)
	Globs    []string `yaml:"globs,omitempty"`    // File patterns for conditional rules (Cursor style)
}

// Rule represents a rule/instruction configuration
type Rule struct {
	Name        string       `json:"name"`
	Frontmatter *Frontmatter `json:"frontmatter,omitempty"`
	Content     string       `json:"content"` // Markdown content
	Path        string       `json:"path"`    // Source file path

	// Runtime fields (not serialized)
	Scope string `json:"-"` // "local" or "global" - where this rule came from
}

// Load loads a rule from a markdown file, parsing optional frontmatter
func Load(path string) (*Rule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(data)
	rule := &Rule{
		Path: path,
	}

	// Extract name from filename
	base := strings.TrimSuffix(path, ".md")
	parts := strings.Split(base, "/")
	rule.Name = parts[len(parts)-1]

	// Check for frontmatter (starts with ---)
	if strings.HasPrefix(content, "---") {
		// Find the closing ---
		scanner := bufio.NewScanner(strings.NewReader(content))
		var frontmatterLines []string
		var contentLines []string
		inFrontmatter := false
		frontmatterClosed := false

		for scanner.Scan() {
			line := scanner.Text()
			if line == "---" {
				if !inFrontmatter {
					inFrontmatter = true
					continue
				} else {
					frontmatterClosed = true
					inFrontmatter = false
					continue
				}
			}

			if inFrontmatter {
				frontmatterLines = append(frontmatterLines, line)
			} else if frontmatterClosed {
				contentLines = append(contentLines, line)
			}
		}

		if len(frontmatterLines) > 0 {
			rule.Frontmatter = &Frontmatter{}
			frontmatterYAML := strings.Join(frontmatterLines, "\n")
			if err := yaml.Unmarshal([]byte(frontmatterYAML), rule.Frontmatter); err != nil {
				// If frontmatter parsing fails, treat entire file as content
				rule.Frontmatter = nil
				rule.Content = content
			} else {
				rule.Content = strings.TrimSpace(strings.Join(contentLines, "\n"))
			}
		} else {
			rule.Content = content
		}
	} else {
		rule.Content = content
	}

	return rule, nil
}

// LoadAll loads all rules from a directory
func LoadAll(dir string) ([]*Rule, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var rules []*Rule
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		rule, err := Load(dir + "/" + entry.Name())
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

// Save saves a rule to a directory as a markdown file
func Save(r *Rule, dir string) error {
	// Ensure directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	var content strings.Builder

	// Write frontmatter if present
	if r.Frontmatter != nil {
		content.WriteString("---\n")
		if r.Frontmatter.Priority != 0 {
			content.WriteString(fmt.Sprintf("priority: %d\n", r.Frontmatter.Priority))
		}
		if len(r.Frontmatter.Tools) > 0 {
			content.WriteString("tools: [")
			content.WriteString(strings.Join(r.Frontmatter.Tools, ", "))
			content.WriteString("]\n")
		}
		if r.Frontmatter.Applies != "" {
			content.WriteString("applies: \"")
			content.WriteString(r.Frontmatter.Applies)
			content.WriteString("\"\n")
		}
		if len(r.Frontmatter.Paths) > 0 {
			content.WriteString("paths:\n")
			for _, p := range r.Frontmatter.Paths {
				content.WriteString("  - \"")
				content.WriteString(p)
				content.WriteString("\"\n")
			}
		}
		if len(r.Frontmatter.Globs) > 0 {
			content.WriteString("globs:\n")
			for _, g := range r.Frontmatter.Globs {
				content.WriteString("  - \"")
				content.WriteString(g)
				content.WriteString("\"\n")
			}
		}
		content.WriteString("---\n\n")
	}

	content.WriteString(r.Content)

	// Determine filename
	name := r.Name
	if name == "" {
		name = "imported-rule"
	}

	// Validate rule name to prevent path traversal (without extension)
	baseName := strings.TrimSuffix(name, ".md")
	if err := pathutil.SanitizeName(baseName); err != nil {
		return fmt.Errorf("invalid rule name: %w", err)
	}

	if !strings.HasSuffix(name, ".md") {
		name += ".md"
	}

	path := dir + "/" + name
	return os.WriteFile(path, []byte(content.String()), 0644)
}
