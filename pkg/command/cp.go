package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/discovery"
	cachedmemory "k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/kubectl/pkg/cmd/cp"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/heyihong/krepl/pkg/config"
	"github.com/heyihong/krepl/pkg/repl"
)

type copyCommandOptions struct {
	container  string
	noPreserve bool
	retries    int
}

type kubectlCopyRunner func(getter genericclioptions.RESTClientGetter, options copyCommandOptions, args []string) error

var runKubectlCopy kubectlCopyRunner = executeKubectlCopy

func newCopyCmd() *cmd {
	var container string
	var noPreserve bool
	var retries int

	cmd := &cmd{
		use:   "cp [flags] <src> <dest>",
		short: "copy files to and from containers",
		long: "Copy files and directories to and from containers using the upstream kubectl cp implementation.\n" +
			"Use kubectl-style specs like `pod:/path` or `namespace/pod:/path`.\n" +
			"Within krepl, `:/path` is shorthand for the active pod.",
		example: "cp ./local.txt :/tmp/remote.txt\ncp :/var/log/app.log ./app.log\ncp ./dump.txt other-ns/pod-a:/tmp/dump.txt",
		args:    exactArgs(2),
	}
	flags := cmd.flags()
	flags.StringVarP(&container, "container", "c", "", "container name (default: first container)")
	flags.BoolVar(&noPreserve, "no-preserve", false, "do not preserve ownership and permissions in the container")
	flags.IntVar(&retries, "retries", 0, "number of retries when copying from a container (negative = infinite)")

	cmd.runE = func(env *repl.Env, args []string) error {
		return runCopy(env, copyCommandOptions{
			container:  container,
			noPreserve: noPreserve,
			retries:    retries,
		}, args)
	}
	return cmd
}

func runCopy(env *repl.Env, options copyCommandOptions, args []string) error {
	if env.CurrentContext() == "" {
		return fmt.Errorf("no active context")
	}

	normalizedArgs, err := normalizeCopyArgs(env, args)
	if err != nil {
		return err
	}

	getter := newCopyRESTClientGetter(env.RawConfig(), env.CurrentContext(), env.Namespace())
	return runKubectlCopy(getter, options, normalizedArgs)
}

func normalizeCopyArgs(env *repl.Env, args []string) ([]string, error) {
	normalized := make([]string, 0, len(args))
	for _, arg := range args {
		value, err := normalizeCopyArg(env, arg)
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, value)
	}
	return normalized, nil
}

func normalizeCopyArg(env *repl.Env, arg string) (string, error) {
	if !strings.HasPrefix(arg, ":") {
		return arg, nil
	}

	// krepl lets users target the selected pod with :/path so common copy
	// flows match other selection-driven commands like logs/exec.
	if len(env.CurrentSelection()) == 0 {
		return "", fmt.Errorf("no active pod; select one by number after running `pods`, or use an explicit `pod:/path` filespec")
	}
	if env.HasRangeSelection() {
		return "", fmt.Errorf("range selection is not supported for cp shorthand; select a single pod or use an explicit `pod:/path` filespec")
	}

	obj := env.CurrentObject()
	if obj == nil || obj.Kind != repl.KindPod {
		return "", fmt.Errorf("cp shorthand requires a single selected pod")
	}

	if obj.Namespace == "" {
		return obj.Name + arg, nil
	}
	return obj.Namespace + "/" + obj.Name + arg, nil
}

func executeKubectlCopy(getter genericclioptions.RESTClientGetter, options copyCommandOptions, args []string) error {
	streams := genericiooptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	copyOptions := cp.NewCopyOptions(streams)
	copyOptions.Container = options.container
	copyOptions.NoPreserve = options.noPreserve
	copyOptions.MaxTries = options.retries

	factory := cmdutil.NewFactory(getter)
	// CopyOptions.Complete expects a Cobra command so it can inspect command
	// metadata such as the parent command path. krepl does not use Cobra for
	// dispatch, so a minimal stub is enough here.
	cmd := &cobra.Command{Use: "cp"}
	if err := copyOptions.Complete(factory, cmd, args); err != nil {
		return err
	}
	if err := copyOptions.Validate(); err != nil {
		return err
	}
	return copyOptions.Run()
}

type copyRESTClientGetter struct {
	rawConfig   clientcmdapi.Config
	contextName string
	namespace   string
}

func newCopyRESTClientGetter(rawConfig clientcmdapi.Config, contextName, namespace string) *copyRESTClientGetter {
	return &copyRESTClientGetter{
		rawConfig:   rawConfig,
		contextName: contextName,
		namespace:   namespace,
	}
}

func (g *copyRESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	return config.BuildRESTConfigForContext(g.rawConfig, g.contextName)
}

func (g *copyRESTClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	restConfig, err := g.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	return cachedmemory.NewMemCacheClient(discoveryClient), nil
}

func (g *copyRESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	discoveryClient, err := g.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	return restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient), nil
}

func (g *copyRESTClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	overrides := &clientcmd.ConfigOverrides{
		CurrentContext: g.contextName,
	}
	// Mirror the REPL's working namespace so kubectl cp resolves implicit
	// namespaces the same way the rest of krepl commands do.
	if g.namespace != "" {
		overrides.Context.Namespace = g.namespace
	}
	return clientcmd.NewDefaultClientConfig(g.rawConfig, overrides)
}
