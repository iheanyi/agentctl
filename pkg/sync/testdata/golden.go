package testdata

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/iheanyi/agentctl/pkg/mcp"
)

// TestDataDir returns the path to the testdata directory
func TestDataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Dir(filename)
}

// FixturesDir returns the path to the fixtures directory
func FixturesDir() string {
	return filepath.Join(TestDataDir(), "fixtures")
}

// GoldenDir returns the path to the golden files directory
func GoldenDir() string {
	return filepath.Join(TestDataDir(), "golden")
}

// AdapterGoldenDir returns the path to golden files for a specific adapter
func AdapterGoldenDir(adapterName string) string {
	return filepath.Join(GoldenDir(), adapterName)
}

// LoadFixtureServers loads servers from a fixture file
func LoadFixtureServers(name string) ([]*mcp.Server, error) {
	path := filepath.Join(FixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read fixture %s: %w", name, err)
	}

	var fixture struct {
		Servers []*mcp.Server `json:"servers"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		return nil, fmt.Errorf("failed to parse fixture %s: %w", name, err)
	}

	return fixture.Servers, nil
}

// LoadGoldenInput loads the input JSON for a golden test
func LoadGoldenInput(adapterName, testName string) (map[string]interface{}, error) {
	path := filepath.Join(AdapterGoldenDir(adapterName), testName+".input.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// No input file means start with empty config
			return make(map[string]interface{}), nil
		}
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// LoadGoldenExpected loads the expected output JSON for a golden test
func LoadGoldenExpected(adapterName, testName string) ([]byte, error) {
	path := filepath.Join(AdapterGoldenDir(adapterName), testName+".golden.json")
	return os.ReadFile(path)
}

// GoldenResult contains the result of a golden file comparison
type GoldenResult struct {
	Match    bool
	Expected []byte
	Actual   []byte
	Diff     string
}

// CompareGolden compares actual output against a golden file
func CompareGolden(t *testing.T, adapterName, testName string, actual []byte) GoldenResult {
	t.Helper()

	expected, err := LoadGoldenExpected(adapterName, testName)
	if err != nil {
		if os.IsNotExist(err) {
			// Golden file doesn't exist yet
			return GoldenResult{
				Match:  false,
				Actual: actual,
				Diff:   "Golden file does not exist. Run with UPDATE_GOLDENS=1 to create.",
			}
		}
		t.Fatalf("Failed to load golden file: %v", err)
	}

	// Normalize JSON for comparison
	expectedNorm := normalizeJSON(expected)
	actualNorm := normalizeJSON(actual)

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

// UpdateGolden updates a golden file with new content
func UpdateGolden(adapterName, testName string, content []byte) error {
	dir := AdapterGoldenDir(adapterName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, testName+".golden.json")

	// Pretty-print the JSON
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, content, "", "  "); err != nil {
		// If it's not valid JSON, write as-is
		return os.WriteFile(path, content, 0644)
	}

	return os.WriteFile(path, prettyJSON.Bytes(), 0644)
}

// ShouldUpdateGoldens returns true if golden files should be auto-updated
func ShouldUpdateGoldens() bool {
	return os.Getenv("UPDATE_GOLDENS") == "1"
}

// IsInteractiveMode returns true if interactive golden update mode is enabled
func IsInteractiveMode() bool {
	return os.Getenv("GOLDEN_INTERACTIVE") == "1"
}

// PromptGoldenUpdate prompts the user to accept or reject a golden file update
// Returns true if the user accepts the change
func PromptGoldenUpdate(adapterName, testName string, result GoldenResult) bool {
	fmt.Printf("\n")
	fmt.Printf("Golden file mismatch: %s/%s\n", adapterName, testName)
	fmt.Printf("%s\n", DiffSummary(string(result.Expected), string(result.Actual)))
	fmt.Println()

	// Show colored diff by default
	coloredDiff := ColoredDiff(string(result.Expected), string(result.Actual))
	fmt.Println(coloredDiff)

	fmt.Print("\nAccept this change? [y/n/s(ide-by-side)/d(iff)/q(uit)]: ")

	reader := bufio.NewReader(os.Stdin)
	for {
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		switch input {
		case "y", "yes":
			return true
		case "n", "no":
			return false
		case "d", "diff":
			fmt.Println(coloredDiff)
			fmt.Print("\nAccept this change? [y/n/s(ide-by-side)/d(iff)/q(uit)]: ")
		case "s", "side":
			sideBySide := SideBySideDiff(string(result.Expected), string(result.Actual), 120)
			fmt.Println(sideBySide)
			fmt.Print("\nAccept this change? [y/n/s(ide-by-side)/d(iff)/q(uit)]: ")
		case "q", "quit":
			os.Exit(0)
		default:
			fmt.Print("Please enter y, n, s, d, or q: ")
		}
	}
}

// AssertGolden is a test helper that compares output against golden files
// and handles updates based on environment variables
func AssertGolden(t *testing.T, adapterName, testName string, actual []byte) {
	t.Helper()

	result := CompareGolden(t, adapterName, testName, actual)
	if result.Match {
		return
	}

	if ShouldUpdateGoldens() {
		if err := UpdateGolden(adapterName, testName, actual); err != nil {
			t.Fatalf("Failed to update golden file: %v", err)
		}
		t.Logf("Updated golden file: %s/%s", adapterName, testName)
		return
	}

	if IsInteractiveMode() {
		if PromptGoldenUpdate(adapterName, testName, result) {
			if err := UpdateGolden(adapterName, testName, actual); err != nil {
				t.Fatalf("Failed to update golden file: %v", err)
			}
			t.Logf("Updated golden file: %s/%s", adapterName, testName)
			return
		}
	}

	t.Errorf("Golden file mismatch for %s/%s:\n%s\n\nRun with UPDATE_GOLDENS=1 to update", adapterName, testName, result.Diff)
}

// normalizeJSON normalizes JSON for comparison (consistent formatting)
func normalizeJSON(data []byte) []byte {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return data
	}
	normalized, err := json.Marshal(v)
	if err != nil {
		return data
	}
	return normalized
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
				diff.WriteString(fmt.Sprintf("-%s\n", expLine))
			}
			if actLine != "" {
				diff.WriteString(fmt.Sprintf("+%s\n", actLine))
			}
		} else {
			diff.WriteString(fmt.Sprintf(" %s\n", expLine))
		}
	}

	return diff.String()
}

// CreateTempConfigDir creates a temporary directory for testing config files
func CreateTempConfigDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "agentctl-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// WriteTestConfig writes a test config to a temporary directory
func WriteTestConfig(t *testing.T, dir, filename string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, filename)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	return path
}

// ReadTestConfig reads a test config file
func ReadTestConfig(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}
	return data
}
