package command

import (
	"github.com/heyihong/krepl/pkg/config"
	"github.com/heyihong/krepl/pkg/repl"
	"github.com/heyihong/krepl/pkg/testutils"
)

var (
	makeTestConfig = config.MakeFakeConfig
	makeTestEnv    = repl.MakeFakeEnv
	captureStdout  = testutils.CaptureStdout
)
