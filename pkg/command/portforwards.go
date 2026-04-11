package command

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"k8s.io/client-go/rest"
	k8sportforward "k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	"github.com/heyihong/krepl/pkg/config"
	"github.com/heyihong/krepl/pkg/portforward"
	"github.com/heyihong/krepl/pkg/repl"
	"github.com/heyihong/krepl/pkg/table"
)

type portForwarder interface {
	ForwardPorts() error
}

type portForwarderFactory func(
	restConfig *rest.Config,
	requestURL *url.URL,
	ports []string,
	stopCh <-chan struct{},
	readyCh chan struct{},
	stdout io.Writer,
	stderr io.Writer,
) (portForwarder, error)

var newPortForwarder portForwarderFactory = func(
	restConfig *rest.Config,
	requestURL *url.URL,
	ports []string,
	stopCh <-chan struct{},
	readyCh chan struct{},
	stdout io.Writer,
	stderr io.Writer,
) (portForwarder, error) {
	transport, upgrader, err := spdy.RoundTripperFor(restConfig)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Transport: transport}
	dialer := spdy.NewDialer(upgrader, httpClient, http.MethodPost, requestURL)
	return k8sportforward.NewOnAddresses(dialer, []string{"localhost"}, ports, stopCh, readyCh, stdout, stderr)
}

var readPortForwardConfirmation = func() (string, error) {
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return line, nil
}

func newPortForwardCmd() *cmd {
	return &cmd{
		use:     "port-forward PORT [PORT...]",
		aliases: []string{"pf"},
		short:   "forward one or more local ports to the active pod",
		long: "Start one or more background port-forwards from localhost to the active pod.\n" +
			"Each port argument may be `remote` or `local:remote`, and the created session can be inspected or stopped with `port-forwards`.",
		args: minimumNArgs(1),
		runE: runPortForward,
	}
}

// runPortForward starts one or more background forwards for the active pod.
func runPortForward(env *repl.Env, args []string) error {
	obj := env.CurrentObject()
	if obj == nil || obj.Kind != repl.KindPod {
		return fmt.Errorf("no active pod; select one by number after running `pods`")
	}
	if env.CurrentContext() == "" {
		return fmt.Errorf("no active context")
	}

	ports := make([]string, 0, len(args))
	for _, arg := range args {
		port, err := normalizePortForwardSpec(arg)
		if err != nil {
			return err
		}
		ports = append(ports, port)
	}

	restConfig, err := config.BuildRESTConfigForContext(env.RawConfig(), env.CurrentContext())
	if err != nil {
		return fmt.Errorf("build rest config: %w", err)
	}

	requestURL, err := portForwardURL(restConfig, obj.Namespace, obj.Name)
	if err != nil {
		return fmt.Errorf("build port-forward request: %w", err)
	}

	session := portforward.NewSession(obj.Name, obj.Namespace, ports)
	output := portforward.NewOutputWriter(session)
	forwarder, err := newPortForwarder(restConfig, requestURL, ports, session.StopChannel(), session.ReadyChannel(), output, output)
	if err != nil {
		return fmt.Errorf("create port forwarder: %w", err)
	}

	errCh := make(chan error, 1)
	go func() {
		err := forwarder.ForwardPorts()
		session.MarkExited(err)
		errCh <- err
	}()

	select {
	case <-session.ReadyChannel():
		session.MarkRunning()
		env.AddPortForward(session)
		fmt.Printf("Forwarding port(s): %s\n", strings.Join(ports, ", "))
		return nil
	case err := <-errCh:
		if err == nil {
			return fmt.Errorf("port forward exited before becoming ready")
		}
		return fmt.Errorf("start port forward: %w", err)
	}
}

func normalizePortForwardSpec(value string) (string, error) {
	parts := strings.Split(value, ":")
	if len(parts) > 2 {
		return "", fmt.Errorf("invalid port specification %q: can only contain one ':'", value)
	}
	if len(parts) == 1 {
		if err := validatePortNumber(parts[0], value); err != nil {
			return "", err
		}
		return value, nil
	}
	if parts[0] == "" {
		parts[0] = "0"
	}
	if err := validatePortNumber(parts[0], value); err != nil {
		return "", err
	}
	if err := validatePortNumber(parts[1], value); err != nil {
		return "", err
	}
	return parts[0] + ":" + parts[1], nil
}

func validatePortNumber(part, original string) error {
	if part == "" {
		return fmt.Errorf("invalid port specification %q", original)
	}
	if _, err := strconv.Atoi(part); err != nil {
		return fmt.Errorf("invalid port specification %q: %q is not numeric", original, part)
	}
	return nil
}

func portForwardURL(restConfig *rest.Config, namespace, podName string) (*url.URL, error) {
	baseURL, err := url.Parse(restConfig.Host)
	if err != nil {
		return nil, err
	}
	baseURL.Path = strings.TrimRight(baseURL.Path, "/") + fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", namespace, podName)
	return baseURL, nil
}

func newPortForwardsCmd() *cmd {
	return &cmd{
		use:     "port-forwards [list|output|stop] [index]",
		aliases: []string{"pfs"},
		short:   "list or control active port forwards",
		long: "List active port-forward sessions, show buffered output for a session, or stop a session by index.\n" +
			"Use `port-forward` to create new sessions before managing them here.",
		args: arbitraryArgs,
		runE: runPortForwards,
	}
}

// runPortForwards lists or controls managed port-forward sessions.
func runPortForwards(env *repl.Env, args []string) error {
	action, index, err := parsePortForwardsArgs(args)
	if err != nil {
		return err
	}

	if action == "list" {
		printPortForwards(env.PortForwards())
		return nil
	}

	session := env.PortForward(index)
	if session == nil {
		fmt.Println("Invalid index (try without args to get a list)")
		return nil
	}

	fmt.Printf("Pod: %s, Port(s): %s", session.PodName, session.PortsString())
	if action == "output" {
		fmt.Printf(" Output:%s\n", session.Output())
		return nil
	}

	fmt.Print("  [y/N]? ")
	response, err := readPortForwardConfirmation()
	if err != nil {
		return fmt.Errorf("read port-forward confirmation: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(response)) {
	case "y", "yes":
		if err := env.StopPortForward(index); err != nil {
			return err
		}
		fmt.Println("Stopped")
	default:
		fmt.Println("Not stopping")
	}
	return nil
}

func parsePortForwardsArgs(args []string) (action string, index int, err error) {
	if len(args) == 0 {
		return "list", -1, nil
	}
	action = args[0]
	switch action {
	case "list":
		if len(args) != 1 {
			return "", 0, fmt.Errorf("list does not take an index")
		}
		return action, -1, nil
	case "output", "stop":
		if len(args) != 2 {
			return "", 0, fmt.Errorf("%s requires an index", action)
		}
		index, err = strconv.Atoi(args[1])
		if err != nil || index < 0 {
			return "", 0, fmt.Errorf("invalid index %q", args[1])
		}
		return action, index, nil
	default:
		return "", 0, fmt.Errorf("unknown action: %q", action)
	}
}

func printPortForwards(sessions []*portforward.Session) {
	if len(sessions) == 0 {
		fmt.Println("No active port forwards, see `port-forward -h` for help creating one")
		return
	}

	tbl := &table.Table{Columns: []table.Column{
		colIndex, colPod, colPorts, colStatus,
	}}
	for i, session := range sessions {
		tbl.AddRow(strconv.Itoa(i), session.PodName, session.PortsString(), session.StatusString())
	}
	tbl.Render()
}

var _ = context.Background
