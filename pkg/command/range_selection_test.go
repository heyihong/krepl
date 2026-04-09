package command

import (
	"strings"
	"testing"

	"github.com/heyihong/krepl/pkg/repl"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestRangeCommand_PrintsSelectedObjects(t *testing.T) {
	env := makeTestEnv()
	env.SetRange([]repl.LastObject{
		{Kind: repl.KindPod, Name: "pod-a", Namespace: "default"},
		{Kind: repl.KindService, Name: "svc-a", Namespace: "default"},
	})

	out := captureStdout(t, func() {
		if err := newRangeCmd().Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"NAME", "TYPE", "NAMESPACE", "pod-a", "Pod", "svc-a", "Service"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got %q", want, out)
		}
	}
}

func TestExecCmd_RangeRejected(t *testing.T) {
	env := makeTestEnv()
	env.SetRange([]repl.LastObject{
		{Kind: repl.KindPod, Name: "pod-a", Namespace: "default"},
		{Kind: repl.KindPod, Name: "pod-b", Namespace: "default"},
	})

	err := newExecCmd().Execute(env, []string{"--", "sh"})
	if err == nil || !strings.Contains(err.Error(), "range selection is not supported for exec") {
		t.Fatalf("expected range rejection, got %v", err)
	}
}

func TestDeleteCmd_RangePromptsEachObject(t *testing.T) {
	env := makeTestEnv()
	env.SetRange([]repl.LastObject{
		{Kind: repl.KindPod, Name: "pod-a", Namespace: "default"},
		{Kind: repl.KindPod, Name: "pod-b", Namespace: "default"},
	})

	oldRead := readDeleteConfirmation
	oldDelete := deleteCurrentObject
	t.Cleanup(func() {
		readDeleteConfirmation = oldRead
		deleteCurrentObject = oldDelete
	})

	answers := []string{"y", "n"}
	readDeleteConfirmation = func() (string, error) {
		answer := answers[0]
		answers = answers[1:]
		return answer, nil
	}
	var deleted []string
	deleteCurrentObject = func(_ clientcmdapi.Config, _ string, obj repl.LastObject, _ deleteOptions) error {
		deleted = append(deleted, obj.Name)
		return nil
	}

	out := captureStdout(t, func() {
		if err := newDeleteCmd().Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if strings.Join(deleted, ",") != "pod-a" {
		t.Fatalf("unexpected deleted objects: %v", deleted)
	}
	if !strings.Contains(out, "Delete pod pod-a [y/N]?") || !strings.Contains(out, "Delete pod pod-b [y/N]?") {
		t.Fatalf("expected per-object prompts, got %q", out)
	}
	if strings.Count(out, "Not deleting") != 1 {
		t.Fatalf("expected one declined delete, got %q", out)
	}
}
