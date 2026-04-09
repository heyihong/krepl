package command

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/heyihong/krepl/pkg/config"
	"github.com/heyihong/krepl/pkg/repl"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

type execOptions struct {
	container string
	tty       bool
	stdin     bool
	command   []string
}

type execStreamer interface {
	StreamWithContext(ctx context.Context, options remotecommand.StreamOptions) error
}

type execStreamerFactory func(restConfig *rest.Config, method string, url *url.URL) (execStreamer, error)

var newExecStreamer execStreamerFactory = func(restConfig *rest.Config, method string, url *url.URL) (execStreamer, error) {
	return buildExecStreamer(restConfig, method, url)
}

var makeExecTerminalRaw = term.MakeRaw

var restoreExecTerminal = term.Restore

var isExecTerminal = term.IsTerminal

type execTTYReadCloser interface {
	io.ReadCloser
	Fd() uintptr
}

var openExecTTY = func() (execTTYReadCloser, error) {
	return os.Open("/dev/tty")
}

var runExecTTYPump = pumpExecTTYInput

var newWebSocketExec = func(restConfig *rest.Config, method string, execURL string) (remotecommand.Executor, error) {
	return remotecommand.NewWebSocketExecutor(restConfig, method, execURL)
}

var newSPDYExec = func(restConfig *rest.Config, method string, execURL *url.URL) (remotecommand.Executor, error) {
	return remotecommand.NewSPDYExecutor(restConfig, method, execURL)
}

var newFallbackExec = func(primary, secondary remotecommand.Executor, shouldFallback func(error) bool) (remotecommand.Executor, error) {
	return remotecommand.NewFallbackExecutor(primary, secondary, shouldFallback)
}

var shouldFallbackExec = httpstream.IsUpgradeFailure

var execRuntimeErrorHandlers = &utilruntime.ErrorHandlers

var execRuntimeErrorHandlersMu sync.Mutex

// ExecCommand runs a command in the currently selected pod via the Kubernetes exec API.
// Unlike Click, krepl implements this directly with client-go instead of invoking kubectl.
func newExecCmd() *cmd {
	var container string
	tty := true
	stdin := true

	cmd := &cmd{
		use:   "exec [flags] [--] command...",
		short: "run a command on the active pod via the Kubernetes exec API",
		long: "Run a command on the active pod via the Kubernetes exec API.\n" +
			"Use -- to separate exec flags from the remote command.\n" +
			"Note: to disable TTY or stdin, use --tty=false or --stdin=false.",
		example: "exec -- sh\nexec -c sidecar -- bash\nexec --tty=false -- cat /etc/os-release",
		args:    minimumNArgs(1),
	}
	cmd.flags().StringVarP(&container, "container", "c", "", "container name (default: first container)")
	cmd.flags().BoolVarP(&tty, "tty", "T", true, "allocate a TTY")
	cmd.flags().BoolVarP(&stdin, "stdin", "i", true, "pass stdin to the container")

	cmd.runE = func(env *repl.Env, args []string) error {
		if len(env.CurrentSelection()) == 0 {
			return fmt.Errorf("no active pod; select one by number after running `pods`")
		}
		if env.HasRangeSelection() {
			return fmt.Errorf("range selection is not supported for exec; select a single pod")
		}
		obj := env.CurrentObject()
		if obj == nil || obj.Kind != repl.KindPod {
			return fmt.Errorf("exec requires a single selected pod")
		}
		if env.CurrentContext() == "" {
			return fmt.Errorf("no active context")
		}

		opts := execOptions{
			container: container,
			tty:       tty,
			stdin:     stdin,
			command:   args,
		}

		restConfig, err := config.BuildRESTConfigForContext(env.RawConfig(), env.CurrentContext())
		if err != nil {
			return fmt.Errorf("build rest config: %w", err)
		}

		client, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			return fmt.Errorf("build client: %w", err)
		}

		req := client.CoreV1().RESTClient().
			Post().
			Resource("pods").
			Name(obj.Name).
			Namespace(obj.Namespace).
			SubResource("exec").
			VersionedParams(&corev1.PodExecOptions{
				Container: opts.container,
				Command:   opts.command,
				Stdin:     opts.stdin,
				Stdout:    true,
				Stderr:    true,
				TTY:       opts.tty,
			}, scheme.ParameterCodec)

		streamer, err := newExecStreamer(restConfig, "POST", req.URL())
		if err != nil {
			return fmt.Errorf("create exec streamer: %w", err)
		}

		streamOptions := remotecommand.StreamOptions{
			Stdout: os.Stdout,
			Stderr: os.Stderr,
			Tty:    opts.tty,
		}
		interactive := opts.tty || opts.stdin
		rawTTY := opts.tty && opts.stdin && isExecTerminal(int(os.Stdin.Fd()))
		stdinReader, closeInput, err := prepareExecInput(opts.stdin, rawTTY)
		if err != nil {
			return err
		}
		if opts.stdin {
			streamOptions.Stdin = stdinReader
		}

		var oldTerminalState *term.State
		if interactive {
			if err := env.SuspendTerminal(); err != nil {
				return fmt.Errorf("suspend repl terminal: %w", err)
			}
			restoreRuntimeHandlers := suppressBenignExecRuntimeErrors()
			defer restoreRuntimeHandlers()
			if rawTTY {
				oldTerminalState, err = makeExecTerminalRaw(int(os.Stdin.Fd()))
				if err != nil {
					_ = env.ResumeTerminal()
					return fmt.Errorf("enter exec raw mode: %w", err)
				}
			}
		}

		streamErr := streamer.StreamWithContext(context.Background(), streamOptions)
		closeInput()
		if interactive {
			if rawTTY {
				if err := restoreExecTerminal(int(os.Stdin.Fd()), oldTerminalState); err != nil {
					_ = env.ResumeTerminal()
					return fmt.Errorf("restore exec terminal: %w", err)
				}
			}
			if err := env.ResumeTerminal(); err != nil {
				return fmt.Errorf("resume repl terminal: %w", err)
			}
			env.RequestTerminalReset()
		}
		if streamErr != nil {
			return fmt.Errorf("stream exec session: %w", streamErr)
		}
		return nil
	}
	return cmd
}

func buildExecStreamer(restConfig *rest.Config, method string, execURL *url.URL) (execStreamer, error) {
	websocketExec, err := newWebSocketExec(restConfig, method, execURL.String())
	if err != nil {
		return nil, err
	}

	spdyExec, err := newSPDYExec(restConfig, method, execURL)
	if err != nil {
		return nil, err
	}

	return newFallbackExec(websocketExec, spdyExec, shouldFallbackExec)
}

func suppressBenignExecRuntimeErrors() func() {
	execRuntimeErrorHandlersMu.Lock()

	original := append([]func(error){}, (*execRuntimeErrorHandlers)...)
	filtered := make([]func(error), 0, len(original))
	for _, handler := range original {
		h := handler
		filtered = append(filtered, func(err error) {
			if isBenignExecRuntimeError(err) {
				return
			}
			h(err)
		})
	}
	*execRuntimeErrorHandlers = filtered

	return func() {
		*execRuntimeErrorHandlers = original
		execRuntimeErrorHandlersMu.Unlock()
	}
}

func isBenignExecRuntimeError(err error) bool {
	if err == nil {
		return false
	}
	if err == io.EOF {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "write on closed stream") || msg == "EOF"
}

func prepareExecInput(enableStdin bool, interactiveTTY bool) (io.Reader, func(), error) {
	if !enableStdin {
		return nil, func() {}, nil
	}
	if !interactiveTTY {
		return os.Stdin, func() {}, nil
	}

	src, err := openExecTTY()
	if err != nil {
		return nil, func() {}, fmt.Errorf("open exec tty: %w", err)
	}

	reader, writer := io.Pipe()
	var once sync.Once
	var pumpDone sync.WaitGroup
	stopCh := make(chan struct{})
	pumpDone.Add(1)
	stop := func() {
		once.Do(func() {
			close(stopCh)
			_ = src.Close()
			_ = writer.Close()
		})
	}
	closeAll := func() {
		stop()
		pumpDone.Wait()
	}

	go func() {
		defer pumpDone.Done()
		defer stop()
		if err := runExecTTYPump(src, writer, stopCh); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrClosedPipe) {
			_ = writer.CloseWithError(err)
		}
	}()

	return reader, closeAll, nil
}

func pumpExecTTYInput(src execTTYReadCloser, writer *io.PipeWriter, stopCh <-chan struct{}) error {
	fd := int(src.Fd())
	if err := unix.SetNonblock(fd, true); err != nil {
		return err
	}

	buf := make([]byte, 4096)
	for {
		select {
		case <-stopCh:
			return nil
		default:
		}

		readN, readErr := src.Read(buf)
		if readN > 0 {
			if _, writeErr := writer.Write(buf[:readN]); writeErr != nil {
				return writeErr
			}
		}
		if readErr != nil {
			if errors.Is(readErr, unix.EAGAIN) || errors.Is(readErr, unix.EWOULDBLOCK) {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return readErr
		}
	}
}
