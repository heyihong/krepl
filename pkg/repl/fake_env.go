package repl

import "github.com/heyihong/krepl/pkg/config"

func MakeFakeEnv() *Env {
	return NewEnv(config.MakeFakeConfig())
}
