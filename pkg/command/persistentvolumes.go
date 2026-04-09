package command

import (
	"context"
	"fmt"
	"github.com/heyihong/krepl/pkg/config"
	"github.com/heyihong/krepl/pkg/repl"
	"github.com/heyihong/krepl/pkg/table"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var listPersistentVolumesForContext = func(ctx context.Context, rawConfig clientcmdapi.Config, contextName string) ([]corev1.PersistentVolume, error) {
	client, err := config.BuildClientForContext(rawConfig, contextName)
	if err != nil {
		return nil, err
	}
	list, err := client.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return append([]corev1.PersistentVolume(nil), list.Items...), nil
}

func newPersistentVolumesCmd() *cmd {
	return &cmd{
		use:     "persistentvolumes",
		aliases: []string{"pvs"},
		short:   "list persistent volumes in the current context",
		long: "List persistent volumes in the active context.\n" +
			"Persistent volumes are cluster-scoped, and rows can be selected for follow-up commands such as `describe`, `events`, and `delete`.",
		args: noArgs,
		runE: runPersistentVolumes,
	}
}

// runPersistentVolumes lists persistent volumes in the current context.
// PVs are cluster-scoped so no namespace filter is applied.
func runPersistentVolumes(env *repl.Env, _ []string) error {
	if env.CurrentContext() == "" {
		return fmt.Errorf("no active context; use `context <name>` to set one")
	}
	if err := rejectNamespaceSelectionForList(env, "persistentvolumes"); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pvs, err := listPersistentVolumesForContext(ctx, env.RawConfig(), env.CurrentContext())
	if err != nil {
		return fmt.Errorf("list persistentvolumes: %w", err)
	}

	if len(pvs) == 0 {
		fmt.Println("No persistent volumes found.")
		return nil
	}

	t := &table.Table{Columns: []table.Column{
		colIndex, colName, colCapacity, colAccessModes, colReclaimPolicy, colStatus, colClaim, colStorageClass, colAge,
	}}

	objs := make([]repl.LastObject, 0, len(pvs))
	now := time.Now()
	for i, pv := range pvs {
		capacity := "<none>"
		if q, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
			capacity = q.String()
		}
		t.AddRow(
			fmt.Sprintf("%d", i),
			pv.Name,
			capacity,
			formatAccessModes(pv.Spec.AccessModes),
			fallbackString(string(pv.Spec.PersistentVolumeReclaimPolicy), "<none>"),
			fallbackString(string(pv.Status.Phase), "<none>"),
			pvClaimRef(pv),
			fallbackString(pv.Spec.StorageClassName, "<none>"),
			formatAge(pv.CreationTimestamp.Time, now),
		)
		// PVs are cluster-scoped: Namespace is intentionally empty.
		objs = append(objs, repl.LastObject{Kind: repl.KindPersistentVolume, Name: pv.Name})
	}
	t.Render()

	env.SetLastObjects(objs)
	return nil
}

// pvClaimRef returns "namespace/name" for the bound claim, or "" if unbound.
func pvClaimRef(pv corev1.PersistentVolume) string {
	ref := pv.Spec.ClaimRef
	if ref == nil {
		return ""
	}
	if ref.Namespace != "" && ref.Name != "" {
		return ref.Namespace + "/" + ref.Name
	}
	return fallbackString(ref.Name, "")
}

// formatAccessModes converts a deduplicated, sorted list of access modes into
// their abbreviations (RWO, ROX, RWX, RWOP), matching click's display format.
func formatAccessModes(modes []corev1.PersistentVolumeAccessMode) string {
	seen := make(map[string]struct{})
	for _, m := range modes {
		seen[accessModeAbbrev(m)] = struct{}{}
	}
	abbrevs := make([]string, 0, len(seen))
	for a := range seen {
		abbrevs = append(abbrevs, a)
	}
	sort.Strings(abbrevs)
	return strings.Join(abbrevs, ",")
}

func accessModeAbbrev(mode corev1.PersistentVolumeAccessMode) string {
	switch mode {
	case corev1.ReadWriteOnce:
		return "RWO"
	case corev1.ReadOnlyMany:
		return "ROX"
	case corev1.ReadWriteMany:
		return "RWX"
	case corev1.ReadWriteOncePod:
		return "RWOP"
	default:
		return string(mode)
	}
}
