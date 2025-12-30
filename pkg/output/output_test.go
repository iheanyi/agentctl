package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriter(t *testing.T) {
	var buf bytes.Buffer
	w := &Writer{Out: &buf, Err: &buf}

	w.Success("done")
	if !strings.Contains(buf.String(), "✓ done") {
		t.Errorf("Success output mismatch: %q", buf.String())
	}

	buf.Reset()
	w.Error("failed")
	if !strings.Contains(buf.String(), "✗ failed") {
		t.Errorf("Error output mismatch: %q", buf.String())
	}

	buf.Reset()
	w.Info("note")
	if !strings.Contains(buf.String(), "• note") {
		t.Errorf("Info output mismatch: %q", buf.String())
	}

	buf.Reset()
	w.Warning("warning")
	if !strings.Contains(buf.String(), "⚠ warning") {
		t.Errorf("Warning output mismatch: %q", buf.String())
	}
}

func TestWriterFormat(t *testing.T) {
	var buf bytes.Buffer
	w := &Writer{Out: &buf}

	w.Success("completed %d tasks", 5)
	if !strings.Contains(buf.String(), "completed 5 tasks") {
		t.Errorf("Format mismatch: %q", buf.String())
	}
}

func TestPrint(t *testing.T) {
	var buf bytes.Buffer
	w := &Writer{Out: &buf}

	w.Print("hello %s", "world")
	if buf.String() != "hello world" {
		t.Errorf("Print mismatch: %q", buf.String())
	}
}

func TestPrintln(t *testing.T) {
	var buf bytes.Buffer
	w := &Writer{Out: &buf}

	w.Println("hello")
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Error("Println should end with newline")
	}
}

func TestTable(t *testing.T) {
	var buf bytes.Buffer
	table := &Table{
		Headers: []string{"Name", "Status"},
		Out:     &buf,
	}

	table.AddRow("server1", "active")
	table.AddRow("server2", "disabled")
	table.Render()

	output := buf.String()
	if !strings.Contains(output, "NAME") {
		t.Error("Table should contain NAME header")
	}
	if !strings.Contains(output, "STATUS") {
		t.Error("Table should contain STATUS header")
	}
	if !strings.Contains(output, "server1") {
		t.Error("Table should contain server1")
	}
	if !strings.Contains(output, "active") {
		t.Error("Table should contain active")
	}
}

func TestTableEmpty(t *testing.T) {
	var buf bytes.Buffer
	table := &Table{
		Headers: []string{"Name"},
		Out:     &buf,
	}

	table.Render()
	if buf.Len() != 0 {
		t.Error("Empty table should not render anything")
	}
}

func TestNewTable(t *testing.T) {
	table := NewTable("Col1", "Col2", "Col3")
	if len(table.Headers) != 3 {
		t.Errorf("Expected 3 headers, got %d", len(table.Headers))
	}
}

func TestList(t *testing.T) {
	var buf bytes.Buffer
	w := &Writer{Out: &buf}

	w.List([]string{"item1", "item2"})
	output := buf.String()
	if !strings.Contains(output, "• item1") {
		t.Error("List should contain item1")
	}
	if !strings.Contains(output, "• item2") {
		t.Error("List should contain item2")
	}
}

func TestDefaultWriter(t *testing.T) {
	w := DefaultWriter()
	if w.Out == nil {
		t.Error("DefaultWriter should have non-nil Out")
	}
	if w.Err == nil {
		t.Error("DefaultWriter should have non-nil Err")
	}
}
