package command

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/yaml"

	"github.com/heyihong/krepl/pkg/config"
	"github.com/heyihong/krepl/pkg/repl"
)

type describeOptions struct {
	json   bool
	yaml   bool
	events bool
}

var fetchDescribePod = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName, namespace, podName string) (*corev1.Pod, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	return client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
}

var fetchDescribeNode = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName, nodeName string) (*corev1.Node, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	return client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
}

var fetchDescribeNamespace = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName, namespaceName string) (*corev1.Namespace, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	return client.CoreV1().Namespaces().Get(ctx, namespaceName, metav1.GetOptions{})
}

var fetchDescribeService = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName, namespace, name string) (*corev1.Service, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	return client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
}

var fetchDescribePersistentVolume = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName, name string) (*corev1.PersistentVolume, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	return client.CoreV1().PersistentVolumes().Get(ctx, name, metav1.GetOptions{})
}

var fetchDescribeJob = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName, namespace, name string) (*batchv1.Job, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	return client.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
}

var fetchDescribeCronJob = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName, namespace, name string) (*batchv1.CronJob, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	return client.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
}

var fetchDescribeStorageClass = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName, name string) (*storagev1.StorageClass, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	return client.StorageV1().StorageClasses().Get(ctx, name, metav1.GetOptions{})
}

var fetchDescribeDaemonSet = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName, namespace, name string) (*appsv1.DaemonSet, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	return client.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
}

var fetchDescribeConfigMap = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName, namespace, name string) (*corev1.ConfigMap, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	return client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
}

var fetchDescribeSecret = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName, namespace, name string) (*corev1.Secret, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	return client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
}

var fetchDescribeDeployment = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName, namespace, name string) (*appsv1.Deployment, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	return client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
}

var fetchDescribeReplicaSet = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName, namespace, name string) (*appsv1.ReplicaSet, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	return client.AppsV1().ReplicaSets(namespace).Get(ctx, name, metav1.GetOptions{})
}

var fetchDescribeStatefulSet = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName, namespace, name string) (*appsv1.StatefulSet, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	return client.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
}

var fetchDescribeDynamicResource = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName string, obj repl.LastObject) (*unstructured.Unstructured, error) {
	if obj.Dynamic == nil {
		return nil, fmt.Errorf("missing dynamic resource descriptor")
	}

	restConfig, err := config.BuildRESTConfigForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}

	client, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	gv, err := schema.ParseGroupVersion(obj.Dynamic.GroupVersion)
	if err != nil {
		return nil, err
	}

	resourceClient := client.Resource(schema.GroupVersionResource{
		Group:    gv.Group,
		Version:  gv.Version,
		Resource: obj.Dynamic.Resource,
	})
	if obj.Dynamic.Namespaced {
		return resourceClient.Namespace(obj.Namespace).Get(ctx, obj.Name, metav1.GetOptions{})
	}
	return resourceClient.Get(ctx, obj.Name, metav1.GetOptions{})
}

var fetchDescribeEvents = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName string, obj repl.LastObject) ([]corev1.Event, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}

	fieldParts := []string{
		fmt.Sprintf("involvedObject.name=%s", obj.Name),
		fmt.Sprintf("involvedObject.kind=%s", describeObjectKindName(obj)),
	}
	namespace := metav1.NamespaceAll
	if obj.Namespace != "" {
		namespace = obj.Namespace
		fieldParts = append(fieldParts, fmt.Sprintf("involvedObject.namespace=%s", obj.Namespace))
	}

	eventList, err := client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: strings.Join(fieldParts, ","),
	})
	if err != nil {
		return nil, err
	}
	events := append([]corev1.Event(nil), eventList.Items...)
	sort.Slice(events, func(i, j int) bool {
		return describeEventTime(events[i]).Before(describeEventTime(events[j]))
	})
	return events, nil
}

func newDescribeCmd() *cmd {
	var jsonOut, yamlOut bool

	cmd := &cmd{
		use:     "describe [flags]",
		aliases: []string{"d"},
		short:   "describe the active object",
		long: "Describe the active object or current range selection.\n" +
			"If no object is selected but a working namespace is set, the namespace is described. Use `--json` or `--yaml` for raw output and `--events=false` to omit related events.",
		args: noArgs,
	}
	cmd.flags().BoolVarP(&jsonOut, "json", "j", false, "output as JSON")
	cmd.flags().BoolVarP(&yamlOut, "yaml", "y", false, "output as YAML")
	cmd.flags().BoolP("events", "e", true, "include events; use --events=false to disable")

	cmd.runE = func(env *repl.Env, _ []string) error {
		if env.CurrentContext() == "" {
			return fmt.Errorf("no active context")
		}
		showEvents, err := cmd.flags().GetBool("events")
		if err != nil {
			return err
		}
		if !cmd.flags().Lookup("events").Changed {
			showEvents = env.DescribeIncludeEvents()
		}
		opts := describeOptions{json: jsonOut, yaml: yamlOut, events: showEvents}
		if len(env.CurrentSelection()) == 0 {
			if env.Namespace() != "" {
				return runDescribeObject(env, repl.LastObject{
					Kind: repl.KindNamespace,
					Name: env.Namespace(),
				}, opts)
			}
			return fmt.Errorf("no active object; select one by number after running `pods`, `nodes`, `deployments`, `replicasets`, `statefulsets`, `configmaps`, `secrets`, `jobs`, `cronjobs`, `daemonsets`, `persistentvolumes`, `services`, `storageclasses`, or `crd <resource>`, or set a working namespace to describe it")
		}
		return env.ApplyToSelection(func(obj repl.LastObject) error {
			return runDescribeObject(env, obj, opts)
		})
	}
	return cmd
}

func runDescribeObject(env *repl.Env, obj repl.LastObject, opts describeOptions) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch obj.Kind {
	case repl.KindPod:
		pod, err := fetchDescribePod(ctx, env.RawConfig(), env.CurrentContext(), obj.Namespace, obj.Name)
		if err != nil {
			return fmt.Errorf("get pod: %w", err)
		}
		if err := writeDescribeOutput(os.Stdout, pod, opts, func(w io.Writer) { writeDescribePodSummary(w, pod) }); err != nil {
			return err
		}
	case repl.KindNode:
		node, err := fetchDescribeNode(ctx, env.RawConfig(), env.CurrentContext(), obj.Name)
		if err != nil {
			return fmt.Errorf("get node: %w", err)
		}
		if err := writeDescribeOutput(os.Stdout, node, opts, func(w io.Writer) { writeDescribeNodeSummary(w, node) }); err != nil {
			return err
		}
	case repl.KindNamespace:
		ns, err := fetchDescribeNamespace(ctx, env.RawConfig(), env.CurrentContext(), obj.Name)
		if err != nil {
			return fmt.Errorf("get namespace: %w", err)
		}
		if err := writeDescribeOutput(os.Stdout, ns, opts, func(w io.Writer) { writeDescribeNamespaceSummary(w, ns) }); err != nil {
			return err
		}
	case repl.KindDeployment:
		dep, err := fetchDescribeDeployment(ctx, env.RawConfig(), env.CurrentContext(), obj.Namespace, obj.Name)
		if err != nil {
			return fmt.Errorf("get deployment: %w", err)
		}
		if err := writeDescribeOutput(os.Stdout, dep, opts, func(w io.Writer) { writeDescribeDeploymentSummary(w, dep) }); err != nil {
			return err
		}
	case repl.KindReplicaSet:
		rs, err := fetchDescribeReplicaSet(ctx, env.RawConfig(), env.CurrentContext(), obj.Namespace, obj.Name)
		if err != nil {
			return fmt.Errorf("get replicaset: %w", err)
		}
		if err := writeDescribeOutput(os.Stdout, rs, opts, func(w io.Writer) { writeDescribeReplicaSetSummary(w, rs) }); err != nil {
			return err
		}
	case repl.KindStatefulSet:
		sts, err := fetchDescribeStatefulSet(ctx, env.RawConfig(), env.CurrentContext(), obj.Namespace, obj.Name)
		if err != nil {
			return fmt.Errorf("get statefulset: %w", err)
		}
		if err := writeDescribeOutput(os.Stdout, sts, opts, func(w io.Writer) { writeDescribeStatefulSetSummary(w, sts) }); err != nil {
			return err
		}
	case repl.KindConfigMap:
		cm, err := fetchDescribeConfigMap(ctx, env.RawConfig(), env.CurrentContext(), obj.Namespace, obj.Name)
		if err != nil {
			return fmt.Errorf("get configmap: %w", err)
		}
		if err := writeDescribeOutput(os.Stdout, cm, opts, func(w io.Writer) { writeDescribeConfigMapSummary(w, cm) }); err != nil {
			return err
		}
	case repl.KindSecret:
		s, err := fetchDescribeSecret(ctx, env.RawConfig(), env.CurrentContext(), obj.Namespace, obj.Name)
		if err != nil {
			return fmt.Errorf("get secret: %w", err)
		}
		if err := writeDescribeOutput(os.Stdout, s, opts, func(w io.Writer) { writeDescribeSecretSummary(w, s) }); err != nil {
			return err
		}
	case repl.KindJob:
		job, err := fetchDescribeJob(ctx, env.RawConfig(), env.CurrentContext(), obj.Namespace, obj.Name)
		if err != nil {
			return fmt.Errorf("get job: %w", err)
		}
		if err := writeDescribeOutput(os.Stdout, job, opts, func(w io.Writer) { writeDescribeJobSummary(w, job) }); err != nil {
			return err
		}
	case repl.KindService:
		svc, err := fetchDescribeService(ctx, env.RawConfig(), env.CurrentContext(), obj.Namespace, obj.Name)
		if err != nil {
			return fmt.Errorf("get service: %w", err)
		}
		if err := writeDescribeOutput(os.Stdout, svc, opts, func(w io.Writer) { writeDescribeServiceSummary(w, svc) }); err != nil {
			return err
		}
	case repl.KindPersistentVolume:
		pv, err := fetchDescribePersistentVolume(ctx, env.RawConfig(), env.CurrentContext(), obj.Name)
		if err != nil {
			return fmt.Errorf("get persistentvolume: %w", err)
		}
		if err := writeDescribeOutput(os.Stdout, pv, opts, func(w io.Writer) { writeDescribePersistentVolumeSummary(w, pv) }); err != nil {
			return err
		}
	case repl.KindCronJob:
		cj, err := fetchDescribeCronJob(ctx, env.RawConfig(), env.CurrentContext(), obj.Namespace, obj.Name)
		if err != nil {
			return fmt.Errorf("get cronjob: %w", err)
		}
		if err := writeDescribeOutput(os.Stdout, cj, opts, func(w io.Writer) { writeDescribeCronJobSummary(w, cj) }); err != nil {
			return err
		}
	case repl.KindDaemonSet:
		ds, err := fetchDescribeDaemonSet(ctx, env.RawConfig(), env.CurrentContext(), obj.Namespace, obj.Name)
		if err != nil {
			return fmt.Errorf("get daemonset: %w", err)
		}
		if err := writeDescribeOutput(os.Stdout, ds, opts, func(w io.Writer) { writeDescribeDaemonSetSummary(w, ds) }); err != nil {
			return err
		}
	case repl.KindStorageClass:
		sc, err := fetchDescribeStorageClass(ctx, env.RawConfig(), env.CurrentContext(), obj.Name)
		if err != nil {
			return fmt.Errorf("get storageclass: %w", err)
		}
		if err := writeDescribeOutput(os.Stdout, sc, opts, func(w io.Writer) { writeDescribeStorageClassSummary(w, sc) }); err != nil {
			return err
		}
	case repl.KindDynamic:
		res, err := fetchDescribeDynamicResource(ctx, env.RawConfig(), env.CurrentContext(), obj)
		if err != nil {
			return fmt.Errorf("get %s: %w", describeObjectKindName(obj), err)
		}
		if !opts.json && !opts.yaml {
			fmt.Printf("%s not supported without -j or -y yet\n", dynamicDescribeName(obj))
			return nil
		}
		if err := writeDescribeOutput(os.Stdout, res.Object, opts, func(io.Writer) {}); err != nil {
			return err
		}
	default:
		return fmt.Errorf("describe does not support selected object kind %q", describeObjectKindName(obj))
	}

	if opts.events {
		events, err := fetchDescribeEvents(ctx, env.RawConfig(), env.CurrentContext(), obj)
		if err != nil {
			return fmt.Errorf("list %s events: %w", strings.ToLower(describeObjectKindName(obj)), err)
		}
		sort.Slice(events, func(i, j int) bool {
			return describeEventTime(events[i]).Before(describeEventTime(events[j]))
		})
		writeDescribeEvents(os.Stdout, events)
	}

	return nil
}

func writeDescribeOutput(out io.Writer, value any, opts describeOptions, writeSummary func(io.Writer)) error {
	switch {
	case opts.json:
		return writeDescribeJSON(out, value)
	case opts.yaml:
		return writeDescribeYAML(out, value)
	default:
		writeSummary(out)
		return nil
	}
}

func writeDescribeJSON(out io.Writer, value any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func writeDescribeYAML(out io.Writer, value any) error {
	data, err := yaml.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		_, err = out.Write([]byte("\n"))
		return err
	}
	return nil
}

func writef(out io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(out, format, args...)
}

func writeln(out io.Writer, args ...any) {
	_, _ = fmt.Fprintln(out, args...)
}

func writeDescribePodSummary(out io.Writer, pod *corev1.Pod) {
	writef(out, "Name:             %s\n", pod.Name)
	writef(out, "Namespace:        %s\n", pod.Namespace)
	writef(out, "Phase:            %s\n", fallbackString(string(pod.Status.Phase), "<none>"))
	writef(out, "Node:             %s\n", fallbackString(pod.Spec.NodeName, "<none>"))
	writef(out, "Pod IP:           %s\n", fallbackString(pod.Status.PodIP, "<none>"))
	writef(out, "Host IP:          %s\n", fallbackString(pod.Status.HostIP, "<none>"))
	writef(out, "Service Account:  %s\n", fallbackString(pod.Spec.ServiceAccountName, "default"))
	writef(out, "Priority Class:   %s\n", fallbackString(pod.Spec.PriorityClassName, "<none>"))
	writef(out, "Restart Policy:   %s\n", fallbackString(string(pod.Spec.RestartPolicy), "<none>"))
	writef(out, "Created At:       %s\n", formatDescribeTime(pod.CreationTimestamp.Time))
	writef(out, "Labels:           %s\n", formatStringMap(pod.Labels))
	writef(out, "Annotations:      %s\n", formatStringMap(pod.Annotations))

	writeln(out)
	writeln(out, "Containers:")
	if len(pod.Status.ContainerStatuses) == 0 {
		writeln(out, "  <none>")
	} else {
		for _, status := range pod.Status.ContainerStatuses {
			writef(out, "  - Name:           %s\n", status.Name)
			writef(out, "    Image:          %s\n", fallbackString(status.Image, "<none>"))
			writef(out, "    Ready:          %t\n", status.Ready)
			writef(out, "    Restart Count:  %d\n", status.RestartCount)
			writef(out, "    State:          %s\n", formatContainerState(status.State))
			writef(out, "    Last State:     %s\n", formatContainerState(status.LastTerminationState))
		}
	}

	writeln(out)
	writeln(out, "Conditions:")
	writeDescribePodConditions(out, pod.Status.Conditions)
}

func writeDescribeNodeSummary(out io.Writer, node *corev1.Node) {
	writef(out, "Name:               %s\n", node.Name)
	writef(out, "Status:             %s\n", nodeStatus(*node))
	writef(out, "Roles:              %s\n", nodeRoles(*node))
	writef(out, "Created At:         %s\n", formatDescribeTime(node.CreationTimestamp.Time))
	writef(out, "Internal IP:        %s\n", nodeAddress(node, corev1.NodeInternalIP))
	writef(out, "External IP:        %s\n", nodeAddress(node, corev1.NodeExternalIP))
	writef(out, "Pod CIDR:           %s\n", fallbackString(node.Spec.PodCIDR, "<none>"))
	writef(out, "OS Image:           %s\n", fallbackString(node.Status.NodeInfo.OSImage, "<none>"))
	writef(out, "Kernel Version:     %s\n", fallbackString(node.Status.NodeInfo.KernelVersion, "<none>"))
	writef(out, "Container Runtime:  %s\n", fallbackString(node.Status.NodeInfo.ContainerRuntimeVersion, "<none>"))
	writef(out, "Kubelet Version:    %s\n", fallbackString(node.Status.NodeInfo.KubeletVersion, "<none>"))
	writef(out, "Architecture:       %s\n", fallbackString(node.Status.NodeInfo.Architecture, "<none>"))
	writef(out, "Operating System:   %s\n", fallbackString(node.Status.NodeInfo.OperatingSystem, "<none>"))
	writef(out, "Unschedulable:      %t\n", node.Spec.Unschedulable)
	writef(out, "Labels:             %s\n", formatStringMap(node.Labels))
	writef(out, "Annotations:        %s\n", formatStringMap(node.Annotations))

	writeln(out)
	writeln(out, "Conditions:")
	if len(node.Status.Conditions) == 0 {
		writeln(out, "  <none>")
		return
	}
	for _, cond := range node.Status.Conditions {
		writef(out, "  - %s=%s", cond.Type, cond.Status)
		if cond.Reason != "" {
			writef(out, " reason=%s", cond.Reason)
		}
		if cond.Message != "" {
			writef(out, " message=%s", cond.Message)
		}
		writeln(out)
	}
}

func writeDescribePodConditions(out io.Writer, conditions []corev1.PodCondition) {
	if len(conditions) == 0 {
		writeln(out, "  <none>")
		return
	}
	for _, cond := range conditions {
		writef(out, "  - %s=%s", cond.Type, cond.Status)
		if cond.Reason != "" {
			writef(out, " reason=%s", cond.Reason)
		}
		if cond.Message != "" {
			writef(out, " message=%s", cond.Message)
		}
		writeln(out)
	}
}

func writeDescribeEvents(out io.Writer, events []corev1.Event) {
	writeln(out)
	writeln(out, "Events:")
	if len(events) == 0 {
		writeln(out, "  <none>")
		return
	}
	for _, event := range events {
		writef(out, "  - %s  %s  %s  %s\n",
			formatDescribeTime(describeEventTime(event)),
			fallbackString(event.Type, "<none>"),
			fallbackString(event.Reason, "<none>"),
			fallbackString(strings.TrimSpace(event.Message), "<none>"),
		)
	}
}

func describeEventTime(event corev1.Event) time.Time {
	if !event.LastTimestamp.IsZero() {
		return event.LastTimestamp.Time
	}
	if !event.EventTime.IsZero() {
		return event.EventTime.Time
	}
	if !event.FirstTimestamp.IsZero() {
		return event.FirstTimestamp.Time
	}
	return time.Time{}
}

func describeKindName(kind repl.LastObjectKind) string {
	switch kind {
	case repl.KindPod:
		return "Pod"
	case repl.KindNode:
		return "Node"
	case repl.KindNamespace:
		return "Namespace"
	case repl.KindDeployment:
		return "Deployment"
	case repl.KindReplicaSet:
		return "ReplicaSet"
	case repl.KindStatefulSet:
		return "StatefulSet"
	case repl.KindConfigMap:
		return "ConfigMap"
	case repl.KindSecret:
		return "Secret"
	case repl.KindJob:
		return "Job"
	case repl.KindService:
		return "Service"
	case repl.KindPersistentVolume:
		return "PersistentVolume"
	case repl.KindCronJob:
		return "CronJob"
	case repl.KindDaemonSet:
		return "DaemonSet"
	case repl.KindStorageClass:
		return "StorageClass"
	case repl.KindDynamic:
		return "Dynamic"
	default:
		return "Unknown"
	}
}

func describeObjectKindName(obj repl.LastObject) string {
	if obj.Kind == repl.KindDynamic && obj.Dynamic != nil {
		if obj.Dynamic.Kind != "" {
			return obj.Dynamic.Kind
		}
		if obj.Dynamic.Resource != "" {
			return obj.Dynamic.Resource
		}
	}
	return describeKindName(obj.Kind)
}

func dynamicDescribeName(obj repl.LastObject) string {
	if obj.Dynamic != nil && obj.Dynamic.Resource != "" {
		return obj.Dynamic.Resource
	}
	return strings.ToLower(describeObjectKindName(obj))
}

func writeDescribeNamespaceSummary(out io.Writer, ns *corev1.Namespace) {
	writef(out, "Name:             %s\n", ns.Name)
	writef(out, "Status:           %s\n", fallbackString(string(ns.Status.Phase), "<none>"))
	writef(out, "Created At:       %s\n", formatDescribeTime(ns.CreationTimestamp.Time))
	writef(out, "Labels:           %s\n", formatStringMap(ns.Labels))
	writef(out, "Annotations:      %s\n", formatStringMap(ns.Annotations))
}

func writeDescribeDeploymentSummary(out io.Writer, dep *appsv1.Deployment) {
	desired := int32(0)
	if dep.Spec.Replicas != nil {
		desired = *dep.Spec.Replicas
	}
	unavailable := dep.Status.UnavailableReplicas
	writef(out, "Name:             %s\n", dep.Name)
	writef(out, "Namespace:        %s\n", dep.Namespace)
	writef(out, "Replicas:         %d desired | %d updated | %d total | %d available | %d unavailable\n",
		desired, dep.Status.UpdatedReplicas, dep.Status.Replicas, dep.Status.AvailableReplicas, unavailable)
	writef(out, "Strategy:         %s\n", fallbackString(string(dep.Spec.Strategy.Type), "<none>"))
	writef(out, "Min Ready:        %d seconds\n", dep.Spec.MinReadySeconds)
	writef(out, "Created At:       %s\n", formatDescribeTime(dep.CreationTimestamp.Time))
	writef(out, "Labels:           %s\n", formatStringMap(dep.Labels))
	writef(out, "Annotations:      %s\n", formatStringMap(dep.Annotations))
	if dep.Spec.Selector != nil {
		writef(out, "Selector:         %s\n", formatStringMap(dep.Spec.Selector.MatchLabels))
	} else {
		writef(out, "Selector:         <none>\n")
	}

	writeln(out)
	writeln(out, "Containers:")
	if len(dep.Spec.Template.Spec.Containers) == 0 {
		writeln(out, "  <none>")
	} else {
		for _, c := range dep.Spec.Template.Spec.Containers {
			writef(out, "  - Name:   %s\n", c.Name)
			writef(out, "    Image:  %s\n", fallbackString(c.Image, "<none>"))
		}
	}

	writeln(out)
	writeln(out, "Conditions:")
	if len(dep.Status.Conditions) == 0 {
		writeln(out, "  <none>")
	} else {
		for _, cond := range dep.Status.Conditions {
			writef(out, "  - %s=%s", cond.Type, cond.Status)
			if cond.Reason != "" {
				writef(out, " reason=%s", cond.Reason)
			}
			if cond.Message != "" {
				writef(out, " message=%s", cond.Message)
			}
			writeln(out)
		}
	}
}

func writeDescribeStatefulSetSummary(out io.Writer, sts *appsv1.StatefulSet) {
	desired := int32(0)
	if sts.Spec.Replicas != nil {
		desired = *sts.Spec.Replicas
	}
	writef(out, "Name:                  %s\n", sts.Name)
	writef(out, "Namespace:             %s\n", sts.Namespace)
	writef(out, "Replicas:              %d desired | %d ready\n", desired, sts.Status.ReadyReplicas)
	writef(out, "Service Name:          %s\n", fallbackString(sts.Spec.ServiceName, "<none>"))
	writef(out, "Pod Management Policy: %s\n", fallbackString(string(sts.Spec.PodManagementPolicy), "<none>"))
	writef(out, "Update Strategy:       %s\n", fallbackString(string(sts.Spec.UpdateStrategy.Type), "<none>"))
	writef(out, "Created At:            %s\n", formatDescribeTime(sts.CreationTimestamp.Time))
	writef(out, "Labels:                %s\n", formatStringMap(sts.Labels))
	writef(out, "Annotations:           %s\n", formatStringMap(sts.Annotations))
	if sts.Spec.Selector != nil {
		writef(out, "Selector:              %s\n", formatStringMap(sts.Spec.Selector.MatchLabels))
	} else {
		writef(out, "Selector:              <none>\n")
	}

	writeln(out)
	writeln(out, "Containers:")
	if len(sts.Spec.Template.Spec.Containers) == 0 {
		writeln(out, "  <none>")
	} else {
		for _, c := range sts.Spec.Template.Spec.Containers {
			writef(out, "  - Name:   %s\n", c.Name)
			writef(out, "    Image:  %s\n", fallbackString(c.Image, "<none>"))
		}
	}
}

func writeDescribeReplicaSetSummary(out io.Writer, rs *appsv1.ReplicaSet) {
	desired := int32(0)
	if rs.Spec.Replicas != nil {
		desired = *rs.Spec.Replicas
	}
	writef(out, "Name:             %s\n", rs.Name)
	writef(out, "Namespace:        %s\n", rs.Namespace)
	writef(out, "Replicas:         %d desired | %d current | %d ready\n", desired, rs.Status.Replicas, rs.Status.ReadyReplicas)
	writef(out, "Created At:       %s\n", formatDescribeTime(rs.CreationTimestamp.Time))
	writef(out, "Labels:           %s\n", formatStringMap(rs.Labels))
	writef(out, "Annotations:      %s\n", formatStringMap(rs.Annotations))
	if rs.Spec.Selector.MatchLabels != nil {
		writef(out, "Selector:         %s\n", formatStringMap(rs.Spec.Selector.MatchLabels))
	} else {
		writef(out, "Selector:         <none>\n")
	}

	writeln(out)
	writeln(out, "Containers:")
	if len(rs.Spec.Template.Spec.Containers) == 0 {
		writeln(out, "  <none>")
	} else {
		for _, c := range rs.Spec.Template.Spec.Containers {
			writef(out, "  - Name:   %s\n", c.Name)
			writef(out, "    Image:  %s\n", fallbackString(c.Image, "<none>"))
		}
	}
}

func writeDescribeJobSummary(out io.Writer, job *batchv1.Job) {
	completions := int32(0)
	if job.Spec.Completions != nil {
		completions = *job.Spec.Completions
	}
	writef(out, "Name:             %s\n", job.Name)
	writef(out, "Namespace:        %s\n", job.Namespace)
	writef(out, "Completions:      %d/%d\n", job.Status.Succeeded, completions)
	writef(out, "Parallelism:      %d\n", valueOrZero(job.Spec.Parallelism))
	writef(out, "Created At:       %s\n", formatDescribeTime(job.CreationTimestamp.Time))
	writef(out, "Labels:           %s\n", formatStringMap(job.Labels))
	writef(out, "Annotations:      %s\n", formatStringMap(job.Annotations))
	if job.Spec.Selector != nil {
		writef(out, "Selector:         %s\n", formatStringMap(job.Spec.Selector.MatchLabels))
	} else {
		writef(out, "Selector:         <none>\n")
	}

	writeln(out)
	writeln(out, "Containers:")
	if len(job.Spec.Template.Spec.Containers) == 0 {
		writeln(out, "  <none>")
	} else {
		for _, c := range job.Spec.Template.Spec.Containers {
			writef(out, "  - Name:   %s\n", c.Name)
			writef(out, "    Image:  %s\n", fallbackString(c.Image, "<none>"))
		}
	}
}

func nodeAddress(node *corev1.Node, addressType corev1.NodeAddressType) string {
	for _, addr := range node.Status.Addresses {
		if addr.Type == addressType && addr.Address != "" {
			return addr.Address
		}
	}
	return "<none>"
}

func formatDescribeTime(t time.Time) string {
	if t.IsZero() {
		return "<none>"
	}
	return t.Format(time.RFC3339)
}

func formatStringMap(values map[string]string) string {
	if len(values) == 0 {
		return "<none>"
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, values[key]))
	}
	return strings.Join(parts, ", ")
}

func formatContainerState(state corev1.ContainerState) string {
	switch {
	case state.Running != nil:
		return fmt.Sprintf("Running since %s", formatDescribeTime(state.Running.StartedAt.Time))
	case state.Waiting != nil:
		if state.Waiting.Reason == "" {
			return "Waiting"
		}
		return fmt.Sprintf("Waiting (%s)", state.Waiting.Reason)
	case state.Terminated != nil:
		if state.Terminated.Reason == "" {
			return fmt.Sprintf("Terminated (exit %d)", state.Terminated.ExitCode)
		}
		return fmt.Sprintf("Terminated (%s, exit %d)", state.Terminated.Reason, state.Terminated.ExitCode)
	default:
		return "<none>"
	}
}

func fallbackString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func writeDescribeServiceSummary(out io.Writer, svc *corev1.Service) {
	writef(out, "Name:                     %s\n", svc.Name)
	writef(out, "Namespace:                %s\n", svc.Namespace)
	writef(out, "Type:                     %s\n", string(svc.Spec.Type))
	writef(out, "Cluster IP:               %s\n", fallbackString(svc.Spec.ClusterIP, "<none>"))
	writef(out, "External IP:              %s\n", serviceExternalIP(*svc))
	writef(out, "Session Affinity:         %s\n", fallbackString(string(svc.Spec.SessionAffinity), "<none>"))
	writef(out, "External Traffic Policy:  %s\n", fallbackString(string(svc.Spec.ExternalTrafficPolicy), "<none>"))
	writef(out, "Selector:                 %s\n", formatStringMap(svc.Spec.Selector))
	writef(out, "Created At:               %s\n", formatDescribeTime(svc.CreationTimestamp.Time))
	writef(out, "Labels:                   %s\n", formatStringMap(svc.Labels))
	writef(out, "Annotations:              %s\n", formatStringMap(svc.Annotations))

	writeln(out)
	writeln(out, "Ports:")
	if len(svc.Spec.Ports) == 0 {
		writeln(out, "  <none>")
	} else {
		for _, p := range svc.Spec.Ports {
			proto := string(p.Protocol)
			if proto == "" {
				proto = "TCP"
			}
			name := p.Name
			if name == "" {
				name = "<unnamed>"
			}
			if p.NodePort != 0 {
				writef(out, "  - Name:      %s\n    Port:      %d/%s\n    NodePort:  %d\n", name, p.Port, proto, p.NodePort)
			} else {
				writef(out, "  - Name:      %s\n    Port:      %d/%s\n", name, p.Port, proto)
			}
		}
	}
}

func writeDescribePersistentVolumeSummary(out io.Writer, pv *corev1.PersistentVolume) {
	capacity := "<none>"
	if q, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
		capacity = q.String()
	}
	volumeMode := "<none>"
	if pv.Spec.VolumeMode != nil {
		volumeMode = string(*pv.Spec.VolumeMode)
	}
	writef(out, "Name:             %s\n", pv.Name)
	writef(out, "Capacity:         %s\n", capacity)
	writef(out, "Access Modes:     %s\n", formatAccessModes(pv.Spec.AccessModes))
	writef(out, "Reclaim Policy:   %s\n", fallbackString(string(pv.Spec.PersistentVolumeReclaimPolicy), "<none>"))
	writef(out, "Status:           %s\n", fallbackString(string(pv.Status.Phase), "<none>"))
	writef(out, "Claim:            %s\n", fallbackString(pvClaimRef(*pv), "<none>"))
	writef(out, "Storage Class:    %s\n", fallbackString(pv.Spec.StorageClassName, "<none>"))
	writef(out, "Volume Mode:      %s\n", volumeMode)
	writef(out, "Created At:       %s\n", formatDescribeTime(pv.CreationTimestamp.Time))
	writef(out, "Labels:           %s\n", formatStringMap(pv.Labels))
	writef(out, "Annotations:      %s\n", formatStringMap(pv.Annotations))
	if pv.Status.Reason != "" {
		writef(out, "Reason:           %s\n", pv.Status.Reason)
	}
	if pv.Status.Message != "" {
		writef(out, "Message:          %s\n", pv.Status.Message)
	}
}

func writeDescribeCronJobSummary(out io.Writer, cj *batchv1.CronJob) {
	suspend := "False"
	if cj.Spec.Suspend != nil && *cj.Spec.Suspend {
		suspend = "True"
	}
	lastSchedule := "<none>"
	if cj.Status.LastScheduleTime != nil {
		lastSchedule = formatDescribeTime(cj.Status.LastScheduleTime.Time)
	}
	lastSuccessful := "<none>"
	if cj.Status.LastSuccessfulTime != nil {
		lastSuccessful = formatDescribeTime(cj.Status.LastSuccessfulTime.Time)
	}
	writef(out, "Name:                %s\n", cj.Name)
	writef(out, "Namespace:           %s\n", cj.Namespace)
	writef(out, "Schedule:            %s\n", cj.Spec.Schedule)
	writef(out, "Suspend:             %s\n", suspend)
	writef(out, "Active:              %d\n", len(cj.Status.Active))
	writef(out, "Last Schedule:       %s\n", lastSchedule)
	writef(out, "Last Successful:     %s\n", lastSuccessful)
	writef(out, "Concurrency Policy:  %s\n", fallbackString(string(cj.Spec.ConcurrencyPolicy), "<none>"))
	writef(out, "Created At:          %s\n", formatDescribeTime(cj.CreationTimestamp.Time))
	writef(out, "Labels:              %s\n", formatStringMap(cj.Labels))
	writef(out, "Annotations:         %s\n", formatStringMap(cj.Annotations))

	writeln(out)
	writeln(out, "Containers:")
	containers := cj.Spec.JobTemplate.Spec.Template.Spec.Containers
	if len(containers) == 0 {
		writeln(out, "  <none>")
	} else {
		for _, c := range containers {
			writef(out, "  - Name:   %s\n", c.Name)
			writef(out, "    Image:  %s\n", fallbackString(c.Image, "<none>"))
		}
	}
}

func writeDescribeDaemonSetSummary(out io.Writer, ds *appsv1.DaemonSet) {
	writef(out, "Name:              %s\n", ds.Name)
	writef(out, "Namespace:         %s\n", ds.Namespace)
	writef(out, "Desired:           %d\n", ds.Status.DesiredNumberScheduled)
	writef(out, "Current:           %d\n", ds.Status.CurrentNumberScheduled)
	writef(out, "Ready:             %d\n", ds.Status.NumberReady)
	writef(out, "Up-To-Date:        %d\n", ds.Status.UpdatedNumberScheduled)
	writef(out, "Available:         %d\n", ds.Status.NumberAvailable)
	writef(out, "Update Strategy:   %s\n", fallbackString(string(ds.Spec.UpdateStrategy.Type), "<none>"))
	writef(out, "Created At:        %s\n", formatDescribeTime(ds.CreationTimestamp.Time))
	writef(out, "Labels:            %s\n", formatStringMap(ds.Labels))
	writef(out, "Annotations:       %s\n", formatStringMap(ds.Annotations))
	if ds.Spec.Selector != nil {
		writef(out, "Selector:          %s\n", formatStringMap(ds.Spec.Selector.MatchLabels))
	} else {
		writef(out, "Selector:          <none>\n")
	}

	writeln(out)
	writeln(out, "Containers:")
	if len(ds.Spec.Template.Spec.Containers) == 0 {
		writeln(out, "  <none>")
	} else {
		for _, c := range ds.Spec.Template.Spec.Containers {
			writef(out, "  - Name:   %s\n", c.Name)
			writef(out, "    Image:  %s\n", fallbackString(c.Image, "<none>"))
		}
	}
}

func writeDescribeStorageClassSummary(out io.Writer, sc *storagev1.StorageClass) {
	bindingMode := "<none>"
	if sc.VolumeBindingMode != nil {
		bindingMode = string(*sc.VolumeBindingMode)
	}
	reclaimPolicy := "<none>"
	if sc.ReclaimPolicy != nil {
		reclaimPolicy = string(*sc.ReclaimPolicy)
	}
	writef(out, "Name:             %s\n", sc.Name)
	writef(out, "Provisioner:      %s\n", fallbackString(sc.Provisioner, "<none>"))
	writef(out, "Reclaim Policy:   %s\n", reclaimPolicy)
	writef(out, "Binding Mode:     %s\n", bindingMode)
	writef(out, "Allow Expansion:  %t\n", sc.AllowVolumeExpansion != nil && *sc.AllowVolumeExpansion)
	writef(out, "Created At:       %s\n", formatDescribeTime(sc.CreationTimestamp.Time))
	writef(out, "Labels:           %s\n", formatStringMap(sc.Labels))
	writef(out, "Annotations:      %s\n", formatStringMap(sc.Annotations))
}

func valueOrZero(v *int32) int32 {
	if v == nil {
		return 0
	}
	return *v
}

func writeDescribeConfigMapSummary(out io.Writer, cm *corev1.ConfigMap) {
	dataCount := len(cm.Data) + len(cm.BinaryData)
	writef(out, "Name:        %s\n", cm.Name)
	writef(out, "Namespace:   %s\n", cm.Namespace)
	writef(out, "Data:        %d entries\n", dataCount)
	writef(out, "Created At:  %s\n", formatDescribeTime(cm.CreationTimestamp.Time))
	writef(out, "Labels:      %s\n", formatStringMap(cm.Labels))
	writef(out, "Annotations: %s\n", formatStringMap(cm.Annotations))

	writeln(out)
	writeln(out, "Keys:")
	if dataCount == 0 {
		writeln(out, "  <none>")
		return
	}
	keys := make([]string, 0, dataCount)
	for k := range cm.Data {
		keys = append(keys, k)
	}
	for k := range cm.BinaryData {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		writef(out, "  - %s\n", k)
	}
}

func writeDescribeSecretSummary(out io.Writer, s *corev1.Secret) {
	writef(out, "Name:        %s\n", s.Name)
	writef(out, "Namespace:   %s\n", s.Namespace)
	writef(out, "Type:        %s\n", fallbackString(string(s.Type), "<none>"))
	writef(out, "Data:        %d entries\n", len(s.Data))
	writef(out, "Created At:  %s\n", formatDescribeTime(s.CreationTimestamp.Time))
	writef(out, "Labels:      %s\n", formatStringMap(s.Labels))
	writef(out, "Annotations: %s\n", formatStringMap(s.Annotations))

	writeln(out)
	writeln(out, "Data:")
	if len(s.Data) == 0 {
		writeln(out, "  <none>")
		return
	}
	keys := make([]string, 0, len(s.Data))
	for k := range s.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		// Values are redacted: show key and size only, never the actual value.
		writef(out, "  - %s: %d bytes\n", k, len(s.Data[k]))
	}
}
