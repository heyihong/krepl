package command

import (
	"context"
	"errors"
	"fmt"
	"github.com/heyihong/krepl/pkg/config"
	"github.com/heyihong/krepl/pkg/repl"
	"io"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/term"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

type fakeExecStreamer struct {
	streamErr error
	calls     []remotecommand.StreamOptions
}

func (f *fakeExecStreamer) StreamWithContext(_ context.Context, options remotecommand.StreamOptions) error {
	f.calls = append(f.calls, options)
	return f.streamErr
}

type fakeExecutor struct{}

func (f *fakeExecutor) Stream(_ remotecommand.StreamOptions) error {
	return nil
}

func (f *fakeExecutor) StreamWithContext(_ context.Context, _ remotecommand.StreamOptions) error {
	return nil
}

type fakeBlockingReadCloser struct {
	dataCh  chan []byte
	closeCh chan struct{}
	closeMu sync.Mutex
	closed  bool
	closeWg sync.WaitGroup
	fd      uintptr
}

func newFakeBlockingReadCloser() *fakeBlockingReadCloser {
	f := &fakeBlockingReadCloser{
		dataCh:  make(chan []byte, 1),
		closeCh: make(chan struct{}),
	}
	f.closeWg.Add(1)
	return f
}

func (f *fakeBlockingReadCloser) Read(p []byte) (int, error) {
	select {
	case data := <-f.dataCh:
		n := copy(p, data)
		return n, nil
	case <-f.closeCh:
		return 0, io.EOF
	}
}

func (f *fakeBlockingReadCloser) Close() error {
	f.closeMu.Lock()
	defer f.closeMu.Unlock()
	if !f.closed {
		f.closed = true
		close(f.closeCh)
		f.closeWg.Done()
	}
	return nil
}

func (f *fakeBlockingReadCloser) WaitClosed() {
	f.closeWg.Wait()
}

func (f *fakeBlockingReadCloser) IsClosed() bool {
	f.closeMu.Lock()
	defer f.closeMu.Unlock()
	return f.closed
}

func (f *fakeBlockingReadCloser) Fd() uintptr {
	return f.fd
}

func TestExecCommand_NoPod(t *testing.T) {
	env := makeTestEnv()
	cmd := newExecCmd()

	err := cmd.Execute(env, []string{"sh"})
	if err == nil {
		t.Fatal("expected error when no pod is active")
	}
}

func TestExecCommand_NoContext(t *testing.T) {
	env := makeExecTestEnvNoContext(t)
	cmd := newExecCmd()

	err := cmd.Execute(env, []string{"sh"})
	if err == nil {
		t.Fatal("expected error when no context is active")
	}
}

func TestExecCmd_UnknownFlag(t *testing.T) {
	env := makeTestEnv()
	err := newExecCmd().Execute(env, []string{"--bogus", "sh"})
	if err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("expected unknown flag error, got %v", err)
	}
}

func TestExecCmd_RequiresCommand(t *testing.T) {
	// MinimumNArgs(1) is enforced before RunE.
	env := makeTestEnv()
	err := newExecCmd().Execute(env, []string{"--container", "app"})
	if err == nil || !strings.Contains(err.Error(), "requires at least 1 arg") {
		t.Fatalf("expected missing command error, got %v", err)
	}
}

func TestBuildExecStreamer_UsesWebSocketPrimaryWithUpgradeFallback(t *testing.T) {
	restConfig := &rest.Config{Host: "https://cluster.example.com"}
	execURL, err := url.Parse("https://cluster.example.com/api/v1/namespaces/default/pods/pod-0/exec")
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	var called []string
	var capturedShouldFallback func(error) bool
	oldWebSocket := newWebSocketExec
	oldSPDY := newSPDYExec
	oldFallback := newFallbackExec
	oldShouldFallback := shouldFallbackExec
	newWebSocketExec = func(cfg *rest.Config, method string, rawURL string) (remotecommand.Executor, error) {
		called = append(called, "websocket")
		if cfg != restConfig || method != "POST" || rawURL != execURL.String() {
			t.Fatalf("unexpected websocket args: cfg=%p method=%q url=%q", cfg, method, rawURL)
		}
		return &fakeExecutor{}, nil
	}
	newSPDYExec = func(cfg *rest.Config, method string, parsedURL *url.URL) (remotecommand.Executor, error) {
		called = append(called, "spdy")
		if cfg != restConfig || method != "POST" || parsedURL.String() != execURL.String() {
			t.Fatalf("unexpected spdy args: cfg=%p method=%q url=%q", cfg, method, parsedURL.String())
		}
		return &fakeExecutor{}, nil
	}
	newFallbackExec = func(primary, secondary remotecommand.Executor, shouldFallback func(error) bool) (remotecommand.Executor, error) {
		called = append(called, "fallback")
		capturedShouldFallback = shouldFallback
		return &fakeExecutor{}, nil
	}
	shouldFallbackExec = httpstream.IsUpgradeFailure
	defer func() {
		newWebSocketExec = oldWebSocket
		newSPDYExec = oldSPDY
		newFallbackExec = oldFallback
		shouldFallbackExec = oldShouldFallback
	}()

	streamer, err := buildExecStreamer(restConfig, "POST", execURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if streamer == nil {
		t.Fatal("expected streamer")
	}
	if strings.Join(called, ",") != "websocket,spdy,fallback" {
		t.Fatalf("unexpected constructor order: %v", called)
	}
	if capturedShouldFallback == nil {
		t.Fatal("expected fallback predicate")
	}
	if !capturedShouldFallback(&httpstream.UpgradeFailureError{Cause: errors.New("upgrade failed")}) {
		t.Fatal("expected upgrade failure to trigger fallback")
	}
	if capturedShouldFallback(errors.New("plain error")) {
		t.Fatal("expected non-upgrade errors not to trigger fallback")
	}
}

func TestIsBenignExecRuntimeError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "eof sentinel", err: io.EOF, want: true},
		{name: "eof text", err: errors.New("EOF"), want: true},
		{name: "closed stream", err: errors.New("write on closed stream 0"), want: true},
		{name: "other", err: errors.New("permission denied"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBenignExecRuntimeError(tt.err); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestPrepareExecInput_InteractiveTTYClosesDedicatedReader(t *testing.T) {
	fakeTTY := newFakeBlockingReadCloser()
	oldOpenTTY := openExecTTY
	oldPump := runExecTTYPump
	openExecTTY = func() (execTTYReadCloser, error) {
		return fakeTTY, nil
	}
	runExecTTYPump = func(src execTTYReadCloser, writer *io.PipeWriter, stopCh <-chan struct{}) error {
		f := src.(*fakeBlockingReadCloser)
		buf := make([]byte, 1)
		for {
			select {
			case <-stopCh:
				return nil
			default:
			}
			_, err := f.Read(buf)
			if err != nil {
				return err
			}
		}
	}
	defer func() {
		openExecTTY = oldOpenTTY
		runExecTTYPump = oldPump
	}()

	reader, cleanup, err := prepareExecInput(true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	readDone := make(chan error, 1)
	go func() {
		buf := make([]byte, 1)
		_, err := reader.Read(buf)
		readDone <- err
	}()

	cleanup()
	fakeTTY.WaitClosed()

	select {
	case err := <-readDone:
		if err != io.EOF {
			t.Fatalf("expected EOF after cleanup, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected reader to unblock after cleanup")
	}
}

func TestExecCommand_ClosesInteractiveInputBeforeResume(t *testing.T) {
	env := makeExecTestEnv(t)
	fakeStreamer := &fakeExecStreamer{}
	fakeTTY := newFakeBlockingReadCloser()
	var ttyClosed bool
	var calls []string

	oldOpenTTY := openExecTTY
	oldIsTerminal := isExecTerminal
	oldMakeRaw := makeExecTerminalRaw
	oldRestore := restoreExecTerminal
	oldFactory := newExecStreamer
	oldPump := runExecTTYPump
	openExecTTY = func() (execTTYReadCloser, error) {
		return fakeTTY, nil
	}
	isExecTerminal = func(_ int) bool { return true }
	makeExecTerminalRaw = func(_ int) (*term.State, error) { return &term.State{}, nil }
	restoreExecTerminal = func(_ int, _ *term.State) error { return nil }
	newExecStreamer = func(_ *rest.Config, _ string, _ *url.URL) (execStreamer, error) {
		return fakeStreamer, nil
	}
	runExecTTYPump = func(src execTTYReadCloser, writer *io.PipeWriter, stopCh <-chan struct{}) error {
		<-stopCh
		return nil
	}
	env.SetTerminalHooks(
		func() error {
			calls = append(calls, "suspend")
			return nil
		},
		func() error {
			ttyClosed = fakeTTY.IsClosed()
			calls = append(calls, "resume")
			return nil
		},
	)
	defer func() {
		openExecTTY = oldOpenTTY
		isExecTerminal = oldIsTerminal
		makeExecTerminalRaw = oldMakeRaw
		restoreExecTerminal = oldRestore
		newExecStreamer = oldFactory
		runExecTTYPump = oldPump
	}()

	if err := (newExecCmd()).Execute(env, []string{"sh"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ttyClosed {
		t.Fatal("expected dedicated exec tty to be closed before resume")
	}
	if len(calls) != 2 || calls[0] != "suspend" || calls[1] != "resume" {
		t.Fatalf("unexpected terminal hook calls: %v", calls)
	}
}

func TestSuppressBenignExecRuntimeErrors_FiltersOnlyBenignErrors(t *testing.T) {
	originalHandlersRef := execRuntimeErrorHandlers
	var seen []string
	handlers := []func(error){
		func(err error) { seen = append(seen, "h1:"+err.Error()) },
		func(err error) { seen = append(seen, "h2:"+err.Error()) },
	}
	execRuntimeErrorHandlers = &handlers
	defer func() { execRuntimeErrorHandlers = originalHandlersRef }()

	restore := suppressBenignExecRuntimeErrors()
	defer restore()

	for _, handler := range handlers {
		handler(errors.New("write on closed stream 0"))
	}
	if len(seen) != 0 {
		t.Fatalf("expected benign errors to be filtered, got %v", seen)
	}

	for _, handler := range handlers {
		handler(errors.New("real failure"))
	}
	if len(seen) != 2 || seen[0] != "h1:real failure" || seen[1] != "h2:real failure" {
		t.Fatalf("expected non-benign errors to pass through, got %v", seen)
	}
}

func TestSuppressBenignExecRuntimeErrors_RestoresOriginalHandlers(t *testing.T) {
	originalHandlersRef := execRuntimeErrorHandlers
	handlers := []func(error){func(error) {}}
	execRuntimeErrorHandlers = &handlers
	defer func() { execRuntimeErrorHandlers = originalHandlersRef }()

	restore := suppressBenignExecRuntimeErrors()
	filteredPtr := *execRuntimeErrorHandlers
	restore()

	if len(*execRuntimeErrorHandlers) != 1 {
		t.Fatalf("expected original handlers restored, got %d handlers", len(*execRuntimeErrorHandlers))
	}
	if len(filteredPtr) != 1 && len(filteredPtr) != len(*execRuntimeErrorHandlers) {
		t.Fatalf("unexpected filtered handlers state")
	}
	if fmt.Sprintf("%p", (*execRuntimeErrorHandlers)[0]) == fmt.Sprintf("%p", filteredPtr[0]) {
		t.Fatal("expected restored handler to differ from wrapped filtered handler")
	}
}

func TestExecCommand_BuildsExecRequestAndStreams(t *testing.T) {
	env := makeExecTestEnv(t)

	restConfig, err := config.BuildRESTConfigForContext(env.RawConfig(), env.CurrentContext())
	if err != nil {
		t.Fatalf("build rest config: %v", err)
	}

	fakeStreamer := &fakeExecStreamer{}
	var gotMethod string
	var gotURL *url.URL

	oldFactory := newExecStreamer
	newExecStreamer = func(cfg *rest.Config, method string, execURL *url.URL) (execStreamer, error) {
		if cfg.Host != restConfig.Host {
			t.Fatalf("expected rest config host %q, got %q", restConfig.Host, cfg.Host)
		}
		gotMethod = method
		gotURL = execURL
		return fakeStreamer, nil
	}
	defer func() { newExecStreamer = oldFactory }()

	cmd := newExecCmd()
	if err := cmd.Execute(env, []string{"--container", "app", "--tty=false", "--stdin=true", "--", "printenv"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != "POST" {
		t.Fatalf("expected POST, got %q", gotMethod)
	}
	if gotURL == nil {
		t.Fatal("expected exec URL")
	}
	if gotURL.Path != "/api/v1/namespaces/default/pods/pod-0/exec" {
		t.Fatalf("unexpected path: %q", gotURL.Path)
	}

	query := gotURL.Query()
	if query.Get("container") != "app" {
		t.Fatalf("expected container app, got %q", query.Get("container"))
	}
	if query.Get("stdin") != "true" || query.Get("stdout") != "true" || query.Get("stderr") != "true" {
		t.Fatalf("unexpected stream flags: %v", query)
	}
	if query.Get("tty") != "" {
		t.Fatalf("expected tty to be omitted when false, got %q", query.Get("tty"))
	}
	if got := query["command"]; len(got) != 1 || got[0] != "printenv" {
		t.Fatalf("unexpected command query: %#v", got)
	}

	if len(fakeStreamer.calls) != 1 {
		t.Fatalf("expected one stream call, got %d", len(fakeStreamer.calls))
	}
	call := fakeStreamer.calls[0]
	if call.Stdin != os.Stdin || call.Stdout != os.Stdout || call.Stderr != os.Stderr {
		t.Fatal("expected stdio to be passed through")
	}
	if call.Tty {
		t.Fatal("expected tty false in stream options")
	}
}

func TestExecCommand_SuspendsAndResumesTerminalForInteractiveExec(t *testing.T) {
	env := makeExecTestEnv(t)
	fakeStreamer := &fakeExecStreamer{}
	var calls []string
	fakeTTY := newFakeBlockingReadCloser()
	oldMakeRaw := makeExecTerminalRaw
	oldRestore := restoreExecTerminal
	oldIsTerminal := isExecTerminal
	oldOpenTTY := openExecTTY
	oldPump := runExecTTYPump
	isExecTerminal = func(_ int) bool { return true }
	openExecTTY = func() (execTTYReadCloser, error) { return fakeTTY, nil }
	runExecTTYPump = func(src execTTYReadCloser, writer *io.PipeWriter, stopCh <-chan struct{}) error {
		<-stopCh
		return nil
	}
	makeExecTerminalRaw = func(_ int) (*term.State, error) {
		calls = append(calls, "make-raw")
		return &term.State{}, nil
	}
	restoreExecTerminal = func(_ int, _ *term.State) error {
		calls = append(calls, "restore-raw")
		return nil
	}
	defer func() {
		makeExecTerminalRaw = oldMakeRaw
		restoreExecTerminal = oldRestore
		isExecTerminal = oldIsTerminal
		openExecTTY = oldOpenTTY
		runExecTTYPump = oldPump
	}()
	env.SetTerminalHooks(
		func() error {
			calls = append(calls, "suspend")
			return nil
		},
		func() error {
			calls = append(calls, "resume")
			return nil
		},
	)

	oldFactory := newExecStreamer
	newExecStreamer = func(_ *rest.Config, _ string, _ *url.URL) (execStreamer, error) {
		return fakeStreamer, nil
	}
	defer func() { newExecStreamer = oldFactory }()

	cmd := newExecCmd()
	if err := cmd.Execute(env, []string{"sh"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(calls) != 4 || calls[0] != "suspend" || calls[1] != "make-raw" || calls[2] != "restore-raw" || calls[3] != "resume" {
		t.Fatalf("unexpected terminal hook calls: %v", calls)
	}
	if !env.ConsumeTerminalReset() {
		t.Fatal("expected interactive exec to request terminal reset")
	}
}

func TestExecCommand_DoesNotSuspendTerminalForNonInteractiveExec(t *testing.T) {
	env := makeExecTestEnv(t)
	fakeStreamer := &fakeExecStreamer{}
	var called bool
	env.SetTerminalHooks(
		func() error {
			called = true
			return nil
		},
		func() error {
			called = true
			return nil
		},
	)

	oldFactory := newExecStreamer
	newExecStreamer = func(_ *rest.Config, _ string, _ *url.URL) (execStreamer, error) {
		return fakeStreamer, nil
	}
	defer func() { newExecStreamer = oldFactory }()

	cmd := newExecCmd()
	if err := cmd.Execute(env, []string{"--tty=false", "--stdin=false", "printenv"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if called {
		t.Fatal("expected terminal hooks to remain unused for non-interactive exec")
	}
	if env.ConsumeTerminalReset() {
		t.Fatal("expected non-interactive exec not to request terminal reset")
	}
}

func TestExecCommand_DoesNotEnterRawModeWithoutTTY(t *testing.T) {
	env := makeExecTestEnv(t)
	fakeStreamer := &fakeExecStreamer{}
	var rawCalled bool

	oldMakeRaw := makeExecTerminalRaw
	oldRestore := restoreExecTerminal
	oldIsTerminal := isExecTerminal
	isExecTerminal = func(_ int) bool { return true }
	makeExecTerminalRaw = func(_ int) (*term.State, error) {
		rawCalled = true
		return &term.State{}, nil
	}
	restoreExecTerminal = func(_ int, _ *term.State) error {
		rawCalled = true
		return nil
	}
	defer func() {
		makeExecTerminalRaw = oldMakeRaw
		restoreExecTerminal = oldRestore
		isExecTerminal = oldIsTerminal
	}()

	oldFactory := newExecStreamer
	newExecStreamer = func(_ *rest.Config, _ string, _ *url.URL) (execStreamer, error) {
		return fakeStreamer, nil
	}
	defer func() { newExecStreamer = oldFactory }()

	cmd := newExecCmd()
	if err := cmd.Execute(env, []string{"--tty=false", "--stdin=true", "cat"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rawCalled {
		t.Fatal("expected raw terminal mode to remain unused when tty=false")
	}
}

func TestExecCommand_DisablesStdinWhenRequested(t *testing.T) {
	env := makeExecTestEnv(t)
	fakeStreamer := &fakeExecStreamer{}

	oldFactory := newExecStreamer
	newExecStreamer = func(_ *rest.Config, _ string, _ *url.URL) (execStreamer, error) {
		return fakeStreamer, nil
	}
	defer func() { newExecStreamer = oldFactory }()

	cmd := newExecCmd()
	if err := cmd.Execute(env, []string{"--stdin=false", "sh"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(fakeStreamer.calls) != 1 {
		t.Fatalf("expected one stream call, got %d", len(fakeStreamer.calls))
	}
	if fakeStreamer.calls[0].Stdin != nil {
		t.Fatal("expected stdin to be nil when disabled")
	}
	if !fakeStreamer.calls[0].Tty {
		t.Fatal("expected tty default true")
	}
}

func TestExecCommand_WrapsFactoryFailure(t *testing.T) {
	env := makeExecTestEnv(t)

	oldFactory := newExecStreamer
	newExecStreamer = func(_ *rest.Config, _ string, _ *url.URL) (execStreamer, error) {
		return nil, errors.New("factory boom")
	}
	defer func() { newExecStreamer = oldFactory }()

	cmd := newExecCmd()
	err := cmd.Execute(env, []string{"sh"})
	if err == nil || !strings.Contains(err.Error(), "create exec streamer") {
		t.Fatalf("expected wrapped factory error, got %v", err)
	}
}

func TestExecCommand_WrapsStreamFailure(t *testing.T) {
	env := makeExecTestEnv(t)
	fakeStreamer := &fakeExecStreamer{streamErr: errors.New("stream boom")}

	oldFactory := newExecStreamer
	newExecStreamer = func(_ *rest.Config, _ string, _ *url.URL) (execStreamer, error) {
		return fakeStreamer, nil
	}
	defer func() { newExecStreamer = oldFactory }()

	cmd := newExecCmd()
	err := cmd.Execute(env, []string{"sh"})
	if err == nil || !strings.Contains(err.Error(), "stream exec session") {
		t.Fatalf("expected wrapped stream error, got %v", err)
	}
}

func makeExecTestEnv(t *testing.T) *repl.Env {
	env := makeTestEnv()
	env.SetLastObjects([]repl.LastObject{{Kind: repl.KindPod, Name: "pod-0", Namespace: "default"}})
	_ = captureStdout(t, func() {
		_ = env.SelectByIndex(0)
	})
	return env
}

func makeExecTestEnvNoContext(t *testing.T) *repl.Env {
	cfg := makeTestConfig()
	cfg.CurrentContext = ""
	env := repl.NewEnv(cfg)
	env.SetLastObjects([]repl.LastObject{{Kind: repl.KindPod, Name: "pod-0", Namespace: "default"}})
	_ = captureStdout(t, func() {
		_ = env.SelectByIndex(0)
	})
	return env
}
