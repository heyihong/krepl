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

func TestNodesCommand_NoContext(t *testing.T) {
	env := makeExecTestEnvNoContext(t)
	cmd := newNodesCmd()
	if err := cmd.Execute(env, nil); err == nil {
		t.Fatal("expected error when no context is active")
	}
}

func TestNodesCommand_PrintsTableAndSetsSelectableObjects(t *testing.T) {
	env := makeTestEnv()
	oldListNodes := listNodesForContext
	listNodesForContext = func(_ context.Context, _ clientcmdapi.Config, contextName string) ([]corev1.Node, error) {
		if contextName != env.CurrentContext() {
			t.Fatalf("unexpected context %q", contextName)
		}
		return []corev1.Node{
			makeNodesListTestNode("node-a", false),
			makeNodesListTestNode("node-b", true),
		}, nil
	}
	defer func() { listNodesForContext = oldListNodes }()

	out := captureStdout(t, func() {
		if err := (newNodesCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"NAME", "STATUS", "ROLES", "VERSION", "node-a", "Ready", "node-b", "NotReady,SchedulingDisabled"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}

	if err := env.SelectByIndex(1); err != nil {
		t.Fatalf("select node: %v", err)
	}
	if env.CurrentObject() == nil || env.CurrentObject().Kind != repl.KindNode || env.CurrentObject().Name != "node-b" {
		t.Fatalf("expected node-b selected, got %+v", env.CurrentObject())
	}
}

func TestNodesCommand_PropagatesListErrors(t *testing.T) {
	env := makeTestEnv()
	oldListNodes := listNodesForContext
	listNodesForContext = func(_ context.Context, _ clientcmdapi.Config, _ string) ([]corev1.Node, error) {
		return nil, errors.New("boom")
	}
	defer func() { listNodesForContext = oldListNodes }()

	err := (newNodesCmd()).Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "list nodes") {
		t.Fatalf("expected wrapped list error, got %v", err)
	}
}

func TestNodeRoles(t *testing.T) {
	node := makeNodesListTestNode("node-a", false)
	if got := nodeRoles(node); got != "control-plane,worker" {
		t.Fatalf("expected sorted roles, got %q", got)
	}
}

func TestNodeStatus(t *testing.T) {
	node := makeNodesListTestNode("node-a", true)
	if got := nodeStatus(node); got != "NotReady,SchedulingDisabled" {
		t.Fatalf("unexpected node status %q", got)
	}
}

func makeNodesListTestNode(name string, notReady bool) corev1.Node {
	status := corev1.ConditionTrue
	if notReady {
		status = corev1.ConditionFalse
	}
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
			Labels: map[string]string{
				"node-role.kubernetes.io/worker":        "",
				"node-role.kubernetes.io/control-plane": "",
			},
		},
		Spec: corev1.NodeSpec{Unschedulable: notReady},
		Status: corev1.NodeStatus{
			NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.31.0"},
			Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: status}},
		},
	}
}
