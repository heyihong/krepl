package command

import (
	"strings"
	"testing"

	"github.com/heyihong/krepl/pkg/portforward"
	"github.com/heyihong/krepl/pkg/repl"
)

func TestFindCommand(t *testing.T) {
	cmds := BuildCommands()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "by name", input: "pods", expected: "pods"},
		{name: "context alias", input: "ctx", expected: "context"},
		{name: "contexts alias", input: "ctxs", expected: "contexts"},
		{name: "delete-context registered", input: "delete-context", expected: "delete-context"},
		{name: "replicasets alias", input: "rs", expected: "replicasets"},
		{name: "deployments alias", input: "deps", expected: "deployments"},
		{name: "statefulsets alias", input: "ss", expected: "statefulsets"},
		{name: "crd registered", input: "crd", expected: "crd"},
		{name: "port-forward alias", input: "pf", expected: "port-forward"},
		{name: "port-forwards alias", input: "pfs", expected: "port-forwards"},
		{name: "cordon registered", input: "cordon", expected: "cordon"},
		{name: "drain registered", input: "drain", expected: "drain"},
		{name: "uncordon registered", input: "uncordon", expected: "uncordon"},
		{name: "unknown", input: "notacommand", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := repl.FindCommand(cmds, tt.input)
			if tt.expected == "" {
				if cmd != nil {
					t.Fatalf("expected nil for %q, got %v", tt.input, cmd)
				}
				return
			}

			if cmd == nil {
				t.Fatalf("expected to find command %q", tt.input)
			}
			if cmd.Name() != tt.expected {
				t.Errorf("expected Name() %q, got %q", tt.expected, cmd.Name())
			}
		})
	}
}

func TestDispatch_EmptyLine(t *testing.T) {
	env := makeTestEnv()
	cmds := BuildCommands()
	if err := repl.Dispatch(env, cmds, "   "); err != nil {
		t.Errorf("dispatch on empty line should not error, got: %v", err)
	}
}

func TestDispatch_UnknownCommand(t *testing.T) {
	env := makeTestEnv()
	cmds := BuildCommands()
	// Should print a message but NOT return an error.
	if err := repl.Dispatch(env, cmds, "unknowncmd"); err != nil {
		t.Errorf("dispatch unknown command should not return error, got: %v", err)
	}
}

func TestDispatch_QuitSetsFlag(t *testing.T) {
	env := makeTestEnv()
	env.AddPortForward(portforward.NewSession("pod-0", "default", []string{"8080"}))
	cmds := BuildCommands()
	if err := repl.Dispatch(env, cmds, "quit"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !env.IsQuit() {
		t.Error("expected env.quit to be true after 'quit' command")
	}
	if env.PortForward(0).Status() != portforward.Stopped {
		t.Error("expected quit to stop active port forwards")
	}
}

func TestDispatch_ClearClearsSelection(t *testing.T) {
	env := makeTestEnv()
	env.SetCurrent(repl.LastObject{Kind: repl.KindPod, Name: "pod-0", Namespace: "default"})
	cmds := BuildCommands()
	if err := repl.Dispatch(env, cmds, "clear"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.CurrentObject() != nil {
		t.Fatalf("expected selection cleared, got %+v", env.CurrentObject())
	}
	if env.Prompt() != "[ctx-a][none][none] > " {
		t.Fatalf("expected prompt reset after clear, got %q", env.Prompt())
	}
}

func TestDispatch_RemovedUnselectAliasIsUnknown(t *testing.T) {
	env := makeTestEnv()
	env.SetRange([]repl.LastObject{
		{Kind: repl.KindPod, Name: "pod-0", Namespace: "default"},
		{Kind: repl.KindPod, Name: "pod-1", Namespace: "default"},
	})
	cmds := BuildCommands()
	if err := repl.Dispatch(env, cmds, "unselect"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(env.CurrentSelection()) != 2 {
		t.Fatalf("expected removed alias to leave range selection unchanged, got %+v", env.CurrentSelection())
	}
	if env.Prompt() != "[ctx-a][none][2 Pods selected] > " {
		t.Fatalf("expected prompt unchanged after removed alias, got %q", env.Prompt())
	}
}

func TestDispatch_ExitAlias(t *testing.T) {
	env := makeTestEnv()
	cmds := BuildCommands()
	_ = repl.Dispatch(env, cmds, "exit")
	if !env.IsQuit() {
		t.Error("expected env.quit to be true after 'exit' alias")
	}
}

func TestDispatch_NumericSelectsPod(t *testing.T) {
	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{
		{Kind: repl.KindPod, Name: "pod-0", Namespace: "default"},
	})
	cmds := BuildCommands()
	_ = captureStdout(t, func() {
		if err := repl.Dispatch(env, cmds, "0"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if env.CurrentObject() == nil || env.CurrentObject().Kind != repl.KindPod || env.CurrentObject().Name != "pod-0" {
		t.Errorf("expected pod-0 selected, got %v", env.CurrentObject())
	}
}

func TestDispatch_NumericSelectsNamespace(t *testing.T) {
	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{
		{Kind: repl.KindNamespace, Name: "kube-system"},
	})
	cmds := BuildCommands()
	_ = captureStdout(t, func() {
		if err := repl.Dispatch(env, cmds, "0"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if env.Namespace() != "" {
		t.Errorf("expected working namespace unchanged, got %q", env.Namespace())
	}
	if env.CurrentObject() == nil || env.CurrentObject().Kind != repl.KindNamespace || env.CurrentObject().Name != "kube-system" {
		t.Errorf("expected namespace object selected, got %v", env.CurrentObject())
	}
}

func TestDispatch_NumericSelectsNode(t *testing.T) {
	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{
		{Kind: repl.KindNode, Name: "node-a"},
	})
	cmds := BuildCommands()
	_ = captureStdout(t, func() {
		if err := repl.Dispatch(env, cmds, "0"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if env.CurrentObject() == nil || env.CurrentObject().Kind != repl.KindNode || env.CurrentObject().Name != "node-a" {
		t.Errorf("expected node-a selected, got %v", env.CurrentObject())
	}
}

func TestDispatch_NumericOutOfRange(t *testing.T) {
	env := makeTestEnv()
	cmds := BuildCommands()
	if err := repl.Dispatch(env, cmds, "99"); err == nil {
		t.Fatal("expected error for out-of-range index")
	}
}

func TestDispatch_NumericSelectsDynamicObject(t *testing.T) {
	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{
		{
			Kind:      repl.KindDynamic,
			Name:      "widget-a",
			Namespace: "default",
			Dynamic: &repl.DynamicResourceDescriptor{
				Resource:     "widgets",
				GroupVersion: "example.com/v1",
				Kind:         "Widget",
				Namespaced:   true,
			},
		},
	})
	cmds := BuildCommands()
	_ = captureStdout(t, func() {
		if err := repl.Dispatch(env, cmds, "0"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if env.CurrentObject() == nil || env.CurrentObject().Kind != repl.KindDynamic || env.CurrentObject().Name != "widget-a" {
		t.Errorf("expected dynamic object selected, got %v", env.CurrentObject())
	}
}

func TestHelpListsCommandsAlphabetically(t *testing.T) {
	cmds := BuildCommands()
	help := newHelpCmd(cmds)

	out := captureStdout(t, func() {
		if err := help.runE(makeTestEnv(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	contextIdx := strings.Index(out, "\n  context")
	contextsIdx := strings.Index(out, "\n  contexts")
	crdIdx := strings.Index(out, "\n  crd")
	deleteIdx := strings.Index(out, "\n  delete")

	if contextIdx == -1 || contextsIdx == -1 || crdIdx == -1 || deleteIdx == -1 {
		t.Fatalf("expected help output to contain sorted command entries, got:\n%s", out)
	}

	if contextIdx >= contextsIdx || contextsIdx >= crdIdx || crdIdx >= deleteIdx {
		t.Fatalf("expected help output to be alphabetical, got:\n%s", out)
	}
}

func TestHelpIncludesSelectionDocumentation(t *testing.T) {
	cmds := BuildCommands()
	help := newHelpCmd(cmds)

	out := captureStdout(t, func() {
		if err := help.runE(makeTestEnv(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{
		"Selection shortcuts:",
		"<index>        select a single item from the most recent list",
		"<start>..<end> select a range of items from the most recent list (example: 2..5)",
		"<a>, <b>, ...  select multiple specific items by index, separated by commas (example: 1, 3, 5)",
		"Use 'range' to show the current multi-selection and 'clear' to remove any active selection.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected help output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestBuildCommands_HaveShortAndLongDescriptions(t *testing.T) {
	cmds := BuildCommands()
	for _, cmd := range cmds {
		meta := helpMetadataForCommand(cmd)
		if strings.TrimSpace(meta.shortHelp()) == "" {
			t.Fatalf("expected %q to have a short description", cmd.Name())
		}
		if strings.TrimSpace(meta.longHelp()) == "" {
			t.Fatalf("expected %q to have a long description", cmd.Name())
		}
	}
}

func TestHelpCommand_ShowsDetailedHelpForCommand(t *testing.T) {
	cmds := BuildCommands()
	help := newHelpCmd(cmds)

	out := captureStdout(t, func() {
		if err := help.Execute(makeTestEnv(), []string{"logs"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{
		"Usage:\n  logs [flags] [container]",
		"Stream logs from the active pod or iterate the current pod range selection.",
		"Examples:",
		"--follow",
		"--editor",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected detailed help output to contain %q, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Selection shortcuts:") {
		t.Fatalf("did not expect selection help in per-command output, got:\n%s", out)
	}
}

func TestHelpCommand_ResolvesAlias(t *testing.T) {
	cmds := BuildCommands()
	help := newHelpCmd(cmds)

	out := captureStdout(t, func() {
		if err := help.Execute(makeTestEnv(), []string{"l"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Usage:\n  logs [flags] [container]") {
		t.Fatalf("expected alias help to resolve to logs usage, got:\n%s", out)
	}
}

func TestHelpCommand_UnknownCommand(t *testing.T) {
	cmds := BuildCommands()
	help := newHelpCmd(cmds)

	err := help.Execute(makeTestEnv(), []string{"notacommand"})
	if err == nil || !strings.Contains(err.Error(), `no help available for command "notacommand"`) {
		t.Fatalf("expected unknown help error, got %v", err)
	}
}

func TestHelpCommand_MatchesCommandHelpOutput(t *testing.T) {
	env := makeTestEnv()
	cmds := BuildCommands()
	help := newHelpCmd(cmds)

	helpOut := captureStdout(t, func() {
		if err := help.Execute(env, []string{"logs"}); err != nil {
			t.Fatalf("unexpected help error: %v", err)
		}
	})
	logsOut := captureStdout(t, func() {
		if err := newLogsCmd().Execute(env, []string{"--help"}); err != nil {
			t.Fatalf("unexpected logs help error: %v", err)
		}
	})

	if helpOut != logsOut {
		t.Fatalf("expected `help logs` and `logs --help` output to match\nhelp output:\n%s\nlogs output:\n%s", helpOut, logsOut)
	}
}

func TestDispatch_RangeSelectsObjects(t *testing.T) {
	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{
		{Kind: repl.KindPod, Name: "pod-0", Namespace: "default"},
		{Kind: repl.KindPod, Name: "pod-1", Namespace: "default"},
		{Kind: repl.KindPod, Name: "pod-2", Namespace: "default"},
	})
	cmds := BuildCommands()
	if err := repl.Dispatch(env, cmds, "1..2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !env.HasRangeSelection() {
		t.Fatal("expected range selection")
	}
	if len(env.CurrentSelection()) != 2 {
		t.Fatalf("expected 2 selected objects, got %d", len(env.CurrentSelection()))
	}
}

func TestDispatch_CommaListSelectsObjects(t *testing.T) {
	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{
		{Kind: repl.KindPod, Name: "pod-0", Namespace: "default"},
		{Kind: repl.KindPod, Name: "pod-1", Namespace: "default"},
		{Kind: repl.KindPod, Name: "pod-2", Namespace: "default"},
	})
	cmds := BuildCommands()
	if err := repl.Dispatch(env, cmds, "0, 2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := len(env.CurrentSelection()); got != 2 {
		t.Fatalf("expected 2 selected objects, got %d", got)
	}
}

func TestLogsCommand_NoPod(t *testing.T) {
	env := makeTestEnv()
	cmd := newLogsCmd()
	err := cmd.Execute(env, nil)
	if err == nil {
		t.Fatal("expected error when no pod is active")
	}
}
