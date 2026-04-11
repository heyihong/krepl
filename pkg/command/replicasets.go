package command

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/heyihong/krepl/pkg/config"
	"github.com/heyihong/krepl/pkg/repl"
	"github.com/heyihong/krepl/pkg/table"
)

var listReplicaSetsForContext = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName, namespace string) ([]appsv1.ReplicaSet, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	rsList, err := client.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return append([]appsv1.ReplicaSet(nil), rsList.Items...), nil
}

func newReplicaSetsCmd() *cmd {
	return &cmd{
		use:     "replicasets",
		aliases: []string{"rs"},
		short:   "list replicasets in the current namespace",
		long: "List replicasets in the active context and working namespace.\n" +
			"Rows can be selected for follow-up commands such as `describe`, `events`, and `delete`.",
		args: noArgs,
		runE: runReplicaSets,
	}
}

// runReplicaSets lists replicasets in the current context/namespace.
func runReplicaSets(env *repl.Env, _ []string) error {
	if env.CurrentContext() == "" {
		return fmt.Errorf("no active context; use `context <name>` to set one")
	}
	if err := rejectNamespaceSelectionForList(env, "replicasets"); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rsList, err := listReplicaSetsForContext(ctx, env.RawConfig(), env.CurrentContext(), env.Namespace())
	if err != nil {
		return fmt.Errorf("list replicasets: %w", err)
	}

	if len(rsList) == 0 {
		fmt.Println("No replicasets found.")
		return nil
	}

	t := &table.Table{Columns: []table.Column{
		colIndex, colName, colNamespace, colDesired, colCurrent, colReady, colAge,
	}}

	objs := make([]repl.LastObject, 0, len(rsList))
	now := time.Now()
	for i, rs := range rsList {
		desired := int32(0)
		if rs.Spec.Replicas != nil {
			desired = *rs.Spec.Replicas
		}
		t.AddRow(
			fmt.Sprintf("%d", i),
			rs.Name,
			rs.Namespace,
			fmt.Sprintf("%d", desired),
			fmt.Sprintf("%d", rs.Status.Replicas),
			fmt.Sprintf("%d", rs.Status.ReadyReplicas),
			formatAge(rs.CreationTimestamp.Time, now),
		)
		objs = append(objs, repl.LastObject{Kind: repl.KindReplicaSet, Name: rs.Name, Namespace: rs.Namespace})
	}
	t.Render()

	env.SetLastObjects(objs)
	return nil
}
