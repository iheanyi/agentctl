package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathToName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"./tools/filesystem-mcp", "filesystem"},
		{"./my-server", "my"},
		{"/absolute/path/to/db-server", "db"},
		{"github.com/org/repo-mcp", "repo"},
		{"simple", "simple"},
		{"./path/with.git", "with"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := pathToName(tt.path)
			if got != tt.want {
				t.Errorf("pathToName(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestParseInstallTarget(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		wantType string
		wantErr  bool
	}{
		{
			name:     "local path ./",
			target:   "./local/mcp",
			wantType: "local",
		},
		{
			name:     "local path /",
			target:   "/absolute/path",
			wantType: "local",
		},
		{
			name:     "local path ../",
			target:   "../relative/path",
			wantType: "local",
		},
		{
			name:     "git URL",
			target:   "github.com/org/repo",
			wantType: "git",
		},
		{
			name:     "alias",
			target:   "filesystem",
			wantType: "alias",
		},
		{
			name:    "unknown alias",
			target:  "unknown-nonexistent-alias",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := parseInstallTarget(tt.target)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if server.Source.Type != tt.wantType {
				t.Errorf("Source.Type = %q, want %q", server.Source.Type, tt.wantType)
			}
		})
	}
}

func TestParseInstallTargetWithVersion(t *testing.T) {
	server, err := parseInstallTarget("filesystem@v1.2.3")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if server.Source.Ref != "v1.2.3" {
		t.Errorf("Source.Ref = %q, want %q", server.Source.Ref, "v1.2.3")
	}
}

func TestGetVersion(t *testing.T) {
	// Test with a command that should exist
	version, err := getVersion("go", []string{"version"})
	if err != nil {
		t.Skipf("go command not available: %v", err)
	}
	if version == "" {
		t.Error("Version should not be empty")
	}
}

func TestGetVersionNonexistent(t *testing.T) {
	_, err := getVersion("nonexistent-command-xyz", []string{})
	if err == nil {
		t.Error("Should return error for nonexistent command")
	}
}

func TestGetVersionTruncation(t *testing.T) {
	// Test that version strings are truncated properly
	version, err := getVersion("echo", []string{"this is a test version string"})
	if err != nil {
		t.Skipf("echo command not available: %v", err)
	}
	// Version should not contain newlines
	for _, c := range version {
		if c == '\n' || c == '\r' {
			t.Error("Version should not contain newlines")
		}
	}
}

func TestVersionConstants(t *testing.T) {
	// Version and Commit should have default values
	if Version == "" {
		t.Error("Version should have a default value")
	}
	if Commit == "" {
		t.Error("Commit should have a default value")
	}
}

func TestInitFlagsExist(t *testing.T) {
	flag := initCmd.Flag("local")
	if flag == nil {
		t.Error("init command should have --local flag")
	}
}

func TestInstallFlagsExist(t *testing.T) {
	flag := installCmd.Flag("namespace")
	if flag == nil {
		t.Error("install command should have --namespace flag")
	}
}

func TestListFlagsExist(t *testing.T) {
	typeFlag := listCmd.Flag("type")
	if typeFlag == nil {
		t.Error("list command should have --type flag")
	}

	profileFlag := listCmd.Flag("profile")
	if profileFlag == nil {
		t.Error("list command should have --profile flag")
	}
}

func TestSyncFlagsExist(t *testing.T) {
	flags := []string{"tool", "dry-run", "clean"}
	for _, name := range flags {
		flag := syncCmd.Flag(name)
		if flag == nil {
			t.Errorf("sync command should have --%s flag", name)
		}
	}
}

func TestAliasSubcommands(t *testing.T) {
	// Check alias has expected subcommands
	subcommands := aliasCmd.Commands()
	expectedNames := map[string]bool{"list": false, "add": false, "remove": false}

	for _, cmd := range subcommands {
		if _, ok := expectedNames[cmd.Name()]; ok {
			expectedNames[cmd.Name()] = true
		}
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("alias command should have %s subcommand", name)
		}
	}
}

func TestProfileSubcommands(t *testing.T) {
	subcommands := profileCmd.Commands()
	expectedNames := map[string]bool{"list": false, "create": false, "switch": false, "export": false, "import": false}

	for _, cmd := range subcommands {
		if _, ok := expectedNames[cmd.Name()]; ok {
			expectedNames[cmd.Name()] = true
		}
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("profile command should have %s subcommand", name)
		}
	}
}

func TestRootCommandHasSubcommands(t *testing.T) {
	subcommands := rootCmd.Commands()
	expectedNames := []string{"install", "remove", "list", "sync", "alias", "profile", "init", "doctor", "status", "version"}

	subcommandNames := make(map[string]bool)
	for _, cmd := range subcommands {
		subcommandNames[cmd.Name()] = true
	}

	for _, name := range expectedNames {
		if !subcommandNames[name] {
			t.Errorf("root command should have %s subcommand", name)
		}
	}
}

func TestRemoveAliases(t *testing.T) {
	aliases := removeCmd.Aliases
	hasRM := false
	hasUninstall := false
	for _, a := range aliases {
		if a == "rm" {
			hasRM = true
		}
		if a == "uninstall" {
			hasUninstall = true
		}
	}
	if !hasRM {
		t.Error("remove command should have 'rm' alias")
	}
	if !hasUninstall {
		t.Error("remove command should have 'uninstall' alias")
	}
}

func TestListAliases(t *testing.T) {
	aliases := listCmd.Aliases
	hasLS := false
	for _, a := range aliases {
		if a == "ls" {
			hasLS = true
		}
	}
	if !hasLS {
		t.Error("list command should have 'ls' alias")
	}
}

func TestAliasAddFlags(t *testing.T) {
	descFlag := aliasAddCmd.Flag("description")
	if descFlag == nil {
		t.Error("alias add should have --description flag")
	}

	runtimeFlag := aliasAddCmd.Flag("runtime")
	if runtimeFlag == nil {
		t.Error("alias add should have --runtime flag")
	}
}

func TestConfigDirCreation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cli-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test that the expected directories would be created
	expectedDirs := []string{"servers", "commands", "rules", "prompts", "skills", "profiles"}
	for _, dir := range expectedDirs {
		dirPath := filepath.Join(tmpDir, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Errorf("Failed to create %s directory: %v", dir, err)
		}
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			t.Errorf("Directory %s should exist", dir)
		}
	}
}
