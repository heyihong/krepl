package command

import (
	"fmt"

	"github.com/heyihong/krepl/pkg/repl"
	"github.com/heyihong/krepl/pkg/table"
)

func newRangeCmd() *cmd {
	return &cmd{
		use:   "range",
		short: "list the objects in the current selection",
		long: "Show the objects in the current range selection.\n" +
			"Use this after index, range, or comma-separated selection shortcuts to confirm which objects are active.",
		args: noArgs,
		runE: func(env *repl.Env, _ []string) error {
			selection := env.CurrentSelection()
			if len(selection) == 0 {
				return fmt.Errorf("no objects currently active")
			}

			t := &table.Table{Columns: []table.Column{
				colName, colType, colNamespace,
			}}
			for _, obj := range selection {
				t.AddRow(obj.Name, describeObjectKindName(obj), obj.Namespace)
			}
			t.Render()
			return nil
		},
	}
}
