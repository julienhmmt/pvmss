// Package inventory owns the in-memory projection of cluster data — a
// periodically refreshed index built from a cluster.Snapshot, indexed for
// the lookups later tranches need (by VM ID, by pool, by node). The index
// is never persisted (AC02: it is a cache, never a source of truth).
package inventory

import (
	"pvmss/server/internal/cluster"
	"slices"
	"sort"
	"strings"
	"time"
)

// Index is the in-memory projection built from one cluster.Snapshot. It is
// immutable once built — the worker swaps the whole pointer on refresh, so
// readers always see either the previous complete index or the new complete
// one, never a partial one (FR-004).
type Index struct {
	Nodes          []cluster.Node
	ByVMID         map[int]cluster.VM
	ByPool         map[string][]cluster.VM
	ByNode         map[string][]cluster.VM
	StoragesByNode map[string][]cluster.Storage
	RefreshedAt    time.Time
}

// BuildIndex constructs an Index from a Snapshot. It is a pure function —
// it never mutates the input Snapshot, and the returned Index owns its own
// copies of all slice and map data (data-model.md invariant 3).
func BuildIndex(snap cluster.Snapshot) Index {
	nodes := make([]cluster.Node, len(snap.Nodes))
	copy(nodes, snap.Nodes)
	slices.SortFunc(nodes, func(a, b cluster.Node) int {
		return strings.Compare(a.Name, b.Name)
	})

	vms := make([]cluster.VM, len(snap.VMs))
	for i, vm := range snap.VMs {
		vms[i] = vm
		if vm.Tags != nil {
			vms[i].Tags = append([]string(nil), vm.Tags...)
		}
	}

	storages := make([]cluster.Storage, len(snap.Storages))
	copy(storages, snap.Storages)

	byVMID := make(map[int]cluster.VM, len(vms))
	byPool := make(map[string][]cluster.VM)
	byNode := make(map[string][]cluster.VM)
	storagesByNode := make(map[string][]cluster.Storage)

	for _, vm := range vms {
		byVMID[vm.VMID] = vm
		byPool[vm.Pool] = append(byPool[vm.Pool], vm)
		byNode[vm.Node] = append(byNode[vm.Node], vm)
	}
	for _, s := range storages {
		storagesByNode[s.Node] = append(storagesByNode[s.Node], s)
	}

	for _, v := range byPool {
		sort.Slice(v, func(i, j int) bool { return v[i].VMID < v[j].VMID })
	}
	for _, v := range byNode {
		sort.Slice(v, func(i, j int) bool { return v[i].VMID < v[j].VMID })
	}
	for _, v := range storagesByNode {
		sort.Slice(v, func(i, j int) bool { return v[i].Name < v[j].Name })
	}

	return Index{
		Nodes:          nodes,
		ByVMID:         byVMID,
		ByPool:         byPool,
		ByNode:         byNode,
		StoragesByNode: storagesByNode,
	}
}
