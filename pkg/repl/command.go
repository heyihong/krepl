package repl

// Command is the interface all REPL commands implement.
type Command interface {
	Name() string
	Aliases() []string
	Execute(env *Env, args []string) error
}

// FindCommand finds a command by primary name or alias. Returns nil if not found.
func FindCommand(commands []Command, name string) Command {
	for _, c := range commands {
		if c.Name() == name {
			return c
		}
		for _, a := range c.Aliases() {
			if a == name {
				return c
			}
		}
	}
	return nil
}
