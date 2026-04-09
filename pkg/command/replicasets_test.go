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

func TestReplicaSetsCommand_NoContext(t *testing.T) {
	env := makeExecTestEnvNoContext(t)
	cmd := newReplicaSetsCmd()
	if err := cmd.Execute(env, nil); err == nil {
		t.Fatal("expected error when no context is active")
	}
}

func TestReplicaSetsCommand_PrintsTableAndSetsSelectableObjects(t *testing.T) {
	env := makeTestEnv()
	oldList := listReplicaSetsForContext
	listReplicaSetsForContext = func(_ context.Context, _ clientcmdapi.Config, contextName, _ string) ([]appsv1.ReplicaSet, error) {
		if contextName != env.CurrentContext() {
			t.Fatalf("unexpected context %q", contextName)
		}
		return []appsv1.ReplicaSet{
			makeTestReplicaSet("rs-a", "default", 3, 3, 3),
			makeTestReplicaSet("rs-b", "default", 2, 2, 1),
		}, nil
	}
	defer func() { listReplicaSetsForContext = oldList }()

	out := captureStdout(t, func() {
		if err := (newReplicaSetsCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"NAME", "DESIRED", "CURRENT", "READY", "AGE", "rs-a", "rs-b"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}

	_ = captureStdout(t, func() {
		if err := env.SelectByIndex(1); err != nil {
			t.Fatalf("select replicaset: %v", err)
		}
	})
	obj := env.CurrentObject()
	if obj == nil || obj.Kind != repl.KindReplicaSet || obj.Name != "rs-b" {
		t.Fatalf("expected rs-b selected as KindReplicaSet, got %+v", obj)
	}
}

func TestReplicaSetsCommand_PropagatesListErrors(t *testing.T) {
	env := makeTestEnv()
	oldList := listReplicaSetsForContext
	listReplicaSetsForContext = func(_ context.Context, _ clientcmdapi.Config, _, _ string) ([]appsv1.ReplicaSet, error) {
		return nil, errors.New("boom")
	}
	defer func() { listReplicaSetsForContext = oldList }()

	err := (newReplicaSetsCmd()).Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "list replicasets") {
		t.Fatalf("expected wrapped list error, got %v", err)
	}
}

func makeTestReplicaSet(name, namespace string, desired, current, ready int32) appsv1.ReplicaSet {
	return appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			CreationTimestamp: metav1.NewTime(time.Now().Add(-90 * time.Minute)),
		},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: &desired,
		},
		Status: appsv1.ReplicaSetStatus{
			Replicas:      current,
			ReadyReplicas: ready,
		},
	}
}
