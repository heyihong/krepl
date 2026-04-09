package command

import (
	"context"
	"fmt"
	"github.com/heyihong/krepl/pkg/config"
	"github.com/heyihong/krepl/pkg/repl"
	"github.com/heyihong/krepl/pkg/table"
	corev1 "k8s.io/api/core/v1"
	"regexp"
	"sort"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var listPodsForContext = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName, namespace string, opts metav1.ListOptions) ([]corev1.Pod, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	podList, err := client.CoreV1().Pods(namespace).List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return podList.Items, nil
}

func newPodsCmd() *cmd {
	var regexValue string
	var reverse bool
	var labelSelector string
	var nodeValue string
	var showValues []string

	cmd := &cmd{
		use:   "pods",
		short: "list pods in the current context/namespace",
		long: "List pods in the active context, filtered by the working namespace when one is set.\n" +
			"Selected rows become the active pod for commands such as `logs`, `exec`, `describe`, and `port-forward`.",
		args: noArgs,
	}
	flags := cmd.flags()
	flags.StringVarP(&regexValue, "regex", "r", "", "filter returned pods by name regex")
	flags.BoolVarP(&reverse, "reverse", "R", false, "reverse the order of the returned list")
	flags.StringVarP(&labelSelector, "label", "l", "", "get pods with specified label selector")
	flags.StringVarP(&nodeValue, "node", "n", "", "only fetch pods on the specified node")
	flags.StringSliceVar(&showValues, "show", nil, "additional columns to show: namespace,node,ip,labels,lastrestart,nominatednode,readinessgates")
	cmd.runE = func(env *repl.Env, _ []string) error {
		return runPods(env, regexValue, reverse, labelSelector, nodeValue, showValues)
	}
	return cmd
}

// runPods lists pods in the current context and namespace.
func runPods(env *repl.Env, regexValue string, reverse bool, labelSelector, nodeValue string, showValues []string) error {
	if env.CurrentContext() == "" {
		return fmt.Errorf("no active context; use `context <name>` to set one")
	}
	if err := rejectNamespaceSelectionForList(env, "pods"); err != nil {
		return err
	}

	filterRe, err := compilePodRegex(regexValue)
	if err != nil {
		return err
	}
	columns, err := buildPodColumns(showValues)
	if err != nil {
		return err
	}

	opts := metav1.ListOptions{}
	if labelSelector != "" {
		opts.LabelSelector = labelSelector
	}
	if nodeValue == "" {
		if obj := env.CurrentObject(); obj != nil && obj.Kind == repl.KindNode {
			nodeValue = obj.Name
		}
	}
	if nodeValue != "" {
		opts.FieldSelector = "spec.nodeName=" + nodeValue
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pods, err := listPodsForContext(ctx, env.RawConfig(), env.CurrentContext(), env.Namespace(), opts)
	if err != nil {
		return fmt.Errorf("list pods: %w", err)
	}
	pods = filterPodsByRegex(pods, filterRe)
	if reverse {
		reversePods(pods)
	}

	if len(pods) == 0 {
		ns := env.Namespace()
		if ns == "" {
			ns = "all namespaces"
		}
		fmt.Printf("No pods found in %s.\n", ns)
		return nil
	}

	t := &table.Table{Columns: columns}

	objs := make([]repl.LastObject, 0, len(pods))
	now := time.Now()
	for i, pod := range pods {
		t.AddRow(podRowValues(i, pod, now, columns)...)
		objs = append(objs, repl.LastObject{Kind: repl.KindPod, Name: pod.Name, Namespace: pod.Namespace})
	}
	t.Render()

	env.SetLastObjects(objs)
	return nil
}

type podColumnSpec struct {
	col   table.Column
	value func(corev1.Pod, time.Time) string
}

var (
	defaultPodColumnNames = []string{"name", "ready", "status", "restarts", "age"}
	podColumnSpecs        = map[string]podColumnSpec{
		"name": {
			col: colName,
			value: func(pod corev1.Pod, _ time.Time) string {
				return pod.Name
			},
		},
		"ready": {
			col: colReady,
			value: func(pod corev1.Pod, _ time.Time) string {
				return podReadyCounts(pod)
			},
		},
		"status": {
			col: colPodStatus,
			value: func(pod corev1.Pod, _ time.Time) string {
				return podDisplayStatus(pod)
			},
		},
		"restarts": {
			col: colRestarts,
			value: func(pod corev1.Pod, _ time.Time) string {
				return fmt.Sprintf("%d", podRestartCount(pod))
			},
		},
		"age": {
			col: colAge,
			value: func(pod corev1.Pod, now time.Time) string {
				return formatAge(pod.CreationTimestamp.Time, now)
			},
		},
		"namespace": {
			col: colNamespace,
			value: func(pod corev1.Pod, _ time.Time) string {
				return pod.Namespace
			},
		},
		"node": {
			col: colNode,
			value: func(pod corev1.Pod, _ time.Time) string {
				if pod.Spec.NodeName == "" {
					return "<none>"
				}
				return pod.Spec.NodeName
			},
		},
		"ip": {
			col: colIP,
			value: func(pod corev1.Pod, _ time.Time) string {
				if pod.Status.PodIP == "" {
					return "<none>"
				}
				return pod.Status.PodIP
			},
		},
		"labels": {
			col: colLabels,
			value: func(pod corev1.Pod, _ time.Time) string {
				return podLabels(pod)
			},
		},
		"lastrestart": {
			col: colLastRestart,
			value: func(pod corev1.Pod, _ time.Time) string {
				return podLastRestart(pod)
			},
		},
		"nominatednode": {
			col: colNominatedNode,
			value: func(pod corev1.Pod, _ time.Time) string {
				if pod.Status.NominatedNodeName == "" {
					return "<none>"
				}
				return pod.Status.NominatedNodeName
			},
		},
		"readinessgates": {
			col: colReadinessGates,
			value: func(pod corev1.Pod, _ time.Time) string {
				return podReadinessGates(pod)
			},
		},
	}
)

func buildPodColumns(showValues []string) ([]table.Column, error) {
	columnNames := append([]string(nil), defaultPodColumnNames...)
	extraNames, err := parsePodShowColumns(showValues)
	if err != nil {
		return nil, err
	}
	columnNames = append(columnNames, extraNames...)

	columns := make([]table.Column, 0, len(columnNames)+1)
	columns = append(columns, colIndex)
	for _, name := range columnNames {
		spec, ok := podColumnSpecs[name]
		if !ok {
			return nil, fmt.Errorf("unknown show column %q", name)
		}
		columns = append(columns, spec.col)
	}
	return columns, nil
}

func parsePodShowColumns(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	var columns []string
	seen := make(map[string]struct{})
	for _, raw := range values {
		for _, part := range strings.Split(raw, ",") {
			name := strings.ToLower(strings.TrimSpace(part))
			if name == "" || name == "[]" {
				continue
			}
			if _, ok := podColumnSpecs[name]; !ok {
				return nil, fmt.Errorf("unknown show column %q", name)
			}
			if containsPodColumn(defaultPodColumnNames, name) {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			columns = append(columns, name)
		}
	}
	return columns, nil
}

func containsPodColumn(columns []string, want string) bool {
	for _, column := range columns {
		if column == want {
			return true
		}
	}
	return false
}

func podRowValues(index int, pod corev1.Pod, now time.Time, columns []table.Column) []string {
	values := make([]string, 0, len(columns))
	values = append(values, fmt.Sprintf("%d", index))
	for _, column := range columns[1:] {
		spec := podColumnSpecByHeader(column.Header)
		values = append(values, spec.value(pod, now))
	}
	return values
}

func podColumnSpecByHeader(header string) podColumnSpec {
	for _, spec := range podColumnSpecs {
		if spec.col.Header == header {
			return spec
		}
	}
	return podColumnSpec{}
}

func compilePodRegex(expr string) (*regexp.Regexp, error) {
	if expr == "" {
		return nil, nil
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, fmt.Errorf("invalid regex %q: %w", expr, err)
	}
	return re, nil
}

func filterPodsByRegex(pods []corev1.Pod, re *regexp.Regexp) []corev1.Pod {
	if re == nil {
		return pods
	}
	filtered := make([]corev1.Pod, 0, len(pods))
	for _, pod := range pods {
		if re.MatchString(pod.Name) {
			filtered = append(filtered, pod)
		}
	}
	return filtered
}

func reversePods(pods []corev1.Pod) {
	for i, j := 0, len(pods)-1; i < j; i, j = i+1, j-1 {
		pods[i], pods[j] = pods[j], pods[i]
	}
}

func podDisplayStatus(pod corev1.Pod) string {
	if pod.DeletionTimestamp != nil {
		return "Terminating"
	}
	if podHasWaitingContainer(pod) {
		return "ContainerCreating"
	}
	phase := string(pod.Status.Phase)
	if phase == "" {
		return "Unknown"
	}
	return phase
}

func podHasWaitingContainer(pod corev1.Pod) bool {
	for _, status := range pod.Status.ContainerStatuses {
		if status.State.Waiting != nil {
			return true
		}
		if status.State.Waiting == nil && status.State.Running == nil && status.State.Terminated == nil {
			return true
		}
	}
	return false
}

func podReadyCounts(pod corev1.Pod) string {
	total := len(pod.Status.ContainerStatuses)
	ready := 0
	for _, status := range pod.Status.ContainerStatuses {
		if status.Ready {
			ready++
		}
	}
	return fmt.Sprintf("%d/%d", ready, total)
}

func podRestartCount(pod corev1.Pod) int32 {
	var count int32
	for _, status := range pod.Status.ContainerStatuses {
		count += status.RestartCount
	}
	return count
}

func podLastRestart(pod corev1.Pod) string {
	var latest *time.Time
	for _, status := range pod.Status.ContainerStatuses {
		finishedAt := status.LastTerminationState.Terminated
		if finishedAt == nil || finishedAt.FinishedAt.IsZero() {
			continue
		}
		t := finishedAt.FinishedAt.Time
		if latest == nil || t.After(*latest) {
			latest = &t
		}
	}
	if latest == nil {
		return ""
	}
	return latest.Format(time.RFC3339)
}

func podLabels(pod corev1.Pod) string {
	if len(pod.Labels) == 0 {
		return "<none>"
	}
	keys := make([]string, 0, len(pod.Labels))
	for key := range pod.Labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+pod.Labels[key])
	}
	return strings.Join(parts, ",")
}

func podReadinessGates(pod corev1.Pod) string {
	if pod.Spec.ReadinessGates == nil {
		return ""
	}
	if len(pod.Spec.ReadinessGates) == 0 {
		return "<none>"
	}
	gates := make([]string, 0, len(pod.Spec.ReadinessGates))
	for _, gate := range pod.Spec.ReadinessGates {
		gates = append(gates, string(gate.ConditionType))
	}
	return strings.Join(gates, ", ")
}
