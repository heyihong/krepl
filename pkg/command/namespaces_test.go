package command

import (
	"testing"
	"time"

	"github.com/heyihong/krepl/pkg/repl"
)

func TestFormatAge(t *testing.T) {
	now := time.Now()
	tests := []struct {
		created  time.Time
		expected string
	}{
		{now.Add(-30 * time.Second), "30s"},
		{now.Add(-5 * time.Minute), "5m"},
		{now.Add(-3 * time.Hour), "3h"},
		{now.Add(-2 * 24 * time.Hour), "2d"},
		{now.Add(-400 * 24 * time.Hour), "1y"},
	}
	for _, tt := range tests {
		got := formatAge(tt.created, now)
		if got != tt.expected {
			t.Errorf("formatAge(%v): expected %q, got %q", now.Sub(tt.created), tt.expected, got)
		}
	}
}

func TestFormatAge_FutureTimestamp(t *testing.T) {
	now := time.Now()
	got := formatAge(now.Add(10*time.Second), now)
	if got != "0s" {
		t.Errorf("expected %q for future timestamp, got %q", "0s", got)
	}
}

func TestNamespacesCommand_NoContext(t *testing.T) {
	env := repl.NewEnv(makeTestConfig())
	cmd := newNamespacesCmd()
	err := cmd.Execute(env, nil)
	if err == nil {
		t.Fatal("expected error when no context is set")
	}
}
