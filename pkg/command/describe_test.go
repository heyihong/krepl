package command

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/heyihong/krepl/pkg/repl"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestFindCommand_DescribeRegistered(t *testing.T) {
	cmds := BuildCommands()
	cmd := repl.FindCommand(cmds, "describe")
	if cmd == nil {
		t.Fatal("expected to find 'describe' command")
	}
	if cmd.Name() != "describe" {
		t.Fatalf("expected describe command, got %q", cmd.Name())
	}
}

func TestFindCommand_DescribeAliasRegistered(t *testing.T) {
	cmds := BuildCommands()
	cmd := repl.FindCommand(cmds, "d")
	if cmd == nil {
		t.Fatal("expected to find 'd' alias for describe command")
	}
	if cmd.Name() != "describe" {
		t.Fatalf("expected describe command for alias, got %q", cmd.Name())
	}
}

func TestDescribeCmd_UnknownFlag(t *testing.T) {
	env := makeTestEnv()
	err := newDescribeCmd().Execute(env, []string{"--bogus"})
	if err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("expected unknown flag error, got %v", err)
	}
}

func TestDescribeCmd_PrefersJSONWhenJSONAndYAMLTogether(t *testing.T) {
	pod := makeDescribeTestPod()
	oldFetchPod := fetchDescribePod
	oldFetchEvents := fetchDescribeEvents
	fetchDescribePod = func(_ context.Context, _ clientcmdapi.Config, _ string, _ string, _ string) (*corev1.Pod, error) {
		return pod, nil
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, nil
	}
	defer func() {
		fetchDescribePod = oldFetchPod
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := newDescribeCmd().Execute(makeExecTestEnv(t), []string{"--json", "--yaml"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "\"kind\": \"Pod\"") || strings.Contains(out, "kind: Pod") {
		t.Fatalf("expected JSON output precedence, got:\n%s", out)
	}
}

func TestDescribeCommand_NoObject(t *testing.T) {
	env := makeTestEnv()
	cmd := newDescribeCmd()

	err := cmd.Execute(env, nil)
	if err == nil {
		t.Fatal("expected error when no object is active")
	}
}

func TestDescribeCommand_UsesCurrentNamespaceWhenNoObjectSelected(t *testing.T) {
	env := makeTestEnv()
	env.SetNamespace("default")
	oldFetchNamespace := fetchDescribeNamespace
	oldFetchEvents := fetchDescribeEvents
	fetchDescribeNamespace = func(_ context.Context, _ clientcmdapi.Config, _ string, name string) (*corev1.Namespace, error) {
		if name != "default" {
			t.Fatalf("unexpected namespace %q", name)
		}
		return &corev1.Namespace{
			TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
			ObjectMeta: metav1.ObjectMeta{Name: "default", CreationTimestamp: metav1.NewTime(time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC))},
			Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
		}, nil
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, obj repl.LastObject) ([]corev1.Event, error) {
		if obj.Kind != repl.KindNamespace || obj.Name != "default" {
			t.Fatalf("unexpected object: %+v", obj)
		}
		return nil, nil
	}
	defer func() {
		fetchDescribeNamespace = oldFetchNamespace
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := newDescribeCmd().Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "Name:             default") || !strings.Contains(out, "Status:           Active") {
		t.Fatalf("expected namespace output, got:\n%s", out)
	}
}

func TestDescribeCommand_NoContext(t *testing.T) {
	env := makeExecTestEnvNoContext(t)
	cmd := newDescribeCmd()

	err := cmd.Execute(env, nil)
	if err == nil {
		t.Fatal("expected error when no context is active")
	}
}

func TestDescribeCommand_PodHumanReadableOutput(t *testing.T) {
	env := makeExecTestEnv(t)
	pod := makeDescribeTestPod()
	oldFetchPod := fetchDescribePod
	oldFetchNode := fetchDescribeNode
	oldFetchEvents := fetchDescribeEvents
	fetchDescribePod = func(_ context.Context, _ clientcmdapi.Config, _ string, _ string, _ string) (*corev1.Pod, error) {
		return pod, nil
	}
	fetchDescribeNode = func(_ context.Context, _ clientcmdapi.Config, _ string, _ string) (*corev1.Node, error) {
		return nil, errors.New("unexpected node fetch")
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, nil
	}
	defer func() {
		fetchDescribePod = oldFetchPod
		fetchDescribeNode = oldFetchNode
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{
		"Name:             pod-0",
		"Namespace:        default",
		"Phase:            Running",
		"Labels:           app=demo, tier=backend",
		"Annotations:      team=platform",
		"Containers:",
		"Ready:          true",
		"Conditions:",
		"PodReadyToStartContainers=True",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestDescribeCommand_PodJSONOutput(t *testing.T) {
	env := makeExecTestEnv(t)
	pod := makeDescribeTestPod()
	oldFetchPod := fetchDescribePod
	oldFetchNode := fetchDescribeNode
	oldFetchEvents := fetchDescribeEvents
	fetchDescribePod = func(_ context.Context, _ clientcmdapi.Config, _ string, _ string, _ string) (*corev1.Pod, error) {
		return pod, nil
	}
	fetchDescribeNode = func(_ context.Context, _ clientcmdapi.Config, _ string, _ string) (*corev1.Node, error) {
		return nil, errors.New("unexpected node fetch")
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, nil
	}
	defer func() {
		fetchDescribePod = oldFetchPod
		fetchDescribeNode = oldFetchNode
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, []string{"--json", "--events=false"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "\"kind\": \"Pod\"") || !strings.Contains(out, "\"name\": \"pod-0\"") {
		t.Fatalf("expected JSON output, got:\n%s", out)
	}
}

func TestDescribeCommand_PodYAMLOutput(t *testing.T) {
	env := makeExecTestEnv(t)
	pod := makeDescribeTestPod()
	oldFetchPod := fetchDescribePod
	oldFetchNode := fetchDescribeNode
	oldFetchEvents := fetchDescribeEvents
	fetchDescribePod = func(_ context.Context, _ clientcmdapi.Config, _ string, _ string, _ string) (*corev1.Pod, error) {
		return pod, nil
	}
	fetchDescribeNode = func(_ context.Context, _ clientcmdapi.Config, _ string, _ string) (*corev1.Node, error) {
		return nil, errors.New("unexpected node fetch")
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, nil
	}
	defer func() {
		fetchDescribePod = oldFetchPod
		fetchDescribeNode = oldFetchNode
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, []string{"--yaml", "--events=false"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "kind: Pod") || !strings.Contains(out, "name: pod-0") {
		t.Fatalf("expected YAML output, got:\n%s", out)
	}
}

func TestDescribeCommand_PodAppendsEvents(t *testing.T) {
	env := makeExecTestEnv(t)
	pod := makeDescribeTestPod()
	oldFetchPod := fetchDescribePod
	oldFetchNode := fetchDescribeNode
	oldFetchEvents := fetchDescribeEvents
	fetchDescribePod = func(_ context.Context, _ clientcmdapi.Config, _ string, _ string, _ string) (*corev1.Pod, error) {
		return pod, nil
	}
	fetchDescribeNode = func(_ context.Context, _ clientcmdapi.Config, _ string, _ string) (*corev1.Node, error) {
		return nil, errors.New("unexpected node fetch")
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, obj repl.LastObject) ([]corev1.Event, error) {
		if obj.Kind != repl.KindPod || obj.Name != "pod-0" || obj.Namespace != "default" {
			t.Fatalf("unexpected object: %+v", obj)
		}
		return []corev1.Event{
			{LastTimestamp: metav1.NewTime(time.Date(2024, 1, 2, 10, 5, 0, 0, time.UTC)), Type: "Normal", Reason: "Pulled", Message: "image pulled"},
			{LastTimestamp: metav1.NewTime(time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC)), Type: "Normal", Reason: "Scheduled", Message: "pod scheduled"},
		}, nil
	}
	defer func() {
		fetchDescribePod = oldFetchPod
		fetchDescribeNode = oldFetchNode
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, []string{"--events"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Events:") || !strings.Contains(out, "Scheduled") || !strings.Contains(out, "Pulled") {
		t.Fatalf("expected events in output, got:\n%s", out)
	}
	if strings.Index(out, "Scheduled") > strings.Index(out, "Pulled") {
		t.Fatalf("expected events in chronological order, got:\n%s", out)
	}
}

func TestDescribeCommand_NodeHumanReadableOutput(t *testing.T) {
	env := makeNodeTestEnv(t)
	node := makeDescribeTestNode()
	oldFetchPod := fetchDescribePod
	oldFetchNode := fetchDescribeNode
	oldFetchEvents := fetchDescribeEvents
	fetchDescribePod = func(_ context.Context, _ clientcmdapi.Config, _ string, _ string, _ string) (*corev1.Pod, error) {
		return nil, errors.New("unexpected pod fetch")
	}
	fetchDescribeNode = func(_ context.Context, _ clientcmdapi.Config, _ string, name string) (*corev1.Node, error) {
		if name != "node-a" {
			t.Fatalf("unexpected node name %q", name)
		}
		return node, nil
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, nil
	}
	defer func() {
		fetchDescribePod = oldFetchPod
		fetchDescribeNode = oldFetchNode
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{
		"Name:               node-a",
		"Status:             Ready",
		"Roles:              control-plane,worker",
		"Internal IP:        10.0.0.10",
		"Container Runtime:  containerd://1.7.0",
		"Conditions:",
		"Ready=True",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestDescribeCommand_NodeAppendsEvents(t *testing.T) {
	env := makeNodeTestEnv(t)
	node := makeDescribeTestNode()
	oldFetchPod := fetchDescribePod
	oldFetchNode := fetchDescribeNode
	oldFetchEvents := fetchDescribeEvents
	fetchDescribePod = func(_ context.Context, _ clientcmdapi.Config, _ string, _ string, _ string) (*corev1.Pod, error) {
		return nil, errors.New("unexpected pod fetch")
	}
	fetchDescribeNode = func(_ context.Context, _ clientcmdapi.Config, _ string, _ string) (*corev1.Node, error) {
		return node, nil
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, obj repl.LastObject) ([]corev1.Event, error) {
		if obj.Kind != repl.KindNode || obj.Name != "node-a" || obj.Namespace != "" {
			t.Fatalf("unexpected object: %+v", obj)
		}
		return []corev1.Event{{LastTimestamp: metav1.NewTime(time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC)), Type: "Warning", Reason: "NodeNotReady", Message: "node not ready"}}, nil
	}
	defer func() {
		fetchDescribePod = oldFetchPod
		fetchDescribeNode = oldFetchNode
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, []string{"--events"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Events:") || !strings.Contains(out, "NodeNotReady") {
		t.Fatalf("expected node events in output, got:\n%s", out)
	}
}

func TestDescribeCommand_PropagatesPodFetchErrors(t *testing.T) {
	env := makeExecTestEnv(t)
	oldFetchPod := fetchDescribePod
	oldFetchNode := fetchDescribeNode
	fetchDescribePod = func(_ context.Context, _ clientcmdapi.Config, _ string, _ string, _ string) (*corev1.Pod, error) {
		return nil, errors.New("boom")
	}
	fetchDescribeNode = func(_ context.Context, _ clientcmdapi.Config, _ string, _ string) (*corev1.Node, error) {
		return nil, errors.New("unexpected node fetch")
	}
	defer func() {
		fetchDescribePod = oldFetchPod
		fetchDescribeNode = oldFetchNode
	}()

	err := (newDescribeCmd()).Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "get pod") {
		t.Fatalf("expected wrapped fetch error, got %v", err)
	}
}

func makeDescribeTestPod() *corev1.Pod {
	startedAt := metav1.NewTime(time.Date(2024, 1, 2, 10, 1, 0, 0, time.UTC))
	createdAt := metav1.NewTime(time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC))
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
		ObjectMeta: metav1.ObjectMeta{
			Name:              "pod-0",
			Namespace:         "default",
			CreationTimestamp: createdAt,
			Labels: map[string]string{
				"tier": "backend",
				"app":  "demo",
			},
			Annotations: map[string]string{
				"team": "platform",
			},
		},
		Spec: corev1.PodSpec{
			NodeName:           "node-a",
			ServiceAccountName: "svc-demo",
			PriorityClassName:  "high-priority",
			RestartPolicy:      corev1.RestartPolicyAlways,
		},
		Status: corev1.PodStatus{
			Phase:  corev1.PodRunning,
			PodIP:  "10.0.0.5",
			HostIP: "192.168.0.10",
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:         "app",
				Image:        "nginx:1.27",
				Ready:        true,
				RestartCount: 2,
				State:        corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: startedAt}},
			}},
			Conditions: []corev1.PodCondition{{
				Type:    "PodReadyToStartContainers",
				Status:  corev1.ConditionTrue,
				Reason:  "AllChecksPassed",
				Message: "container runtime is ready",
			}},
		},
	}
}

func makeDescribeTestNode() *corev1.Node {
	createdAt := metav1.NewTime(time.Date(2024, 1, 2, 9, 0, 0, 0, time.UTC))
	lastHeartbeat := metav1.NewTime(time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC))
	return &corev1.Node{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Node"},
		ObjectMeta: metav1.ObjectMeta{
			Name:              "node-a",
			CreationTimestamp: createdAt,
			Labels: map[string]string{
				"node-role.kubernetes.io/control-plane": "",
				"node-role.kubernetes.io/worker":        "",
			},
			Annotations: map[string]string{
				"cluster": "dev",
			},
		},
		Spec: corev1.NodeSpec{PodCIDR: "10.244.0.0/24"},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "10.0.0.10"},
				{Type: corev1.NodeExternalIP, Address: "34.1.2.3"},
			},
			NodeInfo: corev1.NodeSystemInfo{
				OSImage:                 "Ubuntu 24.04",
				KernelVersion:           "6.8.0",
				ContainerRuntimeVersion: "containerd://1.7.0",
				KubeletVersion:          "v1.31.0",
				Architecture:            "amd64",
				OperatingSystem:         "linux",
			},
			Conditions: []corev1.NodeCondition{{
				Type:               corev1.NodeReady,
				Status:             corev1.ConditionTrue,
				Reason:             "KubeletReady",
				Message:            "kubelet is posting ready status",
				LastHeartbeatTime:  lastHeartbeat,
				LastTransitionTime: lastHeartbeat,
			}},
		},
	}
}

func makeNodeTestEnv(t *testing.T) *repl.Env {
	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{{Kind: repl.KindNode, Name: "node-a"}})
	_ = captureStdout(t, func() {
		_ = env.SelectByIndex(0)
	})
	return env
}

func makeDeploymentTestEnv(t *testing.T) *repl.Env {
	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{{Kind: repl.KindDeployment, Name: "dep-a", Namespace: "default"}})
	_ = captureStdout(t, func() {
		_ = env.SelectByIndex(0)
	})
	return env
}

func makeReplicaSetTestEnv(t *testing.T) *repl.Env {
	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{{Kind: repl.KindReplicaSet, Name: "rs-a", Namespace: "default"}})
	_ = captureStdout(t, func() {
		_ = env.SelectByIndex(0)
	})
	return env
}

func makeStatefulSetTestEnv(t *testing.T) *repl.Env {
	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{{Kind: repl.KindStatefulSet, Name: "sts-a", Namespace: "default"}})
	_ = captureStdout(t, func() {
		_ = env.SelectByIndex(0)
	})
	return env
}

func makeDescribeTestDeployment() *appsv1.Deployment {
	desired := int32(3)
	return &appsv1.Deployment{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{Name: "dep-a", Namespace: "default", CreationTimestamp: metav1.NewTime(time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC))},
		Spec: appsv1.DeploymentSpec{
			Replicas: &desired,
			Strategy: appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType},
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "app", Image: "nginx:1.27"}},
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			Replicas:            3,
			ReadyReplicas:       3,
			UpdatedReplicas:     3,
			AvailableReplicas:   3,
			UnavailableReplicas: 0,
		},
	}
}

func makeDescribeTestReplicaSet() *appsv1.ReplicaSet {
	desired := int32(4)
	return &appsv1.ReplicaSet{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "ReplicaSet"},
		ObjectMeta: metav1.ObjectMeta{Name: "rs-a", Namespace: "default", CreationTimestamp: metav1.NewTime(time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC))},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: &desired,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "app", Image: "nginx:1.27"}},
				},
			},
		},
		Status: appsv1.ReplicaSetStatus{
			Replicas:      4,
			ReadyReplicas: 3,
		},
	}
}

func makeDescribeTestStatefulSet() *appsv1.StatefulSet {
	desired := int32(2)
	return &appsv1.StatefulSet{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "StatefulSet"},
		ObjectMeta: metav1.ObjectMeta{Name: "sts-a", Namespace: "default", CreationTimestamp: metav1.NewTime(time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC))},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &desired,
			ServiceName: "headless-svc",
			Selector:    &metav1.LabelSelector{MatchLabels: map[string]string{"app": "cache"}},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "redis", Image: "redis:7"}},
				},
			},
		},
		Status: appsv1.StatefulSetStatus{
			Replicas:      2,
			ReadyReplicas: 2,
		},
	}
}

func TestDescribeCommand_DeploymentHumanReadableOutput(t *testing.T) {
	env := makeDeploymentTestEnv(t)
	dep := makeDescribeTestDeployment()
	oldFetchDep := fetchDescribeDeployment
	oldFetchSts := fetchDescribeStatefulSet
	oldFetchEvents := fetchDescribeEvents
	fetchDescribeDeployment = func(_ context.Context, _ clientcmdapi.Config, _, _, _ string) (*appsv1.Deployment, error) {
		return dep, nil
	}
	fetchDescribeStatefulSet = func(_ context.Context, _ clientcmdapi.Config, _, _, _ string) (*appsv1.StatefulSet, error) {
		return nil, errors.New("unexpected statefulset fetch")
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, nil
	}
	defer func() {
		fetchDescribeDeployment = oldFetchDep
		fetchDescribeStatefulSet = oldFetchSts
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"Name:             dep-a", "Namespace:        default", "Replicas:", "RollingUpdate", "Containers:", "app"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestDescribeCommand_DeploymentJSONOutput(t *testing.T) {
	env := makeDeploymentTestEnv(t)
	dep := makeDescribeTestDeployment()
	oldFetchDep := fetchDescribeDeployment
	oldFetchEvents := fetchDescribeEvents
	fetchDescribeDeployment = func(_ context.Context, _ clientcmdapi.Config, _, _, _ string) (*appsv1.Deployment, error) {
		return dep, nil
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, nil
	}
	defer func() {
		fetchDescribeDeployment = oldFetchDep
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, []string{"--json", "--events=false"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "\"kind\": \"Deployment\"") || !strings.Contains(out, "\"name\": \"dep-a\"") {
		t.Fatalf("expected JSON output with Deployment kind, got:\n%s", out)
	}
}

func TestDescribeCommand_DeploymentYAMLOutput(t *testing.T) {
	env := makeDeploymentTestEnv(t)
	dep := makeDescribeTestDeployment()
	oldFetchDep := fetchDescribeDeployment
	oldFetchEvents := fetchDescribeEvents
	fetchDescribeDeployment = func(_ context.Context, _ clientcmdapi.Config, _, _, _ string) (*appsv1.Deployment, error) {
		return dep, nil
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, nil
	}
	defer func() {
		fetchDescribeDeployment = oldFetchDep
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, []string{"--yaml", "--events=false"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "kind: Deployment") || !strings.Contains(out, "name: dep-a") {
		t.Fatalf("expected YAML output with Deployment kind, got:\n%s", out)
	}
}

func TestDescribeCommand_StatefulSetHumanReadableOutput(t *testing.T) {
	env := makeStatefulSetTestEnv(t)
	sts := makeDescribeTestStatefulSet()
	oldFetchDep := fetchDescribeDeployment
	oldFetchSts := fetchDescribeStatefulSet
	oldFetchEvents := fetchDescribeEvents
	fetchDescribeDeployment = func(_ context.Context, _ clientcmdapi.Config, _, _, _ string) (*appsv1.Deployment, error) {
		return nil, errors.New("unexpected deployment fetch")
	}
	fetchDescribeStatefulSet = func(_ context.Context, _ clientcmdapi.Config, _, _, name string) (*appsv1.StatefulSet, error) {
		if name != "sts-a" {
			t.Fatalf("unexpected statefulset name %q", name)
		}
		return sts, nil
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, nil
	}
	defer func() {
		fetchDescribeDeployment = oldFetchDep
		fetchDescribeStatefulSet = oldFetchSts
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"Name:                  sts-a", "Namespace:             default", "Service Name:          headless-svc", "Containers:", "redis"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestDescribeCommand_StatefulSetJSONOutput(t *testing.T) {
	env := makeStatefulSetTestEnv(t)
	sts := makeDescribeTestStatefulSet()
	oldFetchSts := fetchDescribeStatefulSet
	oldFetchEvents := fetchDescribeEvents
	fetchDescribeStatefulSet = func(_ context.Context, _ clientcmdapi.Config, _, _, _ string) (*appsv1.StatefulSet, error) {
		return sts, nil
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, nil
	}
	defer func() {
		fetchDescribeStatefulSet = oldFetchSts
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, []string{"--json", "--events=false"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "\"kind\": \"StatefulSet\"") || !strings.Contains(out, "\"name\": \"sts-a\"") {
		t.Fatalf("expected JSON output with StatefulSet kind, got:\n%s", out)
	}
}

func TestDescribeCommand_ReplicaSetHumanReadableOutput(t *testing.T) {
	env := makeReplicaSetTestEnv(t)
	rs := makeDescribeTestReplicaSet()
	oldFetchRS := fetchDescribeReplicaSet
	oldFetchEvents := fetchDescribeEvents
	fetchDescribeReplicaSet = func(_ context.Context, _ clientcmdapi.Config, _, _, name string) (*appsv1.ReplicaSet, error) {
		if name != "rs-a" {
			t.Fatalf("unexpected replicaset name %q", name)
		}
		return rs, nil
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, nil
	}
	defer func() {
		fetchDescribeReplicaSet = oldFetchRS
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"Name:             rs-a", "Namespace:        default", "Replicas:         4 desired | 4 current | 3 ready", "Selector:         app=demo", "Containers:", "app"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestDescribeCommand_ReplicaSetJSONOutput(t *testing.T) {
	env := makeReplicaSetTestEnv(t)
	rs := makeDescribeTestReplicaSet()
	oldFetchRS := fetchDescribeReplicaSet
	oldFetchEvents := fetchDescribeEvents
	fetchDescribeReplicaSet = func(_ context.Context, _ clientcmdapi.Config, _, _, _ string) (*appsv1.ReplicaSet, error) {
		return rs, nil
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, nil
	}
	defer func() {
		fetchDescribeReplicaSet = oldFetchRS
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, []string{"--json"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "\"kind\": \"ReplicaSet\"") || !strings.Contains(out, "\"name\": \"rs-a\"") {
		t.Fatalf("expected JSON output with ReplicaSet kind, got:\n%s", out)
	}
}

func TestDescribeCommand_ReplicaSetYAMLOutput(t *testing.T) {
	env := makeReplicaSetTestEnv(t)
	rs := makeDescribeTestReplicaSet()
	oldFetchRS := fetchDescribeReplicaSet
	oldFetchEvents := fetchDescribeEvents
	fetchDescribeReplicaSet = func(_ context.Context, _ clientcmdapi.Config, _, _, _ string) (*appsv1.ReplicaSet, error) {
		return rs, nil
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, nil
	}
	defer func() {
		fetchDescribeReplicaSet = oldFetchRS
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, []string{"--yaml", "--events=false"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "kind: ReplicaSet") || !strings.Contains(out, "name: rs-a") {
		t.Fatalf("expected YAML output with ReplicaSet kind, got:\n%s", out)
	}
}

func TestDescribeCommand_ReplicaSetEventsUseReplicaSetKind(t *testing.T) {
	env := makeReplicaSetTestEnv(t)
	rs := makeDescribeTestReplicaSet()
	oldFetchRS := fetchDescribeReplicaSet
	oldFetchEvents := fetchDescribeEvents
	fetchDescribeReplicaSet = func(_ context.Context, _ clientcmdapi.Config, _, _, _ string) (*appsv1.ReplicaSet, error) {
		return rs, nil
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, obj repl.LastObject) ([]corev1.Event, error) {
		if describeKindName(obj.Kind) != "ReplicaSet" {
			t.Fatalf("expected ReplicaSet event kind, got %q", describeKindName(obj.Kind))
		}
		return nil, nil
	}
	defer func() {
		fetchDescribeReplicaSet = oldFetchRS
		fetchDescribeEvents = oldFetchEvents
	}()

	if err := (newDescribeCmd()).Execute(env, []string{"--events"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func makeServiceTestEnv(t *testing.T) *repl.Env {
	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{{Kind: repl.KindService, Name: "svc-a", Namespace: "default"}})
	_ = captureStdout(t, func() {
		_ = env.SelectByIndex(0)
	})
	return env
}

func makeDescribeTestService() *corev1.Service {
	return &corev1.Service{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{Name: "svc-a", Namespace: "default", CreationTimestamp: metav1.NewTime(time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC))},
		Spec: corev1.ServiceSpec{
			Type:            corev1.ServiceTypeClusterIP,
			ClusterIP:       "10.96.0.1",
			SessionAffinity: corev1.ServiceAffinityNone,
			Selector:        map[string]string{"app": "demo"},
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 80, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(8080)},
				{Name: "https", Port: 443, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(8443)},
			},
		},
	}
}

func TestDescribeCommand_ServiceHumanReadableOutput(t *testing.T) {
	env := makeServiceTestEnv(t)
	svc := makeDescribeTestService()
	oldFetchSvc := fetchDescribeService
	oldFetchEvents := fetchDescribeEvents
	fetchDescribeService = func(_ context.Context, _ clientcmdapi.Config, _, _, name string) (*corev1.Service, error) {
		if name != "svc-a" {
			t.Fatalf("unexpected service name %q", name)
		}
		return svc, nil
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, nil
	}
	defer func() {
		fetchDescribeService = oldFetchSvc
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{
		"Name:                     svc-a",
		"Namespace:                default",
		"ClusterIP",
		"10.96.0.1",
		"Selector:",
		"app=demo",
		"Ports:",
		"http",
		"80/TCP",
		"https",
		"443/TCP",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestDescribeCommand_ServiceJSONOutput(t *testing.T) {
	env := makeServiceTestEnv(t)
	svc := makeDescribeTestService()
	oldFetchSvc := fetchDescribeService
	oldFetchEvents := fetchDescribeEvents
	fetchDescribeService = func(_ context.Context, _ clientcmdapi.Config, _, _, _ string) (*corev1.Service, error) {
		return svc, nil
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, nil
	}
	defer func() {
		fetchDescribeService = oldFetchSvc
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, []string{"--json"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "\"kind\": \"Service\"") || !strings.Contains(out, "\"name\": \"svc-a\"") {
		t.Fatalf("expected JSON output with Service kind, got:\n%s", out)
	}
}

func makePersistentVolumeTestEnv(t *testing.T) *repl.Env {
	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{{Kind: repl.KindPersistentVolume, Name: "pv-a"}})
	_ = captureStdout(t, func() {
		_ = env.SelectByIndex(0)
	})
	return env
}

func makeDescribeTestPersistentVolume() *corev1.PersistentVolume {
	filesystem := corev1.PersistentVolumeFilesystem
	pv := &corev1.PersistentVolume{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "PersistentVolume"},
		ObjectMeta: metav1.ObjectMeta{Name: "pv-a", CreationTimestamp: metav1.NewTime(time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC))},
		Spec: corev1.PersistentVolumeSpec{
			Capacity:                      corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
			AccessModes:                   []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			StorageClassName:              "standard",
			VolumeMode:                    &filesystem,
			ClaimRef:                      &corev1.ObjectReference{Namespace: "default", Name: "my-pvc"},
		},
		Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeBound},
	}
	return pv
}

func TestDescribeCommand_PersistentVolumeHumanReadableOutput(t *testing.T) {
	env := makePersistentVolumeTestEnv(t)
	pv := makeDescribeTestPersistentVolume()
	oldFetchPV := fetchDescribePersistentVolume
	oldFetchEvents := fetchDescribeEvents
	fetchDescribePersistentVolume = func(_ context.Context, _ clientcmdapi.Config, _, name string) (*corev1.PersistentVolume, error) {
		if name != "pv-a" {
			t.Fatalf("unexpected pv name %q", name)
		}
		return pv, nil
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, nil
	}
	defer func() {
		fetchDescribePersistentVolume = oldFetchPV
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"Name:             pv-a", "10Gi", "RWO", "Retain", "Bound", "default/my-pvc", "standard", "Filesystem"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestDescribeCommand_PersistentVolumeJSONOutput(t *testing.T) {
	env := makePersistentVolumeTestEnv(t)
	pv := makeDescribeTestPersistentVolume()
	oldFetchPV := fetchDescribePersistentVolume
	oldFetchEvents := fetchDescribeEvents
	fetchDescribePersistentVolume = func(_ context.Context, _ clientcmdapi.Config, _, _ string) (*corev1.PersistentVolume, error) {
		return pv, nil
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, nil
	}
	defer func() {
		fetchDescribePersistentVolume = oldFetchPV
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, []string{"--json"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "\"kind\": \"PersistentVolume\"") || !strings.Contains(out, "\"name\": \"pv-a\"") {
		t.Fatalf("expected JSON output with PersistentVolume kind, got:\n%s", out)
	}
}

func makeCronJobTestEnv(t *testing.T) *repl.Env {
	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{{Kind: repl.KindCronJob, Name: "cj-a", Namespace: "default"}})
	_ = captureStdout(t, func() {
		_ = env.SelectByIndex(0)
	})
	return env
}

func makeJobTestEnv(t *testing.T) *repl.Env {
	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{{Kind: repl.KindJob, Name: "job-a", Namespace: "default"}})
	_ = captureStdout(t, func() {
		_ = env.SelectByIndex(0)
	})
	return env
}

func makeDaemonSetTestEnv(t *testing.T) *repl.Env {
	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{{Kind: repl.KindDaemonSet, Name: "ds-a", Namespace: "kube-system"}})
	_ = captureStdout(t, func() {
		_ = env.SelectByIndex(0)
	})
	return env
}

func makeStorageClassTestEnv(t *testing.T) *repl.Env {
	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{{Kind: repl.KindStorageClass, Name: "standard"}})
	_ = captureStdout(t, func() {
		_ = env.SelectByIndex(0)
	})
	return env
}

func makeDescribeTestCronJob() *batchv1.CronJob {
	suspend := false
	lastSchedule := metav1.NewTime(time.Date(2024, 1, 2, 11, 0, 0, 0, time.UTC))
	return &batchv1.CronJob{
		TypeMeta:   metav1.TypeMeta{APIVersion: "batch/v1", Kind: "CronJob"},
		ObjectMeta: metav1.ObjectMeta{Name: "cj-a", Namespace: "default", CreationTimestamp: metav1.NewTime(time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC))},
		Spec: batchv1.CronJobSpec{
			Schedule:          "0 * * * *",
			Suspend:           &suspend,
			ConcurrencyPolicy: batchv1.AllowConcurrent,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{Name: "worker", Image: "alpine:3.18"}},
						},
					},
				},
			},
		},
		Status: batchv1.CronJobStatus{
			LastScheduleTime: &lastSchedule,
		},
	}
}

func makeDescribeTestJob() *batchv1.Job {
	completions := int32(1)
	parallelism := int32(1)
	return &batchv1.Job{
		TypeMeta:   metav1.TypeMeta{APIVersion: "batch/v1", Kind: "Job"},
		ObjectMeta: metav1.ObjectMeta{Name: "job-a", Namespace: "default", CreationTimestamp: metav1.NewTime(time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC))},
		Spec: batchv1.JobSpec{
			Completions: &completions,
			Parallelism: &parallelism,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "worker", Image: "busybox:1.36"}},
				},
			},
		},
		Status: batchv1.JobStatus{Succeeded: 1},
	}
}

func makeDescribeTestDaemonSet() *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "DaemonSet"},
		ObjectMeta: metav1.ObjectMeta{Name: "ds-a", Namespace: "kube-system", CreationTimestamp: metav1.NewTime(time.Date(2024, 1, 2, 9, 0, 0, 0, time.UTC))},
		Spec: appsv1.DaemonSetSpec{
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{Type: appsv1.RollingUpdateDaemonSetStrategyType},
			Selector:       &metav1.LabelSelector{MatchLabels: map[string]string{"app": "agent"}},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "agent", Image: "datadog/agent:7"}},
				},
			},
		},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: 3,
			CurrentNumberScheduled: 3,
			NumberReady:            3,
			UpdatedNumberScheduled: 3,
			NumberAvailable:        3,
		},
	}
}

func makeDescribeTestStorageClass() *storagev1.StorageClass {
	reclaimPolicy := corev1.PersistentVolumeReclaimDelete
	bindingMode := storagev1.VolumeBindingImmediate
	allowExpansion := true
	return &storagev1.StorageClass{
		TypeMeta: metav1.TypeMeta{APIVersion: "storage.k8s.io/v1", Kind: "StorageClass"},
		ObjectMeta: metav1.ObjectMeta{
			Name:              "standard",
			CreationTimestamp: metav1.NewTime(time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC)),
		},
		Provisioner:          "kubernetes.io/no-provisioner",
		ReclaimPolicy:        &reclaimPolicy,
		VolumeBindingMode:    &bindingMode,
		AllowVolumeExpansion: &allowExpansion,
	}
}

func TestDescribeCommand_CronJobHumanReadableOutput(t *testing.T) {
	env := makeCronJobTestEnv(t)
	cj := makeDescribeTestCronJob()
	oldFetchCJ := fetchDescribeCronJob
	oldFetchEvents := fetchDescribeEvents
	fetchDescribeCronJob = func(_ context.Context, _ clientcmdapi.Config, _, _, _ string) (*batchv1.CronJob, error) {
		return cj, nil
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, nil
	}
	defer func() {
		fetchDescribeCronJob = oldFetchCJ
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"Name:                cj-a", "Schedule:            0 * * * *", "Suspend:             False", "Containers:", "worker"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestDescribeCommand_CronJobJSONOutput(t *testing.T) {
	env := makeCronJobTestEnv(t)
	cj := makeDescribeTestCronJob()
	oldFetchCJ := fetchDescribeCronJob
	oldFetchEvents := fetchDescribeEvents
	fetchDescribeCronJob = func(_ context.Context, _ clientcmdapi.Config, _, _, _ string) (*batchv1.CronJob, error) {
		return cj, nil
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, nil
	}
	defer func() {
		fetchDescribeCronJob = oldFetchCJ
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, []string{"--json"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "\"kind\": \"CronJob\"") || !strings.Contains(out, "\"name\": \"cj-a\"") {
		t.Fatalf("expected JSON output with CronJob kind, got:\n%s", out)
	}
}

func TestDescribeCommand_JobHumanReadableOutput(t *testing.T) {
	env := makeJobTestEnv(t)
	job := makeDescribeTestJob()
	oldFetchJob := fetchDescribeJob
	oldFetchEvents := fetchDescribeEvents
	fetchDescribeJob = func(_ context.Context, _ clientcmdapi.Config, _, _, _ string) (*batchv1.Job, error) {
		return job, nil
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, nil
	}
	defer func() {
		fetchDescribeJob = oldFetchJob
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := newDescribeCmd().Execute(env, []string{"--events=false"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"Name:             job-a", "Namespace:        default", "Completions:      1/1", "Containers:", "busybox:1.36"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestDescribeCommand_StorageClassHumanReadableOutput(t *testing.T) {
	env := makeStorageClassTestEnv(t)
	sc := makeDescribeTestStorageClass()
	oldFetchStorageClass := fetchDescribeStorageClass
	oldFetchEvents := fetchDescribeEvents
	fetchDescribeStorageClass = func(_ context.Context, _ clientcmdapi.Config, _, _ string) (*storagev1.StorageClass, error) {
		return sc, nil
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, nil
	}
	defer func() {
		fetchDescribeStorageClass = oldFetchStorageClass
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := newDescribeCmd().Execute(env, []string{"--events=false"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"Name:             standard", "Provisioner:      kubernetes.io/no-provisioner", "Reclaim Policy:   Delete", "Binding Mode:     Immediate"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestDescribeCommand_DaemonSetHumanReadableOutput(t *testing.T) {
	env := makeDaemonSetTestEnv(t)
	ds := makeDescribeTestDaemonSet()
	oldFetchDS := fetchDescribeDaemonSet
	oldFetchEvents := fetchDescribeEvents
	fetchDescribeDaemonSet = func(_ context.Context, _ clientcmdapi.Config, _, _, name string) (*appsv1.DaemonSet, error) {
		if name != "ds-a" {
			t.Fatalf("unexpected daemonset name %q", name)
		}
		return ds, nil
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, nil
	}
	defer func() {
		fetchDescribeDaemonSet = oldFetchDS
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"Name:              ds-a", "Namespace:         kube-system", "Desired:           3", "RollingUpdate", "Containers:", "agent"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestDescribeCommand_DaemonSetJSONOutput(t *testing.T) {
	env := makeDaemonSetTestEnv(t)
	ds := makeDescribeTestDaemonSet()
	oldFetchDS := fetchDescribeDaemonSet
	oldFetchEvents := fetchDescribeEvents
	fetchDescribeDaemonSet = func(_ context.Context, _ clientcmdapi.Config, _, _, _ string) (*appsv1.DaemonSet, error) {
		return ds, nil
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, nil
	}
	defer func() {
		fetchDescribeDaemonSet = oldFetchDS
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, []string{"--json"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "\"kind\": \"DaemonSet\"") || !strings.Contains(out, "\"name\": \"ds-a\"") {
		t.Fatalf("expected JSON output with DaemonSet kind, got:\n%s", out)
	}
}

func makeConfigMapTestEnv(t *testing.T) *repl.Env {
	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{{Kind: repl.KindConfigMap, Name: "cm-a", Namespace: "default"}})
	_ = captureStdout(t, func() {
		_ = env.SelectByIndex(0)
	})
	return env
}

func makeSecretTestEnv(t *testing.T) *repl.Env {
	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{{Kind: repl.KindSecret, Name: "secret-a", Namespace: "default"}})
	_ = captureStdout(t, func() {
		_ = env.SelectByIndex(0)
	})
	return env
}

func makeDescribeTestConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{Name: "cm-a", Namespace: "default", CreationTimestamp: metav1.NewTime(time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC))},
		Data:       map[string]string{"config.yaml": "foo: bar", "settings.json": `{"debug":true}`},
	}
}

func makeDescribeTestSecret() *corev1.Secret {
	return &corev1.Secret{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{Name: "secret-a", Namespace: "default", CreationTimestamp: metav1.NewTime(time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC))},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{"password": []byte("s3cr3t"), "token": []byte("abc123")},
	}
}

func TestDescribeCommand_ConfigMapHumanReadableOutput(t *testing.T) {
	env := makeConfigMapTestEnv(t)
	cm := makeDescribeTestConfigMap()
	oldFetchCM := fetchDescribeConfigMap
	oldFetchEvents := fetchDescribeEvents
	fetchDescribeConfigMap = func(_ context.Context, _ clientcmdapi.Config, _, _, _ string) (*corev1.ConfigMap, error) {
		return cm, nil
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, nil
	}
	defer func() {
		fetchDescribeConfigMap = oldFetchCM
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"Name:        cm-a", "Namespace:   default", "2 entries", "Keys:", "config.yaml", "settings.json"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
	// Values must NOT appear in the output
	if strings.Contains(out, "foo: bar") {
		t.Fatalf("configmap values should not appear in human-readable output, got:\n%s", out)
	}
}

func TestDescribeCommand_ConfigMapJSONOutput(t *testing.T) {
	env := makeConfigMapTestEnv(t)
	cm := makeDescribeTestConfigMap()
	oldFetchCM := fetchDescribeConfigMap
	oldFetchEvents := fetchDescribeEvents
	fetchDescribeConfigMap = func(_ context.Context, _ clientcmdapi.Config, _, _, _ string) (*corev1.ConfigMap, error) {
		return cm, nil
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, nil
	}
	defer func() {
		fetchDescribeConfigMap = oldFetchCM
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, []string{"--json"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "\"kind\": \"ConfigMap\"") || !strings.Contains(out, "\"name\": \"cm-a\"") {
		t.Fatalf("expected JSON output with ConfigMap kind, got:\n%s", out)
	}
}

func TestDescribeCommand_SecretHumanReadableOutput(t *testing.T) {
	env := makeSecretTestEnv(t)
	s := makeDescribeTestSecret()
	oldFetchSecret := fetchDescribeSecret
	oldFetchEvents := fetchDescribeEvents
	fetchDescribeSecret = func(_ context.Context, _ clientcmdapi.Config, _, _, _ string) (*corev1.Secret, error) {
		return s, nil
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, nil
	}
	defer func() {
		fetchDescribeSecret = oldFetchSecret
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"Name:        secret-a", "Namespace:   default", "Opaque", "2 entries", "Data:", "password:", "bytes", "token:", "bytes"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
	// Actual secret values must NOT appear in the output
	for _, secret := range []string{"s3cr3t", "abc123"} {
		if strings.Contains(out, secret) {
			t.Fatalf("secret value %q must be redacted in describe output, got:\n%s", secret, out)
		}
	}
}

func TestDescribeCommand_SecretJSONOutput(t *testing.T) {
	env := makeSecretTestEnv(t)
	s := makeDescribeTestSecret()
	oldFetchSecret := fetchDescribeSecret
	oldFetchEvents := fetchDescribeEvents
	fetchDescribeSecret = func(_ context.Context, _ clientcmdapi.Config, _, _, _ string) (*corev1.Secret, error) {
		return s, nil
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, nil
	}
	defer func() {
		fetchDescribeSecret = oldFetchSecret
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, []string{"--json"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "\"kind\": \"Secret\"") || !strings.Contains(out, "\"name\": \"secret-a\"") {
		t.Fatalf("expected JSON output with Secret kind, got:\n%s", out)
	}
}

func TestDescribeCommand_StatefulSetYAMLOutput(t *testing.T) {
	env := makeStatefulSetTestEnv(t)
	sts := makeDescribeTestStatefulSet()
	oldFetchSts := fetchDescribeStatefulSet
	oldFetchEvents := fetchDescribeEvents
	fetchDescribeStatefulSet = func(_ context.Context, _ clientcmdapi.Config, _, _, _ string) (*appsv1.StatefulSet, error) {
		return sts, nil
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) ([]corev1.Event, error) {
		return nil, nil
	}
	defer func() {
		fetchDescribeStatefulSet = oldFetchSts
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, []string{"--yaml"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "kind: StatefulSet") || !strings.Contains(out, "name: sts-a") {
		t.Fatalf("expected YAML output with StatefulSet kind, got:\n%s", out)
	}
}

func TestDescribeCommand_DynamicJSONOutput(t *testing.T) {
	env := makeDynamicDescribeEnv(t)
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "example.com/v1",
		"kind":       "Widget",
		"metadata": map[string]any{
			"name":      "widget-a",
			"namespace": "default",
		},
		"spec": map[string]any{
			"size": "large",
		},
	}}
	oldFetchDynamic := fetchDescribeDynamicResource
	fetchDescribeDynamicResource = func(_ context.Context, _ clientcmdapi.Config, _ string, got repl.LastObject) (*unstructured.Unstructured, error) {
		if got.Kind != repl.KindDynamic || got.Dynamic == nil || got.Dynamic.Resource != "widgets" {
			t.Fatalf("unexpected dynamic object: %+v", got)
		}
		return obj, nil
	}
	defer func() { fetchDescribeDynamicResource = oldFetchDynamic }()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, []string{"--json", "--events=false"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{`"kind": "Widget"`, `"name": "widget-a"`, `"size": "large"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected JSON output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestDescribeCommand_DynamicYAMLOutput(t *testing.T) {
	env := makeDynamicDescribeEnv(t)
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "example.com/v1",
		"kind":       "Widget",
		"metadata": map[string]any{
			"name":      "widget-a",
			"namespace": "default",
		},
	}}
	oldFetchDynamic := fetchDescribeDynamicResource
	fetchDescribeDynamicResource = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) (*unstructured.Unstructured, error) {
		return obj, nil
	}
	defer func() { fetchDescribeDynamicResource = oldFetchDynamic }()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, []string{"--yaml", "--events=false"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"apiVersion: example.com/v1", "kind: Widget", "name: widget-a"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected YAML output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestDescribeCommand_DynamicPlainOutputRequiresJSONOrYAML(t *testing.T) {
	env := makeDynamicDescribeEnv(t)
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "example.com/v1",
		"kind":       "Widget",
		"metadata":   map[string]any{"name": "widget-a"},
	}}
	oldFetchDynamic := fetchDescribeDynamicResource
	fetchDescribeDynamicResource = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) (*unstructured.Unstructured, error) {
		return obj, nil
	}
	defer func() { fetchDescribeDynamicResource = oldFetchDynamic }()

	out := captureStdout(t, func() {
		if err := (newDescribeCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "widgets not supported without -j or -y yet") {
		t.Fatalf("expected Click-compatible CRD message, got:\n%s", out)
	}
}

func TestDescribeCommand_DynamicSupportsEvents(t *testing.T) {
	env := makeDynamicDescribeEnv(t)
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "example.com/v1",
		"kind":       "Widget",
		"metadata":   map[string]any{"name": "widget-a", "namespace": "default"},
	}}
	oldFetchDynamic := fetchDescribeDynamicResource
	oldFetchEvents := fetchDescribeEvents
	fetchDescribeDynamicResource = func(_ context.Context, _ clientcmdapi.Config, _ string, _ repl.LastObject) (*unstructured.Unstructured, error) {
		return obj, nil
	}
	fetchDescribeEvents = func(_ context.Context, _ clientcmdapi.Config, _ string, got repl.LastObject) ([]corev1.Event, error) {
		if got.Kind != repl.KindDynamic || got.Dynamic == nil || got.Dynamic.Kind != "Widget" {
			t.Fatalf("unexpected object: %+v", got)
		}
		return []corev1.Event{{Reason: "Created", Message: "widget created"}}, nil
	}
	defer func() {
		fetchDescribeDynamicResource = oldFetchDynamic
		fetchDescribeEvents = oldFetchEvents
	}()

	out := captureStdout(t, func() {
		if err := newDescribeCmd().Execute(env, []string{"--events", "--json"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "\"kind\": \"Widget\"") || !strings.Contains(out, "Events:") || !strings.Contains(out, "Created") {
		t.Fatalf("expected dynamic describe with events, got:\n%s", out)
	}
}

func makeDynamicDescribeEnv(t *testing.T) *repl.Env {
	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{{
		Kind:      repl.KindDynamic,
		Name:      "widget-a",
		Namespace: "default",
		Dynamic: &repl.DynamicResourceDescriptor{
			Resource:     "widgets",
			GroupVersion: "example.com/v1",
			Kind:         "Widget",
			Namespaced:   true,
		},
	}})
	_ = captureStdout(t, func() {
		_ = env.SelectByIndex(0)
	})
	return env
}
