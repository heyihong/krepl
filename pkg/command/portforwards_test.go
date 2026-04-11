package command

import (
	"errors"
	"io"
	"net/url"
	"strings"
	"testing"

	"k8s.io/client-go/rest"

	"github.com/heyihong/krepl/pkg/portforward"
	"github.com/heyihong/krepl/pkg/repl"
)

type fakePortForwarder struct {
	forward func() error
}

func (f *fakePortForwarder) ForwardPorts() error {
	if f.forward != nil {
		return f.forward()
	}
	return nil
}

func makePortForwardTestEnv(t *testing.T) *repl.Env {
	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{{Kind: repl.KindPod, Name: "pod-0", Namespace: "default"}})
	_ = captureStdout(t, func() {
		_ = env.SelectByIndex(0)
	})
	return env
}

func TestPortForwardCmd_NormalizesPortSpecs(t *testing.T) {
	// Verify that normalizePortForwardSpec handles these cases correctly.
	tests := []struct {
		input string
		want  string
	}{
		{"5000", "5000"},
		{"8080:9090", "8080:9090"},
		{":3456", "0:3456"},
	}
	for _, tt := range tests {
		got, err := normalizePortForwardSpec(tt.input)
		if err != nil {
			t.Fatalf("normalizePortForwardSpec(%q): unexpected error: %v", tt.input, err)
		}
		if got != tt.want {
			t.Fatalf("normalizePortForwardSpec(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPortForwardCmd_RequiresPorts(t *testing.T) {
	// MinimumNArgs(1) is enforced before RunE.
	env := makePortForwardTestEnv(t)
	err := newPortForwardCmd().Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "requires at least 1 arg") {
		t.Fatalf("expected arg count error, got %v", err)
	}
}

func TestNormalizePortForwardSpec_Invalid(t *testing.T) {
	for _, input := range []string{"", "abc", "1:2:3", "1:", "a:80"} {
		if _, err := normalizePortForwardSpec(input); err == nil {
			t.Fatalf("expected %q to fail", input)
		}
	}
}

func TestPortForwardCommand_NoPod(t *testing.T) {
	env := makeTestEnv()
	err := (newPortForwardCmd()).Execute(env, []string{"8080"})
	if err == nil {
		t.Fatal("expected error when no pod is selected")
	}
}

func TestPortForwardCommand_NoContext(t *testing.T) {
	env := makeExecTestEnvNoContext(t)
	env.SetLastObjects([]repl.LastObject{{Kind: repl.KindPod, Name: "pod-0", Namespace: "default"}})
	_ = captureStdout(t, func() {
		_ = env.SelectByIndex(0)
	})

	err := (newPortForwardCmd()).Execute(env, []string{"8080"})
	if err == nil {
		t.Fatal("expected error when no context is active")
	}
}

func TestPortForwardCommand_ReadyStoresSession(t *testing.T) {
	env := makePortForwardTestEnv(t)
	oldNewPortForwarder := newPortForwarder
	newPortForwarder = func(_ *rest.Config, _ *url.URL, ports []string, stopCh <-chan struct{}, readyCh chan struct{}, stdout io.Writer, stderr io.Writer) (portForwarder, error) {
		if strings.Join(ports, ",") != "0:3456" {
			t.Fatalf("expected normalized ports, got %v", ports)
		}
		return &fakePortForwarder{
			forward: func() error {
				_, _ = stdout.Write([]byte("ready output"))
				close(readyCh)
				<-stopCh
				return nil
			},
		}, nil
	}
	defer func() { newPortForwarder = oldNewPortForwarder }()

	out := captureStdout(t, func() {
		if err := (newPortForwardCmd()).Execute(env, []string{":3456"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Forwarding port(s): 0:3456") {
		t.Fatalf("expected success output, got %q", out)
	}
	if len(env.PortForwards()) != 1 {
		t.Fatalf("expected one session, got %d", len(env.PortForwards()))
	}
	session := env.PortForward(0)
	if session == nil || session.Status() != portforward.Running {
		t.Fatalf("expected running session, got %+v", session)
	}
	if !strings.Contains(session.Output(), "ready output") {
		t.Fatalf("expected captured output, got %q", session.Output())
	}
}

func TestPortForwardCommand_StartupErrorDoesNotStoreSession(t *testing.T) {
	env := makePortForwardTestEnv(t)
	oldNewPortForwarder := newPortForwarder
	newPortForwarder = func(_ *rest.Config, _ *url.URL, _ []string, _ <-chan struct{}, _ chan struct{}, _ io.Writer, _ io.Writer) (portForwarder, error) {
		return &fakePortForwarder{forward: func() error { return errors.New("boom") }}, nil
	}
	defer func() { newPortForwarder = oldNewPortForwarder }()

	err := (newPortForwardCmd()).Execute(env, []string{"8080"})
	if err == nil || !strings.Contains(err.Error(), "start port forward") {
		t.Fatalf("expected startup error, got %v", err)
	}
	if got := len(env.PortForwards()); got != 0 {
		t.Fatalf("expected no stored sessions, got %d", got)
	}
}

func TestPortForwardsCommand_EmptyList(t *testing.T) {
	out := captureStdout(t, func() {
		if err := (newPortForwardsCmd()).Execute(makeTestEnv(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "No active port forwards") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestPortForwardsCommand_ListOutputAndStatuses(t *testing.T) {
	env := makeTestEnv()
	running := portforward.NewSession("pod-a", "default", []string{"8080"})
	running.MarkRunning()
	stopped := portforward.NewSession("pod-b", "default", []string{"9090:80"})
	stopped.Stop()
	exited := portforward.NewSession("pod-c", "default", []string{"0:3456"})
	exited.MarkExited(errors.New("bind failed"))
	env.AddPortForward(running)
	env.AddPortForward(stopped)
	env.AddPortForward(exited)

	out := captureStdout(t, func() {
		if err := (newPortForwardsCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"POD", "PORTS", "STATUS", "pod-a", "Running", "Stopped", "Exited: bind failed"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestPortForwardsCommand_Output(t *testing.T) {
	env := makeTestEnv()
	session := portforward.NewSession("pod-a", "default", []string{"8080"})
	session.AppendOutput("forwarded")
	env.AddPortForward(session)

	out := captureStdout(t, func() {
		if err := (newPortForwardsCmd()).Execute(env, []string{"output", "0"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "forwarded") {
		t.Fatalf("expected output contents, got %q", out)
	}
}

func TestPortForwardsCommand_InvalidIndex(t *testing.T) {
	env := makeTestEnv()
	out := captureStdout(t, func() {
		if err := (newPortForwardsCmd()).Execute(env, []string{"output", "2"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "Invalid index") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestPortForwardsCommand_Stop(t *testing.T) {
	env := makeTestEnv()
	session := portforward.NewSession("pod-a", "default", []string{"8080"})
	session.MarkRunning()
	env.AddPortForward(session)

	oldRead := readPortForwardConfirmation
	readPortForwardConfirmation = func() (string, error) { return "yes\n", nil }
	defer func() { readPortForwardConfirmation = oldRead }()

	out := captureStdout(t, func() {
		if err := (newPortForwardsCmd()).Execute(env, []string{"stop", "0"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Stopped") {
		t.Fatalf("expected stop confirmation, got %q", out)
	}
	if session.Status() != portforward.Stopped {
		t.Fatalf("expected session stopped, got %v", session.Status())
	}
}
