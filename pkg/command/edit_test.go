package command

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/heyihong/krepl/pkg/repl"
)

// --- editGVR ---

func TestEditGVR_SupportedKinds(t *testing.T) {
	tests := []struct {
		kind       repl.LastObjectKind
		wantGVR    schema.GroupVersionResource
		wantScoped bool
	}{
		{
			kind:       repl.KindDeployment,
			wantGVR:    schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			wantScoped: true,
		},
		{
			kind:       repl.KindReplicaSet,
			wantGVR:    schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"},
			wantScoped: true,
		},
		{
			kind:       repl.KindStatefulSet,
			wantGVR:    schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"},
			wantScoped: true,
		},
		{
			kind:       repl.KindDaemonSet,
			wantGVR:    schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"},
			wantScoped: true,
		},
		{
			kind:       repl.KindConfigMap,
			wantGVR:    schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
			wantScoped: true,
		},
		{
			kind:       repl.KindSecret,
			wantGVR:    schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"},
			wantScoped: true,
		},
		{
			kind:       repl.KindCronJob,
			wantGVR:    schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"},
			wantScoped: true,
		},
		{
			kind:       repl.KindService,
			wantGVR:    schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"},
			wantScoped: true,
		},
	}

	for _, tt := range tests {
		obj := repl.LastObject{Kind: tt.kind, Name: "x", Namespace: "default"}
		gvr, namespaced, err := editGVR(obj)
		if err != nil {
			t.Errorf("kind %v: unexpected error: %v", tt.kind, err)
			continue
		}
		if gvr != tt.wantGVR {
			t.Errorf("kind %v: got GVR %v, want %v", tt.kind, gvr, tt.wantGVR)
		}
		if namespaced != tt.wantScoped {
			t.Errorf("kind %v: got namespaced=%v, want %v", tt.kind, namespaced, tt.wantScoped)
		}
	}
}

func TestEditGVR_UnsupportedKinds(t *testing.T) {
	unsupported := []repl.LastObjectKind{
		repl.KindPod,
		repl.KindNode,
		repl.KindPersistentVolume,
	}
	for _, kind := range unsupported {
		obj := repl.LastObject{Kind: kind, Name: "x"}
		_, _, err := editGVR(obj)
		if err == nil {
			t.Errorf("kind %v: expected error, got nil", kind)
		}
	}
}

func TestEditGVR_DynamicKind(t *testing.T) {
	obj := repl.LastObject{
		Kind:      repl.KindDynamic,
		Name:      "my-widget",
		Namespace: "default",
		Dynamic: &repl.DynamicResourceDescriptor{
			Resource:     "widgets",
			GroupVersion: "example.com/v1alpha1",
			Namespaced:   true,
		},
	}
	gvr, namespaced, err := editGVR(obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := schema.GroupVersionResource{Group: "example.com", Version: "v1alpha1", Resource: "widgets"}
	if gvr != want {
		t.Errorf("got GVR %v, want %v", gvr, want)
	}
	if !namespaced {
		t.Errorf("expected namespaced=true")
	}
}

func TestEditGVR_DynamicKind_MissingDescriptor(t *testing.T) {
	obj := repl.LastObject{Kind: repl.KindDynamic, Name: "x"}
	_, _, err := editGVR(obj)
	if err == nil || !strings.Contains(err.Error(), "missing dynamic resource descriptor") {
		t.Fatalf("expected missing descriptor error, got %v", err)
	}
}

// --- resolveEditor ---

func TestResolveEditor_Flag(t *testing.T) {
	t.Setenv("EDITOR", "nano")
	if got := resolveEditor("emacs"); got != "emacs" {
		t.Errorf("flag should win: got %q, want %q", got, "emacs")
	}
}

func TestResolveEditor_EnvVar(t *testing.T) {
	t.Setenv("EDITOR", "nano")
	if got := resolveEditor(""); got != "nano" {
		t.Errorf("env var should be used: got %q, want %q", got, "nano")
	}
}

func TestResolveEditor_Default(t *testing.T) {
	t.Setenv("EDITOR", "")
	if got := resolveEditor(""); got != "vi" {
		t.Errorf("default should be vi: got %q", got)
	}
}

// --- Execute ---

func TestEditCommand_NoActiveObject(t *testing.T) {
	env := makeTestEnv()
	cmd := newEditCmd()
	err := cmd.Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "no active object") {
		t.Fatalf("expected no active object error, got %v", err)
	}
}

func TestEditCommand_NoContext(t *testing.T) {
	cfg := makeTestConfig()
	cfg.CurrentContext = ""
	env := repl.NewEnv(cfg)
	env.SetLastObjects([]repl.LastObject{{Kind: repl.KindDeployment, Name: "web", Namespace: "default"}})
	_ = env.SelectByIndex(0)

	cmd := newEditCmd()
	err := cmd.Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "no active context") {
		t.Fatalf("expected no active context error, got %v", err)
	}
}

func TestEditCommand_UnsupportedKind(t *testing.T) {
	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{{Kind: repl.KindPod, Name: "pod-a", Namespace: "default"}})
	_ = env.SelectByIndex(0)

	cmd := newEditCmd()
	err := cmd.Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "edit does not support Pod") {
		t.Fatalf("expected unsupported pod error, got %v", err)
	}
}

func TestEditCommand_FetchError(t *testing.T) {
	orig := fetchEditObject
	defer func() { fetchEditObject = orig }()
	fetchEditObject = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) (*unstructured.Unstructured, error) {
		return nil, errors.New("not found")
	}

	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{{Kind: repl.KindDeployment, Name: "web", Namespace: "default"}})
	_ = env.SelectByIndex(0)

	cmd := newEditCmd()
	err := cmd.Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "get object") {
		t.Fatalf("expected fetch error, got %v", err)
	}
}

func TestEditCommand_NoChanges(t *testing.T) {
	sampleObj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]interface{}{"name": "web", "namespace": "default"},
	}}

	orig := fetchEditObject
	defer func() { fetchEditObject = orig }()
	fetchEditObject = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) (*unstructured.Unstructured, error) {
		return sampleObj.DeepCopy(), nil
	}

	origEditor := launchEditorProcess
	defer func() { launchEditorProcess = origEditor }()
	// Editor that makes no changes (writes nothing to the file).
	launchEditorProcess = func(editor, path string) error { return nil }

	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{{Kind: repl.KindDeployment, Name: "web", Namespace: "default"}})
	_ = env.SelectByIndex(0)

	cmd := newEditCmd()
	out := captureStdout(t, func() {
		if err := cmd.Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "Edit cancelled, no changes made.") {
		t.Errorf("expected no-changes message, got: %q", out)
	}
}

func TestEditCommand_ApplyChanges(t *testing.T) {
	sampleObj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]interface{}{"name": "web", "namespace": "default"},
		"spec":       map[string]interface{}{"replicas": int64(1)},
	}}

	orig := fetchEditObject
	defer func() { fetchEditObject = orig }()
	fetchEditObject = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) (*unstructured.Unstructured, error) {
		return sampleObj.DeepCopy(), nil
	}

	var appliedObj *unstructured.Unstructured
	origApply := applyEditObject
	defer func() { applyEditObject = origApply }()
	applyEditObject = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject, u *unstructured.Unstructured) error {
		appliedObj = u
		return nil
	}

	origEditor := launchEditorProcess
	defer func() { launchEditorProcess = origEditor }()
	// Editor that modifies the file (appends a label).
	launchEditorProcess = func(editor, path string) error {
		modified := "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n  namespace: default\n  labels:\n    edited: \"true\"\nspec:\n  replicas: 1\n"
		return os.WriteFile(path, []byte(modified), 0600)
	}

	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{{Kind: repl.KindDeployment, Name: "web", Namespace: "default"}})
	_ = env.SelectByIndex(0)

	cmd := newEditCmd()
	out := captureStdout(t, func() {
		if err := cmd.Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "Updated deployment/web") {
		t.Errorf("expected Updated message, got: %q", out)
	}
	if appliedObj == nil {
		t.Fatal("expected applyEditObject to be called")
	}
}

func TestEditCommand_UnknownFlag(t *testing.T) {
	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{{Kind: repl.KindDeployment, Name: "web", Namespace: "default"}})
	_ = env.SelectByIndex(0)

	cmd := newEditCmd()
	err := cmd.Execute(env, []string{"--unknown"})
	if err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("expected unknown flag error, got %v", err)
	}
}
