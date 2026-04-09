package repl

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"
)

// Run starts the interactive REPL loop.
func Run(env *Env, commands []Command) error {
	rl, err := newReadlineInstance(env)
	if err != nil {
		return err
	}
	defer func() {
		_ = rl.Close()
	}()

	for !env.IsQuit() {
		// Update prompt dynamically before each read (context/namespace may have changed).
		rl.SetPrompt(env.Prompt())

		line, err := rl.Readline()
		if err == readline.ErrInterrupt {
			// Ctrl-C: clear the line and continue (matches Rust's Interrupted => {})
			continue
		}
		if err == io.EOF {
			// Ctrl-D: exit cleanly
			fmt.Println()
			break
		}
		if err != nil {
			return err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if err := Dispatch(env, commands, line); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
		}
		if env.ConsumeTerminalReset() {
			if err := rl.Close(); err != nil {
				return fmt.Errorf("close readline: %w", err)
			}
			rl, err = newReadlineInstance(env)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func newReadlineInstance(env *Env) (*readline.Instance, error) {
	rl, err := readline.NewEx(&readline.Config{
		Prompt:            env.Prompt(),
		HistoryFile:       historyPath(),
		HistorySearchFold: true,
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
	})
	if err != nil {
		return nil, fmt.Errorf("readline init: %w", err)
	}

	env.SetTerminalHooks(
		func() error {
			rl.Clean()
			return rl.Terminal.ExitRawMode()
		},
		func() error {
			rl.Clean()
			return nil
		},
	)

	return rl, nil
}

// historyPath returns the path to the REPL history file (~/.kube/krepl.history).
func historyPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".kube", "krepl.history")
}
