package command

import (
	"bufio"
	"context"
	"fmt"
	"github.com/heyihong/krepl/pkg/config"
	"github.com/heyihong/krepl/pkg/repl"
	"io"
	"os"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type deleteOptions struct {
	gracePeriodSeconds *int64
	propagationPolicy  v1.DeletionPropagation
}

type deleteRESTClients struct {
	core  rest.Interface
	apps  rest.Interface
	batch rest.Interface
}

var buildDeleteClientForContext = config.BuildClientForContext

var readDeleteConfirmation = func() (string, error) {
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return line, nil
}

var deleteCurrentObject = performDeleteForObject

var runDeleteRequest = func(ctx context.Context, restClient rest.Interface, namespace, resource, name string, opts v1.DeleteOptions) error {
	var status v1.Status
	result := restClient.Delete().
		NamespaceIfScoped(namespace, namespace != "").
		Resource(resource).
		Name(name).
		Body(&opts).
		Do(ctx)
	if err := result.Into(&status); err != nil {
		return err
	}
	return nil
}

func newDeleteCmd() *cmd {
	var gracePeriod int64 = -1
	var cascadePolicy string
	var nowFlag, forceFlag bool

	cmd := &cmd{
		use:   "delete [flags]",
		short: "delete the active object (will ask for confirmation)",
		long: "Delete the active object or each object in the current range selection.\n" +
			"The command always asks for confirmation before sending the delete request and supports grace-period and cascade controls.",
		args: noArgs,
	}
	cmd.flags().Int64VarP(&gracePeriod, "gracePeriod", "g", -1, "grace period in seconds before forced deletion")
	cmd.flags().StringVarP(&cascadePolicy, "cascade", "c", "background", "cascade policy: background, foreground, or orphan")
	cmd.flags().BoolVar(&nowFlag, "now", false, "delete immediately (sets grace period to 1s)")
	cmd.flags().BoolVar(&forceFlag, "force", false, "force deletion (sets grace period to 0s)")

	cmd.runE = func(env *repl.Env, _ []string) error {
		graceChanged := cmd.flags().Changed("gracePeriod")
		setCount := 0
		if graceChanged {
			setCount++
		}
		if nowFlag {
			setCount++
		}
		if forceFlag {
			setCount++
		}
		if setCount > 1 {
			return fmt.Errorf("--gracePeriod, --now, and --force are mutually exclusive")
		}

		// Validate flags before checking for an active object, so invalid
		// flag values are surfaced without requiring an active selection.
		if graceChanged && gracePeriod < 0 {
			return fmt.Errorf("invalid --gracePeriod value: must be >= 0")
		}
		policy, err := parseDeleteCascade(cascadePolicy)
		if err != nil {
			return err
		}

		if len(env.CurrentSelection()) == 0 {
			return fmt.Errorf("no active object; select one by number after running `pods`, `nodes`, `deployments`, `replicasets`, `statefulsets`, `configmaps`, `secrets`, `cronjobs`, `daemonsets`, `persistentvolumes`, `services`, or `crd <resource>`")
		}
		if env.CurrentContext() == "" {
			return fmt.Errorf("no active context")
		}

		opts := deleteOptions{propagationPolicy: v1.DeletePropagationBackground}
		switch {
		case graceChanged:
			gp := gracePeriod
			opts.gracePeriodSeconds = &gp
		case nowFlag:
			gp := int64(1)
			opts.gracePeriodSeconds = &gp
		case forceFlag:
			gp := int64(0)
			opts.gracePeriodSeconds = &gp
		}
		opts.propagationPolicy = policy

		return env.ApplyToSelection(func(obj repl.LastObject) error {
			fmt.Printf("Delete %s %s [y/N]? ", strings.ToLower(describeObjectKindName(obj)), obj.Name)
			response, err := readDeleteConfirmation()
			if err != nil {
				return fmt.Errorf("read delete confirmation: %w", err)
			}
			switch strings.ToLower(strings.TrimSpace(response)) {
			case "y", "yes":
			default:
				fmt.Println("Not deleting")
				return nil
			}

			if err := deleteCurrentObject(env.RawConfig(), env.CurrentContext(), obj, opts); err != nil {
				return err
			}
			fmt.Println("Deleted")
			return nil
		})
	}
	return cmd
}

func parseDeleteCascade(value string) (v1.DeletionPropagation, error) {
	switch strings.ToLower(value) {
	case "background":
		return v1.DeletePropagationBackground, nil
	case "foreground":
		return v1.DeletePropagationForeground, nil
	case "orphan":
		return v1.DeletePropagationOrphan, nil
	default:
		return "", fmt.Errorf("invalid --cascade value: %q (expected background, foreground, or orphan)", value)
	}
}

func performDeleteForObject(rawConfig clientcmdapi.Config, contextName string, obj repl.LastObject, opts deleteOptions) error {
	client, err := buildDeleteClientForContext(rawConfig, contextName)
	if err != nil {
		return fmt.Errorf("build client: %w", err)
	}
	return performDeleteForObjectWithClientset(context.Background(), client, obj, opts)
}

func performDeleteForObjectWithClientset(ctx context.Context, client *kubernetes.Clientset, obj repl.LastObject, opts deleteOptions) error {
	return performDeleteForObjectWithRESTClients(ctx, deleteRESTClients{
		core:  client.CoreV1().RESTClient(),
		apps:  client.AppsV1().RESTClient(),
		batch: client.BatchV1().RESTClient(),
	}, obj, opts)
}

func performDeleteForObjectWithRESTClients(ctx context.Context, clients deleteRESTClients, obj repl.LastObject, opts deleteOptions) error {
	restClient, namespace, resource, err := deleteRESTTarget(clients, obj)
	if err != nil {
		return err
	}

	deleteOptions := v1.DeleteOptions{
		GracePeriodSeconds: opts.gracePeriodSeconds,
		PropagationPolicy:  &opts.propagationPolicy,
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return runDeleteRequest(timeoutCtx, restClient, namespace, resource, obj.Name, deleteOptions)
}

func deleteRESTTarget(clients deleteRESTClients, obj repl.LastObject) (rest.Interface, string, string, error) {
	switch obj.Kind {
	case repl.KindPod:
		return clients.core, obj.Namespace, "pods", nil
	case repl.KindNode:
		return clients.core, "", "nodes", nil
	case repl.KindDeployment:
		return clients.apps, obj.Namespace, "deployments", nil
	case repl.KindReplicaSet:
		return clients.apps, obj.Namespace, "replicasets", nil
	case repl.KindStatefulSet:
		return clients.apps, obj.Namespace, "statefulsets", nil
	case repl.KindConfigMap:
		return clients.core, obj.Namespace, "configmaps", nil
	case repl.KindSecret:
		return clients.core, obj.Namespace, "secrets", nil
	case repl.KindCronJob:
		return clients.batch, obj.Namespace, "cronjobs", nil
	case repl.KindDaemonSet:
		return clients.apps, obj.Namespace, "daemonsets", nil
	case repl.KindPersistentVolume:
		return clients.core, "", "persistentvolumes", nil
	case repl.KindService:
		return clients.core, obj.Namespace, "services", nil
	default:
		return nil, "", "", fmt.Errorf("delete does not support selected object kind %q", describeObjectKindName(obj))
	}
}
