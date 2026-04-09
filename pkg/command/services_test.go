package command

import (
	"context"
	"errors"
	"github.com/heyihong/krepl/pkg/repl"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestServicesCommand_NoContext(t *testing.T) {
	env := makeExecTestEnvNoContext(t)
	cmd := newServicesCmd()
	if err := cmd.Execute(env, nil); err == nil {
		t.Fatal("expected error when no context is active")
	}
}

func TestServicesCommand_PrintsTableAndSetsSelectableObjects(t *testing.T) {
	env := makeTestEnv()
	oldList := listServicesForContext
	listServicesForContext = func(_ context.Context, _ clientcmdapi.Config, contextName, _ string) ([]corev1.Service, error) {
		if contextName != env.CurrentContext() {
			t.Fatalf("unexpected context %q", contextName)
		}
		return []corev1.Service{
			makeTestService("svc-a", "default", corev1.ServiceTypeClusterIP, "10.0.0.1", nil,
				[]corev1.ServicePort{{Port: 80, Protocol: corev1.ProtocolTCP}}),
			makeTestService("svc-b", "default", corev1.ServiceTypeNodePort, "10.0.0.2", nil,
				[]corev1.ServicePort{{Port: 443, NodePort: 30443, Protocol: corev1.ProtocolTCP}}),
			makeTestService("svc-lb", "default", corev1.ServiceTypeLoadBalancer, "10.0.0.3",
				[]corev1.LoadBalancerIngress{{IP: "1.2.3.4"}},
				[]corev1.ServicePort{{Port: 80, Protocol: corev1.ProtocolTCP}}),
		}, nil
	}
	defer func() { listServicesForContext = oldList }()

	out := captureStdout(t, func() {
		if err := (newServicesCmd()).Execute(env, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{
		"NAME", "TYPE", "CLUSTER-IP", "EXTERNAL-IP", "PORT(S)", "AGE",
		"svc-a", "ClusterIP", "10.0.0.1", "80/TCP",
		"svc-b", "NodePort", "443:30443/TCP",
		"svc-lb", "LoadBalancer", "1.2.3.4",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}

	_ = captureStdout(t, func() {
		if err := env.SelectByIndex(0); err != nil {
			t.Fatalf("select service: %v", err)
		}
	})
	obj := env.CurrentObject()
	if obj == nil || obj.Kind != repl.KindService || obj.Name != "svc-a" {
		t.Fatalf("expected svc-a selected as KindService, got %+v", obj)
	}
}

func TestServicesCommand_PropagatesListErrors(t *testing.T) {
	env := makeTestEnv()
	oldList := listServicesForContext
	listServicesForContext = func(_ context.Context, _ clientcmdapi.Config, _, _ string) ([]corev1.Service, error) {
		return nil, errors.New("boom")
	}
	defer func() { listServicesForContext = oldList }()

	err := (newServicesCmd()).Execute(env, nil)
	if err == nil || !strings.Contains(err.Error(), "list services") {
		t.Fatalf("expected wrapped list error, got %v", err)
	}
}

func TestServiceExternalIP_None(t *testing.T) {
	svc := makeTestService("s", "default", corev1.ServiceTypeClusterIP, "10.0.0.1", nil, nil)
	if got := serviceExternalIP(svc); got != "<none>" {
		t.Fatalf("expected <none>, got %q", got)
	}
}

func TestServiceExternalIP_LoadBalancer(t *testing.T) {
	svc := makeTestService("s", "default", corev1.ServiceTypeLoadBalancer, "10.0.0.1",
		[]corev1.LoadBalancerIngress{{Hostname: "lb.example.com"}, {IP: "5.6.7.8"}}, nil)
	got := serviceExternalIP(svc)
	if got != "lb.example.com,5.6.7.8" {
		t.Fatalf("expected lb.example.com,5.6.7.8, got %q", got)
	}
}

func TestServicePorts_ClusterIP(t *testing.T) {
	svc := makeTestService("s", "default", corev1.ServiceTypeClusterIP, "10.0.0.1", nil,
		[]corev1.ServicePort{{Port: 8080, Protocol: corev1.ProtocolTCP}})
	if got := servicePorts(svc); got != "8080/TCP" {
		t.Fatalf("expected 8080/TCP, got %q", got)
	}
}

func TestServicePorts_NodePort(t *testing.T) {
	svc := makeTestService("s", "default", corev1.ServiceTypeNodePort, "10.0.0.1", nil,
		[]corev1.ServicePort{
			{Port: 80, NodePort: 31080, Protocol: corev1.ProtocolTCP},
			{Port: 443, NodePort: 31443, Protocol: corev1.ProtocolTCP},
		})
	got := servicePorts(svc)
	if got != "80:31080/TCP,443:31443/TCP" {
		t.Fatalf("expected 80:31080/TCP,443:31443/TCP, got %q", got)
	}
}

func makeTestService(name, namespace string, svcType corev1.ServiceType, clusterIP string,
	ingress []corev1.LoadBalancerIngress, ports []corev1.ServicePort) corev1.Service {
	for i := range ports {
		if ports[i].TargetPort.IntVal == 0 && ports[i].TargetPort.StrVal == "" {
			ports[i].TargetPort = intstr.FromInt(int(ports[i].Port))
		}
	}
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			CreationTimestamp: metav1.NewTime(time.Now().Add(-48 * time.Hour)),
		},
		Spec: corev1.ServiceSpec{
			Type:      svcType,
			ClusterIP: clusterIP,
			Ports:     ports,
			Selector:  map[string]string{"app": name},
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{Ingress: ingress},
		},
	}
}
