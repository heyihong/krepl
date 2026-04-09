package command

import (
	"context"
	"fmt"
	"github.com/heyihong/krepl/pkg/config"
	"github.com/heyihong/krepl/pkg/repl"
	"github.com/heyihong/krepl/pkg/table"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var listSecretsForContext = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName, namespace string) ([]corev1.Secret, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	secretList, err := client.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return append([]corev1.Secret(nil), secretList.Items...), nil
}

func newSecretsCmd() *cmd {
	return &cmd{
		use:   "secrets",
		short: "list secrets in the current namespace",
		long: "List secrets in the active context and working namespace.\n" +
			"Rows can be selected for follow-up commands such as `describe`, `events`, and `delete`.",
		args: noArgs,
		runE: runSecrets,
	}
}

// runSecrets lists secrets in the current context/namespace.
func runSecrets(env *repl.Env, _ []string) error {
	if env.CurrentContext() == "" {
		return fmt.Errorf("no active context; use `context <name>` to set one")
	}
	if err := rejectNamespaceSelectionForList(env, "secrets"); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	secrets, err := listSecretsForContext(ctx, env.RawConfig(), env.CurrentContext(), env.Namespace())
	if err != nil {
		return fmt.Errorf("list secrets: %w", err)
	}

	if len(secrets) == 0 {
		fmt.Println("No secrets found.")
		return nil
	}

	t := &table.Table{Columns: []table.Column{
		colIndex, colName, colNamespace, colType, colData, colAge,
	}}

	objs := make([]repl.LastObject, 0, len(secrets))
	now := time.Now()
	for i, s := range secrets {
		t.AddRow(
			fmt.Sprintf("%d", i),
			s.Name,
			s.Namespace,
			fallbackString(string(s.Type), "<none>"),
			fmt.Sprintf("%d", len(s.Data)),
			formatAge(s.CreationTimestamp.Time, now),
		)
		objs = append(objs, repl.LastObject{Kind: repl.KindSecret, Name: s.Name, Namespace: s.Namespace})
	}
	t.Render()

	env.SetLastObjects(objs)
	return nil
}
