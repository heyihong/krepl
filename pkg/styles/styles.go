// Package styles provides semantic color functions for krepl output.
//
// When stdout is not a terminal (piping, redirecting, or NO_COLOR env var set),
// color.NoColor is expected to be true and these functions return plain strings.
package styles

import (
	"strings"

	"github.com/fatih/color"
)

const (
	ansiReset = "\x1b[0m"
	ansiStart = "\x1b["

	codeBold   = "1"
	codeRed    = "31"
	codeGreen  = "32"
	codeYellow = "33"
	codeBlue   = "34"
	codeGray   = "90"
)

func style(text string, codes ...string) string {
	if color.NoColor {
		return text
	}
	return ansiStart + strings.Join(codes, ";") + "m" + text + ansiReset
}

// PromptContext colors the context segment of the REPL prompt (red+bold).
func PromptContext(s string) string { return style(s, codeRed, codeBold) }

// PromptNamespace colors the namespace segment of the REPL prompt (green+bold).
func PromptNamespace(s string) string { return style(s, codeGreen, codeBold) }

// PromptSelection colors the selection segment of the REPL prompt. An actual
// selected object or range label is shown in yellow+bold; the placeholder
// "none" is shown in plain yellow.
func PromptSelection(s string) string {
	if s == "none" {
		return style(s, codeYellow)
	}
	return style(s, codeYellow, codeBold)
}

// PromptPod is kept as a compatibility alias for existing tests and callers.
func PromptPod(s string) string { return PromptSelection(s) }

// ActiveMarker colors a marker string (e.g. "*") in red, matching Click's
// context_table_color which uses Color::Red.
func ActiveMarker(s string) string { return style(s, codeRed) }

// RangeSeparator colors the per-object header printed for range operations.
func RangeSeparator(s string) string { return style(s, codeBlue, codeBold) }

// PodPhase colors a pod phase string to indicate health at a glance.
func PodPhase(phase string) string {
	switch phase {
	case "Running", "Succeeded":
		return style(phase, codeGreen)
	case "Pending", "ContainerCreating":
		return style(phase, codeYellow)
	case "Failed", "Terminating", "Unknown":
		return style(phase, codeRed)
	default:
		return style(phase, codeGray)
	}
}

// NamespaceStatus colors a namespace phase string.
func NamespaceStatus(status string) string {
	switch status {
	case "Active":
		return style(status, codeGreen)
	case "Terminating":
		return style(status, codeRed)
	default:
		return status
	}
}

// NodeStatus colors a node readiness string.
func NodeStatus(status string) string {
	switch {
	case strings.HasPrefix(status, "Ready"):
		return style(status, codeGreen)
	case strings.HasPrefix(status, "NotReady"):
		return style(status, codeRed)
	default:
		return style(status, codeYellow)
	}
}
