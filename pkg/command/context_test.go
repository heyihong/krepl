package command

import (
	"fmt"
	"strings"
	"testing"
)

func TestContextCommand_List(t *testing.T) {
	env := makeTestEnv() // currentContext = "ctx-a"
	cmd := newContextCmd()

	out := captureStdout(t, func() {
		if err := cmd.Execute(env, nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "* ctx-a") {
		t.Errorf("expected active context marked with *, got:\n%s", out)
	}
	if !strings.Contains(out, "  ctx-b") {
		t.Errorf("expected ctx-b listed without *, got:\n%s", out)
	}
}

func TestContextCommand_Switch(t *testing.T) {
	env := makeTestEnv()
	cmd := newContextCmd()

	out := captureStdout(t, func() {
		if err := cmd.Execute(env, []string{"ctx-b"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if env.CurrentContext() != "ctx-b" {
		t.Errorf("expected currentContext %q, got %q", "ctx-b", env.CurrentContext())
	}
	if !strings.Contains(out, "ctx-b") {
		t.Errorf("expected confirmation message, got: %q", out)
	}
}

func TestContextCommand_SwitchInvalid(t *testing.T) {
	env := makeTestEnv()
	cmd := newContextCmd()
	err := cmd.Execute(env, []string{"no-such-context"})
	if err == nil {
		t.Fatal("expected error for unknown context, got nil")
	}
	// Context must be unchanged.
	if env.CurrentContext() != "ctx-a" {
		t.Errorf("context should remain ctx-a, got %q", env.CurrentContext())
	}
}

func TestContextCommand_ListAfterSwitch(t *testing.T) {
	env := makeTestEnv()
	cmd := newContextCmd()

	// Switch to ctx-b first.
	_ = cmd.Execute(env, []string{"ctx-b"})

	out := captureStdout(t, func() {
		_ = cmd.Execute(env, nil)
	})

	if !strings.Contains(out, "* ctx-b") {
		t.Errorf("expected ctx-b to be marked active after switch:\n%s", out)
	}
	// ctx-a should now be listed without *.
	if strings.Contains(out, fmt.Sprintf("* %s", "ctx-a")) {
		t.Errorf("ctx-a should not be marked active after switching to ctx-b:\n%s", out)
	}
}

func TestContextsCommand_List(t *testing.T) {
	env := makeTestEnv() // currentContext = "ctx-a"
	cmd := newContextsCmd()

	out := captureStdout(t, func() {
		if err := cmd.Execute(env, nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "* ctx-a") {
		t.Errorf("expected active context marked with *, got:\n%s", out)
	}
	if !strings.Contains(out, "  ctx-b") {
		t.Errorf("expected ctx-b listed without *, got:\n%s", out)
	}
}
