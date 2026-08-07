package inventory_test

import (
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"reflect"
	"slices"
	"testing"
)

const testPvmssTag = "pvmss"

// fakeSnapshot returns the T01/T02 fake dataset shaped as a Snapshot — 3 nodes,
// 25 VMs, 4 pools, 5 storages. Mirrors server/internal/cluster/fake.go.
func fakeSnapshot() cluster.Snapshot {
	return cluster.Snapshot{
		Nodes: []cluster.Node{
			{
				Name: cluster.FakeNode01, Status: cluster.NodeOnline, CPUCores: 32, CPUUsage: 0.42,
				MemoryTotal: 137438953472, MemoryUsed: 68719476736,
				StorageTotal: 2199023255552, StorageUsed: 879609302220,
			},
			{
				Name: cluster.FakeNode02, Status: cluster.NodeOnline, CPUCores: 16, CPUUsage: 0.15,
				MemoryTotal: 68719476736, MemoryUsed: 17179869184,
				StorageTotal: 1099511627776, StorageUsed: 219902325555,
			},
			{
				Name: cluster.FakeNode03, Status: cluster.NodeOffline, CPUCores: 16, CPUUsage: 0,
				MemoryTotal: 68719476736, MemoryUsed: 0,
				StorageTotal: 1099511627776, StorageUsed: 0,
			},
		},
		VMs: fakeVMs(),
		Storages: []cluster.Storage{
			{Name: "local", Node: cluster.FakeNode01, Type: "dir", Total: 2199023255552, Used: 879609302220},
			{Name: "local-lvm", Node: cluster.FakeNode01, Type: "lvm", Total: 549755813888, Used: 219902325555},
			{Name: "ceph-data", Node: cluster.FakeNode02, Type: "cephfs", Total: 1099511627776, Used: 329853488332},
			{Name: "local", Node: cluster.FakeNode02, Type: "dir", Total: 274877906944, Used: 68719476736},
			{Name: "backup-nfs", Node: cluster.FakeNode03, Type: "nfs", Total: 5497558138880, Used: 1099511627776},
		},
	}
}

func fakeVMs() []cluster.VM {
	vms := []cluster.VM{
		{VMID: 100, Name: "web-01", Node: cluster.FakeNode01, Status: cluster.VMRunning, Pool: cluster.FakePoolAlice, Tags: []string{testPvmssTag, "web"}, CPUCores: 2, MemoryTotal: 4294967296},
		{VMID: 101, Name: "web-02", Node: cluster.FakeNode01, Status: cluster.VMStopped, Pool: cluster.FakePoolAlice, Tags: []string{testPvmssTag, "web"}, CPUCores: 2, MemoryTotal: 4294967296},
		{VMID: 102, Name: "db-01", Node: cluster.FakeNode01, Status: cluster.VMRunning, Pool: cluster.FakePoolAlice, Tags: []string{testPvmssTag, "db"}, CPUCores: 4, MemoryTotal: 8589934592},
		{VMID: 103, Name: "cache-01", Node: cluster.FakeNode01, Status: cluster.VMRunning, Pool: cluster.FakePoolBob, Tags: []string{testPvmssTag, "cache"}, CPUCores: 2, MemoryTotal: 2147483648},
		{VMID: 104, Name: "build-01", Node: cluster.FakeNode01, Status: cluster.VMStopped, Pool: cluster.FakePoolBob, Tags: []string{testPvmssTag, "ci"}, CPUCores: 4, MemoryTotal: 8589934592},
		{VMID: 105, Name: "test-01", Node: cluster.FakeNode02, Status: cluster.VMRunning, Pool: cluster.FakePoolBob, Tags: []string{testPvmssTag, "ci"}, CPUCores: 2, MemoryTotal: 4294967296},
		{VMID: 106, Name: "test-02", Node: cluster.FakeNode02, Status: cluster.VMStopped, Pool: cluster.FakePoolBob, Tags: []string{testPvmssTag, "ci"}, CPUCores: 2, MemoryTotal: 4294967296},
		{VMID: 107, Name: "mail-01", Node: cluster.FakeNode02, Status: cluster.VMRunning, Pool: cluster.FakePoolCarol, Tags: []string{testPvmssTag, "mail"}, CPUCores: 2, MemoryTotal: 4294967296},
		{VMID: 108, Name: "proxy-01", Node: cluster.FakeNode02, Status: cluster.VMRunning, Pool: cluster.FakePoolCarol, Tags: []string{testPvmssTag, "proxy"}, CPUCores: 1, MemoryTotal: 1073741824},
		{VMID: 109, Name: "legacy-01", Node: cluster.FakeNode02, Status: cluster.VMStopped, Pool: cluster.FakePoolCarol, Tags: nil, CPUCores: 4, MemoryTotal: 8589934592},
		{VMID: 110, Name: "legacy-02", Node: cluster.FakeNode02, Status: cluster.VMStopped, Pool: cluster.FakePoolCarol, Tags: nil, CPUCores: 4, MemoryTotal: 8589934592},
		{VMID: 111, Name: "backup-01", Node: cluster.FakeNode03, Status: cluster.VMStopped, Pool: cluster.FakePoolShared, Tags: []string{testPvmssTag, "backup"}, CPUCores: 2, MemoryTotal: 4294967296},
		{VMID: 112, Name: "monitor-01", Node: cluster.FakeNode01, Status: cluster.VMRunning, Pool: cluster.FakePoolShared, Tags: []string{testPvmssTag, "monitoring"}, CPUCores: 2, MemoryTotal: 4294967296},
		{VMID: 113, Name: "monitor-02", Node: cluster.FakeNode01, Status: cluster.VMPaused, Pool: cluster.FakePoolShared, Tags: []string{testPvmssTag, "monitoring"}, CPUCores: 2, MemoryTotal: 4294967296},
		{VMID: 114, Name: "sandbox-01", Node: cluster.FakeNode02, Status: cluster.VMStopped, Pool: cluster.FakePoolAlice, Tags: []string{testPvmssTag, "sandbox"}, CPUCores: 1, MemoryTotal: 1073741824},
		{VMID: 115, Name: "sandbox-02", Node: cluster.FakeNode02, Status: cluster.VMStopped, Pool: cluster.FakePoolAlice, Tags: []string{testPvmssTag, "sandbox"}, CPUCores: 1, MemoryTotal: 1073741824},
		{VMID: 116, Name: "app-01", Node: cluster.FakeNode01, Status: cluster.VMRunning, Pool: cluster.FakePoolBob, Tags: []string{testPvmssTag, "app"}, CPUCores: 4, MemoryTotal: 8589934592},
		{VMID: 117, Name: "app-02", Node: cluster.FakeNode01, Status: cluster.VMRunning, Pool: cluster.FakePoolBob, Tags: []string{testPvmssTag, "app"}, CPUCores: 4, MemoryTotal: 8589934592},
		{VMID: 118, Name: "app-03", Node: cluster.FakeNode02, Status: cluster.VMRunning, Pool: cluster.FakePoolBob, Tags: []string{testPvmssTag, "app"}, CPUCores: 4, MemoryTotal: 8589934592},
		{VMID: 119, Name: "queue-01", Node: cluster.FakeNode02, Status: cluster.VMRunning, Pool: cluster.FakePoolCarol, Tags: []string{testPvmssTag, "queue"}, CPUCores: 2, MemoryTotal: 4294967296},
		{VMID: 120, Name: "search-01", Node: cluster.FakeNode01, Status: cluster.VMRunning, Pool: cluster.FakePoolCarol, Tags: []string{testPvmssTag, "search"}, CPUCores: 4, MemoryTotal: 17179869184},
		{VMID: 121, Name: "archive-01", Node: cluster.FakeNode03, Status: cluster.VMStopped, Pool: cluster.FakePoolShared, Tags: nil, CPUCores: 2, MemoryTotal: 4294967296},
		{VMID: 122, Name: "archive-02", Node: cluster.FakeNode03, Status: cluster.VMStopped, Pool: cluster.FakePoolShared, Tags: nil, CPUCores: 2, MemoryTotal: 4294967296},
		{VMID: 123, Name: "dev-01", Node: cluster.FakeNode01, Status: cluster.VMRunning, Pool: cluster.FakePoolAlice, Tags: []string{testPvmssTag, "dev"}, CPUCores: 2, MemoryTotal: 4294967296},
		{VMID: 124, Name: "dev-02", Node: cluster.FakeNode01, Status: cluster.VMStopped, Pool: cluster.FakePoolAlice, Tags: []string{testPvmssTag, "dev"}, CPUCores: 2, MemoryTotal: 4294967296},
	}

	return vms
}

// TestIndex_VMCountConsistency — invariant 1: every VM appears in exactly one
// node bucket, and the total matches ByVMID.
//
//nolint:paralleltest // serial: shared inventory fixture
func TestIndex_VMCountConsistency(t *testing.T) {
	idx := inventory.BuildIndex(fakeSnapshot())

	totalByNode := 0
	for _, vms := range idx.ByNode {
		totalByNode += len(vms)
	}

	if totalByNode != len(idx.ByVMID) {
		t.Fatalf("len(ByVMID)=%d != sum(len(ByNode.values()))=%d", len(idx.ByVMID), totalByNode)
	}

	if len(idx.ByVMID) != 25 {
		t.Fatalf("expected 25 VMs, got %d", len(idx.ByVMID))
	}
}

// TestIndex_PoolMembershipConsistency — invariant 2: every VM in ByPool[p] has
// VM.Pool == p.
//
//nolint:paralleltest // serial: shared inventory fixture
func TestIndex_PoolMembershipConsistency(t *testing.T) {
	idx := inventory.BuildIndex(fakeSnapshot())

	for pool, vms := range idx.ByPool {
		for _, vm := range vms {
			if vm.Pool != pool {
				t.Errorf("VM %d in ByPool[%q] has Pool=%q", vm.VMID, pool, vm.Pool)
			}
		}
	}
}

// TestIndex_SnapshotImmutability — invariant 3: building an Index from a
// Snapshot never mutates the Snapshot.
//
//nolint:paralleltest // serial: shared inventory fixture
func TestIndex_SnapshotImmutability(t *testing.T) {
	snap := fakeSnapshot()
	snapCopy := cloneSnapshot(snap)

	_ = inventory.BuildIndex(snap)

	if !reflect.DeepEqual(snap, snapCopy) {
		t.Fatal("BuildIndex mutated the input Snapshot")
	}

	// Also verify that mutating the returned Index's slices does not affect
	// the original Snapshot.
	idx := inventory.BuildIndex(snap)
	if len(idx.Nodes) > 0 {
		idx.Nodes[0].Name = "mutated"
	}

	if snap.Nodes[0].Name == "mutated" {
		t.Fatal("mutating Index.Nodes leaked back into Snapshot")
	}
}

// TestIndex_ByPool — US3: querying by pool name returns exactly the VMs in
// that pool, matching the known fake dataset (25 VMs / 4 pools).
//
//nolint:paralleltest // serial: shared inventory fixture
func TestIndex_ByPool(t *testing.T) {
	idx := inventory.BuildIndex(fakeSnapshot())

	expected := map[string]int{
		cluster.FakePoolAlice:  7,
		cluster.FakePoolBob:    7,
		cluster.FakePoolCarol:  6,
		cluster.FakePoolShared: 5,
	}
	for pool, want := range expected {
		got := len(idx.ByPool[pool])
		if got != want {
			t.Errorf("ByPool[%q]: got %d VMs, want %d", pool, got, want)
		}

		for _, vm := range idx.ByPool[pool] {
			if vm.Pool != pool {
				t.Errorf("ByPool[%q] contains VM %d with Pool=%q", pool, vm.VMID, vm.Pool)
			}
		}
	}

	total := 0
	for _, vms := range idx.ByPool {
		total += len(vms)
	}

	if total != 25 {
		t.Fatalf("total VMs across pools: got %d, want 25", total)
	}
}

// TestIndex_ByNode — US3: querying by node name returns the VMs on that node,
// matching the per-node VM count shown on screen (FR-008).
//
//nolint:paralleltest // serial: shared inventory fixture
func TestIndex_ByNode(t *testing.T) {
	idx := inventory.BuildIndex(fakeSnapshot())

	expected := map[string]int{
		cluster.FakeNode01: 12,
		cluster.FakeNode02: 10,
		cluster.FakeNode03: 3,
	}
	for node, want := range expected {
		got := len(idx.ByNode[node])
		if got != want {
			t.Errorf("ByNode[%q]: got %d VMs, want %d", node, got, want)
		}

		for _, vm := range idx.ByNode[node] {
			if vm.Node != node {
				t.Errorf("ByNode[%q] contains VM %d with Node=%q", node, vm.VMID, vm.Node)
			}
		}
	}
}

// TestIndex_NodesSortedByName — Nodes are sorted by name, stable across reads.
//
//nolint:paralleltest // serial: shared inventory fixture
func TestIndex_NodesSortedByName(t *testing.T) {
	idx := inventory.BuildIndex(fakeSnapshot())
	for i := 1; i < len(idx.Nodes); i++ {
		if idx.Nodes[i-1].Name > idx.Nodes[i].Name {
			t.Fatalf("Nodes not sorted by name: %q before %q", idx.Nodes[i-1].Name, idx.Nodes[i].Name)
		}
	}
}

// TestIndex_StoragesByNode — FR-007: storages indexed by node.
//
//nolint:paralleltest // serial: shared inventory fixture
func TestIndex_StoragesByNode(t *testing.T) {
	idx := inventory.BuildIndex(fakeSnapshot())

	expected := map[string]int{
		cluster.FakeNode01: 2,
		cluster.FakeNode02: 2,
		cluster.FakeNode03: 1,
	}
	for node, want := range expected {
		got := len(idx.StoragesByNode[node])
		if got != want {
			t.Errorf("StoragesByNode[%q]: got %d, want %d", node, got, want)
		}
	}
}

// TestIndex_RefreshedAtZero — a freshly built Index has a zero RefreshedAt
// (FR-009: zero means "never successfully refreshed").
//
//nolint:paralleltest // serial: shared inventory fixture
func TestIndex_RefreshedAtZero(t *testing.T) {
	idx := inventory.BuildIndex(fakeSnapshot())
	if !idx.RefreshedAt.IsZero() {
		t.Fatalf("BuildIndex should not set RefreshedAt, got %v", idx.RefreshedAt)
	}
}

// TestIndex_TagsCopied — mutating a VM's Tags in the index must not affect the
// snapshot (deep copy invariant).
//
//nolint:paralleltest // serial: shared inventory fixture
func TestIndex_TagsCopied(t *testing.T) {
	snap := fakeSnapshot()
	snapTagsBefore := append([]string(nil), snap.VMs[0].Tags...)

	idx := inventory.BuildIndex(snap)
	// Find the same VM in the index and mutate its Tags.
	vm := idx.ByVMID[snap.VMs[0].VMID]
	if vm.Tags != nil {
		vm.Tags[0] = "mutated"
	}

	if !slices.Equal(snap.VMs[0].Tags, snapTagsBefore) {
		t.Fatalf("mutating index Tags leaked into snapshot: got %v, want %v", snap.VMs[0].Tags, snapTagsBefore)
	}
}

func cloneSnapshot(s cluster.Snapshot) cluster.Snapshot {
	nodes := make([]cluster.Node, len(s.Nodes))
	copy(nodes, s.Nodes)

	vms := make([]cluster.VM, len(s.VMs))
	for i, vm := range s.VMs {
		vms[i] = vm
		if vm.Tags != nil {
			vms[i].Tags = append([]string(nil), vm.Tags...)
		}
	}

	storages := make([]cluster.Storage, len(s.Storages))
	copy(storages, s.Storages)

	return cluster.Snapshot{Nodes: nodes, VMs: vms, Storages: storages}
}
