package command

import (
	"fmt"

	"github.com/heyihong/krepl/pkg/config"
	"github.com/heyihong/krepl/pkg/repl"
	"github.com/heyihong/krepl/pkg/styles"
)

var deleteKubeconfigContext = config.DeleteContext
var setKubeconfigCurrentContext = config.SetCurrentContext

func printContexts(env *repl.Env) {
	names := env.ListContextNames()
	if len(names) == 0 {
		fmt.Println("No contexts found in kubeconfig.")
		return
	}
	for _, name := range names {
		if name == env.CurrentContext() {
			fmt.Printf("%s %s\n", styles.ActiveMarker("*"), name)
		} else {
			fmt.Printf("  %s\n", name)
		}
	}
}

func newContextCmd() *cmd {
	return &cmd{
		use:     "context <ctx_name>",
		aliases: []string{"ctx"},
		short:   "switch to the named context",
		long: "Switch the REPL to the named context.\n" +
			"Use `contexts` to list available contexts.",
		args: exactArgs(1),
		runE: func(env *repl.Env, args []string) error {
			if err := setKubeconfigCurrentContext(args[0]); err != nil {
				return err
			}
			if err := env.SetContext(args[0]); err != nil {
				return err
			}
			fmt.Printf("Switched to context: %s\n", args[0])
			return nil
		},
	}
}

func newContextsCmd() *cmd {
	return &cmd{
		use:     "contexts",
		aliases: []string{"ctxs"},
		short:   "list available contexts",
		long: "List all contexts from the kubeconfig.\n" +
			"The active context is marked in the output and can be changed with `context <name>`.",
		args: noArgs,
		runE: func(env *repl.Env, _ []string) error {
			printContexts(env)
			return nil
		},
	}
}

func newDeleteContextCmd() *cmd {
	return &cmd{
		use:   "delete-context <ctx_name>",
		short: "delete a kubeconfig context",
		long: "Delete a named context from the kubeconfig.\n" +
			"The active context cannot be deleted; switch to a different context first.",
		args: exactArgs(1),
		runE: func(env *repl.Env, args []string) error {
			name := args[0]
			if name == env.CurrentContext() {
				return fmt.Errorf("cannot delete current context %q", name)
			}
			if err := deleteKubeconfigContext(name); err != nil {
				return err
			}
			if err := env.DeleteContext(name); err != nil {
				return err
			}
			fmt.Printf("Deleted context: %s\n", name)
			return nil
		},
	}
}
