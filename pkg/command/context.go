package command

import (
	"fmt"

	"github.com/heyihong/krepl/pkg/repl"
	"github.com/heyihong/krepl/pkg/styles"
)

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
		use:     "context [ctx_name]",
		aliases: []string{"ctx"},
		short:   "list available contexts, or switch to the named context",
		long: "List contexts from the kubeconfig, or switch the REPL to the named context.\n" +
			"Switching context updates the active cluster target for subsequent commands.",
		args: maximumNArgs(1),
		runE: func(env *repl.Env, args []string) error {
			if len(args) == 0 {
				printContexts(env)
				return nil
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
