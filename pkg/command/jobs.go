package command

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/heyihong/krepl/pkg/config"
	"github.com/heyihong/krepl/pkg/repl"
	"github.com/heyihong/krepl/pkg/table"
)

var listJobsForContext = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName, namespace string) ([]batchv1.Job, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	list, err := client.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return append([]batchv1.Job(nil), list.Items...), nil
}

func newJobsCmd() *cmd {
	return &cmd{
		use:   "jobs",
		short: "list jobs in the current namespace",
		long: "List jobs in the active context and working namespace.\n" +
			"Rows can be selected for follow-up commands such as `describe`, `events`, and `delete`.",
		args: noArgs,
		runE: runJobs,
	}
}

func runJobs(env *repl.Env, _ []string) error {
	if env.CurrentContext() == "" {
		return fmt.Errorf("no active context; use `context <name>` to set one")
	}
	if err := rejectNamespaceSelectionForList(env, "jobs"); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	jobs, err := listJobsForContext(ctx, env.RawConfig(), env.CurrentContext(), env.Namespace())
	if err != nil {
		return fmt.Errorf("list jobs: %w", err)
	}

	if len(jobs) == 0 {
		fmt.Println("No jobs found.")
		return nil
	}

	t := &table.Table{Columns: []table.Column{
		colIndex, colName, colNamespace, colCompletions, colDuration, colAge,
	}}

	objs := make([]repl.LastObject, 0, len(jobs))
	now := time.Now()
	for i, job := range jobs {
		completions := int32(0)
		if job.Spec.Completions != nil {
			completions = *job.Spec.Completions
		}
		t.AddRow(
			fmt.Sprintf("%d", i),
			job.Name,
			job.Namespace,
			fmt.Sprintf("%d/%d", job.Status.Succeeded, completions),
			formatJobDuration(job, now),
			formatAge(job.CreationTimestamp.Time, now),
		)
		objs = append(objs, repl.LastObject{Kind: repl.KindJob, Name: job.Name, Namespace: job.Namespace})
	}
	t.Render()

	env.SetLastObjects(objs)
	return nil
}

func formatJobDuration(job batchv1.Job, now time.Time) string {
	if job.Status.StartTime == nil {
		return "<none>"
	}
	if job.Status.CompletionTime != nil {
		return formatAge(job.Status.StartTime.Time, job.Status.CompletionTime.Time)
	}
	return formatAge(job.Status.StartTime.Time, now)
}
