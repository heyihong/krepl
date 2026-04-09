package command

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/heyihong/krepl/pkg/config"
	"github.com/heyihong/krepl/pkg/repl"
	"github.com/heyihong/krepl/pkg/table"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var listServicesForContext = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName, namespace string) ([]corev1.Service, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	list, err := client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return append([]corev1.Service(nil), list.Items...), nil
}

func newServicesCmd() *cmd {
	return &cmd{
		use:   "services",
		short: "list services in the current namespace",
		long: "List services in the active context and working namespace.\n" +
			"Rows can be selected for follow-up commands such as `describe`, `events`, and `delete`.",
		args: noArgs,
		runE: runServices,
	}
}

// runServices lists services in the current context/namespace.
func runServices(env *repl.Env, _ []string) error {
	if env.CurrentContext() == "" {
		return fmt.Errorf("no active context; use `context <name>` to set one")
	}
	if err := rejectNamespaceSelectionForList(env, "services"); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	svcs, err := listServicesForContext(ctx, env.RawConfig(), env.CurrentContext(), env.Namespace())
	if err != nil {
		return fmt.Errorf("list services: %w", err)
	}

	if len(svcs) == 0 {
		fmt.Println("No services found.")
		return nil
	}

	t := &table.Table{Columns: []table.Column{
		colIndex, colName, colNamespace, colType, colClusterIP, colExternalIP, colPortS, colAge,
	}}

	objs := make([]repl.LastObject, 0, len(svcs))
	now := time.Now()
	for i, svc := range svcs {
		t.AddRow(
			fmt.Sprintf("%d", i),
			svc.Name,
			svc.Namespace,
			string(svc.Spec.Type),
			fallbackString(svc.Spec.ClusterIP, "<none>"),
			serviceExternalIP(svc),
			servicePorts(svc),
			formatAge(svc.CreationTimestamp.Time, now),
		)
		objs = append(objs, repl.LastObject{Kind: repl.KindService, Name: svc.Name, Namespace: svc.Namespace})
	}
	t.Render()

	env.SetLastObjects(objs)
	return nil
}

// serviceExternalIP returns the external IP(s) for a service.
// For LoadBalancer services, returns ingress hostname or IP. Otherwise "<none>".
func serviceExternalIP(svc corev1.Service) string {
	ingresses := svc.Status.LoadBalancer.Ingress
	if len(ingresses) == 0 {
		return "<none>"
	}
	addrs := make([]string, 0, len(ingresses))
	for _, ing := range ingresses {
		if ing.Hostname != "" {
			addrs = append(addrs, ing.Hostname)
		} else if ing.IP != "" {
			addrs = append(addrs, ing.IP)
		}
	}
	if len(addrs) == 0 {
		return "<none>"
	}
	return strings.Join(addrs, ",")
}

// servicePorts formats the port list as "port/protocol" or "port:nodePort/protocol",
// matching Click-style port formatting.
func servicePorts(svc corev1.Service) string {
	if len(svc.Spec.Ports) == 0 {
		return "<none>"
	}
	parts := make([]string, 0, len(svc.Spec.Ports))
	for _, p := range svc.Spec.Ports {
		proto := string(p.Protocol)
		if proto == "" {
			proto = "TCP"
		}
		if p.NodePort != 0 {
			parts = append(parts, fmt.Sprintf("%d:%d/%s", p.Port, p.NodePort, proto))
		} else {
			parts = append(parts, fmt.Sprintf("%d/%s", p.Port, proto))
		}
	}
	return strings.Join(parts, ",")
}
