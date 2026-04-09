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

var listStatefulSetsForContext = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName, namespace string) ([]appsv1.StatefulSet, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	stsList, err := client.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return append([]appsv1.StatefulSet(nil), stsList.Items...), nil
}

func newStatefulSetsCmd() *cmd {
	return &cmd{
		use:     "statefulsets",
		aliases: []string{"ss"},
		short:   "list statefulsets in the current namespace",
		long: "List statefulsets in the active context and working namespace.\n" +
			"Rows can be selected for follow-up commands such as `describe`, `events`, `edit`, and `delete`.",
		args: noArgs,
		runE: runStatefulSets,
	}
}

// runStatefulSets lists statefulsets in the current context/namespace.
func runStatefulSets(env *repl.Env, _ []string) error {
	if env.CurrentContext() == "" {
		return fmt.Errorf("no active context; use `context <name>` to set one")
	}
	if err := rejectNamespaceSelectionForList(env, "statefulsets"); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stsList, err := listStatefulSetsForContext(ctx, env.RawConfig(), env.CurrentContext(), env.Namespace())
	if err != nil {
		return fmt.Errorf("list statefulsets: %w", err)
	}

	if len(stsList) == 0 {
		fmt.Println("No statefulsets found.")
		return nil
	}

	t := &table.Table{Columns: []table.Column{
		colIndex, colName, colNamespace, colReady, colAge,
	}}

	objs := make([]repl.LastObject, 0, len(stsList))
	now := time.Now()
	for i, sts := range stsList {
		desired := int32(0)
		if sts.Spec.Replicas != nil {
			desired = *sts.Spec.Replicas
		}
		ready := sts.Status.ReadyReplicas
		t.AddRow(
			fmt.Sprintf("%d", i),
			sts.Name,
			sts.Namespace,
			fmt.Sprintf("%d/%d", ready, desired),
			formatAge(sts.CreationTimestamp.Time, now),
		)
		objs = append(objs, repl.LastObject{Kind: repl.KindStatefulSet, Name: sts.Name, Namespace: sts.Namespace})
	}
	t.Render()

	env.SetLastObjects(objs)
	return nil
}
