package command

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/heyihong/krepl/pkg/config"
	"github.com/heyihong/krepl/pkg/repl"
	"github.com/heyihong/krepl/pkg/styles"
	"github.com/heyihong/krepl/pkg/table"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newNamespacesCmd() *cmd {
	return &cmd{
		use:     "namespaces",
		aliases: []string{"nss"},
		short:   "list namespaces in the current context",
		long: "List namespaces in the current Kubernetes context.\n" +
			"Rows can be selected for follow-up commands such as `describe`, `events`, and `delete`.",
		args: noArgs,
		runE: runNamespaces,
	}
}

// runNamespaces lists all namespaces in the current context.
// Columns match Click-style formatting.
func runNamespaces(env *repl.Env, _ []string) error {
	if env.CurrentContext() == "" {
		return fmt.Errorf("no active context; use `context <name>` to set one")
	}

	client, err := config.BuildClientForContext(env.RawConfig(), env.CurrentContext())
	if err != nil {
		return fmt.Errorf("build client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	nsList, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list namespaces: %w", err)
	}

	if len(nsList.Items) == 0 {
		fmt.Println("No namespaces found.")
		return nil
	}

	// The NAME column Color function colorizes the "* " active marker prefix
	// (if present) while leaving the namespace name plain. The plain value
	// (including the "* " prefix) is what Render uses for padding, so
	// column alignment is correct even with ANSI codes in the output.
	nameColor := func(s string) string {
		if strings.HasPrefix(s, "* ") {
			return styles.ActiveMarker("*") + s[1:]
		}
		return s
	}

	t := &table.Table{Columns: []table.Column{
		colIndex,
		{Header: "NAME", Color: nameColor}, // nameColor adds "* " prefix for active namespace
		colAge,
		colNamespaceStatus,
	}}

	objs := make([]repl.LastObject, 0, len(nsList.Items))
	now := time.Now()
	for i, ns := range nsList.Items {
		status := string(ns.Status.Phase)
		age := formatAge(ns.CreationTimestamp.Time, now)
		name := ns.Name
		if ns.Name == env.Namespace() {
			name = "* " + ns.Name
		}
		t.AddRow(fmt.Sprintf("%d", i), name, age, status)
		objs = append(objs, repl.LastObject{Kind: repl.KindNamespace, Name: ns.Name})
	}
	t.Render()

	env.SetLastObjects(objs)
	return nil
}

// formatAge returns a human-readable age string (e.g. "3d", "2h", "45m")
// matching the style used by kubectl and Click.
func formatAge(created, now time.Time) string {
	d := now.Sub(created)
	if d < 0 {
		return "0s"
	}
	switch {
	case d >= 365*24*time.Hour:
		return fmt.Sprintf("%dy", int(d.Hours())/24/365)
	case d >= 24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours())/24)
	case d >= time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d >= time.Minute:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
}
