package command

import (
	"fmt"
	"github.com/heyihong/krepl/pkg/repl"
)

func rejectNamespaceSelectionForList(env *repl.Env, commandName string) error {
	for _, obj := range env.CurrentSelection() {
		if obj.Kind == repl.KindNamespace {
			return fmt.Errorf("%s does not support namespace object selections; use `namespace <name>` to set the working namespace or `clear` to remove the selection", commandName)
		}
	}
	return nil
}

func requireSelectionKind(env *repl.Env, kind repl.LastObjectKind, commandName string) error {
	for _, obj := range env.CurrentSelection() {
		if obj.Kind != kind {
			return fmt.Errorf("%s requires a %s selection, got %s", commandName, objectKindLabel(kind), describeObjectKindName(obj))
		}
	}
	return nil
}

func objectKindLabel(kind repl.LastObjectKind) string {
	switch kind {
	case repl.KindPod:
		return "pod"
	case repl.KindNamespace:
		return "namespace"
	case repl.KindNode:
		return "node"
	case repl.KindDeployment:
		return "deployment"
	case repl.KindStatefulSet:
		return "statefulset"
	case repl.KindDaemonSet:
		return "daemonset"
	case repl.KindReplicaSet:
		return "replicaset"
	case repl.KindJob:
		return "job"
	case repl.KindCronJob:
		return "cronjob"
	case repl.KindService:
		return "service"
	case repl.KindConfigMap:
		return "configmap"
	case repl.KindSecret:
		return "secret"
	case repl.KindPersistentVolume:
		return "persistentvolume"
	case repl.KindStorageClass:
		return "storageclass"
	case repl.KindDynamic:
		return "resource"
	default:
		return "object"
	}
}
