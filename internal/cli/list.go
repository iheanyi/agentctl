package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/iheanyi/agentctl/pkg/config"
	"github.com/iheanyi/agentctl/pkg/output"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List installed resources",
	Long: `List installed MCP servers, commands, rules, prompts, and skills.

Scope:
  By default, shows all resources from both local and global configs.
  Use --scope to filter by config scope.

  Scope indicators in output:
    [L] = local (project-specific)
    [G] = global (user-wide)

Examples:
  agentctl list                  # List all resources
  agentctl list --scope local    # List only local/project resources
  agentctl list --scope global   # List only global resources
  agentctl list --type servers   # List only servers
  agentctl list --type commands  # List only commands`,
	RunE: runList,
}

var (
	listType    string
	listProfile string
	listScope   string
)

func init() {
	listCmd.Flags().StringVarP(&listType, "type", "t", "", "Filter by resource type (servers, commands, rules, prompts, skills)")
	listCmd.Flags().StringVarP(&listProfile, "profile", "p", "", "List resources from specific profile")
	listCmd.Flags().StringVarP(&listScope, "scope", "s", "", "Filter by scope: local, global, or all (default: all)")
}

func runList(cmd *cobra.Command, args []string) error {
	// Parse scope filter
	var scope config.Scope
	if listScope != "" {
		var err error
		scope, err = config.ParseScope(listScope)
		if err != nil {
			if JSONOutput {
				jw := output.NewJSONWriter()
				return jw.WriteError(err)
			}
			return err
		}
	} else {
		scope = config.ScopeAll
	}

	// Load config (including project config if present)
	cfg, err := config.LoadWithProject()
	if err != nil {
		if JSONOutput {
			jw := output.NewJSONWriter()
			return jw.WriteError(fmt.Errorf("failed to load config: %w", err))
		}
		return fmt.Errorf("failed to load config: %w", err)
	}

	// JSON output mode
	if JSONOutput {
		return runListJSON(cfg, scope)
	}

	// Show project config notice if applicable
	if cfg.ProjectPath != "" {
		fmt.Printf("Project config: %s\n\n", cfg.ProjectPath)
	}

	// TODO: Apply profile filtering when listProfile != ""
	_ = listProfile // Suppress unused warning until implemented

	hasOutput := false

	// List servers
	if listType == "" || listType == "servers" {
		// Get servers filtered by scope
		servers := cfg.ServersForScope(scope)
		if len(servers) > 0 {
			fmt.Println("MCP Servers:")
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "  NAME\tSCOPE\tSOURCE\tSTATUS")
			for _, server := range servers {
				status := "enabled"
				if server.Disabled {
					status = "disabled"
				}
				sourceInfo := server.Source.Type
				if server.Source.URL != "" {
					sourceInfo = server.Source.URL
				} else if server.Source.Alias != "" {
					sourceInfo = "alias:" + server.Source.Alias
				}
				scopeIndicator := scopeToIndicator(server.Scope)
				fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", server.Name, scopeIndicator, sourceInfo, status)
			}
			w.Flush()
			hasOutput = true
		}
	}

	// List commands
	if listType == "" || listType == "commands" {
		commands := cfg.CommandsForScope(scope)
		if len(commands) > 0 {
			if hasOutput {
				fmt.Println()
			}
			fmt.Println("Commands:")
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "  NAME\tSCOPE\tDESCRIPTION")
			for _, cmd := range commands {
				scopeIndicator := scopeToIndicator(cmd.Scope)
				desc := cmd.Description
				if len(desc) > 40 {
					desc = desc[:37] + "..."
				}
				fmt.Fprintf(w, "  %s\t%s\t%s\n", cmd.Name, scopeIndicator, desc)
			}
			w.Flush()
			hasOutput = true
		}
	}

	// List rules
	if listType == "" || listType == "rules" {
		rules := cfg.RulesForScope(scope)
		if len(rules) > 0 {
			if hasOutput {
				fmt.Println()
			}
			fmt.Println("Rules:")
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "  NAME\tSCOPE\tPATH")
			for _, r := range rules {
				scopeIndicator := scopeToIndicator(r.Scope)
				fmt.Fprintf(w, "  %s\t%s\t%s\n", r.Name, scopeIndicator, r.Path)
			}
			w.Flush()
			hasOutput = true
		}
	}

	// List prompts
	if listType == "" || listType == "prompts" {
		prompts := cfg.PromptsForScope(scope)
		if len(prompts) > 0 {
			if hasOutput {
				fmt.Println()
			}
			fmt.Println("Prompts:")
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "  NAME\tSCOPE\tDESCRIPTION")
			for _, p := range prompts {
				scopeIndicator := scopeToIndicator(p.Scope)
				desc := p.Description
				if len(desc) > 40 {
					desc = desc[:37] + "..."
				}
				fmt.Fprintf(w, "  %s\t%s\t%s\n", p.Name, scopeIndicator, desc)
			}
			w.Flush()
			hasOutput = true
		}
	}

	// List skills
	if listType == "" || listType == "skills" {
		skills := cfg.SkillsForScope(scope)
		if len(skills) > 0 {
			if hasOutput {
				fmt.Println()
			}
			fmt.Println("Skills:")
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "  NAME\tSCOPE\tDESCRIPTION")
			for _, s := range skills {
				scopeIndicator := scopeToIndicator(s.Scope)
				desc := s.Description
				if len(desc) > 40 {
					desc = desc[:37] + "..."
				}
				fmt.Fprintf(w, "  %s\t%s\t%s\n", s.Name, scopeIndicator, desc)
			}
			w.Flush()
			hasOutput = true
		}
	}

	if !hasOutput {
		fmt.Println("No resources installed.")
		fmt.Println("\nGet started:")
		fmt.Println("  agentctl install filesystem  # Install filesystem MCP server")
		fmt.Println("  agentctl search <query>      # Search for MCP servers")
	}

	return nil
}

// runListJSON outputs the list results as JSON
func runListJSON(cfg *config.Config, scope config.Scope) error {
	jw := output.NewJSONWriter()

	listOutput := output.ListOutput{
		ProjectPath: cfg.ProjectPath,
	}

	// Get servers filtered by scope and type
	if listType == "" || listType == "servers" {
		servers := cfg.ServersForScope(scope)
		for _, server := range servers {
			status := "enabled"
			if server.Disabled {
				status = "disabled"
			}
			sourceInfo := server.Source.Type
			if server.Source.URL != "" {
				sourceInfo = server.Source.URL
			} else if server.Source.Alias != "" {
				sourceInfo = "alias:" + server.Source.Alias
			}

			transport := "stdio"
			if server.URL != "" {
				transport = string(server.Transport)
				if transport == "" {
					transport = "http"
				}
			}

			listOutput.Servers = append(listOutput.Servers, output.ServerInfo{
				Name:      server.Name,
				Scope:     server.Scope,
				Source:    sourceInfo,
				Status:    status,
				Command:   server.Command,
				URL:       server.URL,
				Transport: transport,
			})
		}
	}

	// Get commands filtered by scope and type
	if listType == "" || listType == "commands" {
		commands := cfg.CommandsForScope(scope)
		for _, cmd := range commands {
			listOutput.Commands = append(listOutput.Commands, output.CommandInfo{
				Name:        cmd.Name,
				Scope:       cmd.Scope,
				Description: cmd.Description,
			})
		}
	}

	// Get rules filtered by scope and type
	if listType == "" || listType == "rules" {
		rules := cfg.RulesForScope(scope)
		for _, r := range rules {
			listOutput.Rules = append(listOutput.Rules, output.RuleInfo{
				Name:  r.Name,
				Scope: r.Scope,
				Path:  r.Path,
			})
		}
	}

	// Get prompts filtered by scope and type
	if listType == "" || listType == "prompts" {
		prompts := cfg.PromptsForScope(scope)
		for _, p := range prompts {
			listOutput.Prompts = append(listOutput.Prompts, output.PromptInfo{
				Name:        p.Name,
				Scope:       p.Scope,
				Description: p.Description,
			})
		}
	}

	// Get skills filtered by scope and type
	if listType == "" || listType == "skills" {
		skills := cfg.SkillsForScope(scope)
		for _, s := range skills {
			listOutput.Skills = append(listOutput.Skills, output.SkillInfo{
				Name:        s.Name,
				Scope:       s.Scope,
				Description: s.Description,
			})
		}
	}

	return jw.WriteSuccess(listOutput)
}

// scopeToIndicator converts a scope string to a short indicator for display
func scopeToIndicator(scope string) string {
	switch scope {
	case string(config.ScopeLocal):
		return "[L]"
	case string(config.ScopeGlobal):
		return "[G]"
	default:
		return "[G]" // Default to global
	}
}
