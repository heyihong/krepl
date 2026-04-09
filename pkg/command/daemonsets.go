package command

import (
	"context"
	"fmt"
	"github.com/heyihong/krepl/pkg/config"
	"github.com/heyihong/krepl/pkg/repl"
	"github.com/heyihong/krepl/pkg/table"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var listDaemonSetsForContext = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName, namespace string) ([]appsv1.DaemonSet, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	list, err := client.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return append([]appsv1.DaemonSet(nil), list.Items...), nil
}

func newDaemonSetsCmd() *cmd {
	return &cmd{
		use:     "daemonsets",
		aliases: []string{"ds"},
		short:   "list daemonsets in the current namespace",
		long: "List daemonsets in the active context and working namespace.\n" +
			"Rows can be selected for follow-up commands such as `describe`, `events`, and `delete`.",
		args: noArgs,
		runE: runDaemonSets,
	}
}

// runDaemonSets lists daemonsets in the current context/namespace.
func runDaemonSets(env *repl.Env, _ []string) error {
	if env.CurrentContext() == "" {
		return fmt.Errorf("no active context; use `context <name>` to set one")
	}
	if err := rejectNamespaceSelectionForList(env, "daemonsets"); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dsList, err := listDaemonSetsForContext(ctx, env.RawConfig(), env.CurrentContext(), env.Namespace())
	if err != nil {
		return fmt.Errorf("list daemonsets: %w", err)
	}

	if len(dsList) == 0 {
		fmt.Println("No daemonsets found.")
		return nil
	}

	t := &table.Table{Columns: []table.Column{
		colIndex, colName, colNamespace, colDesired, colCurrent, colReady, colUpToDate, colAvailable, colAge,
	}}

	objs := make([]repl.LastObject, 0, len(dsList))
	now := time.Now()
	for i, ds := range dsList {
		t.AddRow(
			fmt.Sprintf("%d", i),
			ds.Name,
			ds.Namespace,
			fmt.Sprintf("%d", ds.Status.DesiredNumberScheduled),
			fmt.Sprintf("%d", ds.Status.CurrentNumberScheduled),
			fmt.Sprintf("%d", ds.Status.NumberReady),
			fmt.Sprintf("%d", ds.Status.UpdatedNumberScheduled),
			fmt.Sprintf("%d", ds.Status.NumberAvailable),
			formatAge(ds.CreationTimestamp.Time, now),
		)
		objs = append(objs, repl.LastObject{Kind: repl.KindDaemonSet, Name: ds.Name, Namespace: ds.Namespace})
	}
	t.Render()

	env.SetLastObjects(objs)
	return nil
}
