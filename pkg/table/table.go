// Package table provides a simple table renderer for terminal output.
// Columns auto-size to the widest header or cell content and always render the
// full cell text on a single line.
// ANSI color codes are handled correctly: padding is based on visible (plain)
// string length, not the byte length after escape codes are injected.
package table

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

const (
	// colSep is the visible separator between columns.
	colSep = "  "
)

// ansiEscapeRe matches ANSI SGR escape sequences (colors, bold, reset, etc.).
var ansiEscapeRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// Column defines one column in a table.
type Column struct {
	// Header is the column header text.
	Header string
	// MinWidth is an optional lower bound on the computed column width.
	// Zero means no minimum beyond the header text width.
	MinWidth int
	// RightAlign controls whether header and cell values are padded on the left.
	RightAlign bool
	// Color, if non-nil, is applied to each cell value in this column.
	// It receives the plain (visible) cell value and returns the colored string.
	Color func(string) string
}

// Table collects rows and renders them using natural column widths.
type Table struct {
	Columns []Column
	// Out is where Render writes. Defaults to os.Stdout when nil.
	Out io.Writer
	// TermWidth is kept for compatibility with callers and tests, but rendering
	// no longer shrinks output to fit the terminal width.
	TermWidth int

	rows [][]string // buffered plain values, one slice per row
}

// AddRow buffers one data row. Values beyond len(Columns) are ignored;
// missing values are treated as empty strings.
func (t *Table) AddRow(values ...string) {
	r := make([]string, len(t.Columns))
	for i := range t.Columns {
		if i < len(values) {
			r[i] = values[i]
		}
	}
	t.rows = append(t.rows, r)
}

// Render computes column widths from buffered content,
// then writes the header, separator, and all data rows to t.Out (os.Stdout by
// default). Render may be called multiple times; each call re-renders from the
// same buffered rows.
func (t *Table) Render() {
	out := t.Out
	if out == nil {
		out = os.Stdout
	}

	ncols := len(t.Columns)
	if ncols == 0 {
		return
	}

	// 1. Compute natural widths (max of header, MinWidth, and all cell content).
	natural := make([]int, ncols)
	for i, col := range t.Columns {
		w := visibleWidth(col.Header)
		if col.MinWidth > w {
			w = col.MinWidth
		}
		natural[i] = w
	}
	for _, row := range t.rows {
		for i, val := range row {
			if i >= ncols {
				break
			}
			plain := stripANSI(val)
			if vw := visibleWidth(plain); vw > natural[i] {
				natural[i] = vw
			}
		}
	}

	finalWidths := natural

	// 2. Render header.
	for i, col := range t.Columns {
		h := col.Header
		pad := finalWidths[i] - visibleWidth(h)
		if i > 0 {
			_, _ = fmt.Fprint(out, colSep)
		}
		if col.RightAlign {
			_, _ = fmt.Fprintf(out, "%s%s", strings.Repeat(" ", pad), h)
			continue
		}
		_, _ = fmt.Fprintf(out, "%-*s", finalWidths[i], h)
	}
	_, _ = fmt.Fprintln(out)

	// 3. Separator line.
	for i, w := range finalWidths {
		if i == 0 {
			_, _ = fmt.Fprint(out, strings.Repeat("-", w))
		} else {
			_, _ = fmt.Fprintf(out, "%s%s", colSep, strings.Repeat("-", w))
		}
	}
	_, _ = fmt.Fprintln(out)

	// 4. Data rows.
	colorFuncs := make([]func(string) string, ncols)
	for i, col := range t.Columns {
		colorFuncs[i] = col.Color
	}
	for _, row := range t.rows {
		for colIdx := range t.Columns {
			val := ""
			if colIdx < len(row) {
				val = stripANSI(row[colIdx])
			}
			colored := val
			if colorFuncs[colIdx] != nil {
				colored = colorFuncs[colIdx](val)
			}
			pad := finalWidths[colIdx] - visibleWidth(val)
			if pad < 0 {
				pad = 0
			}
			spaces := strings.Repeat(" ", pad)
			if colIdx > 0 {
				_, _ = fmt.Fprint(out, colSep)
			}
			if t.Columns[colIdx].RightAlign {
				_, _ = fmt.Fprintf(out, "%s%s", spaces, colored)
			} else {
				_, _ = fmt.Fprintf(out, "%s%s", colored, spaces)
			}
		}
		_, _ = fmt.Fprintln(out)
	}
}

// visibleWidth returns the number of visible terminal columns occupied by s,
// ignoring ANSI SGR escape sequences.
func visibleWidth(s string) int {
	plain := ansiEscapeRe.ReplaceAllString(s, "")
	return len([]rune(plain))
}

// stripANSI removes ANSI SGR escape sequences from s.
func stripANSI(s string) string {
	return ansiEscapeRe.ReplaceAllString(s, "")
}
