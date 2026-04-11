package command

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/heyihong/krepl/pkg/repl"
)

func TestCronJobsCommand_NoContext(t *testing.T) {
	env := makeExecTestEnvNoContext(t)
	cmd := newCronJobsCmd()
	if err := cmd.Execute(env, nil); err == nil {
		t.Fatal("expected error when no context is active")
	}
}

func TestCronJobsCommand_PrintsTableAndSetsSelectableObjects(t *testing.T) {
	env := makeTestEnv()
	oldList := listCronJobsForContext
	listCronJobsForContext = func(_ context.Context, _ clientcmdapi.Config, contextName, _ string) ([]batchv1.CronJob, error) {
		if contextName != env.CurrentContext() {
			t.Fatalf("unexpected context %q", contextName)
		}
		return []batchv1.CronJob{
			makeTestCronJob("cj-a", "default", "0 * * * *", false, 0),
			makeTestCronJob("cj-b", "default", "*/5 * * * *", true, 2),
		}, nil
	}
	defer func() { listCronJobsForContext = oldList }()

	out := captureStdout(t, func() {
		if err := (newCronJobsCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"NAME", "SCHEDULE", "SUSPEND", "ACTIVE", "AGE", "cj-a", "0 * * * *", "False", "cj-b", "*/5 * * * *", "True"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}

	_ = captureStdout(t, func() {
		if err := env.SelectByIndex(0); err != nil {
			t.Fatalf("select cronjob: %v", err)
		}
	})
	obj := env.CurrentObject()
	if obj == nil || obj.Kind != repl.KindCronJob || obj.Name != "cj-a" {
		t.Fatalf("expected cj-a selected as KindCronJob, got %+v", obj)
	}
}

func TestCronJobsCommand_PropagatesListErrors(t *testing.T) {
	env := makeTestEnv()
	oldList := listCronJobsForContext
	listCronJobsForContext = func(_ context.Context, _ clientcmdapi.Config, _, _ string) ([]batchv1.CronJob, error) {
		return nil, errors.New("boom")
	}
	defer func() { listCronJobsForContext = oldList }()

	err := (newCronJobsCmd()).Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "list cronjobs") {
		t.Fatalf("expected wrapped list error, got %v", err)
	}
}

func makeTestCronJob(name, namespace, schedule string, suspend bool, activeCount int) batchv1.CronJob {
	lastSchedule := metav1.NewTime(time.Now().Add(-10 * time.Minute))
	active := make([]corev1.ObjectReference, activeCount)
	return batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			CreationTimestamp: metav1.NewTime(time.Now().Add(-24 * time.Hour)),
		},
		Spec: batchv1.CronJobSpec{
			Schedule: schedule,
			Suspend:  &suspend,
		},
		Status: batchv1.CronJobStatus{
			Active:           active,
			LastScheduleTime: &lastSchedule,
		},
	}
}
