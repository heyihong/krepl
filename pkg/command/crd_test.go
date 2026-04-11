package command

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/heyihong/krepl/pkg/repl"
)

func TestFindDynamicResourceInAPIResourceList_MatchesPlural(t *testing.T) {
	desc := findDynamicResourceInAPIResourceList("example.com/v1", "widgets", []metav1.APIResource{
		{Name: "widgets", SingularName: "widget", Kind: "Widget", Namespaced: true},
	})
	if desc == nil {
		t.Fatal("expected match")
	}
	if desc.Resource != "widgets" || desc.Kind != "Widget" || desc.GroupVersion != "example.com/v1" || !desc.Namespaced {
		t.Fatalf("unexpected descriptor: %+v", desc)
	}
}

func TestFindDynamicResourceInAPIResourceList_MatchesSingular(t *testing.T) {
	desc := findDynamicResourceInAPIResourceList("example.com/v1", "widget", []metav1.APIResource{
		{Name: "widgets", SingularName: "widget", Kind: "Widget", Namespaced: true},
	})
	if desc == nil || desc.Resource != "widgets" {
		t.Fatalf("expected singular match to resolve plural resource, got %+v", desc)
	}
}

func TestFindDynamicResourceInAPIResourceList_SkipsSubresources(t *testing.T) {
	desc := findDynamicResourceInAPIResourceList("example.com/v1", "widgets", []metav1.APIResource{
		{Name: "widgets/status", SingularName: "", Kind: "Widget", Namespaced: true},
		{Name: "widgets/scale", SingularName: "", Kind: "Widget", Namespaced: true},
	})
	if desc != nil {
		t.Fatalf("expected subresources to be ignored, got %+v", desc)
	}
}

func TestCrdCommand_NoContext(t *testing.T) {
	env := makeExecTestEnvNoContext(t)
	cmd := newCrdCmd()
	if err := cmd.Execute(env, []string{"widgets"}); err == nil {
		t.Fatal("expected error when no context is active")
	}
}

func TestCrdCommand_MissingResourcePrintsClickCompatibleMessage(t *testing.T) {
	env := makeTestEnv()
	oldDiscover := discoverDynamicResource
	discoverDynamicResource = func(_ context.Context, _ clientcmdapi.Config, _, _ string) (*repl.DynamicResourceDescriptor, error) {
		return nil, nil
	}
	defer func() { discoverDynamicResource = oldDiscover }()

	out := captureStdout(t, func() {
		if err := (newCrdCmd()).Execute(env, []string{"widgets"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Cluster doesn't have a CRD created resource of type: widgets") {
		t.Fatalf("expected missing resource message, got:\n%s", out)
	}
}

func TestCrdCommand_ListNamespacedUsesActiveNamespace(t *testing.T) {
	env := makeTestEnv()
	env.SetNamespace("team-a")

	oldDiscover := discoverDynamicResource
	oldList := listDynamicResources
	discoverDynamicResource = func(_ context.Context, _ clientcmdapi.Config, _, name string) (*repl.DynamicResourceDescriptor, error) {
		if name != "widgets" {
			t.Fatalf("unexpected name %q", name)
		}
		return &repl.DynamicResourceDescriptor{
			Resource:     "widgets",
			GroupVersion: "example.com/v1",
			Kind:         "Widget",
			Namespaced:   true,
		}, nil
	}
	listDynamicResources = func(_ context.Context, _ clientcmdapi.Config, contextName string, desc repl.DynamicResourceDescriptor, namespace string) ([]unstructured.Unstructured, error) {
		if contextName != env.CurrentContext() {
			t.Fatalf("unexpected context %q", contextName)
		}
		if desc.Resource != "widgets" {
			t.Fatalf("unexpected descriptor %+v", desc)
		}
		if namespace != "team-a" {
			t.Fatalf("expected active namespace team-a, got %q", namespace)
		}
		return []unstructured.Unstructured{
			makeTestDynamicObject("widget-a", "team-a", time.Now().Add(-2*time.Hour)),
		}, nil
	}
	defer func() {
		discoverDynamicResource = oldDiscover
		listDynamicResources = oldList
	}()

	out := captureStdout(t, func() {
		if err := (newCrdCmd()).Execute(env, []string{"widgets"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"NAME", "NAMESPACE", "AGE", "widget-a", "team-a"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
	if env.CurrentObject() != nil {
		t.Fatal("current object should not be auto-selected")
	}
	if err := env.SelectByIndex(0); err != nil {
		t.Fatalf("select dynamic object: %v", err)
	}
	obj := env.CurrentObject()
	if obj == nil || obj.Kind != repl.KindDynamic || obj.Name != "widget-a" || obj.Namespace != "team-a" {
		t.Fatalf("expected selected dynamic object, got %+v", obj)
	}
}

func TestCrdCommand_ListNamespacedAcrossAllNamespaces(t *testing.T) {
	env := makeTestEnv()

	oldDiscover := discoverDynamicResource
	oldList := listDynamicResources
	discoverDynamicResource = func(_ context.Context, _ clientcmdapi.Config, _, _ string) (*repl.DynamicResourceDescriptor, error) {
		return &repl.DynamicResourceDescriptor{
			Resource:     "widgets",
			GroupVersion: "example.com/v1",
			Kind:         "Widget",
			Namespaced:   true,
		}, nil
	}
	listDynamicResources = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.DynamicResourceDescriptor, namespace string) ([]unstructured.Unstructured, error) {
		if namespace != "" {
			t.Fatalf("expected empty namespace for all-namespaces listing, got %q", namespace)
		}
		return []unstructured.Unstructured{
			makeTestDynamicObject("widget-a", "ns-a", time.Now().Add(-2*time.Hour)),
			makeTestDynamicObject("widget-b", "ns-b", time.Now().Add(-time.Hour)),
		}, nil
	}
	defer func() {
		discoverDynamicResource = oldDiscover
		listDynamicResources = oldList
	}()

	out := captureStdout(t, func() {
		if err := (newCrdCmd()).Execute(env, []string{"widgets"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"NAMESPACE", "ns-a", "ns-b"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestCrdCommand_ListClusterScopedOmitsNamespaceColumn(t *testing.T) {
	env := makeTestEnv()

	oldDiscover := discoverDynamicResource
	oldList := listDynamicResources
	discoverDynamicResource = func(_ context.Context, _ clientcmdapi.Config, _, _ string) (*repl.DynamicResourceDescriptor, error) {
		return &repl.DynamicResourceDescriptor{
			Resource:     "clusterwidgets",
			GroupVersion: "example.com/v1",
			Kind:         "ClusterWidget",
			Namespaced:   false,
		}, nil
	}
	listDynamicResources = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.DynamicResourceDescriptor, namespace string) ([]unstructured.Unstructured, error) {
		if namespace != "" {
			t.Fatalf("expected empty namespace for cluster-scoped listing, got %q", namespace)
		}
		return []unstructured.Unstructured{
			makeTestDynamicObject("cluster-widget-a", "", time.Now().Add(-24*time.Hour)),
		}, nil
	}
	defer func() {
		discoverDynamicResource = oldDiscover
		listDynamicResources = oldList
	}()

	out := captureStdout(t, func() {
		if err := (newCrdCmd()).Execute(env, []string{"clusterwidgets"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if strings.Contains(out, "NAMESPACE") {
		t.Fatalf("expected cluster-scoped output to omit namespace column, got:\n%s", out)
	}
	if !strings.Contains(out, "cluster-widget-a") {
		t.Fatalf("expected listed cluster-scoped object, got:\n%s", out)
	}
}

func TestCrdCommand_PropagatesListErrors(t *testing.T) {
	env := makeTestEnv()

	oldDiscover := discoverDynamicResource
	oldList := listDynamicResources
	discoverDynamicResource = func(_ context.Context, _ clientcmdapi.Config, _, _ string) (*repl.DynamicResourceDescriptor, error) {
		return &repl.DynamicResourceDescriptor{
			Resource:     "widgets",
			GroupVersion: "example.com/v1",
			Kind:         "Widget",
			Namespaced:   true,
		}, nil
	}
	listDynamicResources = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.DynamicResourceDescriptor, _ string) ([]unstructured.Unstructured, error) {
		return nil, errors.New("boom")
	}
	defer func() {
		discoverDynamicResource = oldDiscover
		listDynamicResources = oldList
	}()

	err := (newCrdCmd()).Execute(env, []string{"widgets"})
	if err == nil || !strings.Contains(err.Error(), "list crd resources") {
		t.Fatalf("expected wrapped list error, got %v", err)
	}
}

func makeTestDynamicObject(name, namespace string, created time.Time) unstructured.Unstructured {
	obj := unstructured.Unstructured{}
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.SetCreationTimestamp(metav1.NewTime(created))
	return obj
}
