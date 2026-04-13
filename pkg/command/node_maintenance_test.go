package command

import (
	"context"
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/heyihong/krepl/pkg/repl"
)

func TestCordonCommand_NoContext(t *testing.T) {
	env := makeExecTestEnvNoContext(t)
	env.SetCurrent(repl.LastObject{Kind: repl.KindNode, Name: "node-a"})

	err := newCordonCmd().Execute(env, nil)
	if err == nil {
		t.Fatal("expected error when no context is active")
	}
}

func TestCordonCommand_RequiresSelection(t *testing.T) {
	env := makeTestEnv()

	err := newCordonCmd().Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "select one by number after running `nodes`") {
		t.Fatalf("expected missing selection error, got %v", err)
	}
}

func TestCordonCommand_RequiresNodeSelection(t *testing.T) {
	env := makeTestEnv()
	env.SetCurrent(repl.LastObject{Kind: repl.KindPod, Name: "pod-a", Namespace: "default"})

	err := newCordonCmd().Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "cordon requires a node selection") {
		t.Fatalf("expected node selection error, got %v", err)
	}
}

func TestCordonCommand_CordonsSelectedNodes(t *testing.T) {
	env := makeTestEnv()
	env.SetRange([]repl.LastObject{
		{Kind: repl.KindNode, Name: "node-a"},
		{Kind: repl.KindNode, Name: "node-b"},
	})

	var got []struct {
		name        string
		schedulable bool
	}
	oldSetNodeSchedulable := setNodeSchedulableForContext
	setNodeSchedulableForContext = func(_ context.Context, _ clientcmdapi.Config, _ string, nodeName string, schedulable bool) error {
		got = append(got, struct {
			name        string
			schedulable bool
		}{name: nodeName, schedulable: schedulable})
		return nil
	}
	defer func() { setNodeSchedulableForContext = oldSetNodeSchedulable }()

	out := captureStdout(t, func() {
		if err := newCordonCmd().Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if len(got) != 2 {
		t.Fatalf("expected 2 node updates, got %d", len(got))
	}
	for i, want := range []string{"node-a", "node-b"} {
		if got[i].name != want || got[i].schedulable {
			t.Fatalf("unexpected update %d: %+v", i, got[i])
		}
	}
	for _, want := range []string{"--- node-a ---", "node/node-a cordoned", "--- node-b ---", "node/node-b cordoned"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestUncordonCommand_UncordonsSelectedNode(t *testing.T) {
	env := makeTestEnv()
	env.SetCurrent(repl.LastObject{Kind: repl.KindNode, Name: "node-a"})

	var (
		gotName        string
		gotSchedulable bool
	)
	oldSetNodeSchedulable := setNodeSchedulableForContext
	setNodeSchedulableForContext = func(_ context.Context, _ clientcmdapi.Config, _ string, nodeName string, schedulable bool) error {
		gotName = nodeName
		gotSchedulable = schedulable
		return nil
	}
	defer func() { setNodeSchedulableForContext = oldSetNodeSchedulable }()

	out := captureStdout(t, func() {
		if err := newUncordonCmd().Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if gotName != "node-a" || !gotSchedulable {
		t.Fatalf("unexpected update: name=%q schedulable=%t", gotName, gotSchedulable)
	}
	if !strings.Contains(out, "node/node-a uncordoned") {
		t.Fatalf("expected output to contain uncordon message, got:\n%s", out)
	}
}

func TestCordonCommand_PropagatesUpdateErrors(t *testing.T) {
	env := makeTestEnv()
	env.SetCurrent(repl.LastObject{Kind: repl.KindNode, Name: "node-a"})

	oldSetNodeSchedulable := setNodeSchedulableForContext
	setNodeSchedulableForContext = func(_ context.Context, _ clientcmdapi.Config, _ string, _ string, _ bool) error {
		return errors.New("boom")
	}
	defer func() { setNodeSchedulableForContext = oldSetNodeSchedulable }()

	err := newCordonCmd().Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "cordon node node-a: boom") {
		t.Fatalf("expected wrapped update error, got %v", err)
	}
}

func TestDrainCommand_RequiresSelection(t *testing.T) {
	env := makeTestEnv()

	err := newDrainCmd().Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "select one by number after running `nodes`") {
		t.Fatalf("expected missing selection error, got %v", err)
	}
}

func TestDrainCommand_RequiresNodeSelection(t *testing.T) {
	env := makeTestEnv()
	env.SetCurrent(repl.LastObject{Kind: repl.KindPod, Name: "pod-a", Namespace: "default"})

	err := newDrainCmd().Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "drain requires a node selection") {
		t.Fatalf("expected node selection error, got %v", err)
	}
}

func TestDrainCommand_DrainsSelectedNodes(t *testing.T) {
	env := makeTestEnv()
	env.SetRange([]repl.LastObject{
		{Kind: repl.KindNode, Name: "node-a"},
		{Kind: repl.KindNode, Name: "node-b"},
	})

	type drainCall struct {
		name string
		opts drainOptions
	}
	var got []drainCall
	oldDrainNode := drainNodeForContext
	drainNodeForContext = func(_ context.Context, _ clientcmdapi.Config, _ string, nodeName string, opts drainOptions) error {
		got = append(got, drainCall{name: nodeName, opts: opts})
		return nil
	}
	defer func() { drainNodeForContext = oldDrainNode }()

	out := captureStdout(t, func() {
		if err := newDrainCmd().Execute(env, []string{"--ignore-daemonsets", "--delete-emptydir-data", "--force", "--disable-eviction", "--gracePeriod", "0"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if len(got) != 2 {
		t.Fatalf("expected 2 drain calls, got %d", len(got))
	}
	for i, want := range []string{"node-a", "node-b"} {
		if got[i].name != want {
			t.Fatalf("unexpected drain call %d: %+v", i, got[i])
		}
		if !got[i].opts.ignoreDaemonSets || !got[i].opts.deleteEmptyDirData || !got[i].opts.force || !got[i].opts.disableEviction {
			t.Fatalf("expected drain flags to be set, got %+v", got[i].opts)
		}
		if got[i].opts.dryRun {
			t.Fatalf("expected dry-run to be disabled, got %+v", got[i].opts)
		}
		if got[i].opts.gracePeriodSeconds == nil || *got[i].opts.gracePeriodSeconds != 0 {
			t.Fatalf("expected grace period 0, got %+v", got[i].opts.gracePeriodSeconds)
		}
	}
	for _, want := range []string{"--- node-a ---", "node/node-a drained", "--- node-b ---", "node/node-b drained"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestDrainCommand_PropagatesDrainErrors(t *testing.T) {
	env := makeTestEnv()
	env.SetCurrent(repl.LastObject{Kind: repl.KindNode, Name: "node-a"})

	oldDrainNode := drainNodeForContext
	drainNodeForContext = func(_ context.Context, _ clientcmdapi.Config, _ string, _ string, _ drainOptions) error {
		return errors.New("boom")
	}
	defer func() { drainNodeForContext = oldDrainNode }()

	err := newDrainCmd().Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "drain node node-a: boom") {
		t.Fatalf("expected wrapped drain error, got %v", err)
	}
}

func TestDrainCommand_RejectsNegativeGracePeriod(t *testing.T) {
	env := makeTestEnv()
	env.SetCurrent(repl.LastObject{Kind: repl.KindNode, Name: "node-a"})

	err := newDrainCmd().Execute(env, []string{"--gracePeriod", "-2"})
	if err == nil || !strings.Contains(err.Error(), "invalid --gracePeriod value") {
		t.Fatalf("expected invalid grace period error, got %v", err)
	}
}

func TestDrainCommand_DryRunPassesOption(t *testing.T) {
	env := makeTestEnv()
	env.SetCurrent(repl.LastObject{Kind: repl.KindNode, Name: "node-a"})

	var got drainOptions
	oldDrainNode := drainNodeForContext
	drainNodeForContext = func(_ context.Context, _ clientcmdapi.Config, _ string, _ string, opts drainOptions) error {
		got = opts
		return nil
	}
	defer func() { drainNodeForContext = oldDrainNode }()

	out := captureStdout(t, func() {
		if err := newDrainCmd().Execute(env, []string{"--dry-run"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !got.dryRun {
		t.Fatalf("expected dry-run to be enabled, got %+v", got)
	}
	if !strings.Contains(out, "node/node-a drain dry-run complete") {
		t.Fatalf("expected dry-run output, got:\n%s", out)
	}
}

func TestNewKubectlDrainer_MapsOptions(t *testing.T) {
	gracePeriod := int64(5)
	drainer := newKubectlDrainer(context.Background(), makeDrainClientset(nil, nil), drainOptions{
		ignoreDaemonSets:   true,
		deleteEmptyDirData: true,
		force:              true,
		disableEviction:    true,
		gracePeriodSeconds: &gracePeriod,
		dryRun:             true,
	})

	if !drainer.IgnoreAllDaemonSets || !drainer.DeleteEmptyDirData || !drainer.Force || !drainer.DisableEviction {
		t.Fatalf("expected drain options to map onto kubectl drainer, got %+v", drainer)
	}
	if drainer.GracePeriodSeconds != 5 {
		t.Fatalf("expected grace period 5, got %d", drainer.GracePeriodSeconds)
	}
	if drainer.DryRunStrategy == 0 {
		t.Fatalf("expected dry-run strategy to be set, got %+v", drainer.DryRunStrategy)
	}
}

func makeDrainClientset(nodes []corev1.Node, pods []corev1.Pod) *kubefake.Clientset {
	objects := make([]runtime.Object, 0, len(nodes)+len(pods))
	for i := range nodes {
		node := nodes[i]
		objects = append(objects, &node)
	}
	for i := range pods {
		pod := pods[i]
		objects = append(objects, &pod)
	}
	return kubefake.NewSimpleClientset(objects...)
}

func TestDrainNodeWithClient_DryRunDoesNotMutate(t *testing.T) {
	controller := true
	client := makeDrainClientset(
		[]corev1.Node{{ObjectMeta: metav1.ObjectMeta{Name: "node-a"}}},
		[]corev1.Pod{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-a",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "ReplicaSet", Controller: &controller},
				},
			},
			Spec: corev1.PodSpec{NodeName: "node-a"},
		}},
	)

	out := captureStdout(t, func() {
		if err := drainNodeWithClient(context.Background(), client, "node-a", drainOptions{dryRun: true}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	node, err := client.CoreV1().Nodes().Get(context.Background(), "node-a", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get node after dry-run: %v", err)
	}
	if node.Spec.Unschedulable {
		t.Fatalf("expected dry-run not to cordon node")
	}
	pods, err := client.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("list pods after dry-run: %v", err)
	}
	if len(pods.Items) != 1 {
		t.Fatalf("expected dry-run not to delete pods, got %+v", pods.Items)
	}
	for _, want := range []string{"node/node-a would be cordoned", "pod/default/pod-a would be evicted"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}
