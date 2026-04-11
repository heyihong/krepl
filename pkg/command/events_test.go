package command

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/heyihong/krepl/pkg/repl"
)

func TestEventsCommand_NoContext(t *testing.T) {
	env := makeExecTestEnvNoContext(t)
	cmd := newEventsCmd()
	if err := cmd.Execute(env, nil); err == nil {
		t.Fatal("expected error when no context is active")
	}
}

func TestEventsCommand_NoObjectListsNamespaceEvents(t *testing.T) {
	env := makeTestEnv()
	oldFetch := fetchEventsForNamespace
	fetchEventsForNamespace = func(_ context.Context, _ clientcmdapi.Config, contextName, namespace string) ([]corev1.Event, error) {
		if contextName != env.CurrentContext() {
			t.Fatalf("unexpected context %q", contextName)
		}
		// empty namespace means NamespaceAll
		return []corev1.Event{
			makeTestEvent("Pod", "pod-a", "Pulled", "Normal", "image pulled", time.Now().Add(-5*time.Minute)),
			makeTestEvent("Deployment", "dep-a", "ScalingReplicaSet", "Normal", "scaled up", time.Now().Add(-2*time.Minute)),
		}, nil
	}
	defer func() { fetchEventsForNamespace = oldFetch }()

	out := captureStdout(t, func() {
		if err := (newEventsCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"LAST SEEN", "TYPE", "REASON", "OBJECT", "MESSAGE", "Pulled", "Pod/pod-a", "ScalingReplicaSet", "Deployment/dep-a"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestEventsCommand_NoObjectNamespaceEventsAreSortedChronologically(t *testing.T) {
	env := makeTestEnv()
	oldFetch := fetchEventsForNamespace
	fetchEventsForNamespace = func(_ context.Context, _ clientcmdapi.Config, _, _ string) ([]corev1.Event, error) {
		return []corev1.Event{
			makeTestEvent("Pod", "pod-a", "Later", "Normal", "second event", time.Now().Add(-1*time.Minute)),
			makeTestEvent("Pod", "pod-a", "Earlier", "Normal", "first event", time.Now().Add(-10*time.Minute)),
		}, nil
	}
	defer func() { fetchEventsForNamespace = oldFetch }()

	out := captureStdout(t, func() {
		if err := (newEventsCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if strings.Index(out, "Earlier") > strings.Index(out, "Later") {
		t.Fatalf("expected events sorted chronologically (Earlier before Later), got:\n%s", out)
	}
}

func TestEventsCommand_NoObjectPropagatesErrors(t *testing.T) {
	env := makeTestEnv()
	oldFetch := fetchEventsForNamespace
	fetchEventsForNamespace = func(_ context.Context, _ clientcmdapi.Config, _, _ string) ([]corev1.Event, error) {
		return nil, errors.New("boom")
	}
	defer func() { fetchEventsForNamespace = oldFetch }()

	err := (newEventsCmd()).Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "list events") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestEventsCommand_WithObjectListsScopedEvents(t *testing.T) {
	env := makeExecTestEnv(t) // has pod-0 selected
	oldFetch := fetchDescribeEvents
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, obj repl.LastObject) ([]corev1.Event, error) {
		if obj.Kind != repl.KindPod || obj.Name != "pod-0" {
			t.Fatalf("expected events for pod-0, got %+v", obj)
		}
		return []corev1.Event{
			makeTestEvent("Pod", "pod-0", "Scheduled", "Normal", "pod scheduled", time.Now().Add(-3*time.Minute)),
			makeTestEvent("Pod", "pod-0", "Pulled", "Normal", "image pulled", time.Now().Add(-2*time.Minute)),
		}, nil
	}
	defer func() { fetchDescribeEvents = oldFetch }()

	out := captureStdout(t, func() {
		if err := (newEventsCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Object column should NOT appear when scoped to a specific object
	if strings.Contains(out, "OBJECT") {
		t.Fatalf("OBJECT column should not appear when object is selected, got:\n%s", out)
	}
	for _, want := range []string{"LAST SEEN", "TYPE", "REASON", "MESSAGE", "Scheduled", "Pulled"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestEventsCommand_WithObjectSortedChronologically(t *testing.T) {
	env := makeExecTestEnv(t)
	oldFetch := fetchDescribeEvents
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return []corev1.Event{
			makeTestEvent("Pod", "pod-0", "Later", "Normal", "second", time.Now().Add(-1*time.Minute)),
			makeTestEvent("Pod", "pod-0", "Earlier", "Warning", "first", time.Now().Add(-10*time.Minute)),
		}, nil
	}
	defer func() { fetchDescribeEvents = oldFetch }()

	out := captureStdout(t, func() {
		if err := (newEventsCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if strings.Index(out, "Earlier") > strings.Index(out, "Later") {
		t.Fatalf("expected events sorted chronologically, got:\n%s", out)
	}
}

func TestEventsCommand_WithObjectPropagatesErrors(t *testing.T) {
	env := makeExecTestEnv(t)
	oldFetch := fetchDescribeEvents
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, errors.New("boom")
	}
	defer func() { fetchDescribeEvents = oldFetch }()

	err := (newEventsCmd()).Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "list events") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func makeTestEvent(kind, name, reason, eventType, message string, ts time.Time) corev1.Event {
	return corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Namespace: "default"},
		InvolvedObject: corev1.ObjectReference{Kind: kind, Name: name, Namespace: "default"},
		Reason:         reason,
		Type:           eventType,
		Message:        message,
		LastTimestamp:  metav1.NewTime(ts),
	}
}
