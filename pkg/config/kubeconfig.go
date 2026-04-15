package config

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var lookPath = exec.LookPath
var modifyConfig = clientcmd.ModifyConfig

// LoadRawConfig reads and merges all kubeconfig files according to the
// KUBECONFIG env var, falling back to ~/.kube/config.
func LoadRawConfig() (clientcmdapi.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{},
	)
	return clientConfig.RawConfig()
}

// SetCurrentContext updates kubeconfig's current-context on disk.
func SetCurrentContext(name string) error {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{},
	)

	rawConfig, err := clientConfig.RawConfig()
	if err != nil {
		return err
	}
	if _, ok := rawConfig.Contexts[name]; !ok {
		return fmt.Errorf("unknown context %q", name)
	}

	rawConfig.CurrentContext = name
	return modifyConfig(loadingRules, rawConfig, false)
}

// SetContextNamespace updates the namespace for a named kubeconfig context on disk.
func SetContextNamespace(contextName, namespace string) error {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{},
	)

	rawConfig, err := clientConfig.RawConfig()
	if err != nil {
		return err
	}
	ctx, ok := rawConfig.Contexts[contextName]
	if !ok {
		return fmt.Errorf("unknown context %q", contextName)
	}

	ctx.Namespace = namespace
	return modifyConfig(loadingRules, rawConfig, false)
}

// DeleteContext removes a named context from the kubeconfig on disk.
func DeleteContext(name string) error {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{},
	)

	rawConfig, err := clientConfig.RawConfig()
	if err != nil {
		return err
	}
	if _, ok := rawConfig.Contexts[name]; !ok {
		return fmt.Errorf("unknown context %q", name)
	}

	delete(rawConfig.Contexts, name)
	return modifyConfig(loadingRules, rawConfig, false)
}

// BuildClientForContext creates a kubernetes.Clientset for the named context.
func BuildClientForContext(rawConfig clientcmdapi.Config, contextName string) (*kubernetes.Clientset, error) {
	restConfig, err := BuildRESTConfigForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(restConfig)
}

// BuildRESTConfigForContext creates a rest.Config for the named context.
func BuildRESTConfigForContext(rawConfig clientcmdapi.Config, contextName string) (*rest.Config, error) {
	restConfig, err := clientcmd.NewNonInteractiveClientConfig(
		rawConfig,
		contextName,
		&clientcmd.ConfigOverrides{},
		nil,
	).ClientConfig()
	if err != nil {
		return nil, err
	}
	if err := applyGKEAuthFallback(rawConfig, contextName, restConfig); err != nil {
		return nil, err
	}
	return restConfig, nil
}

func applyGKEAuthFallback(rawConfig clientcmdapi.Config, contextName string, restConfig *rest.Config) error {
	authInfo := authInfoForContext(rawConfig, contextName)
	if authInfo == nil || authInfo.Exec == nil {
		return nil
	}
	if filepath.Base(authInfo.Exec.Command) != "gke-gcloud-auth-plugin" {
		return nil
	}
	if _, err := lookPath(authInfo.Exec.Command); err == nil {
		return nil
	}

	if restConfig.BearerToken == "" && restConfig.BearerTokenFile == "" {
		return fmt.Errorf("gke auth plugin %q is missing and kubeconfig bearerToken is empty; set a kubeconfig token/tokenFile or run %q once to install the GKE auth plugin", "gke-gcloud-auth-plugin", "gcloud components install gke-gcloud-auth-plugin")
	}

	restConfig.ExecProvider = nil
	return nil
}

func authInfoForContext(rawConfig clientcmdapi.Config, contextName string) *clientcmdapi.AuthInfo {
	ctx, ok := rawConfig.Contexts[contextName]
	if !ok || ctx == nil || ctx.AuthInfo == "" {
		return nil
	}
	return rawConfig.AuthInfos[ctx.AuthInfo]
}
