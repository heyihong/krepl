package command

import (
	"context"
	"fmt"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	kubectldrain "k8s.io/kubectl/pkg/drain"

	"github.com/heyihong/krepl/pkg/config"
	"github.com/heyihong/krepl/pkg/repl"
)

var setNodeSchedulableForContext = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName, nodeName string, schedulable bool) error {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return err
	}

	node, err := client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	return kubectldrain.RunCordonOrUncordon(&kubectldrain.Helper{
		Ctx:    ctx,
		Client: client,
	}, node, !schedulable)
}

type drainOptions struct {
	ignoreDaemonSets   bool
	deleteEmptyDirData bool
	force              bool
	disableEviction    bool
	gracePeriodSeconds *int64
	dryRun             bool
}

var drainNodeForContext = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName, nodeName string, opts drainOptions) error {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return err
	}
	return drainNodeWithClient(ctx, client, nodeName, opts)
}

func newCordonCmd() *cmd {
	return newNodeSchedulabilityCmd(
		"cordon",
		"mark the selected node as unschedulable",
		"Cordon the selected node or each node in the current range selection.\n"+
			"The command marks nodes as unschedulable so new pods are not scheduled onto them.",
		false,
	)
}

func newUncordonCmd() *cmd {
	return newNodeSchedulabilityCmd(
		"uncordon",
		"mark the selected node as schedulable",
		"Uncordon the selected node or each node in the current range selection.\n"+
			"The command clears the unschedulable flag so the scheduler can place new pods onto the node again.",
		true,
	)
}

func newDrainCmd() *cmd {
	var (
		ignoreDaemonSets   bool
		deleteEmptyDirData bool
		force              bool
		disableEviction    bool
		dryRun             bool
		gracePeriod        int64 = -1
	)

	cmd := &cmd{
		use:   "drain [flags]",
		short: "cordon the selected node and evict drainable pods",
		long: "Drain the selected node or each node in the current range selection.\n" +
			"The command first cordons the node, then evicts or deletes drainable pods while blocking on daemonset-managed pods,\n" +
			"unmanaged pods, and pods using emptyDir volumes unless the matching override flags are provided.",
		example: "drain --ignore-daemonsets --delete-emptydir-data\n" +
			"drain --ignore-daemonsets --delete-emptydir-data --force\n" +
			"drain --dry-run --ignore-daemonsets --delete-emptydir-data --force\n" +
			"drain --disable-eviction --gracePeriod 0 --ignore-daemonsets --delete-emptydir-data --force",
		args: noArgs,
	}
	cmd.flags().BoolVar(&ignoreDaemonSets, "ignore-daemonsets", false, "allow daemonset-managed pods to remain on the node")
	cmd.flags().BoolVar(&deleteEmptyDirData, "delete-emptydir-data", false, "allow eviction of pods that use emptyDir volumes")
	cmd.flags().BoolVar(&force, "force", false, "allow deletion of pods that are not controlled by a workload")
	cmd.flags().BoolVar(&disableEviction, "disable-eviction", false, "delete pods directly instead of using the eviction API")
	cmd.flags().BoolVar(&dryRun, "dry-run", false, "report what drain would do without changing the cluster")
	cmd.flags().Int64VarP(&gracePeriod, "gracePeriod", "g", -1, "grace period in seconds for pod eviction or deletion")

	cmd.runE = func(env *repl.Env, _ []string) error {
		if env.CurrentContext() == "" {
			return fmt.Errorf("no active context; use `context <name>` to set one")
		}
		if len(env.CurrentSelection()) == 0 {
			return fmt.Errorf("no active object; select one by number after running `nodes`")
		}
		if err := requireSelectionKind(env, repl.KindNode, "drain"); err != nil {
			return err
		}

		var gracePeriodSeconds *int64
		if cmd.flags().Changed("gracePeriod") {
			if gracePeriod < 0 {
				return fmt.Errorf("invalid --gracePeriod value: must be >= 0")
			}
			gracePeriodSeconds = &gracePeriod
		}

		opts := drainOptions{
			ignoreDaemonSets:   ignoreDaemonSets,
			deleteEmptyDirData: deleteEmptyDirData,
			force:              force,
			disableEviction:    disableEviction,
			gracePeriodSeconds: gracePeriodSeconds,
			dryRun:             dryRun,
		}

		return env.ApplyToSelection(func(obj repl.LastObject) error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := drainNodeForContext(ctx, env.RawConfig(), env.CurrentContext(), obj.Name, opts); err != nil {
				return fmt.Errorf("drain node %s: %w", obj.Name, err)
			}

			if opts.dryRun {
				fmt.Printf("node/%s drain dry-run complete\n", obj.Name)
			} else {
				fmt.Printf("node/%s drained\n", obj.Name)
			}
			return nil
		})
	}

	return cmd
}

func newNodeSchedulabilityCmd(use, short, long string, schedulable bool) *cmd {
	action := use
	return &cmd{
		use:   use,
		short: short,
		long:  long,
		args:  noArgs,
		runE: func(env *repl.Env, _ []string) error {
			if env.CurrentContext() == "" {
				return fmt.Errorf("no active context; use `context <name>` to set one")
			}
			if len(env.CurrentSelection()) == 0 {
				return fmt.Errorf("no active object; select one by number after running `nodes`")
			}
			if err := requireSelectionKind(env, repl.KindNode, action); err != nil {
				return err
			}

			return env.ApplyToSelection(func(obj repl.LastObject) error {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				if err := setNodeSchedulableForContext(ctx, env.RawConfig(), env.CurrentContext(), obj.Name, schedulable); err != nil {
					return fmt.Errorf("%s node %s: %w", action, obj.Name, err)
				}

				fmt.Printf("node/%s %sed\n", obj.Name, action)
				return nil
			})
		},
	}
}

func drainNodeWithClient(ctx context.Context, client kubernetes.Interface, nodeName string, opts drainOptions) error {
	drainer := newKubectlDrainer(ctx, client, opts)

	node, err := client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if opts.dryRun {
		list, errs := drainer.GetPodsForDeletion(nodeName)
		if errs != nil {
			return utilerrors.NewAggregate(errs)
		}
		if warnings := list.Warnings(); warnings != "" {
			fmt.Fprintf(os.Stderr, "WARNING: %s\n", warnings)
		}

		fmt.Printf("node/%s would be cordoned\n", nodeName)
		for _, pod := range list.Pods() {
			fmt.Printf("pod/%s/%s would be %s\n", pod.Namespace, pod.Name, drainActionVerb(opts))
		}
		return nil
	}

	if err := kubectldrain.RunCordonOrUncordon(drainer, node, true); err != nil {
		return err
	}
	if err := kubectldrain.RunNodeDrain(drainer, nodeName); err != nil {
		return err
	}

	return nil
}

func newKubectlDrainer(ctx context.Context, client kubernetes.Interface, opts drainOptions) *kubectldrain.Helper {
	gracePeriod := -1
	if opts.gracePeriodSeconds != nil {
		gracePeriod = int(*opts.gracePeriodSeconds)
	}

	drainer := &kubectldrain.Helper{
		Ctx:                 ctx,
		Client:              client,
		Force:               opts.force,
		GracePeriodSeconds:  gracePeriod,
		IgnoreAllDaemonSets: opts.ignoreDaemonSets,
		DeleteEmptyDirData:  opts.deleteEmptyDirData,
		DisableEviction:     opts.disableEviction,
		Timeout:             30 * time.Second,
		Out:                 os.Stdout,
		ErrOut:              os.Stderr,
	}
	if opts.dryRun {
		drainer.DryRunStrategy = cmdutil.DryRunClient
	}
	return drainer
}

func drainActionVerb(opts drainOptions) string {
	if opts.disableEviction {
		return "deleted"
	}
	return "evicted"
}
