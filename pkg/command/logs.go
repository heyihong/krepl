package command

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/heyihong/krepl/pkg/config"
	"github.com/heyihong/krepl/pkg/repl"
)

var runLogsForObject = streamLogsForObject
var newFollowLogsContext = func() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt)
}

func newLogsCmd() *cmd {
	var follow, previous bool
	var tail int64 = -1
	var editor string

	cmd := &cmd{
		use:     "logs [flags] [container]",
		aliases: []string{"l"},
		short:   "stream logs from the active pod",
		long: "Stream logs from the active pod or iterate the current pod range selection.\n" +
			"Optionally specify a container name as a positional argument. Use flags to follow, limit, or open logs in an editor.",
		example: "logs -f\nlogs --tail 100 sidecar\nlogs --editor vim",
		args:    maximumNArgs(1),
	}
	cmd.flags().BoolVarP(&follow, "follow", "f", false, "follow log output")
	cmd.flags().BoolVarP(&previous, "previous", "p", false, "show logs from the previous container instance")
	cmd.flags().Int64VarP(&tail, "tail", "t", -1, "number of lines to show from the end (-1 = all)")
	cmd.flags().StringVarP(&editor, "editor", "e", "", "open logs in this editor instead of streaming")

	cmd.runE = func(env *repl.Env, args []string) error {
		if editor != "" && follow {
			return fmt.Errorf("--editor cannot be used with --follow")
		}
		if len(env.CurrentSelection()) == 0 {
			return fmt.Errorf("no active pod; select one by number after running `pods`")
		}
		if env.CurrentContext() == "" {
			return fmt.Errorf("no active context")
		}
		if err := requireSelectionKind(env, repl.KindPod, "logs"); err != nil {
			return err
		}
		if env.HasRangeSelection() && follow {
			return fmt.Errorf("range selection is not supported with --follow; select a single pod")
		}
		if env.HasRangeSelection() && editor != "" {
			return fmt.Errorf("range selection is not supported with --editor; select a single pod")
		}

		podOpts := &corev1.PodLogOptions{
			Follow:   follow,
			Previous: previous,
		}
		if tail >= 0 {
			n := tail
			podOpts.TailLines = &n
		}
		if len(args) == 1 {
			podOpts.Container = args[0]
		}
		resolvedEditor := ""
		if editor != "" {
			resolvedEditor = resolveEditor(editor)
		}

		return env.ApplyToSelection(func(obj repl.LastObject) error {
			return runLogsForObject(env, obj, podOpts, resolvedEditor)
		})
	}
	return cmd
}

func streamLogsForObject(env *repl.Env, obj repl.LastObject, podOpts *corev1.PodLogOptions, resolvedEditor string) error {
	client, err := config.BuildClientForContext(env.RawConfig(), env.CurrentContext())
	if err != nil {
		return fmt.Errorf("build client: %w", err)
	}

	ctx, cancel := logsStreamContext(podOpts.Follow)
	defer cancel()

	req := client.CoreV1().Pods(obj.Namespace).GetLogs(obj.Name, podOpts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return fmt.Errorf("get logs: %w", err)
	}
	defer func() {
		_ = stream.Close()
	}()

	if resolvedEditor == "" {
		_, err = io.Copy(os.Stdout, stream)
		if podOpts.Follow && ctx.Err() == context.Canceled {
			return nil
		}
		return err
	}

	// Editor mode: collect logs into a temp file, then open the editor.
	tmpFile, err := os.CreateTemp("", "krepl-logs-*.log")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if _, err := io.Copy(tmpFile, stream); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write logs to temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := env.SuspendTerminal(); err != nil {
		return fmt.Errorf("suspend repl terminal: %w", err)
	}
	editorErr := launchEditorProcess(resolvedEditor, tmpPath)
	if resumeErr := env.ResumeTerminal(); resumeErr != nil {
		return fmt.Errorf("resume repl terminal: %w", resumeErr)
	}
	env.RequestTerminalReset()

	return editorErr
}

func logsStreamContext(follow bool) (context.Context, context.CancelFunc) {
	if follow {
		return newFollowLogsContext()
	}
	return context.WithTimeout(context.Background(), 30*time.Second)
}
