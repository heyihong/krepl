package command

import (
	"strings"
	"testing"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/heyihong/krepl/pkg/repl"
)

func TestCopyCmd_DelegatesToKubectlCopy(t *testing.T) {
	env := makeTestEnv()
	env.SetNamespace("team-a")
	env.SetCurrent(repl.LastObject{Kind: repl.KindPod, Name: "pod-0", Namespace: "default"})

	var gotArgs []string
	var gotOptions copyCommandOptions
	var gotNamespace string

	oldRunner := runKubectlCopy
	runKubectlCopy = func(getter genericclioptions.RESTClientGetter, options copyCommandOptions, args []string) error {
		gotArgs = append([]string(nil), args...)
		gotOptions = options
		namespace, _, err := getter.ToRawKubeConfigLoader().Namespace()
		if err != nil {
			t.Fatalf("namespace lookup: %v", err)
		}
		gotNamespace = namespace
		return nil
	}
	defer func() { runKubectlCopy = oldRunner }()

	err := newCopyCmd().Execute(env, []string{"--container", "sidecar", "--no-preserve", "--retries", "3", "./local.txt", ":/tmp/remote.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gotArgs) != 2 || gotArgs[0] != "./local.txt" || gotArgs[1] != "default/pod-0:/tmp/remote.txt" {
		t.Fatalf("unexpected args: %#v", gotArgs)
	}
	if gotOptions.container != "sidecar" || !gotOptions.noPreserve || gotOptions.retries != 3 {
		t.Fatalf("unexpected options: %+v", gotOptions)
	}
	if gotNamespace != "team-a" {
		t.Fatalf("expected namespace override %q, got %q", "team-a", gotNamespace)
	}
}

func TestCopyCmd_AllowsExplicitRemoteSpecWithoutSelection(t *testing.T) {
	env := makeTestEnv()

	called := false
	oldRunner := runKubectlCopy
	runKubectlCopy = func(_ genericclioptions.RESTClientGetter, _ copyCommandOptions, args []string) error {
		called = true
		if args[1] != "pod-a:/tmp/remote.txt" {
			t.Fatalf("unexpected explicit remote arg: %#v", args)
		}
		return nil
	}
	defer func() { runKubectlCopy = oldRunner }()

	if err := newCopyCmd().Execute(env, []string{"./local.txt", "pod-a:/tmp/remote.txt"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected kubectl copy runner to be called")
	}
}

func TestCopyCmd_ShorthandRequiresActivePod(t *testing.T) {
	env := makeTestEnv()

	err := newCopyCmd().Execute(env, []string{"./local.txt", ":/tmp/remote.txt"})
	if err == nil || !strings.Contains(err.Error(), "no active pod") {
		t.Fatalf("expected no active pod error, got %v", err)
	}
}

func TestCopyCmd_ShorthandRejectsRangeSelection(t *testing.T) {
	env := makeTestEnv()
	env.SetRange([]repl.LastObject{
		{Kind: repl.KindPod, Name: "pod-0", Namespace: "default"},
		{Kind: repl.KindPod, Name: "pod-1", Namespace: "default"},
	})

	err := newCopyCmd().Execute(env, []string{"./local.txt", ":/tmp/remote.txt"})
	if err == nil || !strings.Contains(err.Error(), "range selection is not supported for cp shorthand") {
		t.Fatalf("expected range selection error, got %v", err)
	}
}

func TestCopyCmd_ShorthandRequiresPodSelection(t *testing.T) {
	env := makeTestEnv()
	env.SetCurrent(repl.LastObject{Kind: repl.KindNode, Name: "node-a"})

	err := newCopyCmd().Execute(env, []string{"./local.txt", ":/tmp/remote.txt"})
	if err == nil || !strings.Contains(err.Error(), "requires a single selected pod") {
		t.Fatalf("expected selected pod error, got %v", err)
	}
}

func TestCopyCmd_RequiresContext(t *testing.T) {
	env := makeExecTestEnvNoContext(t)

	err := newCopyCmd().Execute(env, []string{"./local.txt", "pod-a:/tmp/remote.txt"})
	if err == nil || !strings.Contains(err.Error(), "no active context") {
		t.Fatalf("expected no active context error, got %v", err)
	}
}
