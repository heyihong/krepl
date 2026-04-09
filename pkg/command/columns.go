package command

import (
	"github.com/heyihong/krepl/pkg/styles"
	"github.com/heyihong/krepl/pkg/table"
)

// Centralized column definitions shared across resource list commands.
// Every distinct table column used in the command package is declared here
// so headers, MinWidth hints, and color functions are defined exactly once.
var (
	// Universal columns present in most resource lists.
	colIndex     = table.Column{Header: "#", MinWidth: 4, RightAlign: true}
	colName      = table.Column{Header: "NAME"}
	colNamespace = table.Column{Header: "NAMESPACE"}
	colAge       = table.Column{Header: "AGE"}

	// Status columns with resource-specific color functions.
	colStatus          = table.Column{Header: "STATUS"} // generic (portforwards, PVs)
	colNodeStatus      = table.Column{Header: "STATUS", Color: styles.NodeStatus}
	colPodStatus       = table.Column{Header: "STATUS", Color: styles.PodPhase}
	colNamespaceStatus = table.Column{Header: "STATUS", Color: styles.NamespaceStatus}

	// Readiness / replica count columns.
	colReady     = table.Column{Header: "READY"}
	colDesired   = table.Column{Header: "DESIRED"}
	colCurrent   = table.Column{Header: "CURRENT"}
	colUpToDate  = table.Column{Header: "UP-TO-DATE"}
	colAvailable = table.Column{Header: "AVAILABLE"}

	// Node columns.
	colRoles   = table.Column{Header: "ROLES"}
	colVersion = table.Column{Header: "VERSION"}

	// Pod / workload columns.
	colRestarts       = table.Column{Header: "RESTARTS"}
	colLastRestart    = table.Column{Header: "LAST RESTART"}
	colNominatedNode  = table.Column{Header: "NOMINATED NODE"}
	colReadinessGates = table.Column{Header: "READINESS GATES"}
	colLabels         = table.Column{Header: "LABELS"}
	colNode           = table.Column{Header: "NODE"}
	colIP             = table.Column{Header: "IP"}

	// Job / CronJob columns.
	colCompletions  = table.Column{Header: "COMPLETIONS"}
	colDuration     = table.Column{Header: "DURATION"}
	colSchedule     = table.Column{Header: "SCHEDULE"}
	colSuspend      = table.Column{Header: "SUSPEND"}
	colActive       = table.Column{Header: "ACTIVE"}
	colLastSchedule = table.Column{Header: "LAST SCHEDULE"}

	// Service columns.
	colType       = table.Column{Header: "TYPE"}
	colClusterIP  = table.Column{Header: "CLUSTER-IP"}
	colExternalIP = table.Column{Header: "EXTERNAL-IP"}
	colPortS      = table.Column{Header: "PORT(S)"}

	// Port-forward columns.
	colPod   = table.Column{Header: "POD"}
	colPorts = table.Column{Header: "PORTS"}

	// ConfigMap / Secret columns.
	colData = table.Column{Header: "DATA"}

	// StorageClass columns.
	colProvisioner = table.Column{Header: "PROVISIONER"}

	// PersistentVolume columns.
	colCapacity      = table.Column{Header: "CAPACITY"}
	colAccessModes   = table.Column{Header: "ACCESS MODES"}
	colReclaimPolicy = table.Column{Header: "RECLAIM POLICY"}
	colClaim         = table.Column{Header: "CLAIM"}
	colStorageClass  = table.Column{Header: "STORAGECLASS"}

	// Event columns.
	colLastSeen = table.Column{Header: "LAST SEEN"}
	colReason   = table.Column{Header: "REASON"}
	colMessage  = table.Column{Header: "MESSAGE"}
	colObject   = table.Column{Header: "OBJECT"}
)
