package command

import (
	"fmt"
	"slices"
	"strings"

	"github.com/heyihong/krepl/pkg/repl"
)

const selectionHelp = `Selection shortcuts:
  <index>        select a single item from the most recent list
  <start>..<end> select a range of items from the most recent list (example: 2..5)
  <a>, <b>, ...  select multiple specific items by index, separated by commas (example: 1, 3, 5)

Use 'range' to show the current multi-selection and 'clear' to remove any active selection.`

type helpMetadata interface {
	shortHelp() string
	longHelp() string
	usage() string
}

func helpMetadataForCommand(cmd repl.Command) helpMetadata {
	meta, ok := cmd.(helpMetadata)
	if !ok {
		panic(fmt.Sprintf("command %T does not expose help metadata", cmd))
	}
	return meta
}

func newHelpCmd(cmds []repl.Command) *cmd {
	return &cmd{
		use:     "help [command]",
		aliases: []string{"?"},
		short:   "list available commands; use `help <command>` for detailed help",
		long: "List available commands with their short descriptions.\n" +
			"Pass a command name or alias to show that command's detailed help, including usage, flags, and examples.",
		args: maximumNArgs(1),
		runE: func(_ *repl.Env, args []string) error {
			if len(args) == 1 {
				cmd := repl.FindCommand(cmds, args[0])
				if cmd == nil {
					return fmt.Errorf("no help available for command %q", args[0])
				}
				fmt.Print(helpMetadataForCommand(cmd).usage())
				return nil
			}

			fmt.Println("Available commands:")
			sortedCmds := slices.Clone(cmds)
			slices.SortFunc(sortedCmds, func(a, b repl.Command) int {
				return strings.Compare(a.Name(), b.Name())
			})
			for _, cmd := range sortedCmds {
				aliases := ""
				if len(cmd.Aliases()) > 0 {
					aliases = fmt.Sprintf(" (%s)", strings.Join(cmd.Aliases(), ", "))
				}
				fmt.Printf("  %-12s%s  — %s\n", cmd.Name(), aliases, helpMetadataForCommand(cmd).shortHelp())
			}
			fmt.Printf("\n%s\n", selectionHelp)
			return nil
		},
	}
}
