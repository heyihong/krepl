package portforward

import (
	"fmt"
	"io"
	"strings"
	"sync"
)

// Status tracks the lifecycle of a managed port-forward session.
type Status int

const (
	Starting Status = iota
	Running
	Stopped
	Exited
)

const maxOutputBytes = 16 * 1024

// Session stores REPL-managed state for an active or completed
// port-forward operation.
type Session struct {
	PodName   string
	Namespace string
	Ports     []string

	stopCh  chan struct{}
	readyCh chan struct{}

	mu        sync.Mutex
	status    Status
	exitErr   error
	stopOnce  sync.Once
	outputBuf string
}

// NewSession creates a new managed port-forward session.
func NewSession(podName, namespace string, ports []string) *Session {
	return &Session{
		PodName:   podName,
		Namespace: namespace,
		Ports:     append([]string(nil), ports...),
		stopCh:    make(chan struct{}),
		readyCh:   make(chan struct{}),
		status:    Starting,
	}
}

// StopChannel returns the signal channel used to stop the underlying forwarder.
func (s *Session) StopChannel() chan struct{} { return s.stopCh }

// ReadyChannel returns the channel closed by the underlying forwarder when ready.
func (s *Session) ReadyChannel() chan struct{} { return s.readyCh }

// MarkRunning records that the forwarder became ready.
func (s *Session) MarkRunning() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status == Starting {
		s.status = Running
	}
}

// MarkExited records that the forwarder terminated.
func (s *Session) MarkExited(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status == Stopped {
		return
	}
	if err == nil {
		s.status = Stopped
		s.exitErr = nil
		return
	}
	s.status = Exited
	s.exitErr = err
}

// Stop requests that the session stop and marks it as stopped.
func (s *Session) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = Stopped
	s.exitErr = nil
}

// AppendOutput records forwarder output while bounding total retained text.
func (s *Session) AppendOutput(text string) {
	if text == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.outputBuf += text
	if len(s.outputBuf) > maxOutputBytes {
		s.outputBuf = s.outputBuf[len(s.outputBuf)-maxOutputBytes:]
	}
}

// Output returns the retained forwarder output.
func (s *Session) Output() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.outputBuf
}

// Status returns the session lifecycle status.
func (s *Session) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// ExitError returns the terminal error, if any.
func (s *Session) ExitError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.exitErr
}

// StatusString formats the current lifecycle state for table output.
func (s *Session) StatusString() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch s.status {
	case Starting:
		return "Starting"
	case Running:
		return "Running"
	case Stopped:
		return "Stopped"
	case Exited:
		if s.exitErr != nil {
			return fmt.Sprintf("Exited: %v", s.exitErr)
		}
		return "Exited"
	default:
		return "Unknown"
	}
}

// PortsString formats the configured ports for display.
func (s *Session) PortsString() string {
	return strings.Join(s.Ports, ", ")
}

type outputWriter struct {
	session *Session
}

// NewOutputWriter returns an io.Writer sink for forwarder output.
func NewOutputWriter(session *Session) io.Writer {
	return &outputWriter{session: session}
}

func (w *outputWriter) Write(p []byte) (int, error) {
	w.session.AppendOutput(string(p))
	return len(p), nil
}
