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

func TestConfigMapsCommand_NoContext(t *testing.T) {
	env := makeExecTestEnvNoContext(t)
	cmd := newConfigMapsCmd()
	if err := cmd.Execute(env, nil); err == nil {
		t.Fatal("expected error when no context is active")
	}
}

func TestConfigMapsCommand_PrintsTableAndSetsSelectableObjects(t *testing.T) {
	env := makeTestEnv()
	oldList := listConfigMapsForContext
	listConfigMapsForContext = func(_ context.Context, _ clientcmdapi.Config, contextName, _ string) ([]corev1.ConfigMap, error) {
		if contextName != env.CurrentContext() {
			t.Fatalf("unexpected context %q", contextName)
		}
		return []corev1.ConfigMap{
			makeTestConfigMap("cm-a", "default", map[string]string{"key1": "val1", "key2": "val2"}),
			makeTestConfigMap("cm-b", "default", map[string]string{}),
		}, nil
	}
	defer func() { listConfigMapsForContext = oldList }()

	out := captureStdout(t, func() {
		if err := (newConfigMapsCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"NAME", "DATA", "AGE", "cm-a", "cm-b", "2", "0"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}

	_ = captureStdout(t, func() {
		if err := env.SelectByIndex(0); err != nil {
			t.Fatalf("select configmap: %v", err)
		}
	})
	obj := env.CurrentObject()
	if obj == nil || obj.Kind != repl.KindConfigMap || obj.Name != "cm-a" {
		t.Fatalf("expected cm-a selected as KindConfigMap, got %+v", obj)
	}
}

func TestConfigMapsCommand_PropagatesListErrors(t *testing.T) {
	env := makeTestEnv()
	oldList := listConfigMapsForContext
	listConfigMapsForContext = func(_ context.Context, _ clientcmdapi.Config, _, _ string) ([]corev1.ConfigMap, error) {
		return nil, errors.New("boom")
	}
	defer func() { listConfigMapsForContext = oldList }()

	err := (newConfigMapsCmd()).Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "list configmaps") {
		t.Fatalf("expected wrapped list error, got %v", err)
	}
}

func makeTestConfigMap(name, namespace string, data map[string]string) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			CreationTimestamp: metav1.NewTime(time.Now().Add(-1 * time.Hour)),
		},
		Data: data,
	}
}
