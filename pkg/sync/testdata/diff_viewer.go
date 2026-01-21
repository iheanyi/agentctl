package testdata

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// DiffStyle configures the appearance of diff output
type DiffStyle struct {
	Added     lipgloss.Style
	Removed   lipgloss.Style
	Context   lipgloss.Style
	Header    lipgloss.Style
	LineNum   lipgloss.Style
	Separator lipgloss.Style
}

// DefaultDiffStyle returns the default diff styling
func DefaultDiffStyle() DiffStyle {
	return DiffStyle{
		Added:     lipgloss.NewStyle().Foreground(lipgloss.Color("2")),            // Green
		Removed:   lipgloss.NewStyle().Foreground(lipgloss.Color("1")),            // Red
		Context:   lipgloss.NewStyle().Foreground(lipgloss.Color("8")),            // Gray
		Header:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6")), // Cyan
		LineNum:   lipgloss.NewStyle().Foreground(lipgloss.Color("8")),            // Gray
		Separator: lipgloss.NewStyle().Foreground(lipgloss.Color("8")),            // Gray
	}
}

// ColoredDiff generates a colored unified diff
func ColoredDiff(expected, actual string) string {
	style := DefaultDiffStyle()
	expectedLines := strings.Split(expected, "\n")
	actualLines := strings.Split(actual, "\n")

	var diff strings.Builder
	diff.WriteString(style.Header.Render("--- Expected (golden)") + "\n")
	diff.WriteString(style.Header.Render("+++ Actual (output)") + "\n")
	diff.WriteString(style.Separator.Render(strings.Repeat("-", 60)) + "\n")

	// Use a simple LCS-based diff
	ops := computeDiffOps(expectedLines, actualLines)

	lineNumExp := 1
	lineNumAct := 1
	for _, op := range ops {
		switch op.Type {
		case DiffEqual:
			numStr := fmt.Sprintf("%4d", lineNumExp)
			diff.WriteString(style.LineNum.Render(numStr) + " " + style.Context.Render(" "+op.Text) + "\n")
			lineNumExp++
			lineNumAct++
		case DiffRemove:
			numStr := fmt.Sprintf("%4d", lineNumExp)
			diff.WriteString(style.LineNum.Render(numStr) + " " + style.Removed.Render("-"+op.Text) + "\n")
			lineNumExp++
		case DiffAdd:
			numStr := fmt.Sprintf("%4d", lineNumAct)
			diff.WriteString(style.LineNum.Render(numStr) + " " + style.Added.Render("+"+op.Text) + "\n")
			lineNumAct++
		}
	}

	return diff.String()
}

// SideBySideDiff generates a side-by-side diff view
func SideBySideDiff(expected, actual string, width int) string {
	style := DefaultDiffStyle()
	expectedLines := strings.Split(expected, "\n")
	actualLines := strings.Split(actual, "\n")

	halfWidth := (width - 3) / 2 // 3 for " | " separator

	var diff strings.Builder
	diff.WriteString(style.Header.Render(padRight("Expected (golden)", halfWidth)) + " | " +
		style.Header.Render("Actual (output)") + "\n")
	diff.WriteString(style.Separator.Render(strings.Repeat("-", width)) + "\n")

	ops := computeDiffOps(expectedLines, actualLines)

	i := 0
	for i < len(ops) {
		op := ops[i]
		switch op.Type {
		case DiffEqual:
			left := truncate(op.Text, halfWidth)
			right := truncate(op.Text, halfWidth)
			diff.WriteString(style.Context.Render(padRight(left, halfWidth)) + " | " +
				style.Context.Render(right) + "\n")
			i++
		case DiffRemove:
			left := truncate(op.Text, halfWidth)
			right := ""
			// Check if next op is an add (replacement)
			if i+1 < len(ops) && ops[i+1].Type == DiffAdd {
				right = truncate(ops[i+1].Text, halfWidth)
				i++
			}
			diff.WriteString(style.Removed.Render(padRight(left, halfWidth)) + " | " +
				style.Added.Render(right) + "\n")
			i++
		case DiffAdd:
			left := ""
			right := truncate(op.Text, halfWidth)
			diff.WriteString(style.Context.Render(padRight(left, halfWidth)) + " | " +
				style.Added.Render(right) + "\n")
			i++
		}
	}

	return diff.String()
}

// DiffOpType represents the type of diff operation
type DiffOpType int

const (
	DiffEqual DiffOpType = iota
	DiffAdd
	DiffRemove
)

// DiffOp represents a single diff operation
type DiffOp struct {
	Type DiffOpType
	Text string
}

// computeDiffOps computes diff operations between two line slices
func computeDiffOps(a, b []string) []DiffOp {
	// Simple line-by-line diff using LCS
	lcs := longestCommonSubsequence(a, b)
	var ops []DiffOp

	ai, bi, li := 0, 0, 0
	for li < len(lcs) {
		// Add removes for lines in a not in lcs
		for ai < len(a) && a[ai] != lcs[li] {
			ops = append(ops, DiffOp{Type: DiffRemove, Text: a[ai]})
			ai++
		}
		// Add adds for lines in b not in lcs
		for bi < len(b) && b[bi] != lcs[li] {
			ops = append(ops, DiffOp{Type: DiffAdd, Text: b[bi]})
			bi++
		}
		// Add equal for common line
		ops = append(ops, DiffOp{Type: DiffEqual, Text: lcs[li]})
		ai++
		bi++
		li++
	}
	// Remaining lines
	for ai < len(a) {
		ops = append(ops, DiffOp{Type: DiffRemove, Text: a[ai]})
		ai++
	}
	for bi < len(b) {
		ops = append(ops, DiffOp{Type: DiffAdd, Text: b[bi]})
		bi++
	}

	return ops
}

// longestCommonSubsequence finds the LCS of two string slices
func longestCommonSubsequence(a, b []string) []string {
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				dp[i][j] = max(dp[i-1][j], dp[i][j-1])
			}
		}
	}

	// Backtrack to find the LCS
	lcs := make([]string, dp[m][n])
	i, j := m, n
	for k := dp[m][n] - 1; k >= 0; {
		if a[i-1] == b[j-1] {
			lcs[k] = a[i-1]
			i--
			j--
			k--
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}

	return lcs
}

func padRight(s string, length int) string {
	if len(s) >= length {
		return s[:length]
	}
	return s + strings.Repeat(" ", length-len(s))
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// DiffSummary returns a summary of changes
func DiffSummary(expected, actual string) string {
	expectedLines := strings.Split(expected, "\n")
	actualLines := strings.Split(actual, "\n")
	ops := computeDiffOps(expectedLines, actualLines)

	added, removed := 0, 0
	for _, op := range ops {
		switch op.Type {
		case DiffAdd:
			added++
		case DiffRemove:
			removed++
		}
	}

	style := DefaultDiffStyle()
	summary := fmt.Sprintf("%s, %s",
		style.Added.Render(fmt.Sprintf("+%d lines", added)),
		style.Removed.Render(fmt.Sprintf("-%d lines", removed)))

	return summary
}
