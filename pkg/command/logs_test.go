package command

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/heyihong/krepl/pkg/repl"

	corev1 "k8s.io/api/core/v1"
)

func TestFindCommand_LogsAliasRegistered(t *testing.T) {
	cmds := BuildCommands()
	cmd := repl.FindCommand(cmds, "l")
	if cmd == nil {
		t.Fatal("expected to find 'l' alias for logs command")
	}
	if cmd.Name() != "logs" {
		t.Fatalf("expected logs command for alias, got %q", cmd.Name())
	}
}

func TestFindCommand_LogsLegacyAliasRemoved(t *testing.T) {
	cmds := BuildCommands()
	cmd := repl.FindCommand(cmds, "log")
	if cmd != nil {
		t.Fatalf("expected 'log' alias to be removed, got %q", cmd.Name())
	}
}

// Logs flag tests use Execute with a bare env (no active pod) so they can
// exercise flag parsing and mutual-exclusivity checks without real K8s calls.
// Flag parse errors (pflag-level) are returned before RunE is invoked.
// Business-logic errors (--editor + --follow conflict) are checked at the top
// of RunE, before the "no active pod" guard, so they surface cleanly.

func TestLogsCmd_UnknownFlag(t *testing.T) {
	env := makeTestEnv()
	err := newLogsCmd().Execute(env, []string{"--unknown"})
	if err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("expected unknown flag error, got %v", err)
	}
}

func TestLogsCmd_MissingEditorValue(t *testing.T) {
	env := makeTestEnv()
	err := newLogsCmd().Execute(env, []string{"--editor"})
	if err == nil {
		t.Fatalf("expected error for missing --editor value, got nil")
	}
}

func TestLogsCmd_EditorAndFollowConflict(t *testing.T) {
	env := makeTestEnv()
	err := newLogsCmd().Execute(env, []string{"--follow", "--editor", "vim"})
	if err == nil || !strings.Contains(err.Error(), "--editor cannot be used with --follow") {
		t.Fatalf("expected conflict error, got %v", err)
	}
}

func TestLogsCmd_EditorAndFollowConflict_ReverseOrder(t *testing.T) {
	env := makeTestEnv()
	err := newLogsCmd().Execute(env, []string{"--editor", "vim", "--follow"})
	if err == nil || !strings.Contains(err.Error(), "--editor cannot be used with --follow") {
		t.Fatalf("expected conflict error, got %v", err)
	}
}

func TestLogsCmd_NoPod(t *testing.T) {
	env := makeTestEnv()
	// Valid flags, but no pod selected → should get the "no active pod" error from RunE.
	err := newLogsCmd().Execute(env, []string{"-f"})
	if err == nil || !strings.Contains(err.Error(), "no active pod") {
		t.Fatalf("expected no active pod error, got %v", err)
	}
}

func TestLogsCmd_TooManyPositional(t *testing.T) {
	env := makeTestEnv()
	// MaximumNArgs(1) is enforced before RunE.
	err := newLogsCmd().Execute(env, []string{"container-a", "container-b"})
	if err == nil || !strings.Contains(err.Error(), "accepts at most 1 arg") {
		t.Fatalf("expected too many args error, got %v", err)
	}
}

func TestLogsCmd_Help(t *testing.T) {
	env := makeTestEnv()
	err := newLogsCmd().Execute(env, []string{"--help"})
	if err != nil {
		t.Fatalf("--help should not return an error, got %v", err)
	}
}

func TestLogsCmd_UsageString(t *testing.T) {
	cmd := newLogsCmd()
	usage := cmd.usageString()
	for _, want := range []string{"Usage:", "logs", "--follow", "--tail", "--editor", "--previous"} {
		if !strings.Contains(usage, want) {
			t.Errorf("usageString() missing %q\nfull output:\n%s", want, usage)
		}
	}
}

func TestLogsCmd_RangeFollowRejected(t *testing.T) {
	env := makeTestEnv()
	env.SetRange([]repl.LastObject{
		{Kind: repl.KindPod, Name: "pod-a", Namespace: "default"},
		{Kind: repl.KindPod, Name: "pod-b", Namespace: "default"},
	})
	err := newLogsCmd().Execute(env, []string{"--follow"})
	if err == nil || !strings.Contains(err.Error(), "range selection is not supported with --follow") {
		t.Fatalf("expected range follow rejection, got %v", err)
	}
}

func TestLogsCmd_RangeIteratesPods(t *testing.T) {
	env := makeTestEnv()
	env.SetRange([]repl.LastObject{
		{Kind: repl.KindPod, Name: "pod-a", Namespace: "default"},
		{Kind: repl.KindPod, Name: "pod-b", Namespace: "default"},
	})

	var seen []string
	oldFn := runLogsForObject
	runLogsForObject = func(env *repl.Env, obj repl.LastObject, podOpts *corev1.PodLogOptions, resolvedEditor string) error {
		seen = append(seen, obj.Name)
		return nil
	}
	t.Cleanup(func() { runLogsForObject = oldFn })

	out := captureStdout(t, func() {
		if err := newLogsCmd().Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if strings.Join(seen, ",") != "pod-a,pod-b" {
		t.Fatalf("unexpected objects: %v", seen)
	}
	if !strings.Contains(out, "--- pod-a ---") || !strings.Contains(out, "--- pod-b ---") {
		t.Fatalf("expected range separators, got %q", out)
	}
}

func TestLogsCmd_RangeWithNonPodRejectedImmediately(t *testing.T) {
	env := makeTestEnv()
	env.SetRange([]repl.LastObject{
		{Kind: repl.KindPod, Name: "pod-a", Namespace: "default"},
		{Kind: repl.KindDeployment, Name: "deploy-a", Namespace: "default"},
	})

	called := false
	oldFn := runLogsForObject
	runLogsForObject = func(env *repl.Env, obj repl.LastObject, podOpts *corev1.PodLogOptions, resolvedEditor string) error {
		called = true
		return nil
	}
	t.Cleanup(func() { runLogsForObject = oldFn })

	err := newLogsCmd().Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "logs requires a pod selection") {
		t.Fatalf("expected non-pod selection rejection, got %v", err)
	}
	if called {
		t.Fatal("expected logs execution to stop before iterating selection")
	}
}

func TestLogsStreamContext_NonFollowHasTimeout(t *testing.T) {
	ctx, cancel := logsStreamContext(false)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected non-follow logs context to have a deadline")
	}
	if time.Until(deadline) <= 0 {
		t.Fatal("expected non-follow logs context deadline to be in the future")
	}
}

func TestLogsStreamContext_FollowUsesInterruptContext(t *testing.T) {
	oldFactory := newFollowLogsContext
	defer func() { newFollowLogsContext = oldFactory }()

	called := false
	newFollowLogsContext = func() (context.Context, context.CancelFunc) {
		called = true
		return context.WithCancel(context.Background())
	}

	ctx, cancel := logsStreamContext(true)
	defer cancel()

	if !called {
		t.Fatal("expected follow logs context to use the interrupt-aware factory")
	}
	if _, ok := ctx.Deadline(); ok {
		t.Fatal("expected follow logs context to not have a timeout deadline")
	}
}

func TestLogsStreamContext_FollowCanceledByInterrupt(t *testing.T) {
	ctx, cancel := newFollowLogsContext()
	defer cancel()

	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("find process: %v", err)
	}
	if err := proc.Signal(os.Interrupt); err != nil {
		t.Fatalf("send interrupt: %v", err)
	}

	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("expected interrupt-aware follow context to be canceled by Ctrl-C")
	}

	if ctx.Err() != context.Canceled {
		t.Fatalf("expected follow context cancellation after interrupt, got %v", ctx.Err())
	}
}
