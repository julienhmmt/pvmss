package vm

import (
	"errors"
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

// Entity is a single VM resolved through the ownership gate — the only value
// a write handler may use to identify its target (FR-001). The Node field is
// authoritative: it comes from the Index, never from request input (FR-003,
// S01 root cause). Carries the detail-view metrics (V15) so GET /vms/:id can
// return it directly.
type Entity struct {
	Cluster     string
	VMID        int
	Name        string
	Node        string
	Pool        string
	Status      cluster.VMStatus
	Tags        []string
	CPUCores    int
	MemoryTotal int64
	DiskTotal   int64
	Uptime      time.Duration
	Description string
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
func Resolve(index *inventory.Index, actor auth.Identity, clusterName string, vmid int) (Entity, error) {
	machine, ok := index.ByVMID[vmid]
	if !ok {
		return Entity{}, ErrNotFound
	}
	if !slices.Contains(machine.Tags, pvmssTag) {
		return Entity{}, ErrNotFound
	}
	if !actor.IsAdmin && machine.Pool != actor.Pool {
		return Entity{}, ErrForbidden
	}
	return Entity{
		Cluster:     clusterName,
		VMID:        machine.VMID,
		Name:        machine.Name,
		Node:        machine.Node,
		Pool:        machine.Pool,
		Status:      machine.Status,
		Tags:        append([]string(nil), machine.Tags...),
		CPUCores:    machine.CPUCores,
		MemoryTotal: machine.MemoryTotal,
		DiskTotal:   machine.DiskTotal,
		Uptime:      machine.Uptime,
		Description: machine.Description,
	}, nil
}
