package command

import "github.com/heyihong/krepl/pkg/repl"

func newClearCmd() *cmd {
	return &cmd{
		use:   "clear",
		short: "clear the current object selection",
		long: "Clear the current object or range selection.\n" +
			"This does not change the active context or working namespace.",
		args: noArgs,
		runE: func(env *repl.Env, _ []string) error {
			env.ClearSelection()
			return nil
		},
	}
}
