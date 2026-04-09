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

func TestStatefulSetsCommand_NoContext(t *testing.T) {
	env := makeExecTestEnvNoContext(t)
	cmd := newStatefulSetsCmd()
	if err := cmd.Execute(env, nil); err == nil {
		t.Fatal("expected error when no context is active")
	}
}

func TestStatefulSetsCommand_PrintsTableAndSetsSelectableObjects(t *testing.T) {
	env := makeTestEnv()
	oldList := listStatefulSetsForContext
	listStatefulSetsForContext = func(_ context.Context, _ clientcmdapi.Config, contextName, _ string) ([]appsv1.StatefulSet, error) {
		if contextName != env.CurrentContext() {
			t.Fatalf("unexpected context %q", contextName)
		}
		return []appsv1.StatefulSet{
			makeTestStatefulSet("sts-a", "default", 3, 3),
			makeTestStatefulSet("sts-b", "default", 2, 1),
		}, nil
	}
	defer func() { listStatefulSetsForContext = oldList }()

	out := captureStdout(t, func() {
		if err := (newStatefulSetsCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"NAME", "READY", "AGE", "sts-a", "sts-b", "3/3", "1/2"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}

	_ = captureStdout(t, func() {
		if err := env.SelectByIndex(1); err != nil {
			t.Fatalf("select statefulset: %v", err)
		}
	})
	obj := env.CurrentObject()
	if obj == nil || obj.Kind != repl.KindStatefulSet || obj.Name != "sts-b" {
		t.Fatalf("expected sts-b selected as KindStatefulSet, got %+v", obj)
	}
}

func TestStatefulSetsCommand_PropagatesListErrors(t *testing.T) {
	env := makeTestEnv()
	oldList := listStatefulSetsForContext
	listStatefulSetsForContext = func(_ context.Context, _ clientcmdapi.Config, _, _ string) ([]appsv1.StatefulSet, error) {
		return nil, errors.New("boom")
	}
	defer func() { listStatefulSetsForContext = oldList }()

	err := (newStatefulSetsCmd()).Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "list statefulsets") {
		t.Fatalf("expected wrapped list error, got %v", err)
	}
}

func makeTestStatefulSet(name, namespace string, desired, ready int32) appsv1.StatefulSet {
	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &desired,
			ServiceName: "headless-svc",
		},
		Status: appsv1.StatefulSetStatus{
			ReadyReplicas: ready,
			Replicas:      desired,
		},
	}
}
