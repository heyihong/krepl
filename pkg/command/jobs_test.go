package command

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/heyihong/krepl/pkg/repl"
)

func TestJobsCommand_NoContext(t *testing.T) {
	env := makeExecTestEnvNoContext(t)
	if err := newJobsCmd().Execute(env, nil); err == nil {
		t.Fatal("expected error when no context is active")
	}
}

func TestJobsCommand_PrintsTableAndSetsSelectableObjects(t *testing.T) {
	env := makeTestEnv()
	oldList := listJobsForContext
	listJobsForContext = func(_ context.Context, _ clientcmdapi.Config, contextName, _ string) ([]batchv1.Job, error) {
		if contextName != env.CurrentContext() {
			t.Fatalf("unexpected context %q", contextName)
		}
		return []batchv1.Job{
			makeTestJob("job-a", "default", 1, 1),
			makeTestJob("job-b", "default", 0, 3),
		}, nil
	}
	defer func() { listJobsForContext = oldList }()

	out := captureStdout(t, func() {
		if err := newJobsCmd().Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"NAME", "NAMESPACE", "COMPLETIONS", "DURATION", "AGE", "job-a", "1/1", "job-b", "0/3"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}

	_ = captureStdout(t, func() {
		if err := env.SelectByIndex(0); err != nil {
			t.Fatalf("select job: %v", err)
		}
	})
	obj := env.CurrentObject()
	if obj == nil || obj.Kind != repl.KindJob || obj.Name != "job-a" {
		t.Fatalf("expected job-a selected as KindJob, got %+v", obj)
	}
}

func TestJobsCommand_PropagatesListErrors(t *testing.T) {
	env := makeTestEnv()
	oldList := listJobsForContext
	listJobsForContext = func(_ context.Context, _ clientcmdapi.Config, _, _ string) ([]batchv1.Job, error) {
		return nil, errors.New("boom")
	}
	defer func() { listJobsForContext = oldList }()

	err := newJobsCmd().Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "list jobs") {
		t.Fatalf("expected wrapped list error, got %v", err)
	}
}

func makeTestJob(name, namespace string, succeeded, completions int32) batchv1.Job {
	return batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
		},
		Spec: batchv1.JobSpec{
			Completions: &completions,
		},
		Status: batchv1.JobStatus{
			Succeeded: succeeded,
			StartTime: &metav1.Time{Time: time.Now().Add(-30 * time.Minute)},
			CompletionTime: func() *metav1.Time {
				if succeeded >= completions && completions > 0 {
					return &metav1.Time{Time: time.Now().Add(-5 * time.Minute)}
				}
				return nil
			}(),
		},
	}
}
