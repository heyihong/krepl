package command

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/yaml"

	"github.com/heyihong/krepl/pkg/config"
	"github.com/heyihong/krepl/pkg/repl"
)

var fetchEditObject = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName string, obj repl.LastObject) (*unstructured.Unstructured, error) {
	gvr, namespaced, err := editGVR(obj)
	if err != nil {
		return nil, err
	}

	restConfig, err := config.BuildRESTConfigForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}

	client, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	resourceClient := client.Resource(gvr)
	if namespaced && obj.Namespace != "" {
		return resourceClient.Namespace(obj.Namespace).Get(ctx, obj.Name, metav1.GetOptions{})
	}
	return resourceClient.Get(ctx, obj.Name, metav1.GetOptions{})
}

var applyEditObject = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName string, obj repl.LastObject, updated *unstructured.Unstructured) error {
	gvr, namespaced, err := editGVR(obj)
	if err != nil {
		return err
	}

	restConfig, err := config.BuildRESTConfigForContext(rawConfig, contextName)
	if err != nil {
		return err
	}

	client, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	resourceClient := client.Resource(gvr)
	if namespaced && obj.Namespace != "" {
		_, err = resourceClient.Namespace(obj.Namespace).Update(ctx, updated, metav1.UpdateOptions{})
	} else {
		_, err = resourceClient.Update(ctx, updated, metav1.UpdateOptions{})
	}
	return err
}

var launchEditorProcess = func(editor, path string) error {
	cmd, err := buildEditorCommand(editor, path)
	if err != nil {
		return err
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func buildEditorCommand(editor, path string) (*exec.Cmd, error) {
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return nil, fmt.Errorf("editor command is empty")
	}
	if len(parts) > 1 {
		return nil, fmt.Errorf("editor arguments are not supported; set EDITOR to a single executable name")
	}

	switch filepath.Base(parts[0]) {
	case "code":
		return exec.Command("code", "--wait", path), nil
	case "emacs":
		return exec.Command("emacs", path), nil
	case "hx":
		return exec.Command("hx", path), nil
	case "nano":
		return exec.Command("nano", path), nil
	case "nvim":
		return exec.Command("nvim", path), nil
	case "vi":
		return exec.Command("vi", path), nil
	case "vim":
		return exec.Command("vim", path), nil
	case "zed":
		return exec.Command("zed", path), nil
	default:
		return nil, fmt.Errorf("unsupported editor %q; use one of code, emacs, hx, nano, nvim, vi, vim, or zed", parts[0])
	}
}

func readModifiedTempFile(tmpFile *os.File) ([]byte, error) {
	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("rewind temp file: %w", err)
	}
	data, err := io.ReadAll(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("read temp file: %w", err)
	}
	return data, nil
}

// editGVR maps a supported LastObject kind to the GroupVersionResource and
// namespaced flag needed by the dynamic client.
//
// Pod, Node, and PersistentVolume are excluded: their specs are largely
// immutable after creation, making in-place edits impractical.
func editGVR(obj repl.LastObject) (schema.GroupVersionResource, bool, error) {
	switch obj.Kind {
	case repl.KindDeployment:
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, true, nil
	case repl.KindReplicaSet:
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}, true, nil
	case repl.KindStatefulSet:
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}, true, nil
	case repl.KindDaemonSet:
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}, true, nil
	case repl.KindConfigMap:
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}, true, nil
	case repl.KindSecret:
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}, true, nil
	case repl.KindCronJob:
		return schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"}, true, nil
	case repl.KindService:
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}, true, nil
	case repl.KindDynamic:
		if obj.Dynamic == nil {
			return schema.GroupVersionResource{}, false, fmt.Errorf("missing dynamic resource descriptor")
		}
		gv, err := schema.ParseGroupVersion(obj.Dynamic.GroupVersion)
		if err != nil {
			return schema.GroupVersionResource{}, false, fmt.Errorf("parse group version: %w", err)
		}
		return schema.GroupVersionResource{Group: gv.Group, Version: gv.Version, Resource: obj.Dynamic.Resource}, obj.Dynamic.Namespaced, nil
	case repl.KindPod:
		return schema.GroupVersionResource{}, false, fmt.Errorf("edit does not support Pod: most spec fields are immutable after creation; use `deployments` or `statefulsets` to edit the workload instead")
	case repl.KindNode:
		return schema.GroupVersionResource{}, false, fmt.Errorf("edit does not support Node: label changes only take effect on node re-registration")
	case repl.KindPersistentVolume:
		return schema.GroupVersionResource{}, false, fmt.Errorf("edit does not support PersistentVolume: spec is largely immutable after creation")
	default:
		return schema.GroupVersionResource{}, false, fmt.Errorf("edit does not support selected object kind %q", describeObjectKindName(obj))
	}
}

// resolveEditor returns the editor to use: --editor flag, then $EDITOR, then "vi".
func resolveEditor(flag string) string {
	if flag != "" {
		return flag
	}
	if ed := os.Getenv("EDITOR"); ed != "" {
		return ed
	}
	return "vi"
}

func newEditCmd() *cmd {
	var editorFlag string

	cmd := &cmd{
		use:   "edit [--editor EDITOR]",
		short: "edit the active object in an editor and apply changes",
		long: "Edit the active object in an editor and apply changes. Supports deployments, " +
			"replicasets, statefulsets, daemonsets, configmaps, secrets, cronjobs, services, " +
			"and CRD-backed dynamic resources.",
		args: noArgs,
	}
	cmd.flags().StringVarP(&editorFlag, "editor", "e", "", "editor to use (default: $EDITOR or vi)")

	cmd.runE = func(env *repl.Env, _ []string) error {
		obj := env.CurrentObject()
		if obj == nil {
			return fmt.Errorf("no active object; select one by number after running `deployments`, `replicasets`, `statefulsets`, `daemonsets`, `configmaps`, `secrets`, `cronjobs`, `services`, or `crd <resource>`")
		}
		if env.CurrentContext() == "" {
			return fmt.Errorf("no active context")
		}
		editor := resolveEditor(editorFlag)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		fetched, err := fetchEditObject(ctx, env.RawConfig(), env.CurrentContext(), *obj)
		if err != nil {
			return fmt.Errorf("get object: %w", err)
		}

		// Strip managedFields to reduce noise, same as kubectl edit.
		unstructured.RemoveNestedField(fetched.Object, "metadata", "managedFields")

		originalYAML, err := yaml.Marshal(fetched.Object)
		if err != nil {
			return fmt.Errorf("marshal to yaml: %w", err)
		}

		tmpFile, err := os.CreateTemp("", "krepl-edit-*.yaml")
		if err != nil {
			return fmt.Errorf("create temp file: %w", err)
		}
		tmpPath := tmpFile.Name()
		defer func() {
			_ = os.Remove(tmpPath)
		}()

		if _, err := tmpFile.Write(originalYAML); err != nil {
			_ = tmpFile.Close()
			return fmt.Errorf("write temp file: %w", err)
		}
		if err := tmpFile.Sync(); err != nil {
			_ = tmpFile.Close()
			return fmt.Errorf("sync temp file: %w", err)
		}

		if err := env.SuspendTerminal(); err != nil {
			_ = tmpFile.Close()
			return fmt.Errorf("suspend repl terminal: %w", err)
		}
		editorErr := launchEditorProcess(editor, tmpPath)
		if resumeErr := env.ResumeTerminal(); resumeErr != nil {
			_ = tmpFile.Close()
			return fmt.Errorf("resume repl terminal: %w", resumeErr)
		}
		env.RequestTerminalReset()
		defer func() {
			_ = tmpFile.Close()
		}()

		if editorErr != nil {
			return fmt.Errorf("editor exited with error: %w", editorErr)
		}

		modifiedYAML, err := readModifiedTempFile(tmpFile)
		if err != nil {
			return err
		}

		if bytes.Equal(originalYAML, modifiedYAML) {
			fmt.Println("Edit cancelled, no changes made.")
			return nil
		}

		var modifiedMap map[string]interface{}
		if err := yaml.Unmarshal(modifiedYAML, &modifiedMap); err != nil {
			return fmt.Errorf("parse modified yaml: %w", err)
		}

		updated := &unstructured.Unstructured{Object: modifiedMap}

		applyCtx, applyCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer applyCancel()

		if err := applyEditObject(applyCtx, env.RawConfig(), env.CurrentContext(), *obj, updated); err != nil {
			return fmt.Errorf("apply changes: %w", err)
		}

		fmt.Printf("Updated %s/%s\n", strings.ToLower(describeObjectKindName(*obj)), obj.Name)
		return nil
	}
	return cmd
}
