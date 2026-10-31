package vm

import (
	"context"
	"errors"
	"fmt"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"slices"
	"time"
)

// ErrNotFound is returned by Resolve when the (cluster, vmid) pair is absent
// from the Index OR when the VM exists but lacks the mandatory pvmss tag
// (a VM outside PVMSS scope is indistinguishable from a nonexistent one - the response never
// confirms a VM's existence to a caller who shouldn't see it).
var ErrNotFound = errors.New("vm not found")

// ErrForbidden is returned by Resolve when the VM is tagged pvmss but the
// caller's pool does not own it and the caller is not an admin. This
// is the only branch that distinguishes "wrong owner" from "doesn't exist" -
// and only to tell a legitimate owner-in-training their action was understood
// but denied, not to leak existence to a probing caller.
var ErrForbidden = errors.New("forbidden")

// pvmssTag is the mandatory tag that scopes a VM into PVMSS. A VM
// without it is invisible to every PVMSS endpoint, read or write.
const pvmssTag = "pvmss"

// FormatVMRef formats a VM reference for human-readable audit and UI output.
// The result is presentation-only and must never be parsed back into route parts.
func FormatVMRef(clusterName string, vmid int) string {
	return fmt.Sprintf("%s:%d", clusterName, vmid)
}

// adminActionRecorder is the subset of the store needed to record resolve
// failures. It avoids a direct store import by a name that would be circular.
type adminActionRecorder interface {
	RecordAdminAction(ctx context.Context, actor, action, targetType, targetID, detail, ip string) error
}

func recordResolveFailed(auditor adminActionRecorder, actor auth.Identity, clusterName string, vmid int, reason string) {
	if auditor == nil {
		return
	}

	targetID := FormatVMRef(clusterName, vmid)
	detail := fmt.Sprintf(`{"summary":"%s","changes":[]}`, reason)
	_ = auditor.RecordAdminAction(context.Background(), actor.Username, "vm.resolve_failed", "vm", targetID, detail, "")
}

// Entity is a single VM resolved through the ownership gate - the only value
// a write handler may use to identify its target. The Node field is
// authoritative: it comes from the Index, never from request input (root cause). Carries the
// detail-view metrics so GET /vms/:id can
// return it directly.
type Entity struct {
	Cluster string
	VMID    int
	Name    string
	Node    string
	Pool    string
	Status  cluster.VMStatus
	Tags    []string
	// OSType mirrors cluster.VM.OSType - Proxmox's kernel family, not a distro.
	OSType            string
	CPUCores          int
	Sockets           int
	Cores             int
	MemoryTotal       int64
	DiskTotal         int64
	Uptime            time.Duration
	Description       string
	BootOrder         []string
	Disks             []cluster.Disk
	CDROM             cluster.CDROMState
	NetworkInterfaces []cluster.NetworkInterface
	// HasSerial is true when the VM carries a serial port (serial0), so the
	// PVMSS Text/serial console is reachable. Mirrors cluster.VM.HasSerial.
	HasSerial bool
	// Agent mirrors cluster.VM.Agent - the config's guest-agent flag - so
	// the detail endpoint can explain absent live IPs without probing a
	// channel that is configured off.
	Agent bool
}

// Resolve is the ONLY function capable of turning a (cluster, vmid) pair into
// a writable VM entity anywhere in the codebase. Every write
// handler - action, delete, patch - calls this first and uses nothing else to
// identify the target VM or its node., no future handler can reach
// Proxmox for a write without going through it: the class of bug
// represents becomes structurally unreachable, not merely patched at one call
// site.
//
// In order:
// 1. Look up (cluster, vmid) in index.ByVMID. Not found → ErrNotFound.
// 2. Check the pvmss tag. Absent → ErrNotFound (same error as step 1).
// 3. If actor.IsAdmin, skip to step 5.
// 4. Check entity.Pool == actor.Pool. Mismatch → ErrForbidden.
// 5. Return Entity - the node exactly as recorded in the Index.
//
// Resolve is pure: it reads the Index snapshot it is handed and never calls
// the cluster client. Callers do, after Resolve returns (reads and writes are separated). It is re-evaluated on every single write
// request - no request-scoped or session-scoped caching of a prior
// authorization result.
//
// Source widened from a concrete *inventory.Index to the one-method
// inventory.LookupSource interface so Resolve can dispatch through whichever
// cluster's projection clusterName names (single decode point preserved -
// rather than pushing per-cluster lookup out to every
// caller). *inventory.Index still implements LookupSource, so this is a
// backward-compatible widening: every existing call site keeps compiling and
// behaving identically. actor/clusterName/vmid's order, types, and the
// (Entity, error) return are unchanged.
func Resolve(source inventory.LookupSource, actor auth.Identity, clusterName string, vmid int, auditors ...adminActionRecorder) (Entity, error) {
	// Resolve failures are audited when the caller passes a recorder - the
	// same opt-in variadic List uses for its policy read.
	var auditor adminActionRecorder
	if len(auditors) > 0 {
		auditor = auditors[0]
	}

	// A typed-nil store sails past the interface nil check and panics on use.
	if st, ok := auditor.(*store.Store); ok && st == nil {
		auditor = nil
	}

	// The caller already picked the index for clusterName, so a pinned actor
	// must name its own cluster exactly: an empty name would resolve to its
	// cluster here but may have selected another cluster's index upstream.
	if resolved, ok := TargetCluster(actor, clusterName); !ok || resolved != clusterName {
		recordResolveFailed(auditor, actor, clusterName, vmid, "cross-cluster access denied")
		return Entity{}, ErrNotFound
	}

	machine, ok := source.Lookup(clusterName, vmid)
	if !ok {
		recordResolveFailed(auditor, actor, clusterName, vmid, "vm not found or not in PVMSS scope")
		return Entity{}, ErrNotFound
	}

	if !slices.Contains(machine.Tags, pvmssTag) {
		recordResolveFailed(auditor, actor, clusterName, vmid, "vm not in PVMSS scope")
		return Entity{}, ErrNotFound
	}

	if !actor.IsAdmin && (actor.Pool == "" || machine.Pool != actor.Pool) {
		recordResolveFailed(auditor, actor, clusterName, vmid, "cross-pool access denied")
		return Entity{}, ErrForbidden
	}

	disks := cloneDisks(machine.Disks, machine.BootOrder)

	return Entity{
		Cluster:           clusterName,
		VMID:              machine.VMID,
		Name:              machine.Name,
		Node:              machine.Node,
		Pool:              machine.Pool,
		Status:            machine.Status,
		Tags:              append([]string(nil), machine.Tags...),
		OSType:            machine.OSType,
		CPUCores:          machine.CPUCores,
		Sockets:           machine.Sockets,
		Cores:             machine.Cores,
		MemoryTotal:       machine.MemoryTotal,
		DiskTotal:         machine.DiskTotal,
		Uptime:            machine.Uptime,
		Description:       machine.Description,
		BootOrder:         append([]string(nil), machine.BootOrder...),
		Disks:             disks,
		CDROM:             machine.CDROM,
		NetworkInterfaces: cluster.CloneNetworkInterfaces(machine.NetworkInterfaces),
		HasSerial:         machine.HasSerial,
		Agent:             machine.Agent,
	}, nil
}

// TargetCluster resolves the cluster actor may act on for a request naming
// requested. A non-admin is confined to the cluster it signed in to: pool
// names are per cluster, so the same pool name on another cluster may belong
// to someone else. An empty requested resolves to that cluster. Admins and
// sessions without a cluster (legacy single-cluster) are not confined.
func TargetCluster(actor auth.Identity, requested string) (string, bool) {
	if actor.IsAdmin || actor.Cluster == "" {
		return requested, true
	}

	if requested == "" {
		return actor.Cluster, true
	}

	return requested, requested == actor.Cluster
}

func cloneDisks(disks []cluster.Disk, bootOrder []string) []cluster.Disk {
	cloned := make([]cluster.Disk, len(disks))
	for i, disk := range disks {
		cloned[i] = disk
		cloned[i].IsBoot = isBootDisk(disk, bootOrder)
	}

	return cloned
}

func isBootDisk(disk cluster.Disk, bootOrder []string) bool {
	if len(bootOrder) == 0 {
		return disk.BusIndex == 0
	}

	return slices.Contains(bootOrder, disk.Key)
}
