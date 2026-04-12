package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	clientcmdapiv1 "k8s.io/client-go/tools/clientcmd/api/v1"
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

func TestBuildRESTConfigForContextUsesKubeconfigBearerToken(t *testing.T) {
	origLookPath := lookPath
	t.Cleanup(func() {
		lookPath = origLookPath
	})

	lookPath = func(file string) (string, error) {
		if file == "gke-gcloud-auth-plugin" {
			return "", errors.New("not found")
		}
		return "/bin/" + file, nil
	}

	cfg := gkeExecConfig()
	cfg.AuthInfos["user-a"].Token = "kubeconfig-token"
	restConfig, err := BuildRESTConfigForContext(cfg, "ctx-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if restConfig.BearerToken != "kubeconfig-token" {
		t.Fatalf("expected kubeconfig token, got %q", restConfig.BearerToken)
	}
	if restConfig.ExecProvider != nil {
		t.Fatal("expected exec provider to be cleared after fallback")
	}
}

func TestBuildRESTConfigForContextReturnsHelpfulErrorWhenBearerTokenMissing(t *testing.T) {
	origLookPath := lookPath
	t.Cleanup(func() {
		lookPath = origLookPath
	})

	lookPath = func(file string) (string, error) {
		if file == "gke-gcloud-auth-plugin" {
			return "", errors.New("not found")
		}
		return "/bin/" + file, nil
	}

	cfg := gkeExecConfig()
	_, err := BuildRESTConfigForContext(cfg, "ctx-a")
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); !containsAll(got, "gke-gcloud-auth-plugin", "bearerToken is empty", "gcloud components install gke-gcloud-auth-plugin") {
		t.Fatalf("expected helpful error, got %q", got)
	}
}

func TestBuildRESTConfigForContextUsesKubeconfigBearerTokenFile(t *testing.T) {
	origLookPath := lookPath
	t.Cleanup(func() {
		lookPath = origLookPath
	})

	lookPath = func(file string) (string, error) {
		if file == "gke-gcloud-auth-plugin" {
			return "", errors.New("not found")
		}
		return "/bin/" + file, nil
	}

	tokenFile, err := os.CreateTemp(t.TempDir(), "gke-token-*")
	if err != nil {
		t.Fatalf("create temp token file: %v", err)
	}
	if _, err := tokenFile.WriteString("file-token\n"); err != nil {
		t.Fatalf("write temp token file: %v", err)
	}
	if err := tokenFile.Close(); err != nil {
		t.Fatalf("close temp token file: %v", err)
	}

	cfg := gkeExecConfig()
	cfg.AuthInfos["user-a"].TokenFile = tokenFile.Name()
	restConfig, err := BuildRESTConfigForContext(cfg, "ctx-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if restConfig.BearerTokenFile != tokenFile.Name() {
		t.Fatalf("expected kubeconfig token file, got %q", restConfig.BearerTokenFile)
	}
	if restConfig.ExecProvider != nil {
		t.Fatal("expected exec provider to be cleared after fallback")
	}
}

func gkeExecConfig() clientcmdapi.Config {
	cfg := clientcmdapi.NewConfig()
	cfg.Clusters["cluster-a"] = &clientcmdapi.Cluster{Server: "https://a.example.com"}
	cfg.AuthInfos["user-a"] = &clientcmdapi.AuthInfo{
		Exec: &clientcmdapi.ExecConfig{
			Command:         "gke-gcloud-auth-plugin",
			APIVersion:      clientcmdapiv1.SchemeGroupVersion.String(),
			InteractiveMode: clientcmdapi.IfAvailableExecInteractiveMode,
		},
	}
	cfg.Contexts["ctx-a"] = &clientcmdapi.Context{Cluster: "cluster-a", AuthInfo: "user-a"}
	return *cfg
}

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}

func TestSetCurrentContext(t *testing.T) {
	origKubeconfig := os.Getenv("KUBECONFIG")
	t.Cleanup(func() {
		if origKubeconfig == "" {
			_ = os.Unsetenv("KUBECONFIG")
			return
		}
		_ = os.Setenv("KUBECONFIG", origKubeconfig)
	})

	dir := t.TempDir()
	kubeconfigPath := filepath.Join(dir, "config")

	cfg := MakeFakeConfig()
	if err := clientcmd.WriteToFile(cfg, kubeconfigPath); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	if err := os.Setenv("KUBECONFIG", kubeconfigPath); err != nil {
		t.Fatalf("set KUBECONFIG: %v", err)
	}

	if err := SetCurrentContext("ctx-b"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		t.Fatalf("reload kubeconfig: %v", err)
	}
	if updated.CurrentContext != "ctx-b" {
		t.Fatalf("expected current context ctx-b, got %q", updated.CurrentContext)
	}
}

func TestSetCurrentContext_Unknown(t *testing.T) {
	origKubeconfig := os.Getenv("KUBECONFIG")
	t.Cleanup(func() {
		if origKubeconfig == "" {
			_ = os.Unsetenv("KUBECONFIG")
			return
		}
		_ = os.Setenv("KUBECONFIG", origKubeconfig)
	})

	dir := t.TempDir()
	kubeconfigPath := filepath.Join(dir, "config")

	cfg := MakeFakeConfig()
	if err := clientcmd.WriteToFile(cfg, kubeconfigPath); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	if err := os.Setenv("KUBECONFIG", kubeconfigPath); err != nil {
		t.Fatalf("set KUBECONFIG: %v", err)
	}

	err := SetCurrentContext("missing")
	if err == nil || !strings.Contains(err.Error(), `unknown context "missing"`) {
		t.Fatalf("expected unknown context error, got %v", err)
	}
}

func TestDeleteContext(t *testing.T) {
	origKubeconfig := os.Getenv("KUBECONFIG")
	t.Cleanup(func() {
		if origKubeconfig == "" {
			_ = os.Unsetenv("KUBECONFIG")
			return
		}
		_ = os.Setenv("KUBECONFIG", origKubeconfig)
	})

	dir := t.TempDir()
	kubeconfigPath := filepath.Join(dir, "config")

	cfg := MakeFakeConfig()
	if err := clientcmd.WriteToFile(cfg, kubeconfigPath); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	if err := os.Setenv("KUBECONFIG", kubeconfigPath); err != nil {
		t.Fatalf("set KUBECONFIG: %v", err)
	}

	if err := DeleteContext("ctx-b"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		t.Fatalf("reload kubeconfig: %v", err)
	}
	if _, ok := updated.Contexts["ctx-b"]; ok {
		t.Fatal("expected ctx-b to be deleted from kubeconfig")
	}
	if updated.CurrentContext != "ctx-a" {
		t.Fatalf("expected current context to remain ctx-a, got %q", updated.CurrentContext)
	}
}

func TestDeleteContext_Unknown(t *testing.T) {
	origKubeconfig := os.Getenv("KUBECONFIG")
	t.Cleanup(func() {
		if origKubeconfig == "" {
			_ = os.Unsetenv("KUBECONFIG")
			return
		}
		_ = os.Setenv("KUBECONFIG", origKubeconfig)
	})

	dir := t.TempDir()
	kubeconfigPath := filepath.Join(dir, "config")

	cfg := MakeFakeConfig()
	if err := clientcmd.WriteToFile(cfg, kubeconfigPath); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	if err := os.Setenv("KUBECONFIG", kubeconfigPath); err != nil {
		t.Fatalf("set KUBECONFIG: %v", err)
	}

	err := DeleteContext("missing")
	if err == nil || !strings.Contains(err.Error(), `unknown context "missing"`) {
		t.Fatalf("expected unknown context error, got %v", err)
	}
}
