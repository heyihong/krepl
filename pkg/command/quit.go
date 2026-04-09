package command

import "github.com/heyihong/krepl/pkg/repl"

func newQuitCmd() *cmd {
	return &cmd{
		use:     "quit",
		aliases: []string{"exit", "q"},
		short:   "exit the REPL",
		long: "Exit the REPL session.\n" +
			"Any active managed port-forward sessions are stopped before the process terminates.",
		args: noArgs,
		runE: func(env *repl.Env, _ []string) error {
			env.StopAllPortForwards()
			env.Quit()
			return nil
		},
	}
}
