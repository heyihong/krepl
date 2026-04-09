package command

import (
	"context"
	"fmt"
	"github.com/heyihong/krepl/pkg/config"
	"github.com/heyihong/krepl/pkg/repl"
	"github.com/heyihong/krepl/pkg/table"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var listNodesForContext = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName string) ([]corev1.Node, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	nodeList, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return append([]corev1.Node(nil), nodeList.Items...), nil
}

func newNodesCmd() *cmd {
	return &cmd{
		use:   "nodes",
		short: "list nodes in the current context",
		long: "List nodes in the active Kubernetes context.\n" +
			"Rows can be selected for follow-up commands such as `describe`, `events`, and `delete`.",
		args: noArgs,
		runE: runNodes,
	}
}

// runNodes lists nodes in the current context.
func runNodes(env *repl.Env, _ []string) error {
	if env.CurrentContext() == "" {
		return fmt.Errorf("no active context; use `context <name>` to set one")
	}
	if err := rejectNamespaceSelectionForList(env, "nodes"); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	nodes, err := listNodesForContext(ctx, env.RawConfig(), env.CurrentContext())
	if err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	if len(nodes) == 0 {
		fmt.Println("No nodes found.")
		return nil
	}

	t := &table.Table{Columns: []table.Column{
		colIndex, colName, colNodeStatus, colRoles, colAge, colVersion,
	}}

	objs := make([]repl.LastObject, 0, len(nodes))
	now := time.Now()
	for i, node := range nodes {
		t.AddRow(
			fmt.Sprintf("%d", i),
			node.Name,
			nodeStatus(node),
			nodeRoles(node),
			formatAge(node.CreationTimestamp.Time, now),
			nodeVersion(node),
		)
		objs = append(objs, repl.LastObject{Kind: repl.KindNode, Name: node.Name})
	}
	t.Render()

	env.SetLastObjects(objs)
	return nil
}

func nodeStatus(node corev1.Node) string {
	ready := "Unknown"
	for _, cond := range node.Status.Conditions {
		if cond.Type != corev1.NodeReady {
			continue
		}
		switch cond.Status {
		case corev1.ConditionTrue:
			ready = "Ready"
		case corev1.ConditionFalse:
			ready = "NotReady"
		default:
			ready = "Unknown"
		}
		break
	}
	if node.Spec.Unschedulable {
		return ready + ",SchedulingDisabled"
	}
	return ready
}

func nodeRoles(node corev1.Node) string {
	var roles []string
	for key, value := range node.Labels {
		switch {
		case key == "kubernetes.io/role" && value != "":
			roles = append(roles, value)
		case strings.HasPrefix(key, "node-role.kubernetes.io/"):
			role := strings.TrimPrefix(key, "node-role.kubernetes.io/")
			if role == "" {
				role = "<none>"
			}
			roles = append(roles, role)
		}
	}
	if len(roles) == 0 {
		return "<none>"
	}
	sort.Strings(roles)
	return strings.Join(roles, ",")
}

func nodeVersion(node corev1.Node) string {
	if node.Status.NodeInfo.KubeletVersion == "" {
		return "<none>"
	}
	return node.Status.NodeInfo.KubeletVersion
}
