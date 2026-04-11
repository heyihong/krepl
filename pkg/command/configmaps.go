package command

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/heyihong/krepl/pkg/config"
	"github.com/heyihong/krepl/pkg/repl"
	"github.com/heyihong/krepl/pkg/table"
)

var listConfigMapsForContext = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName, namespace string) ([]corev1.ConfigMap, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	cmList, err := client.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return append([]corev1.ConfigMap(nil), cmList.Items...), nil
}

func newConfigMapsCmd() *cmd {
	return &cmd{
		use:     "configmaps",
		aliases: []string{"cm"},
		short:   "list configmaps in the current namespace",
		long: "List configmaps in the active context and working namespace.\n" +
			"Rows can be selected for follow-up commands such as `describe`, `events`, `edit`, and `delete`.",
		args: noArgs,
		runE: runConfigMaps,
	}
}

// runConfigMaps lists configmaps in the current context/namespace.
func runConfigMaps(env *repl.Env, _ []string) error {
	if env.CurrentContext() == "" {
		return fmt.Errorf("no active context; use `context <name>` to set one")
	}
	if err := rejectNamespaceSelectionForList(env, "configmaps"); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cms, err := listConfigMapsForContext(ctx, env.RawConfig(), env.CurrentContext(), env.Namespace())
	if err != nil {
		return fmt.Errorf("list configmaps: %w", err)
	}

	if len(cms) == 0 {
		fmt.Println("No configmaps found.")
		return nil
	}

	t := &table.Table{Columns: []table.Column{
		colIndex, colName, colNamespace, colData, colAge,
	}}

	objs := make([]repl.LastObject, 0, len(cms))
	now := time.Now()
	for i, cm := range cms {
		t.AddRow(
			fmt.Sprintf("%d", i),
			cm.Name,
			cm.Namespace,
			fmt.Sprintf("%d", len(cm.Data)+len(cm.BinaryData)),
			formatAge(cm.CreationTimestamp.Time, now),
		)
		objs = append(objs, repl.LastObject{Kind: repl.KindConfigMap, Name: cm.Name, Namespace: cm.Namespace})
	}
	t.Render()

	env.SetLastObjects(objs)
	return nil
}
