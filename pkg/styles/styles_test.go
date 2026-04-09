package styles

import (
	"strings"
	"testing"

	"github.com/fatih/color"
)

// withColors temporarily enables color output for the duration of fn, then
// restores the previous NoColor value. Used to assert that ANSI escape codes
// are actually emitted when a real terminal is present.
func withColors(fn func()) {
	prev := color.NoColor
	color.NoColor = false
	defer func() { color.NoColor = prev }()
	fn()
}

// ansiEscape is the start byte of every ANSI color sequence.
const ansiEscape = "\x1b["

// ---- PromptContext -----------------------------------------------------------

func TestPromptContext_ContainsText(t *testing.T) {
	if got := PromptContext("my-ctx"); !strings.Contains(got, "my-ctx") {
		t.Errorf("PromptContext: expected output to contain %q, got %q", "my-ctx", got)
	}
}

func TestPromptContext_HasANSIWhenColorsEnabled(t *testing.T) {
	var got string
	withColors(func() { got = PromptContext("my-ctx") })
	if !strings.Contains(got, ansiEscape) {
		t.Errorf("PromptContext: expected ANSI codes when colors enabled, got %q", got)
	}
}

// ---- PromptNamespace --------------------------------------------------------

func TestPromptNamespace_ContainsText(t *testing.T) {
	if got := PromptNamespace("default"); !strings.Contains(got, "default") {
		t.Errorf("PromptNamespace: expected output to contain %q, got %q", "default", got)
	}
}

func TestPromptNamespace_HasANSIWhenColorsEnabled(t *testing.T) {
	var got string
	withColors(func() { got = PromptNamespace("default") })
	if !strings.Contains(got, ansiEscape) {
		t.Errorf("PromptNamespace: expected ANSI codes when colors enabled, got %q", got)
	}
}

// ---- PromptPod --------------------------------------------------------------

func TestPromptPod_None(t *testing.T) {
	if got := PromptPod("none"); !strings.Contains(got, "none") {
		t.Errorf("PromptPod(none): expected output to contain %q, got %q", "none", got)
	}
}

func TestPromptPod_RealPod(t *testing.T) {
	if got := PromptPod("my-pod-abc"); !strings.Contains(got, "my-pod-abc") {
		t.Errorf("PromptPod: expected output to contain %q, got %q", "my-pod-abc", got)
	}
}

// "none" and a real pod name should use different color instances (noneColor vs
// podColor). When colors are enabled both should still carry ANSI sequences.
func TestPromptPod_NoneAndPodBothColoredWhenEnabled(t *testing.T) {
	var gotNone, gotPod string
	withColors(func() {
		gotNone = PromptPod("none")
		gotPod = PromptPod("my-pod")
	})
	if !strings.Contains(gotNone, ansiEscape) {
		t.Errorf("PromptPod(none): expected ANSI codes when colors enabled, got %q", gotNone)
	}
	if !strings.Contains(gotPod, ansiEscape) {
		t.Errorf("PromptPod(real): expected ANSI codes when colors enabled, got %q", gotPod)
	}
}

// ---- ActiveMarker -----------------------------------------------------------

func TestActiveMarker_ContainsText(t *testing.T) {
	if got := ActiveMarker("*"); !strings.Contains(got, "*") {
		t.Errorf("ActiveMarker: expected output to contain %q, got %q", "*", got)
	}
}

func TestActiveMarker_HasANSIWhenColorsEnabled(t *testing.T) {
	var got string
	withColors(func() { got = ActiveMarker("*") })
	if !strings.Contains(got, ansiEscape) {
		t.Errorf("ActiveMarker: expected ANSI codes when colors enabled, got %q", got)
	}
}

func TestRangeSeparator_HasANSIWhenColorsEnabled(t *testing.T) {
	var got string
	withColors(func() { got = RangeSeparator("=== pod-a:default ===") })
	if !strings.Contains(got, "=== pod-a:default ===") {
		t.Fatalf("RangeSeparator: expected original text, got %q", got)
	}
	if !strings.Contains(got, ansiEscape) {
		t.Fatalf("RangeSeparator: expected ANSI codes when colors enabled, got %q", got)
	}
}

// ---- PodPhase ---------------------------------------------------------------

func TestPodPhase_ContainsPhaseString(t *testing.T) {
	phases := []string{"Running", "Succeeded", "Pending", "Failed", "Terminating", "Unknown", "CrashLoopBackOff"}
	for _, phase := range phases {
		got := PodPhase(phase)
		if !strings.Contains(got, phase) {
			t.Errorf("PodPhase(%q): expected output to contain the phase string, got %q", phase, got)
		}
	}
}

// Each distinct phase should produce a different color when colors are enabled.
func TestPodPhase_DistinctColorsForDistinctPhases(t *testing.T) {
	var running, failed, pending string
	withColors(func() {
		running = PodPhase("Running")
		failed = PodPhase("Failed")
		pending = PodPhase("Pending")
	})
	if running == failed {
		t.Errorf("PodPhase: Running and Failed should have different ANSI codes")
	}
	if running == pending {
		t.Errorf("PodPhase: Running and Pending should have different ANSI codes")
	}
	if failed == pending {
		t.Errorf("PodPhase: Failed and Pending should have different ANSI codes")
	}
}

func TestPodPhase_AllHaveANSIWhenColorsEnabled(t *testing.T) {
	phases := []string{"Running", "Succeeded", "Pending", "Failed", "Terminating", "Unknown"}
	for _, phase := range phases {
		var got string
		withColors(func() { got = PodPhase(phase) })
		if !strings.Contains(got, ansiEscape) {
			t.Errorf("PodPhase(%q): expected ANSI codes when colors enabled, got %q", phase, got)
		}
	}
}

// ---- NamespaceStatus --------------------------------------------------------

func TestNamespaceStatus_ContainsStatusString(t *testing.T) {
	statuses := []string{"Active", "Terminating", "Unknown", ""}
	for _, s := range statuses {
		got := NamespaceStatus(s)
		if !strings.Contains(got, s) {
			t.Errorf("NamespaceStatus(%q): expected output to contain the status string, got %q", s, got)
		}
	}
}

// Active and Terminating should get ANSI codes; an unknown status should be returned as-is.
func TestNamespaceStatus_ColoredVsPlain(t *testing.T) {
	var active, terminating, unknown string
	withColors(func() {
		active = NamespaceStatus("Active")
		terminating = NamespaceStatus("Terminating")
		unknown = NamespaceStatus("SomeOtherPhase")
	})
	if !strings.Contains(active, ansiEscape) {
		t.Errorf("NamespaceStatus(Active): expected ANSI codes when colors enabled, got %q", active)
	}
	if !strings.Contains(terminating, ansiEscape) {
		t.Errorf("NamespaceStatus(Terminating): expected ANSI codes when colors enabled, got %q", terminating)
	}
	// Unknown status must not be colored.
	if strings.Contains(unknown, ansiEscape) {
		t.Errorf("NamespaceStatus(unknown): expected no ANSI codes, got %q", unknown)
	}
}

func TestNamespaceStatus_ActiveAndTerminatingDifferentColors(t *testing.T) {
	var active, terminating string
	withColors(func() {
		active = NamespaceStatus("Active")
		terminating = NamespaceStatus("Terminating")
	})
	if active == terminating {
		t.Errorf("NamespaceStatus: Active and Terminating should have different colors")
	}
}
