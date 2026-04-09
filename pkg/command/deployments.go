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

var listDeploymentsForContext = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName, namespace string) ([]appsv1.Deployment, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	depList, err := client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return append([]appsv1.Deployment(nil), depList.Items...), nil
}

func newDeploymentsCmd() *cmd {
	return &cmd{
		use:     "deployments",
		aliases: []string{"deps"},
		short:   "list deployments in the current namespace",
		long: "List deployments in the active context and working namespace.\n" +
			"Rows can be selected for follow-up commands such as `describe`, `events`, `edit`, and `delete`.",
		args: noArgs,
		runE: runDeployments,
	}
}

// runDeployments lists deployments in the current context/namespace.
func runDeployments(env *repl.Env, _ []string) error {
	if env.CurrentContext() == "" {
		return fmt.Errorf("no active context; use `context <name>` to set one")
	}
	if err := rejectNamespaceSelectionForList(env, "deployments"); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	deps, err := listDeploymentsForContext(ctx, env.RawConfig(), env.CurrentContext(), env.Namespace())
	if err != nil {
		return fmt.Errorf("list deployments: %w", err)
	}

	if len(deps) == 0 {
		fmt.Println("No deployments found.")
		return nil
	}

	t := &table.Table{Columns: []table.Column{
		colIndex, colName, colNamespace, colReady, colUpToDate, colAvailable, colAge,
	}}

	objs := make([]repl.LastObject, 0, len(deps))
	now := time.Now()
	for i, dep := range deps {
		desired := int32(0)
		if dep.Spec.Replicas != nil {
			desired = *dep.Spec.Replicas
		}
		ready := dep.Status.ReadyReplicas
		t.AddRow(
			fmt.Sprintf("%d", i),
			dep.Name,
			dep.Namespace,
			fmt.Sprintf("%d/%d", ready, desired),
			fmt.Sprintf("%d", dep.Status.UpdatedReplicas),
			fmt.Sprintf("%d", dep.Status.AvailableReplicas),
			formatAge(dep.CreationTimestamp.Time, now),
		)
		objs = append(objs, repl.LastObject{Kind: repl.KindDeployment, Name: dep.Name, Namespace: dep.Namespace})
	}
	t.Render()

	env.SetLastObjects(objs)
	return nil
}
