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

func TestSecretsCommand_NoContext(t *testing.T) {
	env := makeExecTestEnvNoContext(t)
	cmd := newSecretsCmd()
	if err := cmd.Execute(env, nil); err == nil {
		t.Fatal("expected error when no context is active")
	}
}

func TestSecretsCommand_PrintsTableAndSetsSelectableObjects(t *testing.T) {
	env := makeTestEnv()
	oldList := listSecretsForContext
	listSecretsForContext = func(_ context.Context, _ clientcmdapi.Config, contextName, _ string) ([]corev1.Secret, error) {
		if contextName != env.CurrentContext() {
			t.Fatalf("unexpected context %q", contextName)
		}
		return []corev1.Secret{
			makeTestSecret("secret-a", "default", corev1.SecretTypeOpaque, map[string][]byte{"password": []byte("s3cr3t")}),
			makeTestSecret("secret-b", "default", corev1.SecretTypeDockerConfigJson, map[string][]byte{".dockerconfigjson": []byte("{}")}),
		}, nil
	}
	defer func() { listSecretsForContext = oldList }()

	out := captureStdout(t, func() {
		if err := (newSecretsCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"NAME", "TYPE", "DATA", "AGE", "secret-a", "Opaque", "secret-b", "kubernetes.io/dockerconfigjson"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}

	_ = captureStdout(t, func() {
		if err := env.SelectByIndex(1); err != nil {
			t.Fatalf("select secret: %v", err)
		}
	})
	obj := env.CurrentObject()
	if obj == nil || obj.Kind != repl.KindSecret || obj.Name != "secret-b" {
		t.Fatalf("expected secret-b selected as KindSecret, got %+v", obj)
	}
}

func TestSecretsCommand_PropagatesListErrors(t *testing.T) {
	env := makeTestEnv()
	oldList := listSecretsForContext
	listSecretsForContext = func(_ context.Context, _ clientcmdapi.Config, _, _ string) ([]corev1.Secret, error) {
		return nil, errors.New("boom")
	}
	defer func() { listSecretsForContext = oldList }()

	err := (newSecretsCmd()).Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "list secrets") {
		t.Fatalf("expected wrapped list error, got %v", err)
	}
}

func makeTestSecret(name, namespace string, secretType corev1.SecretType, data map[string][]byte) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			CreationTimestamp: metav1.NewTime(time.Now().Add(-30 * time.Minute)),
		},
		Type: secretType,
		Data: data,
	}
}
