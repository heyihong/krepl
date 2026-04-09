package repl

import (
	"fmt"
	"strconv"
	"strings"
)

// Dispatch parses one input line and routes to the matching command.
// If the line is a bare non-negative integer, it selects that row from the
// last list (pods -> active pod, namespaces -> selected namespace object).
func Dispatch(env *Env, commands []Command, line string) error {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	parts := strings.Fields(line)
	name := parts[0]
	args := parts[1:]

	// Bare integer: numeric selection from last list.
	if len(parts) == 1 {
		if n, err := strconv.Atoi(name); err == nil && n >= 0 {
			return env.SelectByIndex(n)
		}
		if indices, ok := TryParseRange(name, len(env.lastObjects)); ok {
			env.SetRangeByIndices(indices)
			return nil
		}
	}

	if indices, ok := TryParseCSL(line); ok {
		env.SetRangeByIndices(indices)
		return nil
	}

	cmd := FindCommand(commands, name)
	if cmd == nil {
		fmt.Printf("Unknown command: %q. Type 'help' for available commands.\n", name)
		return nil
	}
	return cmd.Execute(env, args)
}
