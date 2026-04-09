package config

import (
	"testing"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestBuildRESTConfigForContext(t *testing.T) {
	cfg := clientcmdapi.NewConfig()
	cfg.Clusters["cluster-a"] = &clientcmdapi.Cluster{Server: "https://a.example.com"}
	cfg.AuthInfos["user-a"] = &clientcmdapi.AuthInfo{}
	cfg.Contexts["ctx-a"] = &clientcmdapi.Context{Cluster: "cluster-a", AuthInfo: "user-a"}

	restConfig, err := BuildRESTConfigForContext(*cfg, "ctx-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if restConfig.Host != "https://a.example.com" {
		t.Fatalf("expected host https://a.example.com, got %q", restConfig.Host)
	}
}
