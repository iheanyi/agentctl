package cli

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/iheanyi/agentctl/pkg/skill"
)

func TestDiscoverInstallCandidatesFromSkillsDirectory(t *testing.T) {
	root := t.TempDir()
	createSkillFixture(t, filepath.Join(root, "skills", "alpha"), "alpha", "Alpha skill")
	createSkillFixture(t, filepath.Join(root, "skills", "beta"), "beta", "Beta skill")

	candidates, err := discoverInstallCandidates(root, "")
	if err != nil {
		t.Fatalf("discoverInstallCandidates() error = %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("discoverInstallCandidates() count = %d, want 2", len(candidates))
	}

	var names []string
	for _, candidate := range candidates {
		names = append(names, candidate.Skill.Name)
	}
	sort.Strings(names)

	if names[0] != "alpha" || names[1] != "beta" {
		t.Fatalf("discoverInstallCandidates() names = %v, want [alpha beta]", names)
	}
}

func TestFilterInstallCandidatesByName(t *testing.T) {
	candidates := []installCandidate{
		{Skill: &skill.Skill{Name: "alpha"}},
		{Skill: &skill.Skill{Name: "beta"}},
	}

	filtered, err := filterInstallCandidates(candidates, []string{"beta"})
	if err != nil {
		t.Fatalf("filterInstallCandidates() error = %v", err)
	}
	if len(filtered) != 1 || filtered[0].Skill.Name != "beta" {
		t.Fatalf("filterInstallCandidates() = %+v, want only beta", filtered)
	}
}

func TestFilterInstallCandidatesMissingSkill(t *testing.T) {
	candidates := []installCandidate{
		{Skill: &skill.Skill{Name: "alpha"}},
	}

	_, err := filterInstallCandidates(candidates, []string{"missing"})
	if err == nil {
		t.Fatal("filterInstallCandidates() expected missing-skill error")
	}
}

func TestSkillSourceMetadataRoundTrip(t *testing.T) {
	dir := t.TempDir()
	metadata := &skillSourceMetadata{
		SourceType:  "github",
		Source:      "github.com/example/repo",
		RepoURL:     "https://github.com/example/repo",
		RepoSubPath: "skills",
		SkillPath:   "skills/alpha",
		Commit:      "abc123",
		InstalledAt: "2026-01-01T00:00:00Z",
	}

	if err := writeSkillSourceMetadata(dir, metadata); err != nil {
		t.Fatalf("writeSkillSourceMetadata() error = %v", err)
	}

	got, err := readSkillSourceMetadata(dir)
	if err != nil {
		t.Fatalf("readSkillSourceMetadata() error = %v", err)
	}
	if got.SourceType != metadata.SourceType || got.Commit != metadata.Commit || got.SkillPath != metadata.SkillPath {
		t.Fatalf("metadata round-trip mismatch: got %+v want %+v", got, metadata)
	}
}

func TestSelectSkillStatusesForUpdateDefaultsToAvailable(t *testing.T) {
	statuses := []skillUpdateStatus{
		{
			Skill:    &skill.Skill{Name: "alpha"},
			Metadata: &skillSourceMetadata{SourceType: "github", RepoURL: "https://github.com/example/repo"},
			Status:   "update-available",
		},
		{
			Skill:    &skill.Skill{Name: "beta"},
			Metadata: &skillSourceMetadata{SourceType: "github", RepoURL: "https://github.com/example/repo"},
			Status:   "up-to-date",
		},
	}

	selected, err := selectSkillStatusesForUpdate(statuses, nil)
	if err != nil {
		t.Fatalf("selectSkillStatusesForUpdate() error = %v", err)
	}
	if len(selected) != 1 || selected[0].Skill.Name != "alpha" {
		t.Fatalf("selectSkillStatusesForUpdate() = %+v, want only alpha", selected)
	}
}

func TestSelectSkillStatusesForUpdateRejectsUntracked(t *testing.T) {
	statuses := []skillUpdateStatus{
		{
			Skill:  &skill.Skill{Name: "alpha"},
			Status: "untracked",
		},
	}

	_, err := selectSkillStatusesForUpdate(statuses, []string{"alpha"})
	if err == nil {
		t.Fatal("selectSkillStatusesForUpdate() expected untracked error")
	}
}

func TestSkillMatchesQuery(t *testing.T) {
	s := &skill.Skill{
		Name:        "frontend-design",
		Description: "Build polished UI components",
		Commands: []*skill.Command{
			{Name: "audit", Description: "Audit component quality"},
		},
	}

	if !skillMatchesQuery(s, "frontend") {
		t.Fatal("skillMatchesQuery() should match by skill name")
	}
	if !skillMatchesQuery(s, "quality") {
		t.Fatal("skillMatchesQuery() should match by command description")
	}
	if skillMatchesQuery(s, "database") {
		t.Fatal("skillMatchesQuery() should not match unrelated query")
	}
}

func TestNormalizeRelativePath(t *testing.T) {
	if got := normalizeRelativePath("."); got != "" {
		t.Fatalf("normalizeRelativePath(\".\") = %q, want empty string", got)
	}
	if got := normalizeRelativePath("skills\\alpha"); got != "skills/alpha" {
		t.Fatalf("normalizeRelativePath() slash normalization failed: got %q", got)
	}
}

func createSkillFixture(t *testing.T, dir, name, description string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create skill dir: %v", err)
	}

	content := "---\n" +
		"name: " + name + "\n" +
		"description: " + description + "\n" +
		"---\n\n" +
		"# " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, skill.SkillFileName), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write skill fixture: %v", err)
	}
}
