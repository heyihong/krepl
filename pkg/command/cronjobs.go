package command

import (
	"context"
	"fmt"
	"github.com/heyihong/krepl/pkg/config"
	"github.com/heyihong/krepl/pkg/repl"
	"github.com/heyihong/krepl/pkg/table"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var listCronJobsForContext = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName, namespace string) ([]batchv1.CronJob, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	list, err := client.BatchV1().CronJobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return append([]batchv1.CronJob(nil), list.Items...), nil
}

func newCronJobsCmd() *cmd {
	return &cmd{
		use:   "cronjobs",
		short: "list cronjobs in the current namespace",
		long: "List cronjobs in the active context and working namespace.\n" +
			"Rows can be selected for follow-up commands such as `describe`, `events`, and `delete`.",
		args: noArgs,
		runE: runCronJobs,
	}
}

// runCronJobs lists cronjobs in the current context/namespace.
func runCronJobs(env *repl.Env, _ []string) error {
	if env.CurrentContext() == "" {
		return fmt.Errorf("no active context; use `context <name>` to set one")
	}
	if err := rejectNamespaceSelectionForList(env, "cronjobs"); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cronjobs, err := listCronJobsForContext(ctx, env.RawConfig(), env.CurrentContext(), env.Namespace())
	if err != nil {
		return fmt.Errorf("list cronjobs: %w", err)
	}

	if len(cronjobs) == 0 {
		fmt.Println("No cronjobs found.")
		return nil
	}

	t := &table.Table{Columns: []table.Column{
		colIndex, colName, colNamespace, colSchedule, colSuspend, colActive, colLastSchedule, colAge,
	}}

	objs := make([]repl.LastObject, 0, len(cronjobs))
	now := time.Now()
	for i, cj := range cronjobs {
		suspend := "False"
		if cj.Spec.Suspend != nil && *cj.Spec.Suspend {
			suspend = "True"
		}
		active := fmt.Sprintf("%d", len(cj.Status.Active))
		lastSchedule := "<none>"
		if cj.Status.LastScheduleTime != nil {
			lastSchedule = formatAge(cj.Status.LastScheduleTime.Time, now)
		}
		t.AddRow(
			fmt.Sprintf("%d", i),
			cj.Name,
			cj.Namespace,
			cj.Spec.Schedule,
			suspend,
			active,
			lastSchedule,
			formatAge(cj.CreationTimestamp.Time, now),
		)
		objs = append(objs, repl.LastObject{Kind: repl.KindCronJob, Name: cj.Name, Namespace: cj.Namespace})
	}
	t.Render()

	env.SetLastObjects(objs)
	return nil
}
