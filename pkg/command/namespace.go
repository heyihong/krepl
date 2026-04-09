package command

import (
	"fmt"
	"github.com/heyihong/krepl/pkg/repl"
	"regexp"
)

var rfc1123Re = regexp.MustCompile(`^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`)

func newNamespaceCmd() *cmd {
	return &cmd{
		use:     "namespace [ns_name]",
		aliases: []string{"ns"},
		short:   "set working namespace (no argument to clear)",
		long: "Set the working namespace used by namespaced commands.\n" +
			"Run without an argument to clear the namespace filter and return to all namespaces where supported.",
		args: maximumNArgs(1),
		runE: func(env *repl.Env, args []string) error {
			if len(args) == 0 {
				env.SetNamespace("")
				fmt.Println("Namespace cleared.")
				return nil
			}
			if err := validateRFC1123Label(args[0]); err != nil {
				return err
			}
			env.SetNamespace(args[0])
			fmt.Printf("Namespace set to: %s\n", args[0])
			return nil
		},
	}
}

func validateRFC1123Label(s string) error {
	if len(s) == 0 {
		return fmt.Errorf("namespace name must not be empty")
	}
	if len(s) > 63 {
		return fmt.Errorf("namespace name too long (max 63 characters): %q", s)
	}
	if !rfc1123Re.MatchString(s) {
		return fmt.Errorf("invalid namespace name %q: must be lowercase alphanumeric or '-', and must start and end with an alphanumeric character", s)
	}
	return nil
}
