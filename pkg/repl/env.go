package repl

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/heyihong/krepl/pkg/portforward"
	"github.com/heyihong/krepl/pkg/styles"
)

// LastObjectKind identifies what type of object is stored in a LastObject.
type LastObjectKind int

const (
	KindPod              LastObjectKind = iota
	KindNamespace        LastObjectKind = iota
	KindNode             LastObjectKind = iota
	KindDeployment       LastObjectKind = iota
	KindReplicaSet       LastObjectKind = iota
	KindStatefulSet      LastObjectKind = iota
	KindConfigMap        LastObjectKind = iota
	KindSecret           LastObjectKind = iota
	KindJob              LastObjectKind = iota
	KindCronJob          LastObjectKind = iota
	KindDaemonSet        LastObjectKind = iota
	KindPersistentVolume LastObjectKind = iota
	KindService          LastObjectKind = iota
	KindStorageClass     LastObjectKind = iota
	KindDynamic          LastObjectKind = iota
)

// DynamicResourceDescriptor identifies a discovered CRD-backed resource type.
type DynamicResourceDescriptor struct {
	Resource     string
	GroupVersion string
	Kind         string
	Namespaced   bool
}

// LastObject holds a reference to an item from the most recent list command,
// enabling numeric selection.
type LastObject struct {
	Kind      LastObjectKind
	Name      string
	Namespace string // empty for cluster-scoped objects (e.g. namespaces)
	Dynamic   *DynamicResourceDescriptor
}

// SelectionKind identifies how many objects are currently active.
type SelectionKind int

const (
	SelectionNone SelectionKind = iota
	SelectionSingle
	SelectionRange
)

// Selection captures the current REPL selection state.
type Selection struct {
	Kind  SelectionKind
	Items []LastObject
	Label string
}

// Env is the central mutable REPL state.
type Env struct {
	// rawConfig is the full parsed kubeconfig, used to enumerate contexts.
	rawConfig clientcmdapi.Config

	// currentContext is the name of the active context (empty string = none).
	currentContext string

	// namespace is the working namespace (empty string = all/none).
	namespace string

	// lastObjects holds items from the most recent list command,
	// keyed by their row index for numeric selection.
	lastObjects []LastObject

	// selection is the current active selection, which may be none, single, or range.
	selection Selection

	// prompt is the pre-built prompt string, rebuilt on any state change.
	prompt string

	// rangeSeparator controls the per-object header printed for range operations.
	rangeSeparator string

	// describeIncludeEvents is the default --events setting for describe.
	describeIncludeEvents bool

	// quit signals the REPL loop to exit.
	quit bool

	// suspendTerminal temporarily releases any interactive terminal state owned by
	// the REPL so child interactive operations can use the TTY directly.
	suspendTerminal func() error

	// resumeTerminal reacquires any interactive terminal state released by
	// suspendTerminal after a child interactive operation completes.
	resumeTerminal func() error

	// resetTerminal requests that the REPL rebuild its terminal reader before
	// the next prompt, used after interactive child sessions that can leave
	// readline's internal state stale.
	resetTerminal bool

	// portForwards stores managed port-forward sessions created by the REPL.
	portForwards []*portforward.Session
}

// NewEnv creates a new Env from a loaded kubeconfig, seeding currentContext
// from the kubeconfig's CurrentContext field.
func NewEnv(rawConfig clientcmdapi.Config) *Env {
	e := &Env{
		rawConfig:             rawConfig,
		rangeSeparator:        "--- {name} ---",
		describeIncludeEvents: true,
	}
	if rawConfig.CurrentContext != "" {
		e.currentContext = rawConfig.CurrentContext
	}
	e.rebuildPrompt()
	return e
}

// rebuildPrompt reconstructs the prompt string from current state.
func (e *Env) rebuildPrompt() {
	ctx := e.currentContext
	if ctx == "" {
		ctx = "none"
	}
	ns := e.namespace
	if ns == "" {
		ns = "none"
	}
	selected := "none"
	switch e.selection.Kind {
	case SelectionSingle:
		selected = e.selection.Items[0].Name
	case SelectionRange:
		selected = e.selection.Label
	}
	e.prompt = fmt.Sprintf("[%s][%s][%s] > ",
		styles.PromptContext(ctx),
		styles.PromptNamespace(ns),
		styles.PromptSelection(selected),
	)
}

// Prompt returns the current prompt string.
func (e *Env) Prompt() string { return e.prompt }

// RangeSeparator returns the configured separator format for range operations.
func (e *Env) RangeSeparator() string { return e.rangeSeparator }

// SetRangeSeparator updates the separator format used for range operations.
func (e *Env) SetRangeSeparator(format string) {
	e.rangeSeparator = format
}

// DescribeIncludeEvents reports the default --events behavior for describe.
func (e *Env) DescribeIncludeEvents() bool { return e.describeIncludeEvents }

// SetDescribeIncludeEvents updates the default --events behavior for describe.
func (e *Env) SetDescribeIncludeEvents(enabled bool) {
	e.describeIncludeEvents = enabled
}

// IsQuit reports whether the REPL should exit.
func (e *Env) IsQuit() bool { return e.quit }

// Quit signals the REPL loop to exit.
func (e *Env) Quit() { e.quit = true }

// SetTerminalHooks configures optional callbacks for temporarily releasing and
// restoring the REPL's terminal ownership around interactive child operations.
func (e *Env) SetTerminalHooks(suspend func() error, resume func() error) {
	e.suspendTerminal = suspend
	e.resumeTerminal = resume
}

// SuspendTerminal releases REPL terminal ownership if a suspend hook is configured.
func (e *Env) SuspendTerminal() error {
	if e.suspendTerminal == nil {
		return nil
	}
	return e.suspendTerminal()
}

// ResumeTerminal restores REPL terminal ownership if a resume hook is configured.
func (e *Env) ResumeTerminal() error {
	if e.resumeTerminal == nil {
		return nil
	}
	return e.resumeTerminal()
}

// RequestTerminalReset marks the REPL terminal for recreation before the next prompt.
func (e *Env) RequestTerminalReset() {
	e.resetTerminal = true
}

// ConsumeTerminalReset reports whether a terminal reset was requested and clears the flag.
func (e *Env) ConsumeTerminalReset() bool {
	reset := e.resetTerminal
	e.resetTerminal = false
	return reset
}

// RawConfig returns the loaded kubeconfig state.
func (e *Env) RawConfig() clientcmdapi.Config { return e.rawConfig }

// CurrentContext returns the active context name, or an empty string if unset.
func (e *Env) CurrentContext() string { return e.currentContext }

// Namespace returns the active namespace, or an empty string if unset.
func (e *Env) Namespace() string { return e.namespace }

// SetContext switches to a named context. Returns an error if the name is
// not found in the kubeconfig.
func (e *Env) SetContext(name string) error {
	if _, ok := e.rawConfig.Contexts[name]; !ok {
		return fmt.Errorf("unknown context %q", name)
	}
	e.currentContext = name
	e.lastObjects = nil
	e.ClearSelection()
	return nil
}

// SetNamespace sets the working namespace. An empty string clears it.
func (e *Env) SetNamespace(ns string) {
	e.namespace = ns
	e.lastObjects = nil
	e.ClearSelection()
}

// SetLastObjects replaces the last-listed objects (called by pods/namespaces commands).
func (e *Env) SetLastObjects(objs []LastObject) {
	e.lastObjects = objs
}

// Selection returns the current selection state.
func (e *Env) Selection() Selection {
	return e.selection
}

// HasRangeSelection reports whether the current selection contains multiple items.
func (e *Env) HasRangeSelection() bool {
	return e.selection.Kind == SelectionRange
}

// CurrentSelection returns the selected objects, if any.
func (e *Env) CurrentSelection() []LastObject {
	return append([]LastObject(nil), e.selection.Items...)
}

// ClearSelection removes the current selection and updates the prompt.
func (e *Env) ClearSelection() {
	e.selection = Selection{Kind: SelectionNone}
	e.rebuildPrompt()
}

// SetCurrent stores a single selected object and updates the prompt.
func (e *Env) SetCurrent(obj LastObject) {
	e.selection = Selection{
		Kind:  SelectionSingle,
		Items: []LastObject{obj},
	}
	e.rebuildPrompt()
}

// SetRange stores a multi-object selection and updates the prompt.
func (e *Env) SetRange(objs []LastObject) {
	if len(objs) == 0 {
		e.ClearSelection()
		return
	}
	label := fmt.Sprintf("%d %s selected", len(objs), selectionTypeName(objs[0], len(objs) > 1))
	e.selection = Selection{
		Kind:  SelectionRange,
		Items: append([]LastObject(nil), objs...),
		Label: label,
	}
	e.rebuildPrompt()
}

// ItemAt returns the object from the last list at the specified index.
func (e *Env) ItemAt(i int) *LastObject {
	if i < 0 || i >= len(e.lastObjects) {
		return nil
	}
	obj := e.lastObjects[i]
	return &obj
}

// SetRangeByIndices selects objects from the last list by index.
func (e *Env) SetRangeByIndices(indices []int) {
	objs := make([]LastObject, 0, len(indices))
	for _, idx := range indices {
		obj := e.ItemAt(idx)
		if obj == nil {
			break
		}
		objs = append(objs, *obj)
	}
	e.SetRange(objs)
}

// SelectByIndex selects the object at row index i from the last list.
// Pods and nodes become the active selected object; namespaces set the active namespace.
func (e *Env) SelectByIndex(i int) error {
	if i < 0 || i >= len(e.lastObjects) {
		return fmt.Errorf("index %d out of range (last list has %d items)", i, len(e.lastObjects))
	}
	obj := e.lastObjects[i]
	switch obj.Kind {
	case KindPod:
		e.SetCurrent(obj)
		fmt.Printf("Selected pod: %s\n", obj.Name)
	case KindNode:
		e.SetCurrent(obj)
		fmt.Printf("Selected node: %s\n", obj.Name)
	case KindDeployment:
		e.SetCurrent(obj)
		fmt.Printf("Selected deployment: %s\n", obj.Name)
	case KindReplicaSet:
		e.SetCurrent(obj)
		fmt.Printf("Selected replicaset: %s\n", obj.Name)
	case KindStatefulSet:
		e.SetCurrent(obj)
		fmt.Printf("Selected statefulset: %s\n", obj.Name)
	case KindConfigMap:
		e.SetCurrent(obj)
		fmt.Printf("Selected configmap: %s\n", obj.Name)
	case KindSecret:
		e.SetCurrent(obj)
		fmt.Printf("Selected secret: %s\n", obj.Name)
	case KindJob:
		e.SetCurrent(obj)
		fmt.Printf("Selected job: %s\n", obj.Name)
	case KindCronJob:
		e.SetCurrent(obj)
		fmt.Printf("Selected cronjob: %s\n", obj.Name)
	case KindDaemonSet:
		e.SetCurrent(obj)
		fmt.Printf("Selected daemonset: %s\n", obj.Name)
	case KindPersistentVolume:
		e.SetCurrent(obj)
		fmt.Printf("Selected persistentvolume: %s\n", obj.Name)
	case KindService:
		e.SetCurrent(obj)
		fmt.Printf("Selected service: %s\n", obj.Name)
	case KindStorageClass:
		e.SetCurrent(obj)
		fmt.Printf("Selected storageclass: %s\n", obj.Name)
	case KindNamespace:
		e.SetCurrent(obj)
		fmt.Printf("Selected namespace: %s\n", obj.Name)
	case KindDynamic:
		e.SetCurrent(obj)
		resource := "resource"
		if obj.Dynamic != nil && obj.Dynamic.Resource != "" {
			resource = obj.Dynamic.Resource
		}
		fmt.Printf("Selected %s: %s\n", resource, obj.Name)
	}
	return nil
}

// CurrentObject returns the currently active selected object, or nil if none is selected.
func (e *Env) CurrentObject() *LastObject {
	if e.selection.Kind != SelectionSingle || len(e.selection.Items) == 0 {
		return nil
	}
	obj := e.selection.Items[0]
	return &obj
}

// CurrentObjectRequired returns the single selected object or a descriptive error.
func (e *Env) CurrentObjectRequired(noun string) (*LastObject, error) {
	switch e.selection.Kind {
	case SelectionSingle:
		obj := e.selection.Items[0]
		return &obj, nil
	case SelectionRange:
		return nil, fmt.Errorf("range selection is not supported for %s; select a single object", noun)
	default:
		return nil, fmt.Errorf("no active object")
	}
}

// ApplyToSelection runs fn for the active selection. For ranges, it prints a separator
// before each item and handles Click-style continuation prompts on errors.
func (e *Env) ApplyToSelection(fn func(LastObject) error) error {
	switch e.selection.Kind {
	case SelectionNone:
		return fmt.Errorf("no objects currently active")
	case SelectionSingle:
		return fn(e.selection.Items[0])
	case SelectionRange:
		continueAll := false
		for _, obj := range e.selection.Items {
			fmt.Println(e.formatRangeSeparator(obj))
			if err := fn(obj); err != nil {
				fmt.Printf("Error applying operation to %s: %v\n", obj.Name, err)
				if !continueAll {
					fmt.Print("Continue? [o/a/N]? ")
					answer, readErr := bufio.NewReader(os.Stdin).ReadString('\n')
					if readErr != nil && readErr != io.EOF {
						return fmt.Errorf("aborting range action: read response: %w", readErr)
					}
					switch strings.ToLower(strings.TrimSpace(answer)) {
					case "o", "once":
					case "a", "all":
						continueAll = true
					default:
						return fmt.Errorf("aborting range action")
					}
				}
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown selection state")
	}
}

// AddPortForward appends a managed port-forward session.
func (e *Env) AddPortForward(session *portforward.Session) {
	e.portForwards = append(e.portForwards, session)
}

// PortForwards returns all managed port-forward sessions.
func (e *Env) PortForwards() []*portforward.Session {
	return e.portForwards
}

// PortForward returns the session at index i, or nil if out of range.
func (e *Env) PortForward(i int) *portforward.Session {
	if i < 0 || i >= len(e.portForwards) {
		return nil
	}
	return e.portForwards[i]
}

// StopPortForward stops the managed session at index i.
func (e *Env) StopPortForward(i int) error {
	session := e.PortForward(i)
	if session == nil {
		return fmt.Errorf("index %d out of range", i)
	}
	session.Stop()
	return nil
}

// StopAllPortForwards stops all managed port-forward sessions.
func (e *Env) StopAllPortForwards() {
	for _, session := range e.portForwards {
		session.Stop()
	}
}

// ListContextNames returns a sorted list of all context names from the kubeconfig.
func (e *Env) ListContextNames() []string {
	names := make([]string, 0, len(e.rawConfig.Contexts))
	for name := range e.rawConfig.Contexts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func selectionTypeName(obj LastObject, plural bool) string {
	name := objectTypeName(obj)
	if plural {
		return name + "s"
	}
	return name
}

func objectTypeName(obj LastObject) string {
	switch obj.Kind {
	case KindPod:
		return "Pod"
	case KindNamespace:
		return "Namespace"
	case KindNode:
		return "Node"
	case KindDeployment:
		return "Deployment"
	case KindReplicaSet:
		return "ReplicaSet"
	case KindStatefulSet:
		return "StatefulSet"
	case KindConfigMap:
		return "ConfigMap"
	case KindSecret:
		return "Secret"
	case KindJob:
		return "Job"
	case KindCronJob:
		return "CronJob"
	case KindDaemonSet:
		return "DaemonSet"
	case KindPersistentVolume:
		return "PersistentVolume"
	case KindService:
		return "Service"
	case KindStorageClass:
		return "StorageClass"
	case KindDynamic:
		if obj.Dynamic != nil && obj.Dynamic.Kind != "" {
			return obj.Dynamic.Kind
		}
		return "Resource"
	default:
		return "Object"
	}
}

func (e *Env) formatRangeSeparator(obj LastObject) string {
	namespace := obj.Namespace
	if namespace == "" {
		namespace = "[none]"
	}
	separator := e.rangeSeparator
	if separator == "" {
		separator = "--- {name} ---"
	}
	separator = strings.ReplaceAll(separator, "{name}", obj.Name)
	separator = strings.ReplaceAll(separator, "{namespace}", namespace)
	return styles.RangeSeparator(separator)
}
