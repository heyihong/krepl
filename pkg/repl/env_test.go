package repl

import (
	"errors"
	"strings"
	"testing"
)

func TestNewEnv_SetsCurrentContext(t *testing.T) {
	env := NewEnv(makeTestConfig())
	if env.CurrentContext() != "ctx-a" {
		t.Errorf("expected currentContext %q, got %q", "ctx-a", env.CurrentContext())
	}
}

func TestNewEnv_PromptReflectsContext(t *testing.T) {
	env := NewEnv(makeTestConfig())
	expected := "[ctx-a][none][none] > "
	if env.Prompt() != expected {
		t.Errorf("expected prompt %q, got %q", expected, env.Prompt())
	}
}

func TestSetContext_Valid(t *testing.T) {
	env := makeTestEnv()
	if err := env.SetContext("ctx-b"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.CurrentContext() != "ctx-b" {
		t.Errorf("expected currentContext %q, got %q", "ctx-b", env.CurrentContext())
	}
	expected := "[ctx-b][none][none] > "
	if env.Prompt() != expected {
		t.Errorf("expected prompt %q, got %q", expected, env.Prompt())
	}
}

func TestSetContext_Invalid(t *testing.T) {
	env := makeTestEnv()
	if err := env.SetContext("does-not-exist"); err == nil {
		t.Fatal("expected error for unknown context, got nil")
	}
	if env.CurrentContext() != "ctx-a" {
		t.Errorf("context should not have changed, got %q", env.CurrentContext())
	}
}

func TestSetContext_ClearsLastObjects(t *testing.T) {
	env := makeTestEnv()
	env.SetLastObjects([]LastObject{{Kind: KindPod, Name: "pod-1"}, {Kind: KindPod, Name: "pod-2"}})
	_ = env.SetContext("ctx-b")
	if len(env.lastObjects) != 0 {
		t.Errorf("expected lastObjects to be cleared on context switch")
	}
}

func TestPortForwardLifecycle(t *testing.T) {
	env := makeTestEnv()
	session := NewPortForwardSession("pod-0", "default", []string{"8080", "9090:90"})

	env.AddPortForward(session)
	if got := env.PortForward(0); got != session {
		t.Fatalf("expected stored session, got %+v", got)
	}
	if session.StatusString() != "Starting" {
		t.Fatalf("expected Starting status, got %q", session.StatusString())
	}

	session.MarkRunning()
	if session.StatusString() != "Running" {
		t.Fatalf("expected Running status, got %q", session.StatusString())
	}

	session.MarkExited(errors.New("boom"))
	if session.StatusString() != "Exited: boom" {
		t.Fatalf("expected exited status, got %q", session.StatusString())
	}
}

func TestStopAllPortForwards(t *testing.T) {
	env := makeTestEnv()
	a := NewPortForwardSession("pod-a", "default", []string{"8080"})
	b := NewPortForwardSession("pod-b", "default", []string{"9090"})
	env.AddPortForward(a)
	env.AddPortForward(b)

	env.StopAllPortForwards()

	if a.Status() != PortForwardStopped || b.Status() != PortForwardStopped {
		t.Fatalf("expected all sessions stopped, got %v and %v", a.Status(), b.Status())
	}
}

func TestPortForwardOutputBounded(t *testing.T) {
	session := NewPortForwardSession("pod-a", "default", []string{"8080"})
	session.AppendOutput(strings.Repeat("a", maxPortForwardOutputBytes))
	session.AppendOutput("tail")

	if !strings.HasSuffix(session.Output(), "tail") {
		t.Fatalf("expected retained tail output, got %q", session.Output())
	}
	if len(session.Output()) != maxPortForwardOutputBytes {
		t.Fatalf("expected bounded output length %d, got %d", maxPortForwardOutputBytes, len(session.Output()))
	}
}

func TestSetNamespace_UpdatesPrompt(t *testing.T) {
	env := makeTestEnv()
	env.SetNamespace("my-ns")
	expected := "[ctx-a][my-ns][none] > "
	if env.Prompt() != expected {
		t.Errorf("expected prompt %q, got %q", expected, env.Prompt())
	}
	if env.Namespace() != "my-ns" {
		t.Errorf("expected namespace %q, got %q", "my-ns", env.Namespace())
	}
}

func TestSetNamespace_Clear(t *testing.T) {
	env := makeTestEnv()
	env.SetNamespace("my-ns")
	env.SetNamespace("")
	if env.Namespace() != "" {
		t.Errorf("expected namespace to be empty, got %q", env.Namespace())
	}
	expected := "[ctx-a][none][none] > "
	if env.Prompt() != expected {
		t.Errorf("expected prompt %q, got %q", expected, env.Prompt())
	}
}

func TestSetNamespace_ClearsLastObjects(t *testing.T) {
	env := makeTestEnv()
	env.SetLastObjects([]LastObject{{Kind: KindPod, Name: "pod-1"}})
	env.SetNamespace("other-ns")
	if len(env.lastObjects) != 0 {
		t.Errorf("expected lastObjects to be cleared on namespace switch")
	}
}

func TestSelectByIndex_Pod(t *testing.T) {
	env := makeTestEnv()
	env.SetLastObjects([]LastObject{
		{Kind: KindPod, Name: "pod-0", Namespace: "default"},
		{Kind: KindPod, Name: "pod-1", Namespace: "default"},
	})
	_ = captureStdout(t, func() {
		if err := env.SelectByIndex(1); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if env.CurrentObject() == nil || env.CurrentObject().Kind != KindPod || env.CurrentObject().Name != "pod-1" {
		t.Errorf("expected currentObject to be pod-1, got %v", env.CurrentObject())
	}
	if !strings.Contains(env.Prompt(), "pod-1") {
		t.Errorf("expected prompt to contain pod name, got %q", env.Prompt())
	}
}

func TestSelectByIndex_Node(t *testing.T) {
	env := makeTestEnv()
	env.SetLastObjects([]LastObject{
		{Kind: KindNode, Name: "node-a"},
		{Kind: KindNode, Name: "node-b"},
	})
	_ = captureStdout(t, func() {
		if err := env.SelectByIndex(1); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if env.CurrentObject() == nil || env.CurrentObject().Kind != KindNode || env.CurrentObject().Name != "node-b" {
		t.Errorf("expected currentObject to be node-b, got %v", env.CurrentObject())
	}
	if !strings.Contains(env.Prompt(), "node-b") {
		t.Errorf("expected prompt to contain node name, got %q", env.Prompt())
	}
}

func TestSelectByIndex_Namespace(t *testing.T) {
	env := makeTestEnv()
	env.SetLastObjects([]LastObject{
		{Kind: KindNamespace, Name: "default"},
		{Kind: KindNamespace, Name: "kube-system"},
	})
	_ = captureStdout(t, func() {
		if err := env.SelectByIndex(1); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if env.Namespace() != "" {
		t.Errorf("expected working namespace unchanged, got %q", env.Namespace())
	}
	obj := env.CurrentObject()
	if obj == nil || obj.Kind != KindNamespace || obj.Name != "kube-system" {
		t.Fatalf("expected currentObject to be kube-system namespace, got %+v", obj)
	}
	if !strings.Contains(env.Prompt(), "kube-system") {
		t.Errorf("expected prompt to contain namespace name, got %q", env.Prompt())
	}
}

func TestSelectByIndex_Deployment(t *testing.T) {
	env := makeTestEnv()
	env.SetLastObjects([]LastObject{
		{Kind: KindDeployment, Name: "dep-a", Namespace: "default"},
		{Kind: KindDeployment, Name: "dep-b", Namespace: "default"},
	})
	_ = captureStdout(t, func() {
		if err := env.SelectByIndex(1); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	obj := env.CurrentObject()
	if obj == nil || obj.Kind != KindDeployment || obj.Name != "dep-b" {
		t.Fatalf("expected dep-b as KindDeployment, got %+v", obj)
	}
	if !strings.Contains(env.Prompt(), "dep-b") {
		t.Errorf("expected prompt to contain deployment name, got %q", env.Prompt())
	}
}

func TestSelectByIndex_ReplicaSet(t *testing.T) {
	env := makeTestEnv()
	env.SetLastObjects([]LastObject{
		{Kind: KindReplicaSet, Name: "rs-a", Namespace: "default"},
		{Kind: KindReplicaSet, Name: "rs-b", Namespace: "default"},
	})
	_ = captureStdout(t, func() {
		if err := env.SelectByIndex(0); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	obj := env.CurrentObject()
	if obj == nil || obj.Kind != KindReplicaSet || obj.Name != "rs-a" {
		t.Fatalf("expected rs-a as KindReplicaSet, got %+v", obj)
	}
	if !strings.Contains(env.Prompt(), "rs-a") {
		t.Errorf("expected prompt to contain replicaset name, got %q", env.Prompt())
	}
}

func TestSelectByIndex_StatefulSet(t *testing.T) {
	env := makeTestEnv()
	env.SetLastObjects([]LastObject{
		{Kind: KindStatefulSet, Name: "sts-a", Namespace: "default"},
		{Kind: KindStatefulSet, Name: "sts-b", Namespace: "default"},
	})
	_ = captureStdout(t, func() {
		if err := env.SelectByIndex(0); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	obj := env.CurrentObject()
	if obj == nil || obj.Kind != KindStatefulSet || obj.Name != "sts-a" {
		t.Fatalf("expected sts-a as KindStatefulSet, got %+v", obj)
	}
	if !strings.Contains(env.Prompt(), "sts-a") {
		t.Errorf("expected prompt to contain statefulset name, got %q", env.Prompt())
	}
}

func TestSelectByIndex_ConfigMap(t *testing.T) {
	env := makeTestEnv()
	env.SetLastObjects([]LastObject{
		{Kind: KindConfigMap, Name: "cm-a", Namespace: "default"},
	})
	_ = captureStdout(t, func() {
		if err := env.SelectByIndex(0); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	obj := env.CurrentObject()
	if obj == nil || obj.Kind != KindConfigMap || obj.Name != "cm-a" {
		t.Fatalf("expected cm-a as KindConfigMap, got %+v", obj)
	}
	if !strings.Contains(env.Prompt(), "cm-a") {
		t.Errorf("expected prompt to contain configmap name, got %q", env.Prompt())
	}
}

func TestSelectByIndex_Secret(t *testing.T) {
	env := makeTestEnv()
	env.SetLastObjects([]LastObject{
		{Kind: KindSecret, Name: "secret-a", Namespace: "default"},
	})
	_ = captureStdout(t, func() {
		if err := env.SelectByIndex(0); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	obj := env.CurrentObject()
	if obj == nil || obj.Kind != KindSecret || obj.Name != "secret-a" {
		t.Fatalf("expected secret-a as KindSecret, got %+v", obj)
	}
	if !strings.Contains(env.Prompt(), "secret-a") {
		t.Errorf("expected prompt to contain secret name, got %q", env.Prompt())
	}
}

func TestSelectByIndex_Dynamic(t *testing.T) {
	env := makeTestEnv()
	env.SetLastObjects([]LastObject{
		{
			Kind:      KindDynamic,
			Name:      "widget-a",
			Namespace: "default",
			Dynamic: &DynamicResourceDescriptor{
				Resource:     "widgets",
				GroupVersion: "example.com/v1",
				Kind:         "Widget",
				Namespaced:   true,
			},
		},
	})
	out := captureStdout(t, func() {
		if err := env.SelectByIndex(0); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	obj := env.CurrentObject()
	if obj == nil || obj.Kind != KindDynamic || obj.Name != "widget-a" {
		t.Fatalf("expected widget-a as KindDynamic, got %+v", obj)
	}
	if obj.Dynamic == nil || obj.Dynamic.Resource != "widgets" {
		t.Fatalf("expected dynamic descriptor to be preserved, got %+v", obj.Dynamic)
	}
	if !strings.Contains(out, "Selected widgets: widget-a") {
		t.Fatalf("expected selection message for dynamic resource, got %q", out)
	}
	if !strings.Contains(env.Prompt(), "widget-a") {
		t.Errorf("expected prompt to contain dynamic object name, got %q", env.Prompt())
	}
}

func TestSelectByIndex_CronJob(t *testing.T) {
	env := makeTestEnv()
	env.SetLastObjects([]LastObject{
		{Kind: KindCronJob, Name: "cj-a", Namespace: "default"},
	})
	_ = captureStdout(t, func() {
		if err := env.SelectByIndex(0); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	obj := env.CurrentObject()
	if obj == nil || obj.Kind != KindCronJob || obj.Name != "cj-a" {
		t.Fatalf("expected cj-a as KindCronJob, got %+v", obj)
	}
	if !strings.Contains(env.Prompt(), "cj-a") {
		t.Errorf("expected prompt to contain cronjob name, got %q", env.Prompt())
	}
}

func TestSelectByIndex_DaemonSet(t *testing.T) {
	env := makeTestEnv()
	env.SetLastObjects([]LastObject{
		{Kind: KindDaemonSet, Name: "ds-a", Namespace: "kube-system"},
	})
	_ = captureStdout(t, func() {
		if err := env.SelectByIndex(0); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	obj := env.CurrentObject()
	if obj == nil || obj.Kind != KindDaemonSet || obj.Name != "ds-a" {
		t.Fatalf("expected ds-a as KindDaemonSet, got %+v", obj)
	}
	if !strings.Contains(env.Prompt(), "ds-a") {
		t.Errorf("expected prompt to contain daemonset name, got %q", env.Prompt())
	}
}

func TestSelectByIndex_PersistentVolume(t *testing.T) {
	env := makeTestEnv()
	env.SetLastObjects([]LastObject{
		{Kind: KindPersistentVolume, Name: "pv-a"},
	})
	_ = captureStdout(t, func() {
		if err := env.SelectByIndex(0); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	obj := env.CurrentObject()
	if obj == nil || obj.Kind != KindPersistentVolume || obj.Name != "pv-a" {
		t.Fatalf("expected pv-a as KindPersistentVolume, got %+v", obj)
	}
	if !strings.Contains(env.Prompt(), "pv-a") {
		t.Errorf("expected prompt to contain pv name, got %q", env.Prompt())
	}
}

func TestSelectByIndex_Service(t *testing.T) {
	env := makeTestEnv()
	env.SetLastObjects([]LastObject{
		{Kind: KindService, Name: "svc-a", Namespace: "default"},
	})
	_ = captureStdout(t, func() {
		if err := env.SelectByIndex(0); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	obj := env.CurrentObject()
	if obj == nil || obj.Kind != KindService || obj.Name != "svc-a" {
		t.Fatalf("expected svc-a as KindService, got %+v", obj)
	}
	if !strings.Contains(env.Prompt(), "svc-a") {
		t.Errorf("expected prompt to contain service name, got %q", env.Prompt())
	}
}

func TestSelectByIndex_OutOfRange(t *testing.T) {
	env := makeTestEnv()
	env.SetLastObjects([]LastObject{{Kind: KindPod, Name: "pod-0"}})
	if err := env.SelectByIndex(5); err == nil {
		t.Fatal("expected error for out-of-range index")
	}
}

func TestSelectByIndex_EmptyList(t *testing.T) {
	env := makeTestEnv()
	if err := env.SelectByIndex(0); err == nil {
		t.Fatal("expected error when lastObjects is empty")
	}
}

func TestSetContext_ClearsCurrentObject(t *testing.T) {
	env := makeTestEnv()
	env.SetCurrent(LastObject{Kind: KindPod, Name: "some-pod"})
	_ = env.SetContext("ctx-b")
	if env.CurrentObject() != nil {
		t.Errorf("expected currentObject to be cleared on context switch")
	}
}

func TestSetRange_UpdatesPromptAndCurrentObject(t *testing.T) {
	env := makeTestEnv()
	env.SetRange([]LastObject{
		{Kind: KindPod, Name: "pod-a", Namespace: "default"},
		{Kind: KindPod, Name: "pod-b", Namespace: "default"},
	})

	if !env.HasRangeSelection() {
		t.Fatal("expected range selection")
	}
	if env.CurrentObject() != nil {
		t.Fatalf("expected CurrentObject to be nil for a range selection, got %+v", env.CurrentObject())
	}
	if !strings.Contains(env.Prompt(), "2 Pods selected") {
		t.Fatalf("expected range prompt, got %q", env.Prompt())
	}
}

func TestApplyToSelection_SingleAndRange(t *testing.T) {
	env := makeTestEnv()
	env.SetCurrent(LastObject{Kind: KindPod, Name: "pod-a", Namespace: "default"})

	var got []string
	if err := env.ApplyToSelection(func(obj LastObject) error {
		got = append(got, obj.Name)
		return nil
	}); err != nil {
		t.Fatalf("single ApplyToSelection returned error: %v", err)
	}

	env.SetRange([]LastObject{
		{Kind: KindPod, Name: "pod-a", Namespace: "default"},
		{Kind: KindPod, Name: "pod-b", Namespace: "default"},
	})
	out := captureStdout(t, func() {
		if err := env.ApplyToSelection(func(obj LastObject) error {
			got = append(got, obj.Name)
			return nil
		}); err != nil {
			t.Fatalf("range ApplyToSelection returned error: %v", err)
		}
	})

	if !strings.Contains(out, "--- pod-a ---") || !strings.Contains(out, "--- pod-b ---") {
		t.Fatalf("expected range separators, got %q", out)
	}
	if strings.Join(got, ",") != "pod-a,pod-a,pod-b" {
		t.Fatalf("unexpected iteration order: %v", got)
	}
}

func TestApplyToSelection_UsesConfiguredRangeSeparator(t *testing.T) {
	env := makeTestEnv()
	env.SetRangeSeparator("=== {name}:{namespace} ===")
	env.SetRange([]LastObject{{Kind: KindPod, Name: "pod-a", Namespace: "default"}})

	out := captureStdout(t, func() {
		if err := env.ApplyToSelection(func(obj LastObject) error { return nil }); err != nil {
			t.Fatalf("ApplyToSelection returned error: %v", err)
		}
	})

	if !strings.Contains(out, "=== pod-a:default ===") {
		t.Fatalf("expected configured range separator, got %q", out)
	}
}

func TestListContextNames_Sorted(t *testing.T) {
	env := makeTestEnv()
	names := env.ListContextNames()
	if len(names) != 2 {
		t.Fatalf("expected 2 contexts, got %d", len(names))
	}
	if names[0] != "ctx-a" || names[1] != "ctx-b" {
		t.Errorf("expected sorted [ctx-a ctx-b], got %v", names)
	}
}

func TestTerminalHooks_DefaultNoop(t *testing.T) {
	env := makeTestEnv()
	if err := env.SuspendTerminal(); err != nil {
		t.Fatalf("unexpected suspend error: %v", err)
	}
	if err := env.ResumeTerminal(); err != nil {
		t.Fatalf("unexpected resume error: %v", err)
	}
}

func TestTerminalHooks_UseConfiguredCallbacks(t *testing.T) {
	env := makeTestEnv()
	var calls []string
	env.SetTerminalHooks(
		func() error {
			calls = append(calls, "suspend")
			return nil
		},
		func() error {
			calls = append(calls, "resume")
			return nil
		},
	)

	if err := env.SuspendTerminal(); err != nil {
		t.Fatalf("unexpected suspend error: %v", err)
	}
	if err := env.ResumeTerminal(); err != nil {
		t.Fatalf("unexpected resume error: %v", err)
	}
	if len(calls) != 2 || calls[0] != "suspend" || calls[1] != "resume" {
		t.Fatalf("unexpected hook calls: %v", calls)
	}
}

func TestTerminalResetFlag(t *testing.T) {
	env := makeTestEnv()
	if env.ConsumeTerminalReset() {
		t.Fatal("expected reset flag to be false by default")
	}

	env.RequestTerminalReset()
	if !env.ConsumeTerminalReset() {
		t.Fatal("expected reset flag to be consumed as true")
	}
	if env.ConsumeTerminalReset() {
		t.Fatal("expected reset flag to be cleared after consume")
	}
}
