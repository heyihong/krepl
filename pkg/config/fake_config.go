package config

import clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

func MakeFakeConfig() clientcmdapi.Config {
	cfg := clientcmdapi.NewConfig()
	cfg.Clusters["cluster-a"] = &clientcmdapi.Cluster{Server: "https://a.example.com"}
	cfg.Clusters["cluster-b"] = &clientcmdapi.Cluster{Server: "https://b.example.com"}
	cfg.AuthInfos["user-a"] = &clientcmdapi.AuthInfo{}
	cfg.AuthInfos["user-b"] = &clientcmdapi.AuthInfo{}
	cfg.Contexts["ctx-a"] = &clientcmdapi.Context{Cluster: "cluster-a", AuthInfo: "user-a"}
	cfg.Contexts["ctx-b"] = &clientcmdapi.Context{Cluster: "cluster-b", AuthInfo: "user-b"}
	cfg.CurrentContext = "ctx-a"
	return *cfg
}
