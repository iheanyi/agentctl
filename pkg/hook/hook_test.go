package hook

import (
	"testing"
)

func TestParseClaudeSettings(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected int // number of hooks
	}{
		{
			name:     "empty settings",
			json:     `{}`,
			expected: 0,
		},
		{
			name:     "no hooks key",
			json:     `{"allowedTools": ["Bash"]}`,
			expected: 0,
		},
		{
			name: "simple string hooks",
			json: `{
				"hooks": {
					"PostToolUse": [
						{
							"matcher": "*",
							"hooks": ["echo done"]
						}
					]
				}
			}`,
			expected: 1,
		},
		{
			name: "object hooks with command",
			json: `{
				"hooks": {
					"PreToolUse": [
						{
							"matcher": "Bash",
							"hooks": [
								{"command": "validate-command.sh", "type": "command"}
							]
						}
					]
				}
			}`,
			expected: 1,
		},
		{
			name: "multiple hook types and entries",
			json: `{
				"hooks": {
					"PostToolUse": [
						{
							"matcher": "*",
							"hooks": ["echo post"]
						}
					],
					"PreToolUse": [
						{
							"matcher": "Bash",
							"hooks": ["echo pre-bash"]
						},
						{
							"matcher": "Edit",
							"hooks": ["echo pre-edit"]
						}
					]
				}
			}`,
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hooks, err := parseClaudeSettings([]byte(tt.json), "test")
			if err != nil {
				t.Fatalf("parseClaudeSettings failed: %v", err)
			}
			if len(hooks) != tt.expected {
				t.Errorf("got %d hooks, want %d", len(hooks), tt.expected)
			}
		})
	}
}

func TestParseClaudeSettingsHookDetails(t *testing.T) {
	json := `{
		"hooks": {
			"PostToolUse": [
				{
					"matcher": "Bash",
					"hooks": ["echo done"]
				}
			]
		}
	}`

	hooks, err := parseClaudeSettings([]byte(json), "claude")
	if err != nil {
		t.Fatalf("parseClaudeSettings failed: %v", err)
	}
	if len(hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(hooks))
	}

	hook := hooks[0]
	if hook.Type != "PostToolUse" {
		t.Errorf("Type = %q, want %q", hook.Type, "PostToolUse")
	}
	if hook.Matcher != "Bash" {
		t.Errorf("Matcher = %q, want %q", hook.Matcher, "Bash")
	}
	if hook.Command != "echo done" {
		t.Errorf("Command = %q, want %q", hook.Command, "echo done")
	}
	if hook.Source != "claude" {
		t.Errorf("Source = %q, want %q", hook.Source, "claude")
	}
}

func TestParseGeminiSettings(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected int // number of hooks
	}{
		{
			name:     "empty settings",
			json:     `{}`,
			expected: 0,
		},
		{
			name: "hooks disabled",
			json: `{
				"hooks": {
					"enabled": false,
					"BeforeTool": [
						{
							"matcher": "*",
							"hooks": [{"command": "echo test"}]
						}
					]
				}
			}`,
			expected: 1, // Still parses hooks even if disabled
		},
		{
			name: "single hook entry",
			json: `{
				"hooks": {
					"enabled": true,
					"BeforeTool": [
						{
							"matcher": "write_file",
							"hooks": [
								{
									"name": "lint-check",
									"type": "command",
									"command": "./lint.sh"
								}
							]
						}
					]
				}
			}`,
			expected: 1,
		},
		{
			name: "multiple hook events",
			json: `{
				"hooks": {
					"enabled": true,
					"BeforeTool": [
						{
							"matcher": "*",
							"hooks": [{"command": "echo before"}]
						}
					],
					"AfterTool": [
						{
							"matcher": "*",
							"hooks": [{"command": "echo after"}]
						}
					],
					"SessionStart": [
						{
							"matcher": "startup",
							"hooks": [{"command": "echo start"}]
						}
					]
				}
			}`,
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hooks, err := parseGeminiSettings([]byte(tt.json), "test")
			if err != nil {
				t.Fatalf("parseGeminiSettings failed: %v", err)
			}
			if len(hooks) != tt.expected {
				t.Errorf("got %d hooks, want %d", len(hooks), tt.expected)
			}
		})
	}
}

func TestParseClaudeTasukuFormat(t *testing.T) {
	// Test the exact format used by Tasuku's .claude/settings.json
	json := `{
		"hooks": {
			"PostToolUse": [
				{
					"hooks": [
						{
							"command": "tk hooks plan-sync",
							"type": "command"
						}
					],
					"matcher": "ExitPlanMode"
				},
				{
					"hooks": [
						{
							"command": "tk hooks todo-check",
							"type": "command"
						}
					],
					"matcher": "TodoWrite"
				}
			],
			"SessionStart": [
				{
					"hooks": [
						{
							"command": "tk hooks session",
							"type": "command"
						}
					]
				}
			],
			"Stop": [
				{
					"hooks": [
						{
							"command": "tk hooks stop-reminder",
							"type": "command"
						}
					]
				}
			]
		}
	}`

	hooks, err := parseClaudeSettings([]byte(json), "claude-local")
	if err != nil {
		t.Fatalf("parseClaudeSettings failed: %v", err)
	}

	// Should find 4 hooks total:
	// - PostToolUse/ExitPlanMode
	// - PostToolUse/TodoWrite
	// - SessionStart (no matcher)
	// - Stop (no matcher)
	if len(hooks) != 4 {
		t.Errorf("expected 4 hooks, got %d", len(hooks))
		for i, h := range hooks {
			t.Logf("  Hook %d: type=%s matcher=%s cmd=%s", i, h.Type, h.Matcher, h.Command)
		}
	}

	// Verify specific hooks
	var foundSession, foundStop bool
	for _, h := range hooks {
		if h.Type == "SessionStart" && h.Command == "tk hooks session" {
			foundSession = true
		}
		if h.Type == "Stop" && h.Command == "tk hooks stop-reminder" {
			foundStop = true
		}
	}
	if !foundSession {
		t.Error("SessionStart hook not found")
	}
	if !foundStop {
		t.Error("Stop hook not found")
	}
}

func TestParseGeminiSettingsHookDetails(t *testing.T) {
	json := `{
		"hooks": {
			"enabled": true,
			"BeforeTool": [
				{
					"matcher": "write_file|replace",
					"hooks": [
						{
							"name": "pre-write-check",
							"type": "command",
							"command": "./validate.sh",
							"description": "Validate before write",
							"timeout": 5000
						}
					]
				}
			]
		}
	}`

	hooks, err := parseGeminiSettings([]byte(json), "gemini")
	if err != nil {
		t.Fatalf("parseGeminiSettings failed: %v", err)
	}
	if len(hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(hooks))
	}

	hook := hooks[0]
	if hook.Type != "BeforeTool" {
		t.Errorf("Type = %q, want %q", hook.Type, "BeforeTool")
	}
	if hook.Matcher != "write_file|replace" {
		t.Errorf("Matcher = %q, want %q", hook.Matcher, "write_file|replace")
	}
	if hook.Command != "./validate.sh" {
		t.Errorf("Command = %q, want %q", hook.Command, "./validate.sh")
	}
	if hook.Name != "pre-write-check" {
		t.Errorf("Name = %q, want %q", hook.Name, "pre-write-check")
	}
	if hook.Source != "gemini" {
		t.Errorf("Source = %q, want %q", hook.Source, "gemini")
	}
}
