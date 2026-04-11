package command

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/heyihong/krepl/pkg/repl"
)

func TestDeploymentsCommand_NoContext(t *testing.T) {
	env := makeExecTestEnvNoContext(t)
	cmd := newDeploymentsCmd()
	if err := cmd.Execute(env, nil); err == nil {
		t.Fatal("expected error when no context is active")
	}
}

func TestDeploymentsCommand_PrintsTableAndSetsSelectableObjects(t *testing.T) {
	env := makeTestEnv()
	oldList := listDeploymentsForContext
	listDeploymentsForContext = func(_ context.Context, _ clientcmdapi.Config, contextName, _ string) ([]appsv1.Deployment, error) {
		if contextName != env.CurrentContext() {
			t.Fatalf("unexpected context %q", contextName)
		}
		return []appsv1.Deployment{
			makeTestDeployment("dep-a", "default", 3, 3),
			makeTestDeployment("dep-b", "default", 2, 1),
		}, nil
	}
	defer func() { listDeploymentsForContext = oldList }()

	out := captureStdout(t, func() {
		if err := (newDeploymentsCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"NAME", "READY", "UP-TO-DATE", "AVAILABLE", "AGE", "dep-a", "dep-b", "3/3", "1/2"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}

	_ = captureStdout(t, func() {
		if err := env.SelectByIndex(0); err != nil {
			t.Fatalf("select deployment: %v", err)
		}
	})
	obj := env.CurrentObject()
	if obj == nil || obj.Kind != repl.KindDeployment || obj.Name != "dep-a" {
		t.Fatalf("expected dep-a selected as KindDeployment, got %+v", obj)
	}
}

func TestDeploymentsCommand_PropagatesListErrors(t *testing.T) {
	env := makeTestEnv()
	oldList := listDeploymentsForContext
	listDeploymentsForContext = func(_ context.Context, _ clientcmdapi.Config, _, _ string) ([]appsv1.Deployment, error) {
		return nil, errors.New("boom")
	}
	defer func() { listDeploymentsForContext = oldList }()

	err := (newDeploymentsCmd()).Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "list deployments") {
		t.Fatalf("expected wrapped list error, got %v", err)
	}
}

func makeTestDeployment(name, namespace string, desired, ready int32) appsv1.Deployment {
	return appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			CreationTimestamp: metav1.NewTime(time.Now().Add(-1 * time.Hour)),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &desired,
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas:     ready,
			Replicas:          desired,
			UpdatedReplicas:   desired,
			AvailableReplicas: ready,
		},
	}
}
