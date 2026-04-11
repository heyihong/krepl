package command

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/heyihong/krepl/pkg/config"
	"github.com/heyihong/krepl/pkg/repl"
	"github.com/heyihong/krepl/pkg/table"
)

var fetchEventsForNamespace = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName, namespace string) ([]corev1.Event, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	list, err := client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return append([]corev1.Event(nil), list.Items...), nil
}

func newEventsCmd() *cmd {
	return &cmd{
		use:   "events",
		short: "list events; scoped to the active object when one is selected, otherwise lists namespace events",
		long: "List Kubernetes events in the active context.\n" +
			"When an object or range selection is active, only events for those objects are shown; otherwise the command lists events for the working namespace or all namespaces when no namespace is set.",
		args: noArgs,
		runE: runEvents,
	}
}

// runEvents lists events, optionally scoped to the active selected object.
// When an object is selected, only events for that object are shown.
// When no object is selected, all events for the current namespace are shown.
func runEvents(env *repl.Env, _ []string) error {
	if env.CurrentContext() == "" {
		return fmt.Errorf("no active context; use `context <name>` to set one")
	}
	if err := rejectNamespaceSelectionForList(env, "events"); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if len(env.CurrentSelection()) > 0 {
		return env.ApplyToSelection(func(obj repl.LastObject) error {
			return printEventsForObject(ctx, env, obj)
		})
	}
	return printEventsForNamespace(ctx, env)
}

func printEventsForObject(ctx context.Context, env *repl.Env, obj repl.LastObject) error {
	events, err := fetchDescribeEvents(ctx, env.RawConfig(), env.CurrentContext(), obj)
	if err != nil {
		return fmt.Errorf("list events: %w", err)
	}

	sortEvents(events)

	if len(events) == 0 {
		fmt.Printf("No events found for %s %q.\n", describeKindName(obj.Kind), obj.Name)
		return nil
	}

	t := &table.Table{Columns: []table.Column{
		colLastSeen, colType, colReason, colMessage,
	}}

	now := time.Now()
	for _, ev := range events {
		t.AddRow(
			formatAge(eventTime(ev), now),
			fallbackString(ev.Type, "<none>"),
			fallbackString(ev.Reason, "<none>"),
			fallbackString(strings.TrimSpace(ev.Message), "<none>"),
		)
	}
	t.Render()
	return nil
}

func printEventsForNamespace(ctx context.Context, env *repl.Env) error {
	namespace := env.Namespace()
	if namespace == "" {
		namespace = metav1.NamespaceAll
	}

	events, err := fetchEventsForNamespace(ctx, env.RawConfig(), env.CurrentContext(), namespace)
	if err != nil {
		return fmt.Errorf("list events: %w", err)
	}

	sortEvents(events)

	if len(events) == 0 {
		fmt.Println("No events found.")
		return nil
	}

	t := &table.Table{Columns: []table.Column{
		colLastSeen, colType, colReason, colObject, colMessage,
	}}

	now := time.Now()
	for _, ev := range events {
		object := fmt.Sprintf("%s/%s", ev.InvolvedObject.Kind, ev.InvolvedObject.Name)
		t.AddRow(
			formatAge(eventTime(ev), now),
			fallbackString(ev.Type, "<none>"),
			fallbackString(ev.Reason, "<none>"),
			object,
			fallbackString(strings.TrimSpace(ev.Message), "<none>"),
		)
	}
	t.Render()
	return nil
}

func sortEvents(events []corev1.Event) {
	sort.Slice(events, func(i, j int) bool {
		return eventTime(events[i]).Before(eventTime(events[j]))
	})
}

func eventTime(ev corev1.Event) time.Time {
	if !ev.LastTimestamp.IsZero() {
		return ev.LastTimestamp.Time
	}
	if !ev.EventTime.IsZero() {
		return ev.EventTime.Time
	}
	if !ev.FirstTimestamp.IsZero() {
		return ev.FirstTimestamp.Time
	}
	return time.Time{}
}
