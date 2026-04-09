package command

import (
	"context"
	"errors"
	"github.com/heyihong/krepl/pkg/repl"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestDaemonSetsCommand_NoContext(t *testing.T) {
	env := makeExecTestEnvNoContext(t)
	cmd := newDaemonSetsCmd()
	if err := cmd.Execute(env, nil); err == nil {
		t.Fatal("expected error when no context is active")
	}
}

func TestDaemonSetsCommand_PrintsTableAndSetsSelectableObjects(t *testing.T) {
	env := makeTestEnv()
	oldList := listDaemonSetsForContext
	listDaemonSetsForContext = func(_ context.Context, _ clientcmdapi.Config, contextName, _ string) ([]appsv1.DaemonSet, error) {
		if contextName != env.CurrentContext() {
			t.Fatalf("unexpected context %q", contextName)
		}
		return []appsv1.DaemonSet{
			makeTestDaemonSet("ds-a", "kube-system", 3, 3, 3, 3, 3),
			makeTestDaemonSet("ds-b", "kube-system", 5, 4, 3, 4, 4),
		}, nil
	}
	defer func() { listDaemonSetsForContext = oldList }()

	out := captureStdout(t, func() {
		if err := (newDaemonSetsCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"NAME", "DESIRED", "CURRENT", "READY", "UP-TO-DATE", "AVAILABLE", "AGE", "ds-a", "ds-b"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}

	_ = captureStdout(t, func() {
		if err := env.SelectByIndex(1); err != nil {
			t.Fatalf("select daemonset: %v", err)
		}
	})
	obj := env.CurrentObject()
	if obj == nil || obj.Kind != repl.KindDaemonSet || obj.Name != "ds-b" {
		t.Fatalf("expected ds-b selected as KindDaemonSet, got %+v", obj)
	}
}

func TestDaemonSetsCommand_PropagatesListErrors(t *testing.T) {
	env := makeTestEnv()
	oldList := listDaemonSetsForContext
	listDaemonSetsForContext = func(_ context.Context, _ clientcmdapi.Config, _, _ string) ([]appsv1.DaemonSet, error) {
		return nil, errors.New("boom")
	}
	defer func() { listDaemonSetsForContext = oldList }()

	err := (newDaemonSetsCmd()).Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "list daemonsets") {
		t.Fatalf("expected wrapped list error, got %v", err)
	}
}

func makeTestDaemonSet(name, namespace string, desired, current, ready, upToDate, available int32) appsv1.DaemonSet {
	return appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			CreationTimestamp: metav1.NewTime(time.Now().Add(-3 * 24 * time.Hour)),
		},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: desired,
			CurrentNumberScheduled: current,
			NumberReady:            ready,
			UpdatedNumberScheduled: upToDate,
			NumberAvailable:        available,
		},
	}
}
