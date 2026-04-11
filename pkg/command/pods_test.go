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

func TestPodsCommand_NoContext(t *testing.T) {
	env := makeExecTestEnvNoContext(t)
	cmd := newPodsCmd()
	if err := cmd.Execute(env, nil); err == nil {
		t.Fatal("expected error when no context is active")
	}
}

func TestPodsCommand_PrintsDefaultTableAndSetsSelectableObjects(t *testing.T) {
	env := makeTestEnv()
	oldList := listPodsForContext
	listPodsForContext = func(_ context.Context, _ clientcmdapi.Config, contextName, namespace string, opts metav1.ListOptions) ([]corev1.Pod, error) {
		if contextName != env.CurrentContext() {
			t.Fatalf("unexpected context %q", contextName)
		}
		if namespace != env.Namespace() {
			t.Fatalf("unexpected namespace %q", namespace)
		}
		if opts.LabelSelector != "" || opts.FieldSelector != "" {
			t.Fatalf("unexpected list options: %+v", opts)
		}
		return []corev1.Pod{
			makeTestPod("pod-a", "default", corev1.PodRunning, podFixture{
				Created: time.Now().Add(-2 * time.Hour),
				Statuses: []corev1.ContainerStatus{
					{Ready: true, RestartCount: 2, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
					{Ready: false, RestartCount: 1, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
				},
			}),
			makeTestPod("pod-b", "default", corev1.PodPending, podFixture{
				Created: time.Now().Add(-30 * time.Minute),
			}),
		}, nil
	}
	defer func() { listPodsForContext = oldList }()

	out := captureStdout(t, func() {
		if err := (newPodsCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"NAME", "READY", "STATUS", "RESTARTS", "AGE", "pod-a", "1/2", "Running", "3", "2h", "pod-b", "0/0", "Pending"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "NAMESPACE") {
		t.Fatalf("did not expect namespace column in default output, got:\n%s", out)
	}

	_ = captureStdout(t, func() {
		if err := env.SelectByIndex(0); err != nil {
			t.Fatalf("select pod: %v", err)
		}
	})
	obj := env.CurrentObject()
	if obj == nil || obj.Kind != repl.KindPod || obj.Name != "pod-a" {
		t.Fatalf("expected pod-a selected as KindPod, got %+v", obj)
	}
}

func TestPodsCommand_ShowsTerminatingWhenDeletionTimestampSet(t *testing.T) {
	env := makeTestEnv()
	oldList := listPodsForContext
	listPodsForContext = func(_ context.Context, _ clientcmdapi.Config, _, _ string, _ metav1.ListOptions) ([]corev1.Pod, error) {
		pod := makeTestPod("pod-a", "default", corev1.PodRunning, podFixture{})
		now := metav1.Now()
		pod.DeletionTimestamp = &now
		return []corev1.Pod{pod}, nil
	}
	defer func() { listPodsForContext = oldList }()

	out := captureStdout(t, func() {
		if err := (newPodsCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Terminating") {
		t.Fatalf("expected terminating status, got:\n%s", out)
	}
	if strings.Contains(out, "Running") {
		t.Fatalf("expected terminating to replace running status, got:\n%s", out)
	}
}

func TestPodsCommand_ShowsContainerCreatingWhenContainerWaiting(t *testing.T) {
	env := makeTestEnv()
	oldList := listPodsForContext
	listPodsForContext = func(_ context.Context, _ clientcmdapi.Config, _, _ string, _ metav1.ListOptions) ([]corev1.Pod, error) {
		return []corev1.Pod{
			makeTestPod("pod-a", "default", corev1.PodPending, podFixture{
				Statuses: []corev1.ContainerStatus{
					{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}}},
				},
			}),
		}, nil
	}
	defer func() { listPodsForContext = oldList }()

	out := captureStdout(t, func() {
		if err := (newPodsCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "ContainerCreating") {
		t.Fatalf("expected ContainerCreating status, got:\n%s", out)
	}
}

func TestPodsCommand_RegexFiltersPods(t *testing.T) {
	env := makeTestEnv()
	oldList := listPodsForContext
	listPodsForContext = func(_ context.Context, _ clientcmdapi.Config, _, _ string, _ metav1.ListOptions) ([]corev1.Pod, error) {
		return []corev1.Pod{
			makeTestPod("api-0", "default", corev1.PodRunning, podFixture{}),
			makeTestPod("worker-0", "default", corev1.PodRunning, podFixture{}),
		}, nil
	}
	defer func() { listPodsForContext = oldList }()

	out := captureStdout(t, func() {
		if err := (newPodsCmd()).Execute(env, []string{"-r", "^api-"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "api-0") {
		t.Fatalf("expected filtered output to contain api-0, got:\n%s", out)
	}
	if strings.Contains(out, "worker-0") {
		t.Fatalf("did not expect worker-0 in filtered output, got:\n%s", out)
	}
}

func TestPodsCommand_InvalidRegexReturnsError(t *testing.T) {
	env := makeTestEnv()

	err := (newPodsCmd()).Execute(env, []string{"-r", "["})
	if err == nil || !strings.Contains(err.Error(), "invalid regex") {
		t.Fatalf("expected invalid regex error, got %v", err)
	}
}

func TestPodsCommand_ReverseOrderingUpdatesRenderedAndSelectableOrder(t *testing.T) {
	env := makeTestEnv()
	oldList := listPodsForContext
	listPodsForContext = func(_ context.Context, _ clientcmdapi.Config, _, _ string, _ metav1.ListOptions) ([]corev1.Pod, error) {
		return []corev1.Pod{
			makeTestPod("pod-a", "default", corev1.PodRunning, podFixture{}),
			makeTestPod("pod-b", "default", corev1.PodRunning, podFixture{}),
		}, nil
	}
	defer func() { listPodsForContext = oldList }()

	out := captureStdout(t, func() {
		if err := (newPodsCmd()).Execute(env, []string{"-R"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if strings.Index(out, "pod-b") > strings.Index(out, "pod-a") {
		t.Fatalf("expected pod-b before pod-a in reversed output, got:\n%s", out)
	}

	_ = captureStdout(t, func() {
		if err := env.SelectByIndex(0); err != nil {
			t.Fatalf("select pod: %v", err)
		}
	})
	obj := env.CurrentObject()
	if obj == nil || obj.Name != "pod-b" {
		t.Fatalf("expected reversed selection order to pick pod-b at index 0, got %+v", obj)
	}
}

func TestPodsCommand_LabelSelectorPassedToListCall(t *testing.T) {
	env := makeTestEnv()
	oldList := listPodsForContext
	listPodsForContext = func(_ context.Context, _ clientcmdapi.Config, _, _ string, opts metav1.ListOptions) ([]corev1.Pod, error) {
		if opts.LabelSelector != "app=nginx" {
			t.Fatalf("expected label selector to be passed through, got %+v", opts)
		}
		return []corev1.Pod{makeTestPod("pod-a", "default", corev1.PodRunning, podFixture{})}, nil
	}
	defer func() { listPodsForContext = oldList }()

	if err := (newPodsCmd()).Execute(env, []string{"-l", "app=nginx"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPodsCommand_ExplicitNodeSelectorPassedToListCall(t *testing.T) {
	env := makeTestEnv()
	oldList := listPodsForContext
	listPodsForContext = func(_ context.Context, _ clientcmdapi.Config, _, _ string, opts metav1.ListOptions) ([]corev1.Pod, error) {
		if opts.FieldSelector != "spec.nodeName=node-a" {
			t.Fatalf("expected node field selector, got %+v", opts)
		}
		return []corev1.Pod{makeTestPod("pod-a", "default", corev1.PodRunning, podFixture{})}, nil
	}
	defer func() { listPodsForContext = oldList }()

	if err := (newPodsCmd()).Execute(env, []string{"-n", "node-a"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPodsCommand_UsesSelectedNodeWhenNodeFlagOmitted(t *testing.T) {
	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{{Kind: repl.KindNode, Name: "node-b"}})
	_ = captureStdout(t, func() {
		if err := env.SelectByIndex(0); err != nil {
			t.Fatalf("select node: %v", err)
		}
	})

	oldList := listPodsForContext
	listPodsForContext = func(_ context.Context, _ clientcmdapi.Config, _, _ string, opts metav1.ListOptions) ([]corev1.Pod, error) {
		if opts.FieldSelector != "spec.nodeName=node-b" {
			t.Fatalf("expected selected node field selector, got %+v", opts)
		}
		return []corev1.Pod{makeTestPod("pod-a", "default", corev1.PodRunning, podFixture{})}, nil
	}
	defer func() { listPodsForContext = oldList }()

	if err := (newPodsCmd()).Execute(env, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPodsCommand_RejectsSelectedNamespaceRange(t *testing.T) {
	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{
		{Kind: repl.KindNamespace, Name: "default"},
		{Kind: repl.KindNamespace, Name: "kube-system"},
		{Kind: repl.KindNamespace, Name: "kube-public"},
	})
	env.SetRangeByIndices([]int{0, 1, 2})

	err := newPodsCmd().Execute(env, []string{"--show", "namespace"})
	if err == nil || !strings.Contains(err.Error(), "does not support namespace object selections") {
		t.Fatalf("expected unsupported namespace selection error, got %v", err)
	}
}

func TestPodsCommand_RejectsSelectedNamespaceObject(t *testing.T) {
	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{{Kind: repl.KindNamespace, Name: "default"}})
	env.SetCurrent(repl.LastObject{Kind: repl.KindNamespace, Name: "default"})

	err := newPodsCmd().Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "does not support namespace object selections") {
		t.Fatalf("expected unsupported namespace selection error, got %v", err)
	}
}

func TestPodsCommand_ShowColumnsAddsRequestedFields(t *testing.T) {
	env := makeTestEnv()
	oldList := listPodsForContext
	listPodsForContext = func(_ context.Context, _ clientcmdapi.Config, _, _ string, _ metav1.ListOptions) ([]corev1.Pod, error) {
		lastRestart := metav1.NewTime(time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC))
		return []corev1.Pod{
			makeTestPod("pod-a", "default", corev1.PodRunning, podFixture{
				NodeName:          "node-a",
				PodIP:             "10.0.0.5",
				NominatedNodeName: "node-candidate",
				Labels: map[string]string{
					"app":  "api",
					"tier": "backend",
				},
				ReadinessGates: []corev1.PodReadinessGate{
					{ConditionType: "example.com/ready"},
				},
				Statuses: []corev1.ContainerStatus{
					{
						LastTerminationState: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{FinishedAt: lastRestart},
						},
					},
				},
			}),
		}, nil
	}
	defer func() { listPodsForContext = oldList }()

	out := captureStdout(t, func() {
		if err := (newPodsCmd()).Execute(env, []string{"--show", "namespace,node,ip,labels,lastrestart,nominatednode,readinessgates"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{
		"NAMESPACE", "NODE", "IP", "LABELS", "LAST RESTART", "NOMINATED NODE", "READINESS GATES",
		"default", "node-a", "10.0.0.5", "app=api,tier=backend", "2024-01-02T03:04:05Z", "node-candidate", "example.com/ready",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestPodsCommand_UnknownShowColumnReturnsError(t *testing.T) {
	env := makeTestEnv()

	err := (newPodsCmd()).Execute(env, []string{"--show", "wat"})
	if err == nil || !strings.Contains(err.Error(), "unknown show column") {
		t.Fatalf("expected unknown show column error, got %v", err)
	}
}

func TestDispatch_ReusedPodsCommandResetsShowFlagValues(t *testing.T) {
	env := makeTestEnv()
	cmds := BuildCommands()
	oldList := listPodsForContext
	listPodsForContext = func(_ context.Context, _ clientcmdapi.Config, _, _ string, _ metav1.ListOptions) ([]corev1.Pod, error) {
		return []corev1.Pod{makeTestPod("pod-a", "default", corev1.PodRunning, podFixture{})}, nil
	}
	defer func() { listPodsForContext = oldList }()

	first := captureStdout(t, func() {
		if err := repl.Dispatch(env, cmds, "pods --show namespace"); err != nil {
			t.Fatalf("first execute: %v", err)
		}
	})
	if !strings.Contains(first, "NAMESPACE") {
		t.Fatalf("expected first output to contain NAMESPACE column, got:\n%s", first)
	}

	second := captureStdout(t, func() {
		if err := repl.Dispatch(env, cmds, "pods"); err != nil {
			t.Fatalf("second execute: %v", err)
		}
	})
	if strings.Contains(second, "NAMESPACE") {
		t.Fatalf("expected second output to reset show columns, got:\n%s", second)
	}
}

func TestPodsCommand_NoPodsFoundMessages(t *testing.T) {
	t.Run("namespaced", func(t *testing.T) {
		env := makeTestEnv()
		env.SetNamespace("default")
		oldList := listPodsForContext
		listPodsForContext = func(_ context.Context, _ clientcmdapi.Config, _, _ string, _ metav1.ListOptions) ([]corev1.Pod, error) {
			return nil, nil
		}
		defer func() { listPodsForContext = oldList }()

		out := captureStdout(t, func() {
			if err := (newPodsCmd()).Execute(env, nil); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
		if !strings.Contains(out, "No pods found in default.") {
			t.Fatalf("expected namespaced empty output, got:\n%s", out)
		}
	})

	t.Run("all namespaces", func(t *testing.T) {
		env := makeTestEnv()
		env.SetNamespace("")
		oldList := listPodsForContext
		listPodsForContext = func(_ context.Context, _ clientcmdapi.Config, _, namespace string, _ metav1.ListOptions) ([]corev1.Pod, error) {
			if namespace != "" {
				t.Fatalf("expected empty namespace for all namespaces, got %q", namespace)
			}
			return nil, nil
		}
		defer func() { listPodsForContext = oldList }()

		out := captureStdout(t, func() {
			if err := (newPodsCmd()).Execute(env, nil); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
		if !strings.Contains(out, "No pods found in all namespaces.") {
			t.Fatalf("expected all namespaces empty output, got:\n%s", out)
		}
	})
}

func TestPodsCommand_PropagatesListErrors(t *testing.T) {
	env := makeTestEnv()
	oldList := listPodsForContext
	listPodsForContext = func(_ context.Context, _ clientcmdapi.Config, _, _ string, _ metav1.ListOptions) ([]corev1.Pod, error) {
		return nil, errors.New("boom")
	}
	defer func() { listPodsForContext = oldList }()

	err := (newPodsCmd()).Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "list pods") {
		t.Fatalf("expected wrapped list error, got %v", err)
	}
}

func TestPodDisplayStatus(t *testing.T) {
	now := metav1.Now()
	tests := []struct {
		name string
		pod  corev1.Pod
		want string
	}{
		{name: "terminating overrides running", pod: corev1.Pod{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now}, Status: corev1.PodStatus{Phase: corev1.PodRunning}}, want: "Terminating"},
		{name: "container creating when waiting", pod: corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodPending, ContainerStatuses: []corev1.ContainerStatus{{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}}}}}}, want: "ContainerCreating"},
		{name: "running", pod: corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodRunning}}, want: "Running"},
		{name: "unknown fallback", pod: corev1.Pod{}, want: "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := podDisplayStatus(tt.pod); got != tt.want {
				t.Fatalf("podDisplayStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

type podFixture struct {
	Created           time.Time
	Statuses          []corev1.ContainerStatus
	NodeName          string
	PodIP             string
	Labels            map[string]string
	NominatedNodeName string
	ReadinessGates    []corev1.PodReadinessGate
}

func makeTestPod(name, namespace string, phase corev1.PodPhase, fixture podFixture) corev1.Pod {
	created := fixture.Created
	if created.IsZero() {
		created = time.Now().Add(-1 * time.Hour)
	}
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			CreationTimestamp: metav1.NewTime(created),
			Labels:            fixture.Labels,
		},
		Spec: corev1.PodSpec{
			NodeName:       fixture.NodeName,
			ReadinessGates: fixture.ReadinessGates,
		},
		Status: corev1.PodStatus{
			Phase:             phase,
			PodIP:             fixture.PodIP,
			NominatedNodeName: fixture.NominatedNodeName,
			ContainerStatuses: fixture.Statuses,
		},
	}
}
