package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/heyihong/krepl/pkg/repl"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	restfake "k8s.io/client-go/rest/fake"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestDeleteCommand_NoActiveObject(t *testing.T) {
	env := makeTestEnv()
	cmd := newDeleteCmd()

	err := cmd.Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "no active object") {
		t.Fatalf("expected no active object error, got %v", err)
	}
}

func TestDeleteCommand_NoContext(t *testing.T) {
	cfg := makeTestConfig()
	cfg.CurrentContext = ""
	env := repl.NewEnv(cfg)
	env.SetLastObjects([]repl.LastObject{{Kind: repl.KindPod, Name: "pod-a", Namespace: "default"}})
	_ = env.SelectByIndex(0)

	cmd := newDeleteCmd()
	err := cmd.Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "no active context") {
		t.Fatalf("expected no active context error, got %v", err)
	}
}

func TestDeleteCmd_InvalidCascade(t *testing.T) {
	// Bad cascade value is caught in RunE before the no-active-object check.
	env := makeTestEnv()
	err := newDeleteCmd().Execute(env, []string{"--cascade", "sideways"})
	if err == nil || !strings.Contains(err.Error(), "invalid --cascade value") {
		t.Fatalf("expected invalid cascade error, got %v", err)
	}
}

func TestDeleteCmd_RejectsConflicts(t *testing.T) {
	// Mutual exclusivity is checked at the top of RunE, before no-active-object.
	tests := [][]string{
		{"--gracePeriod", "30", "--now"},
		{"--now", "--force"},
		{"--force", "--gracePeriod", "0"},
	}
	env := makeTestEnv()
	for _, args := range tests {
		if err := newDeleteCmd().Execute(env, args); err == nil {
			t.Fatalf("expected conflict error for args %v", args)
		}
	}
}

func TestDeleteCommand_ConfirmationDeclines(t *testing.T) {
	for _, answer := range []string{"", "n"} {
		t.Run("answer="+answer, func(t *testing.T) {
			env := makeDeleteTestEnv(repl.LastObject{Kind: repl.KindPod, Name: "pod-a", Namespace: "default"})
			cmd := newDeleteCmd()

			oldRead := readDeleteConfirmation
			oldDelete := deleteCurrentObject
			t.Cleanup(func() {
				readDeleteConfirmation = oldRead
				deleteCurrentObject = oldDelete
			})

			readDeleteConfirmation = func() (string, error) { return answer, nil }
			deleteCalled := false
			deleteCurrentObject = func(clientcmdapi.Config, string, repl.LastObject, deleteOptions) error {
				deleteCalled = true
				return nil
			}

			out := captureStdout(t, func() {
				if err := cmd.Execute(env, nil); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})

			if deleteCalled {
				t.Fatal("delete should not be called when confirmation declines")
			}
			if !strings.Contains(out, "Delete pod pod-a [y/N]?") {
				t.Fatalf("expected confirmation prompt, got %q", out)
			}
			if !strings.Contains(out, "Not deleting") {
				t.Fatalf("expected not deleting message, got %q", out)
			}
		})
	}
}

func TestDeleteCommand_ConfirmationAccepts(t *testing.T) {
	for _, answer := range []string{"y", "yes"} {
		t.Run("answer="+answer, func(t *testing.T) {
			env := makeDeleteTestEnv(repl.LastObject{Kind: repl.KindPod, Name: "pod-a", Namespace: "default"})
			cmd := newDeleteCmd()

			oldRead := readDeleteConfirmation
			oldDelete := deleteCurrentObject
			t.Cleanup(func() {
				readDeleteConfirmation = oldRead
				deleteCurrentObject = oldDelete
			})

			readDeleteConfirmation = func() (string, error) { return answer, nil }
			deleteCalled := false
			deleteCurrentObject = func(_ clientcmdapi.Config, contextName string, obj repl.LastObject, _ deleteOptions) error {
				deleteCalled = true
				if contextName != "ctx-a" {
					t.Fatalf("unexpected context %q", contextName)
				}
				if obj.Name != "pod-a" {
					t.Fatalf("unexpected object %+v", obj)
				}
				return nil
			}

			out := captureStdout(t, func() {
				if err := cmd.Execute(env, nil); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})

			if !deleteCalled {
				t.Fatal("expected delete to be called")
			}
			if !strings.Contains(out, "Deleted") {
				t.Fatalf("expected deleted message, got %q", out)
			}
		})
	}
}

func TestDeleteCommand_PrintsDeletedMessageForAcceptedDelete(t *testing.T) {
	env := makeDeleteTestEnv(repl.LastObject{Kind: repl.KindPod, Name: "pod-a", Namespace: "default"})
	cmd := newDeleteCmd()

	oldRead := readDeleteConfirmation
	oldDelete := deleteCurrentObject
	t.Cleanup(func() {
		readDeleteConfirmation = oldRead
		deleteCurrentObject = oldDelete
	})

	readDeleteConfirmation = func() (string, error) { return "y", nil }
	deleteCurrentObject = func(clientcmdapi.Config, string, repl.LastObject, deleteOptions) error {
		return nil
	}

	out := captureStdout(t, func() {
		if err := cmd.Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Deleted") {
		t.Fatalf("expected deleted message, got %q", out)
	}
}

func TestDeleteRESTTarget_UsesAppsClientForNamespacedResource(t *testing.T) {
	coreClient := newFakeDeleteRESTClient(t, schema.GroupVersion{Version: "v1"}, "/api/v1", http.StatusOK)
	appsClient := newFakeDeleteRESTClient(t, schema.GroupVersion{Group: "apps", Version: "v1"}, "/apis/apps/v1", http.StatusOK)
	batchClient := newFakeDeleteRESTClient(t, schema.GroupVersion{Group: "batch", Version: "v1"}, "/apis/batch/v1", http.StatusOK)

	err := performDeleteForObjectWithRESTClients(context.Background(), deleteRESTClients{
		core:  coreClient,
		apps:  appsClient,
		batch: batchClient,
	}, repl.LastObject{Kind: repl.KindDeployment, Name: "dep-a", Namespace: "default"}, deleteOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if appsClient.Req == nil {
		t.Fatal("expected apps client request")
	}
	if coreClient.Req != nil || batchClient.Req != nil {
		t.Fatal("expected only apps client to be used")
	}
	if appsClient.Req.Method != http.MethodDelete {
		t.Fatalf("unexpected method %q", appsClient.Req.Method)
	}
	if appsClient.Req.URL.Path != "/apis/apps/v1/namespaces/default/deployments/dep-a" {
		t.Fatalf("unexpected request path %q", appsClient.Req.URL.Path)
	}
}

func TestDeleteRESTTarget_UsesAppsClientForReplicaSet(t *testing.T) {
	coreClient := newFakeDeleteRESTClient(t, schema.GroupVersion{Version: "v1"}, "/api/v1", http.StatusOK)
	appsClient := newFakeDeleteRESTClient(t, schema.GroupVersion{Group: "apps", Version: "v1"}, "/apis/apps/v1", http.StatusOK)
	batchClient := newFakeDeleteRESTClient(t, schema.GroupVersion{Group: "batch", Version: "v1"}, "/apis/batch/v1", http.StatusOK)

	err := performDeleteForObjectWithRESTClients(context.Background(), deleteRESTClients{
		core:  coreClient,
		apps:  appsClient,
		batch: batchClient,
	}, repl.LastObject{Kind: repl.KindReplicaSet, Name: "rs-a", Namespace: "default"}, deleteOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if appsClient.Req == nil {
		t.Fatal("expected apps client request")
	}
	if coreClient.Req != nil || batchClient.Req != nil {
		t.Fatal("expected only apps client to be used")
	}
	if appsClient.Req.URL.Path != "/apis/apps/v1/namespaces/default/replicasets/rs-a" {
		t.Fatalf("unexpected request path %q", appsClient.Req.URL.Path)
	}
}

func TestDeleteRESTTarget_UsesCoreClientForClusterScopedResource(t *testing.T) {
	coreClient := newFakeDeleteRESTClient(t, schema.GroupVersion{Version: "v1"}, "/api/v1", http.StatusAccepted)
	appsClient := newFakeDeleteRESTClient(t, schema.GroupVersion{Group: "apps", Version: "v1"}, "/apis/apps/v1", http.StatusOK)
	batchClient := newFakeDeleteRESTClient(t, schema.GroupVersion{Group: "batch", Version: "v1"}, "/apis/batch/v1", http.StatusOK)

	err := performDeleteForObjectWithRESTClients(context.Background(), deleteRESTClients{
		core:  coreClient,
		apps:  appsClient,
		batch: batchClient,
	}, repl.LastObject{Kind: repl.KindNode, Name: "node-a"}, deleteOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if coreClient.Req == nil {
		t.Fatal("expected core client request")
	}
	if appsClient.Req != nil || batchClient.Req != nil {
		t.Fatal("expected only core client to be used")
	}
	if coreClient.Req.URL.Path != "/api/v1/nodes/node-a" {
		t.Fatalf("unexpected request path %q", coreClient.Req.URL.Path)
	}
}

func TestRunDeleteRequest_ReturnsServerError(t *testing.T) {
	client := newFakeDeleteRESTClient(t, schema.GroupVersion{Version: "v1"}, "/api/v1", http.StatusOK)
	client.Err = errors.New("boom")

	err := runDeleteRequest(context.Background(), client, "default", "pods", "pod-a", metav1.DeleteOptions{})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected server error, got %v", err)
	}
}

func TestHelpCmd_ListsDelete(t *testing.T) {
	cmds := BuildCommands()
	help := newHelpCmd(cmds)

	out := captureStdout(t, func() {
		if err := help.Execute(makeTestEnv(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "delete") {
		t.Fatalf("expected delete in help output, got:\n%s", out)
	}
}

func makeDeleteTestEnv(obj repl.LastObject) *repl.Env {
	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{obj})
	_ = env.SelectByIndex(0)
	return env
}

func newFakeDeleteRESTClient(t *testing.T, gv schema.GroupVersion, apiPath string, statusCode int) *restfake.RESTClient {
	t.Helper()

	status := metav1.Status{Status: metav1.StatusSuccess}
	data, err := json.Marshal(&status)
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}

	return &restfake.RESTClient{
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		GroupVersion:         gv,
		VersionedAPIPath:     apiPath,
		Resp: &http.Response{
			StatusCode: statusCode,
			Header:     http.Header{"Content-Type": []string{runtime.ContentTypeJSON}},
			Body:       io.NopCloser(bytes.NewReader(data)),
		},
	}
}
