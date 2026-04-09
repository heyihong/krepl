package command

import (
	"context"
	"errors"
	"github.com/heyihong/krepl/pkg/repl"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestPersistentVolumesCommand_NoContext(t *testing.T) {
	env := makeExecTestEnvNoContext(t)
	cmd := newPersistentVolumesCmd()
	if err := cmd.Execute(env, nil); err == nil {
		t.Fatal("expected error when no context is active")
	}
}

func TestPersistentVolumesCommand_PrintsTableAndSetsSelectableObjects(t *testing.T) {
	env := makeTestEnv()
	oldList := listPersistentVolumesForContext
	listPersistentVolumesForContext = func(_ context.Context, _ clientcmdapi.Config, contextName string) ([]corev1.PersistentVolume, error) {
		if contextName != env.CurrentContext() {
			t.Fatalf("unexpected context %q", contextName)
		}
		return []corev1.PersistentVolume{
			makeTestPersistentVolume("pv-a", "10Gi", corev1.PersistentVolumeReclaimRetain,
				corev1.VolumeBound, "default", "my-pvc",
				[]corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}),
			makeTestPersistentVolume("pv-b", "5Gi", corev1.PersistentVolumeReclaimDelete,
				corev1.VolumeAvailable, "", "",
				[]corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany, corev1.ReadOnlyMany}),
		}, nil
	}
	defer func() { listPersistentVolumesForContext = oldList }()

	out := captureStdout(t, func() {
		if err := (newPersistentVolumesCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{
		"NAME", "CAPACITY", "ACCESS MODES", "RECLAIM POLICY", "STATUS", "CLAIM", "STORAGECLASS", "AGE",
		"pv-a", "10Gi", "RWO", "Retain", "Bound", "default/my-pvc",
		"pv-b", "5Gi", "ROX,RWX", "Delete", "Available",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}

	_ = captureStdout(t, func() {
		if err := env.SelectByIndex(0); err != nil {
			t.Fatalf("select pv: %v", err)
		}
	})
	obj := env.CurrentObject()
	if obj == nil || obj.Kind != repl.KindPersistentVolume || obj.Name != "pv-a" {
		t.Fatalf("expected pv-a selected as KindPersistentVolume, got %+v", obj)
	}
	// PVs are cluster-scoped — namespace must be empty
	if obj.Namespace != "" {
		t.Fatalf("expected empty namespace for cluster-scoped PV, got %q", obj.Namespace)
	}
}

func TestPersistentVolumesCommand_PropagatesListErrors(t *testing.T) {
	env := makeTestEnv()
	oldList := listPersistentVolumesForContext
	listPersistentVolumesForContext = func(_ context.Context, _ clientcmdapi.Config, _ string) ([]corev1.PersistentVolume, error) {
		return nil, errors.New("boom")
	}
	defer func() { listPersistentVolumesForContext = oldList }()

	err := (newPersistentVolumesCmd()).Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "list persistentvolumes") {
		t.Fatalf("expected wrapped list error, got %v", err)
	}
}

func TestFormatAccessModes(t *testing.T) {
	cases := []struct {
		modes []corev1.PersistentVolumeAccessMode
		want  string
	}{
		{[]corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, "RWO"},
		{[]corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany, corev1.ReadOnlyMany}, "ROX,RWX"},
		{[]corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce, corev1.ReadWriteOnce}, "RWO"}, // dedup
		{[]corev1.PersistentVolumeAccessMode{corev1.ReadWriteOncePod}, "RWOP"},
		{nil, ""},
	}
	for _, tc := range cases {
		got := formatAccessModes(tc.modes)
		if got != tc.want {
			t.Errorf("formatAccessModes(%v) = %q, want %q", tc.modes, got, tc.want)
		}
	}
}

func makeTestPersistentVolume(name, capacity string, reclaimPolicy corev1.PersistentVolumeReclaimPolicy,
	phase corev1.PersistentVolumePhase, claimNS, claimName string,
	accessModes []corev1.PersistentVolumeAccessMode) corev1.PersistentVolume {

	pv := corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			CreationTimestamp: metav1.NewTime(time.Now().Add(-7 * 24 * time.Hour)),
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity:                      corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(capacity)},
			AccessModes:                   accessModes,
			PersistentVolumeReclaimPolicy: reclaimPolicy,
			StorageClassName:              "standard",
		},
		Status: corev1.PersistentVolumeStatus{Phase: phase},
	}
	if claimName != "" {
		pv.Spec.ClaimRef = &corev1.ObjectReference{Namespace: claimNS, Name: claimName}
	}
	return pv
}
