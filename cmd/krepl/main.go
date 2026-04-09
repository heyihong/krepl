package main

import (
	"fmt"
	"github.com/heyihong/krepl/pkg/command"
	"github.com/heyihong/krepl/pkg/config"
	"github.com/heyihong/krepl/pkg/repl"
	"os"
)

func main() {
	rawConfig, err := config.LoadRawConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error loading kubeconfig:", err)
		os.Exit(1)
	}

	env := repl.NewEnv(rawConfig)
	commands := command.BuildCommands()

	if err := repl.Run(env, commands); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
