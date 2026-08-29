package vm

import (
	"context"
	"errors"
	"fmt"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"slices"
	"time"
)

// ErrNotFound is returned by Resolve when the (cluster, vmid) pair is absent
// from the Index OR when the VM exists but lacks the mandatory pvmss tag
// (FR-002: a VM outside PVMSS scope is indistinguishable from a nonexistent
// one — the response never confirms a VM's existence to a caller who shouldn't
// see it).
var ErrNotFound = errors.New("vm not found")

// ErrForbidden is returned by Resolve when the VM is tagged pvmss but the
// caller's pool does not own it and the caller is not an admin (FR-002). This
// is the only branch that distinguishes "wrong owner" from "doesn't exist" —
// and only to tell a legitimate owner-in-training their action was understood
// but denied, not to leak existence to a probing caller.
var ErrForbidden = errors.New("forbidden")

// pvmssTag is the mandatory tag that scopes a VM into PVMSS (FR-002). A VM
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

var resolveAuditor adminActionRecorder

// SetResolveAuditor wires the audit recorder used by Resolve for NotFound and
// Forbidden paths. It is set from the main composition root.
func SetResolveAuditor(recorder adminActionRecorder) {
	resolveAuditor = recorder
}

func recordResolveFailed(actor auth.Identity, clusterName string, vmid int, reason string) {
	if resolveAuditor == nil {
		return
	}

	targetID := FormatVMRef(clusterName, vmid)
	detail := fmt.Sprintf(`{"summary":"%s","changes":[]}`, reason)
	_ = resolveAuditor.RecordAdminAction(context.Background(), actor.Username, "vm.resolve_failed", "vm", targetID, detail, "")
}

// Entity is a single VM resolved through the ownership gate — the only value
// a write handler may use to identify its target (FR-001). The Node field is
// authoritative: it comes from the Index, never from request input (FR-003,
// S01 root cause). Carries the detail-view metrics (V15) so GET /vms/:id can
// return it directly.
type Entity struct {
	Cluster           string
	VMID              int
	Name              string
	Node              string
	Pool              string
	Status            cluster.VMStatus
	Tags              []string
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
}

// Resolve is the ONLY function capable of turning a (cluster, vmid) pair into
// a writable VM entity anywhere in the codebase (FR-001, SC-005). Every write
// handler — action, delete, patch — calls this first and uses nothing else to
// identify the target VM or its node. After T05, no future handler can reach
// Proxmox for a write without going through it: the class of bug S01
// represents becomes structurally unreachable, not merely patched at one call
// site.
//
// Steps, in order (data-model.md):
//  1. Look up (cluster, vmid) in index.ByVMID. Not found → ErrNotFound.
//  2. Check the pvmss tag. Absent → ErrNotFound (same error as step 1).
//  3. If actor.IsAdmin, skip to step 5.
//  4. Check entity.Pool == actor.Pool. Mismatch → ErrForbidden.
//  5. Return Entity — the node exactly as recorded in the Index.
//
// Resolve is pure: it reads the Index snapshot it is handed and never calls
// the cluster client. Callers do, after Resolve returns (constitution IV:
// reads and writes are separated). It is re-evaluated on every single write
// request — no request-scoped or session-scoped caching of a prior
// authorization result (FR-011, constitution VI).
//
// T15: source widened from a concrete *inventory.Index to the one-method
// inventory.LookupSource interface so Resolve can dispatch through whichever
// cluster's projection clusterName names (single decode point preserved —
// constitution III/V — rather than pushing per-cluster lookup out to every
// caller). *inventory.Index still implements LookupSource, so this is a
// backward-compatible widening: every pre-T15 call site keeps compiling and
// behaving identically. actor/clusterName/vmid's order, types, and the
// (Entity, error) return are unchanged.
func Resolve(source inventory.LookupSource, actor auth.Identity, clusterName string, vmid int) (Entity, error) {
	machine, ok := source.Lookup(clusterName, vmid)
	if !ok {
		recordResolveFailed(actor, clusterName, vmid, "vm not found or not in PVMSS scope")
		return Entity{}, ErrNotFound
	}

	if !slices.Contains(machine.Tags, pvmssTag) {
		recordResolveFailed(actor, clusterName, vmid, "vm not in PVMSS scope")
		return Entity{}, ErrNotFound
	}

	if !actor.IsAdmin && machine.Pool != actor.Pool {
		recordResolveFailed(actor, clusterName, vmid, "cross-pool access denied")
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
		NetworkInterfaces: cloneNetworkInterfaces(machine.NetworkInterfaces),
		HasSerial:         machine.HasSerial,
	}, nil
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

func cloneNetworkInterfaces(interfaces []cluster.NetworkInterface) []cluster.NetworkInterface {
	cloned := make([]cluster.NetworkInterface, len(interfaces))
	for i, iface := range interfaces {
		cloned[i] = iface
		cloned[i].IPAddresses = append([]string(nil), iface.IPAddresses...)
	}

	return cloned
}
