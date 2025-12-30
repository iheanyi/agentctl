package output

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Writer handles formatted output (inspired by gh CLI's iostreams)
type Writer struct {
	Out    io.Writer
	Err    io.Writer
	IsaTTY bool
}

// DefaultWriter creates a writer for stdout/stderr
func DefaultWriter() *Writer {
	return &Writer{
		Out:    os.Stdout,
		Err:    os.Stderr,
		IsaTTY: true, // TODO: detect properly
	}
}

// Success prints a success message with a checkmark
func (w *Writer) Success(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(w.Out, "✓ %s\n", msg)
}

// Error prints an error message with an X
func (w *Writer) Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(w.Err, "✗ %s\n", msg)
}

// Info prints an info message
func (w *Writer) Info(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(w.Out, "• %s\n", msg)
}

// Warning prints a warning message
func (w *Writer) Warning(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(w.Out, "⚠ %s\n", msg)
}

// Print prints a message without prefix
func (w *Writer) Print(format string, args ...interface{}) {
	fmt.Fprintf(w.Out, format, args...)
}

// Println prints a message with newline
func (w *Writer) Println(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintln(w.Out, msg)
}

// Table represents a simple table for output
type Table struct {
	Headers []string
	Rows    [][]string
	Out     io.Writer
}

// NewTable creates a new table
func NewTable(headers ...string) *Table {
	return &Table{
		Headers: headers,
		Rows:    [][]string{},
		Out:     os.Stdout,
	}
}

// AddRow adds a row to the table
func (t *Table) AddRow(cells ...string) {
	t.Rows = append(t.Rows, cells)
}

// Render outputs the table
func (t *Table) Render() {
	if len(t.Rows) == 0 {
		return
	}

	// Calculate column widths
	widths := make([]int, len(t.Headers))
	for i, h := range t.Headers {
		widths[i] = len(h)
	}
	for _, row := range t.Rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print header
	var headerParts []string
	for i, h := range t.Headers {
		headerParts = append(headerParts, fmt.Sprintf("%-*s", widths[i], strings.ToUpper(h)))
	}
	fmt.Fprintln(t.Out, strings.Join(headerParts, "  "))

	// Print rows
	for _, row := range t.Rows {
		var rowParts []string
		for i, cell := range row {
			if i < len(widths) {
				rowParts = append(rowParts, fmt.Sprintf("%-*s", widths[i], cell))
			}
		}
		fmt.Fprintln(t.Out, strings.Join(rowParts, "  "))
	}
}

// List prints a bulleted list
func (w *Writer) List(items []string) {
	for _, item := range items {
		fmt.Fprintf(w.Out, "  • %s\n", item)
	}
}

// ListWithMarker prints a list with a custom marker for one item
func (w *Writer) ListWithMarker(items []string, markerIndex int, marker string) {
	for i, item := range items {
		if i == markerIndex {
			fmt.Fprintf(w.Out, "  %s %s\n", marker, item)
		} else {
			fmt.Fprintf(w.Out, "    %s\n", item)
		}
	}
}
