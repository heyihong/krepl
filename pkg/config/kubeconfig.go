package config

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

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
	return clientcmd.NewNonInteractiveClientConfig(
		rawConfig,
		contextName,
		&clientcmd.ConfigOverrides{},
		nil,
	).ClientConfig()
}
