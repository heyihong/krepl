package command

import (
	"github.com/heyihong/krepl/pkg/repl"
)

type registeredCommand struct {
	exemplar repl.Command
	factory  func() repl.Command
}

var _ repl.Command = registeredCommand{}
var _ helpMetadata = registeredCommand{}

func (c registeredCommand) Name() string {
	return c.exemplar.Name()
}

func (c registeredCommand) Aliases() []string {
	return c.exemplar.Aliases()
}

func (c registeredCommand) shortHelp() string {
	return helpMetadataForCommand(c.exemplar).shortHelp()
}

func (c registeredCommand) longHelp() string {
	return helpMetadataForCommand(c.exemplar).longHelp()
}

func (c registeredCommand) usage() string {
	return helpMetadataForCommand(c.exemplar).usage()
}

func (c registeredCommand) Execute(env *repl.Env, args []string) error {
	return c.factory().Execute(env, args)
}

// BuildCommands returns the ordered slice of all registered commands.
func BuildCommands() []repl.Command {
	factories := []func() repl.Command{
		func() repl.Command { return newClearCmd() },
		func() repl.Command { return newContextCmd() },
		func() repl.Command { return newContextsCmd() },
		func() repl.Command { return newDeleteContextCmd() },
		func() repl.Command { return newNamespaceCmd() },
		func() repl.Command { return newNamespacesCmd() },
		func() repl.Command { return newPodsCmd() },
		func() repl.Command { return newNodesCmd() },
		func() repl.Command { return newDeploymentsCmd() },
		func() repl.Command { return newReplicaSetsCmd() },
		func() repl.Command { return newStatefulSetsCmd() },
		func() repl.Command { return newConfigMapsCmd() },
		func() repl.Command { return newSecretsCmd() },
		func() repl.Command { return newJobsCmd() },
		func() repl.Command { return newCronJobsCmd() },
		func() repl.Command { return newDaemonSetsCmd() },
		func() repl.Command { return newEventsCmd() },
		func() repl.Command { return newPersistentVolumesCmd() },
		func() repl.Command { return newServicesCmd() },
		func() repl.Command { return newStorageClassesCmd() },
		func() repl.Command { return newCrdCmd() },
		func() repl.Command { return newDescribeCmd() },
		func() repl.Command { return newLogsCmd() },
		func() repl.Command { return newExecCmd() },
		func() repl.Command { return newPortForwardCmd() },
		func() repl.Command { return newPortForwardsCmd() },
		func() repl.Command { return newDeleteCmd() },
		func() repl.Command { return newCordonCmd() },
		func() repl.Command { return newDrainCmd() },
		func() repl.Command { return newUncordonCmd() },
		func() repl.Command { return newEditCmd() },
		func() repl.Command { return newRangeCmd() },
		func() repl.Command { return newQuitCmd() },
	}

	cmds := make([]repl.Command, 0, len(factories)+1)
	for _, factory := range factories {
		cmds = append(cmds, registeredCommand{
			exemplar: factory(),
			factory:  factory,
		})
	}

	helpFactory := func() repl.Command { return newHelpCmd(cmds) }
	cmds = append(cmds, registeredCommand{
		exemplar: helpFactory(),
		factory:  helpFactory,
	})

	return cmds
}
