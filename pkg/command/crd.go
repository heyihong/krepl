package command

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/heyihong/krepl/pkg/config"
	"github.com/heyihong/krepl/pkg/repl"
	"github.com/heyihong/krepl/pkg/table"
)

var discoverDynamicResource = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName, name string) (*repl.DynamicResourceDescriptor, error) {
	restConfig, err := config.BuildRESTConfigForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}

	client, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	groupList, err := client.ServerGroups()
	if err != nil {
		return nil, err
	}

	for _, group := range groupList.Groups {
		groupVersion := group.PreferredVersion.GroupVersion
		if groupVersion == "" && len(group.Versions) > 0 {
			groupVersion = group.Versions[0].GroupVersion
		}
		if groupVersion == "" {
			continue
		}

		resourceList, err := client.ServerResourcesForGroupVersion(groupVersion)
		if err != nil {
			return nil, err
		}

		if desc := findDynamicResourceInAPIResourceList(groupVersion, name, resourceList.APIResources); desc != nil {
			return desc, nil
		}
	}

	return nil, nil
}

var listDynamicResources = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName string, desc repl.DynamicResourceDescriptor, namespace string) ([]unstructured.Unstructured, error) {
	restConfig, err := config.BuildRESTConfigForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}

	client, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	gv, err := schema.ParseGroupVersion(desc.GroupVersion)
	if err != nil {
		return nil, err
	}

	resourceClient := client.Resource(schema.GroupVersionResource{
		Group:    gv.Group,
		Version:  gv.Version,
		Resource: desc.Resource,
	})

	var list *unstructured.UnstructuredList
	if desc.Namespaced {
		targetNamespace := namespace
		if targetNamespace == "" {
			targetNamespace = metav1.NamespaceAll
		}
		list, err = resourceClient.Namespace(targetNamespace).List(ctx, metav1.ListOptions{})
	} else {
		list, err = resourceClient.List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, err
	}

	return append([]unstructured.Unstructured(nil), list.Items...), nil
}

func newCrdCmd() *cmd {
	return &cmd{
		use:   "crd <resource>",
		short: "list instances of the named CRD-backed resource",
		long: "Discover the named custom resource from the active cluster and list its instances.\n" +
			"Namespaced resources honor the working namespace, and listed rows can be selected for follow-up commands such as `describe` and `events`.",
		args: exactArgs(1),
		runE: runCrd,
	}
}

// runCrd lists instances of a CRD-backed resource discovered at runtime.
func runCrd(env *repl.Env, args []string) error {
	if env.CurrentContext() == "" {
		return fmt.Errorf("no active context; use `context <name>` to set one")
	}
	if err := rejectNamespaceSelectionForList(env, "crd"); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	desc, err := discoverDynamicResource(ctx, env.RawConfig(), env.CurrentContext(), args[0])
	if err != nil {
		return fmt.Errorf("discover crd resource: %w", err)
	}
	if desc == nil {
		fmt.Printf("Cluster doesn't have a CRD created resource of type: %s\n", args[0])
		return nil
	}

	items, err := listDynamicResources(ctx, env.RawConfig(), env.CurrentContext(), *desc, env.Namespace())
	if err != nil {
		return fmt.Errorf("list crd resources: %w", err)
	}
	if len(items) == 0 {
		fmt.Printf("No %s found.\n", desc.Resource)
		return nil
	}

	columns := []table.Column{colIndex, colName}
	if desc.Namespaced {
		columns = append(columns, colNamespace)
	}
	columns = append(columns, colAge)

	t := &table.Table{Columns: columns}

	now := time.Now()
	objs := make([]repl.LastObject, 0, len(items))
	for i, item := range items {
		row := []string{fmt.Sprintf("%d", i), item.GetName()}
		if desc.Namespaced {
			row = append(row, fallbackString(item.GetNamespace(), "<none>"))
		}
		row = append(row, formatAge(item.GetCreationTimestamp().Time, now))
		t.AddRow(row...)

		objs = append(objs, repl.LastObject{
			Kind:      repl.KindDynamic,
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
			Dynamic:   desc,
		})
	}
	t.Render()

	env.SetLastObjects(objs)
	return nil
}

func findDynamicResourceInAPIResourceList(groupVersion, name string, resources []metav1.APIResource) *repl.DynamicResourceDescriptor {
	for _, resource := range resources {
		if strings.Contains(resource.Name, "/") {
			continue
		}
		if resource.Name != name && resource.SingularName != name {
			continue
		}

		return &repl.DynamicResourceDescriptor{
			Resource:     resource.Name,
			GroupVersion: groupVersion,
			Kind:         resource.Kind,
			Namespaced:   resource.Namespaced,
		}
	}
	return nil
}
