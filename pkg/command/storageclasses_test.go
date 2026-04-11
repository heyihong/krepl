package command

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/heyihong/krepl/pkg/repl"
)

func TestStorageClassesCommand_NoContext(t *testing.T) {
	env := makeExecTestEnvNoContext(t)
	if err := newStorageClassesCmd().Execute(env, nil); err == nil {
		t.Fatal("expected error when no context is active")
	}
}

func TestStorageClassesCommand_PrintsTableAndSetsSelectableObjects(t *testing.T) {
	env := makeTestEnv()
	oldList := listStorageClassesForContext
	listStorageClassesForContext = func(_ context.Context, _ clientcmdapi.Config, contextName string) ([]storagev1.StorageClass, error) {
		if contextName != env.CurrentContext() {
			t.Fatalf("unexpected context %q", contextName)
		}
		return []storagev1.StorageClass{
			makeTestStorageClass("standard", "kubernetes.io/no-provisioner"),
			makeTestStorageClass("fast", "ebs.csi.aws.com"),
		}, nil
	}
	defer func() { listStorageClassesForContext = oldList }()

	out := captureStdout(t, func() {
		if err := newStorageClassesCmd().Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"NAME", "PROVISIONER", "AGE", "standard", "fast", "ebs.csi.aws.com"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}

	_ = captureStdout(t, func() {
		if err := env.SelectByIndex(1); err != nil {
			t.Fatalf("select storageclass: %v", err)
		}
	})
	obj := env.CurrentObject()
	if obj == nil || obj.Kind != repl.KindStorageClass || obj.Name != "fast" {
		t.Fatalf("expected fast selected as KindStorageClass, got %+v", obj)
	}
}

func TestStorageClassesCommand_PropagatesListErrors(t *testing.T) {
	env := makeTestEnv()
	oldList := listStorageClassesForContext
	listStorageClassesForContext = func(_ context.Context, _ clientcmdapi.Config, _ string) ([]storagev1.StorageClass, error) {
		return nil, errors.New("boom")
	}
	defer func() { listStorageClassesForContext = oldList }()

	err := newStorageClassesCmd().Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "list storageclasses") {
		t.Fatalf("expected wrapped list error, got %v", err)
	}
}

func makeTestStorageClass(name, provisioner string) storagev1.StorageClass {
	return storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			CreationTimestamp: metav1.NewTime(time.Now().Add(-24 * time.Hour)),
		},
		Provisioner: provisioner,
	}
}
