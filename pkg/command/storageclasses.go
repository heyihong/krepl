package command

import (
	"context"
	"fmt"
	"time"

	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/heyihong/krepl/pkg/config"
	"github.com/heyihong/krepl/pkg/repl"
	"github.com/heyihong/krepl/pkg/table"
)

var listStorageClassesForContext = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName string) ([]storagev1.StorageClass, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	list, err := client.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return append([]storagev1.StorageClass(nil), list.Items...), nil
}

func newStorageClassesCmd() *cmd {
	return &cmd{
		use:   "storageclasses",
		short: "list storage classes in the current context",
		long: "List storage classes in the active context.\n" +
			"Storage classes are cluster-scoped, and rows can be selected for follow-up commands such as `describe` and `events`.",
		args: noArgs,
		runE: runStorageClasses,
	}
}

func runStorageClasses(env *repl.Env, _ []string) error {
	if env.CurrentContext() == "" {
		return fmt.Errorf("no active context; use `context <name>` to set one")
	}
	if err := rejectNamespaceSelectionForList(env, "storageclasses"); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	storageClasses, err := listStorageClassesForContext(ctx, env.RawConfig(), env.CurrentContext())
	if err != nil {
		return fmt.Errorf("list storageclasses: %w", err)
	}

	if len(storageClasses) == 0 {
		fmt.Println("No storageclasses found.")
		return nil
	}

	t := &table.Table{Columns: []table.Column{
		colIndex, colName, colProvisioner, colAge,
	}}

	objs := make([]repl.LastObject, 0, len(storageClasses))
	now := time.Now()
	for i, sc := range storageClasses {
		t.AddRow(
			fmt.Sprintf("%d", i),
			sc.Name,
			fallbackString(sc.Provisioner, "<none>"),
			formatAge(sc.CreationTimestamp.Time, now),
		)
		objs = append(objs, repl.LastObject{Kind: repl.KindStorageClass, Name: sc.Name})
	}
	t.Render()

	env.SetLastObjects(objs)
	return nil
}
