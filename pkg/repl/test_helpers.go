package repl

import (
	"github.com/heyihong/krepl/pkg/config"
	"github.com/heyihong/krepl/pkg/testutils"
)

var (
	makeTestConfig = config.MakeFakeConfig
	makeTestEnv    = MakeFakeEnv
	captureStdout  = testutils.CaptureStdout
)
