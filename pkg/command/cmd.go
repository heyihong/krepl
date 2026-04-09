package command

import (
	"fmt"
	"strings"

	"github.com/heyihong/krepl/pkg/repl"
	"github.com/spf13/pflag"
)

// positionalArgs validates positional arguments after flag parsing.
// Return a non-nil error to reject the invocation.
type positionalArgs func(args []string) error

// noArgs rejects any positional arguments.
func noArgs(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown argument(s): %v", args)
	}
	return nil
}

// arbitraryArgs accepts any number of positional arguments.
func arbitraryArgs(_ []string) error { return nil }

// exactArgs requires exactly n positional arguments.
func exactArgs(n int) positionalArgs {
	return func(args []string) error {
		if len(args) != n {
			return fmt.Errorf("accepts %d arg(s), received %d", n, len(args))
		}
		return nil
	}
}

// minimumNArgs requires at least n positional arguments.
func minimumNArgs(n int) positionalArgs {
	return func(args []string) error {
		if len(args) < n {
			return fmt.Errorf("requires at least %d arg(s), received %d", n, len(args))
		}
		return nil
	}
}

// maximumNArgs requires at most n positional arguments.
func maximumNArgs(n int) positionalArgs {
	return func(args []string) error {
		if len(args) > n {
			return fmt.Errorf("accepts at most %d arg(s), received %d", n, len(args))
		}
		return nil
	}
}

// cmd implements the Command interface.
// Command files construct one via a newXxxCmd() factory function,
// configure it via direct field assignment and cmd.flags().XxxVarP() calls.
type cmd struct {
	// Use is the one-line usage string; the first word is the primary name.
	// Example: "logs [flags] [container]"
	use string

	// aliases are alternate names accepted by findCommand.
	aliases []string

	// Short is the one-line description shown in `help` listings.
	short string

	// Long is the detailed description shown by --help. Falls back to Short.
	long string

	// Example is shown in the --help output under an "Examples:" section.
	example string

	// args is the positional-argument validator. Defaults to arbitraryArgs if nil.
	args positionalArgs

	// runE is the body of the command. Flags are already parsed; positional
	// args have been validated. Return an error to report failure.
	runE func(env *repl.Env, args []string) error

	// flagSet is lazily initialised in flags().
	flagSet *pflag.FlagSet
}

var _ repl.Command = (*cmd)(nil)
var _ helpMetadata = (*cmd)(nil)

// Name returns the primary name (first word of Use).
// Implements the Command interface.
func (c *cmd) Name() string {
	fields := strings.Fields(c.use)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// Aliases returns the command's aliases.
// Implements the Command interface.
func (c *cmd) Aliases() []string {
	if c.aliases == nil {
		return []string{}
	}
	return c.aliases
}

// shortHelp returns the short description used in `help` listings.
func (c *cmd) shortHelp() string {
	return c.short
}

// longHelp returns the detailed description shown in full command help.
func (c *cmd) longHelp() string {
	return c.long
}

// usage returns the formatted help text shown by --help.
func (c *cmd) usage() string {
	return c.usageString()
}

// flags lazily initialises and returns the command's FlagSet, with -h/--help
// pre-registered. Uses ContinueOnError so Execute can intercept flag errors
// without os.Exit terminating the REPL process.
func (c *cmd) flags() *pflag.FlagSet {
	if c.flagSet == nil {
		c.flagSet = pflag.NewFlagSet(c.Name(), pflag.ContinueOnError)
		// Suppress pflag's built-in usage output; we print our own via usageString.
		c.flagSet.Usage = func() {}
		c.flagSet.BoolP("help", "h", false, "show help for this command")
	}
	return c.flagSet
}

// Execute implements the Command interface. It:
//  1. Resets all flag values to their defaults so repeated REPL invocations
//     don't accumulate state across calls.
//  2. Parses args with pflag.
//  3. Handles -h/--help by printing usageString() and returning nil.
//  4. Validates positional arguments with c.args.
//  5. Calls c.runE.
func (c *cmd) Execute(env *repl.Env, args []string) error {
	fs := c.flags()
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w\n\n%s", err, c.usageString())
	}

	// Handle -h / --help.
	if help, _ := fs.GetBool("help"); help {
		fmt.Print(c.usageString())
		return nil
	}

	positional := fs.Args()

	validator := c.args
	if validator == nil {
		validator = arbitraryArgs
	}
	if err := validator(positional); err != nil {
		return fmt.Errorf("%w\n\n%s", err, c.usageString())
	}

	if c.runE == nil {
		return nil
	}
	return c.runE(env, positional)
}

// usageString produces the formatted help text printed by --help or on error.
func (c *cmd) usageString() string {
	var b strings.Builder

	b.WriteString("Usage:\n  ")
	b.WriteString(c.use)
	b.WriteString("\n\n")

	description := c.long
	if description == "" {
		description = c.short
	}
	if description != "" {
		b.WriteString(description)
		b.WriteString("\n\n")
	}

	if c.example != "" {
		b.WriteString("Examples:\n")
		for _, line := range strings.Split(c.example, "\n") {
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("Flags:\n")
	b.WriteString(c.flags().FlagUsages())

	return b.String()
}
