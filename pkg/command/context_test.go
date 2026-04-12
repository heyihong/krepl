package command

import (
	"fmt"
	"strings"
	"testing"
)

func TestContextCommand_SwitchPersistsCurrentContext(t *testing.T) {
	env := makeTestEnv()
	cmd := newContextCmd()

	origSet := setKubeconfigCurrentContext
	t.Cleanup(func() {
		setKubeconfigCurrentContext = origSet
	})

	var persisted string
	setKubeconfigCurrentContext = func(name string) error {
		persisted = name
		return nil
	}

	out := captureStdout(t, func() {
		if err := cmd.Execute(env, []string{"ctx-b"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if persisted != "ctx-b" {
		t.Fatalf("expected persisted current context ctx-b, got %q", persisted)
	}
	if env.CurrentContext() != "ctx-b" {
		t.Errorf("expected currentContext %q, got %q", "ctx-b", env.CurrentContext())
	}
	if !strings.Contains(out, "ctx-b") {
		t.Errorf("expected confirmation message, got: %q", out)
	}
}

func TestContextCommand_SwitchPropagatesPersistError(t *testing.T) {
	env := makeTestEnv()
	cmd := newContextCmd()

	origSet := setKubeconfigCurrentContext
	t.Cleanup(func() {
		setKubeconfigCurrentContext = origSet
	})

	setKubeconfigCurrentContext = func(string) error {
		return fmt.Errorf("persist failed")
	}

	err := cmd.Execute(env, []string{"ctx-b"})
	if err == nil || !strings.Contains(err.Error(), "persist failed") {
		t.Fatalf("expected persist error, got %v", err)
	}
	if env.CurrentContext() != "ctx-a" {
		t.Errorf("expected context to remain ctx-a, got %q", env.CurrentContext())
	}
}

func TestDeleteContextCommand_DeletesNonCurrentContext(t *testing.T) {
	env := makeTestEnv()
	cmd := newDeleteContextCmd()

	origDelete := deleteKubeconfigContext
	t.Cleanup(func() {
		deleteKubeconfigContext = origDelete
	})

	var deleted string
	deleteKubeconfigContext = func(name string) error {
		deleted = name
		return nil
	}

	out := captureStdout(t, func() {
		if err := cmd.Execute(env, []string{"ctx-b"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if deleted != "ctx-b" {
		t.Fatalf("expected kubeconfig delete for ctx-b, got %q", deleted)
	}
	if strings.Contains(strings.Join(env.ListContextNames(), ","), "ctx-b") {
		t.Fatalf("expected ctx-b to be removed from env, got %v", env.ListContextNames())
	}
	if !strings.Contains(out, "Deleted context: ctx-b") {
		t.Fatalf("expected confirmation output, got %q", out)
	}
}

func TestDeleteContextCommand_RejectsCurrentContext(t *testing.T) {
	env := makeTestEnv()
	cmd := newDeleteContextCmd()

	origDelete := deleteKubeconfigContext
	t.Cleanup(func() {
		deleteKubeconfigContext = origDelete
	})

	called := false
	deleteKubeconfigContext = func(string) error {
		called = true
		return nil
	}

	err := cmd.Execute(env, []string{"ctx-a"})
	if err == nil || !strings.Contains(err.Error(), `cannot delete current context "ctx-a"`) {
		t.Fatalf("expected current context error, got %v", err)
	}
	if called {
		t.Fatal("expected kubeconfig delete not to be called")
	}
}

func TestDeleteContextCommand_PropagatesDeleteError(t *testing.T) {
	env := makeTestEnv()
	cmd := newDeleteContextCmd()

	origDelete := deleteKubeconfigContext
	t.Cleanup(func() {
		deleteKubeconfigContext = origDelete
	})

	deleteKubeconfigContext = func(string) error {
		return fmt.Errorf("boom")
	}

	err := cmd.Execute(env, []string{"ctx-b"})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected delete error, got %v", err)
	}
	if strings.Contains(strings.Join(env.ListContextNames(), ","), "ctx-b") == false {
		t.Fatalf("expected env to remain unchanged on failure, got %v", env.ListContextNames())
	}
}

func TestContextCommand_Switch(t *testing.T) {
	env := makeTestEnv()
	cmd := newContextCmd()

	origSet := setKubeconfigCurrentContext
	t.Cleanup(func() {
		setKubeconfigCurrentContext = origSet
	})
	setKubeconfigCurrentContext = func(string) error { return nil }

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

	origSet := setKubeconfigCurrentContext
	t.Cleanup(func() {
		setKubeconfigCurrentContext = origSet
	})
	setKubeconfigCurrentContext = func(string) error { return nil }

	err := cmd.Execute(env, []string{"no-such-context"})
	if err == nil {
		t.Fatal("expected error for unknown context, got nil")
	}
	// Context must be unchanged.
	if env.CurrentContext() != "ctx-a" {
		t.Errorf("context should remain ctx-a, got %q", env.CurrentContext())
	}
}

func TestContextCommand_RequiresArgument(t *testing.T) {
	env := makeTestEnv()
	cmd := newContextCmd()

	err := cmd.Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "accepts 1 arg(s), received 0") {
		t.Fatalf("expected missing argument error, got %v", err)
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
