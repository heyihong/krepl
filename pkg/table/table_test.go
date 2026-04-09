package table

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fatih/color"
)

// withColors temporarily enables color output for the duration of fn.
func withColors(fn func()) {
	prev := color.NoColor
	color.NoColor = false
	defer func() { color.NoColor = prev }()
	fn()
}

// newTestTable returns a Table wired to buf with a fixed terminal width,
// so tests are independent of the actual terminal.
func newTestTable(buf *bytes.Buffer, termWidth int, cols ...Column) *Table {
	return &Table{Columns: cols, Out: buf, TermWidth: termWidth}
}

// ---- visibleWidth -----------------------------------------------------------

func TestVisibleWidth_PlainString(t *testing.T) {
	if got := visibleWidth("hello"); got != 5 {
		t.Errorf("visibleWidth: want 5, got %d", got)
	}
}

func TestVisibleWidth_WithANSI(t *testing.T) {
	// "\x1b[32mOK\x1b[0m" — green "OK", 2 visible chars
	s := "\x1b[32mOK\x1b[0m"
	if got := visibleWidth(s); got != 2 {
		t.Errorf("visibleWidth with ANSI: want 2, got %d", got)
	}
}

func TestVisibleWidth_Empty(t *testing.T) {
	if got := visibleWidth(""); got != 0 {
		t.Errorf("visibleWidth empty: want 0, got %d", got)
	}
}

// ---- Render: header + separator ---------------------------------------------

func TestRender_HeaderContainsAllColumns(t *testing.T) {
	var buf bytes.Buffer
	tbl := newTestTable(&buf, 80,
		Column{Header: "#"},
		Column{Header: "NAME"},
		Column{Header: "STATUS"},
	)
	tbl.Render()
	out := buf.String()
	for _, h := range []string{"#", "NAME", "STATUS"} {
		if !strings.Contains(out, h) {
			t.Errorf("Render header: expected %q in output, got:\n%s", h, out)
		}
	}
}

func TestRender_SeparatorPresent(t *testing.T) {
	var buf bytes.Buffer
	tbl := newTestTable(&buf, 40,
		Column{Header: "#"},
		Column{Header: "NAME"},
	)
	tbl.Render()
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("Render: expected at least header + separator, got %d lines", len(lines))
	}
	sep := lines[1]
	if !strings.Contains(sep, "-") {
		t.Errorf("Render: separator line has no dashes: %q", sep)
	}
}

// ---- Render: data rows ------------------------------------------------------

func TestRender_ValuesAppearInOutput(t *testing.T) {
	var buf bytes.Buffer
	tbl := newTestTable(&buf, 80,
		Column{Header: "#"},
		Column{Header: "NAME"},
		Column{Header: "STATUS"},
	)
	tbl.AddRow("0", "my-pod", "Running")
	tbl.Render()
	out := buf.String()
	for _, want := range []string{"0", "my-pod", "Running"} {
		if !strings.Contains(out, want) {
			t.Errorf("Render row: expected %q in output", want)
		}
	}
}

func TestRender_RightAlignedColumn(t *testing.T) {
	var buf bytes.Buffer
	tbl := newTestTable(&buf, 80,
		Column{Header: "#", MinWidth: 4, RightAlign: true},
		Column{Header: "NAME"},
	)
	tbl.AddRow("0", "pod")
	tbl.Render()
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	// Data row is lines[2] (after header and separator).
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}
	dataLine := lines[2]
	// A single-char index in a ≥4-wide column should be preceded by spaces.
	if !strings.HasPrefix(dataLine, "   0") {
		t.Errorf("right-aligned column: expected leading spaces before '0', got %q", dataLine)
	}
}

func TestRender_FirstColumnNotRightAlignedByDefault(t *testing.T) {
	var buf bytes.Buffer
	tbl := newTestTable(&buf, 80,
		Column{Header: "#", MinWidth: 4},
		Column{Header: "NAME"},
	)
	tbl.AddRow("0", "pod")
	tbl.Render()
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}
	dataLine := lines[2]
	if !strings.HasPrefix(dataLine, "0") {
		t.Errorf("default alignment: expected first column to be left-aligned, got %q", dataLine)
	}
}

func TestRender_ColumnsSeparatedByTwoSpaces(t *testing.T) {
	var buf bytes.Buffer
	tbl := newTestTable(&buf, 80,
		Column{Header: "#", MinWidth: 4},
		Column{Header: "A"},
		Column{Header: "B"},
	)
	tbl.AddRow("1", "hello", "world")
	tbl.Render()
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	dataLine := lines[2]
	if !strings.Contains(dataLine, "  hello") {
		t.Errorf("two-space separator missing before 'hello': %q", dataLine)
	}
	if !strings.Contains(dataLine, "  world") {
		t.Errorf("two-space separator missing before 'world': %q", dataLine)
	}
}

func TestRender_LongCellRendersInFull(t *testing.T) {
	var buf bytes.Buffer
	tbl := newTestTable(&buf, 40,
		Column{Header: "#"},
		Column{Header: "NAME"},
		Column{Header: "STATUS"},
	)
	tbl.AddRow("0", "my-pod", "Running")
	tbl.AddRow("1", "a-very-very-very-long-pod-name", "Pending")
	tbl.Render()

	for _, line := range strings.Split(buf.String(), "\n") {
		if line == "" {
			continue
		}
		if strings.Contains(line, "a-very-very-very-long-pod-name") {
			return
		}
	}
	t.Fatalf("expected full long value in output, got:\n%s", buf.String())
}

func TestRender_DynamicWidth_NaturalFit(t *testing.T) {
	// With a wide terminal, columns should use natural (content) widths.
	var buf bytes.Buffer
	tbl := newTestTable(&buf, 200,
		Column{Header: "#"},
		Column{Header: "NAME"},
	)
	tbl.AddRow("0", "my-pod")
	tbl.Render()
	out := buf.String()
	// Full name should appear without truncation or wrapping.
	if !strings.Contains(out, "my-pod") {
		t.Errorf("Render natural fit: expected full name in output")
	}
}

// ---- Render: color ----------------------------------------------------------

func TestRender_ColorFuncApplied(t *testing.T) {
	called := false
	var buf bytes.Buffer
	tbl := newTestTable(&buf, 80,
		Column{Header: "#", MinWidth: 4, RightAlign: true},
		Column{Header: "STATUS", Color: func(s string) string {
			called = true
			return "[" + s + "]"
		}},
	)
	tbl.AddRow("0", "Running")
	tbl.Render()
	if !called {
		t.Error("Color function was not called")
	}
	if !strings.Contains(buf.String(), "[Running]") {
		t.Errorf("Color-transformed value not in output: %q", buf.String())
	}
}

func TestRender_ColorDoesNotBreakAlignment(t *testing.T) {
	var buf bytes.Buffer
	tbl := &Table{
		Columns: []Column{
			{Header: "#", MinWidth: 4, RightAlign: true},
			{Header: "STATUS", Color: func(s string) string {
				return color.New(color.FgGreen).Sprint(s)
			}},
		},
		Out:       &buf,
		TermWidth: 80,
	}

	withColors(func() {
		tbl.AddRow("0", "Running")
		tbl.Render()
	})

	// Strip ANSI from output; visible layout must still be aligned.
	stripped := stripANSI(buf.String())
	lines := strings.Split(strings.TrimRight(stripped, "\n"), "\n")
	// data row is lines[2]
	if len(lines) < 3 {
		t.Fatalf("expected ≥3 lines, got %d", len(lines))
	}
	dataLine := lines[2]
	// Index "0" should be right-aligned in a ≥4-wide column, "Running" follows.
	if !strings.HasPrefix(dataLine, "   0") {
		t.Errorf("alignment broken by color: %q", dataLine)
	}
	if !strings.Contains(dataLine, "Running") {
		t.Errorf("Running missing from output: %q", dataLine)
	}
}

// ---- Render: multiple rows --------------------------------------------------

func TestRender_MultipleRows(t *testing.T) {
	var buf bytes.Buffer
	tbl := newTestTable(&buf, 80,
		Column{Header: "#"},
		Column{Header: "NAME"},
	)
	tbl.AddRow("0", "alpha")
	tbl.AddRow("1", "beta")
	tbl.AddRow("2", "gamma")
	tbl.Render()

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	// header + separator + 3 data rows = 5 lines minimum
	if len(lines) < 5 {
		t.Fatalf("expected ≥5 lines, got %d:\n%s", len(lines), buf.String())
	}
	for _, want := range []string{"alpha", "beta", "gamma"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("expected %q in output", want)
		}
	}
}
