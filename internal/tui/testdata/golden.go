package testdata

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestDataDir returns the path to the testdata directory
func TestDataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Dir(filename)
}

// GoldenDir returns the path to the golden files directory
func GoldenDir() string {
	return filepath.Join(TestDataDir(), "golden")
}

// GoldenPath returns the path to a specific golden file
func GoldenPath(name string) string {
	return filepath.Join(GoldenDir(), name+".golden")
}

// LoadGolden loads the expected output from a golden file
func LoadGolden(name string) ([]byte, error) {
	return os.ReadFile(GoldenPath(name))
}

// UpdateGolden updates a golden file with new content
func UpdateGolden(name string, content []byte) error {
	dir := GoldenDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(GoldenPath(name), content, 0644)
}

// ShouldUpdateGoldens returns true if golden files should be auto-updated
func ShouldUpdateGoldens() bool {
	return os.Getenv("UPDATE_GOLDENS") == "1"
}

// GoldenResult contains the result of a golden file comparison
type GoldenResult struct {
	Match    bool
	Expected []byte
	Actual   []byte
	Diff     string
}

// CompareGolden compares actual output against a golden file
func CompareGolden(t *testing.T, name string, actual []byte) GoldenResult {
	t.Helper()

	expected, err := LoadGolden(name)
	if err != nil {
		if os.IsNotExist(err) {
			return GoldenResult{
				Match:  false,
				Actual: actual,
				Diff:   "Golden file does not exist. Run with UPDATE_GOLDENS=1 to create.",
			}
		}
		t.Fatalf("Failed to load golden file: %v", err)
	}

	// Normalize line endings for comparison
	expectedNorm := normalizeLineEndings(expected)
	actualNorm := normalizeLineEndings(actual)

	if bytes.Equal(expectedNorm, actualNorm) {
		return GoldenResult{Match: true, Expected: expected, Actual: actual}
	}

	diff := generateDiff(string(expected), string(actual))
	return GoldenResult{
		Match:    false,
		Expected: expected,
		Actual:   actual,
		Diff:     diff,
	}
}

// AssertGolden is a test helper that compares output against golden files
// and handles updates based on environment variables
func AssertGolden(t *testing.T, name string, actual []byte) {
	t.Helper()

	result := CompareGolden(t, name, actual)
	if result.Match {
		return
	}

	if ShouldUpdateGoldens() {
		if err := UpdateGolden(name, actual); err != nil {
			t.Fatalf("Failed to update golden file: %v", err)
		}
		t.Logf("Updated golden file: %s", name)
		return
	}

	t.Errorf("Golden file mismatch for %s:\n%s\n\nRun with UPDATE_GOLDENS=1 to update", name, result.Diff)
}

// normalizeLineEndings converts all line endings to \n
func normalizeLineEndings(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
}

// generateDiff generates a simple line-by-line diff
func generateDiff(expected, actual string) string {
	expectedLines := strings.Split(expected, "\n")
	actualLines := strings.Split(actual, "\n")

	var diff strings.Builder
	diff.WriteString("--- Expected (golden)\n")
	diff.WriteString("+++ Actual (output)\n")

	maxLen := len(expectedLines)
	if len(actualLines) > maxLen {
		maxLen = len(actualLines)
	}

	for i := 0; i < maxLen; i++ {
		var expLine, actLine string
		if i < len(expectedLines) {
			expLine = expectedLines[i]
		}
		if i < len(actualLines) {
			actLine = actualLines[i]
		}

		if expLine != actLine {
			if expLine != "" {
				diff.WriteString("-" + expLine + "\n")
			}
			if actLine != "" {
				diff.WriteString("+" + actLine + "\n")
			}
		} else {
			diff.WriteString(" " + expLine + "\n")
		}
	}

	return diff.String()
}

// StripANSI removes ANSI escape sequences from a string
// This is useful for comparing rendered output without color codes
func StripANSI(s string) string {
	var result strings.Builder
	inEscape := false

	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if s[i] == 'm' {
				inEscape = false
			}
			continue
		}
		result.WriteByte(s[i])
	}

	return result.String()
}

// StripTimestamps replaces timestamps in the format "HH:MM:SS" with a placeholder
// This is useful for comparing output that contains dynamic timestamps
func StripTimestamps(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		// Look for pattern HH:MM:SS (exactly 8 chars: digit digit colon digit digit colon digit digit)
		if i+8 <= len(s) &&
			isDigit(s[i]) && isDigit(s[i+1]) && s[i+2] == ':' &&
			isDigit(s[i+3]) && isDigit(s[i+4]) && s[i+5] == ':' &&
			isDigit(s[i+6]) && isDigit(s[i+7]) {
			result.WriteString("HH:MM:SS")
			i += 8
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String()
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
