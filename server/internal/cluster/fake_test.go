package cluster

import (
	"slices"
	"testing"
)

// White-box: these invariants concern the raw dataset literals, not the
// public Client contract (that is contract_test.go's job).

func TestFakeDataset_NodeReferences(t *testing.T) {
	names := nodeNames(t)
	for _, vm := range fakeVMs {
		if !names[vm.Node] {
			t.Errorf("VM %d (%s) references unknown node %q", vm.VMID, vm.Name, vm.Node)
		}
	}
	for _, s := range fakeStorages {
		if !names[s.Node] {
			t.Errorf("storage %q references unknown node %q", s.Name, s.Node)
		}
	}
}

func TestFakeDataset_PoolReferences(t *testing.T) {
	pools := make(map[string]bool, len(fakePools))
	for _, p := range fakePools {
		pools[p.Name] = true
	}
	for _, vm := range fakeVMs {
		if !pools[vm.Pool] {
			t.Errorf("VM %d (%s) references unknown pool %q", vm.VMID, vm.Name, vm.Pool)
		}
	}
}

func TestFakeDataset_UsageWithinTotal(t *testing.T) {
	for _, n := range fakeNodes {
		if n.MemoryUsed > n.MemoryTotal {
			t.Errorf("node %q: memoryUsed %d > memoryTotal %d", n.Name, n.MemoryUsed, n.MemoryTotal)
		}
		if n.StorageUsed > n.StorageTotal {
			t.Errorf("node %q: storageUsed %d > storageTotal %d", n.Name, n.StorageUsed, n.StorageTotal)
		}
	}
	for _, s := range fakeStorages {
		if s.Used > s.Total {
			t.Errorf("storage %q: used %d > total %d", s.Name, s.Used, s.Total)
		}
	}
}

func TestFakeDataset_UniqueVMIDs(t *testing.T) {
	seen := make(map[int]bool, len(fakeVMs))
	for _, vm := range fakeVMs {
		if seen[vm.VMID] {
			t.Errorf("duplicate VMID %d", vm.VMID)
		}
		seen[vm.VMID] = true
	}
}

func TestFakeDataset_MixedVMStatuses(t *testing.T) {
	var hasRunning, hasStopped bool
	for _, vm := range fakeVMs {
		switch vm.Status {
		case VMRunning:
			hasRunning = true
		case VMStopped:
			hasStopped = true
		}
	}
	if !hasRunning {
		t.Error("expected at least one running VM")
	}
	if !hasStopped {
		t.Error("expected at least one stopped VM")
	}
}

func TestFakeDataset_MixedPvmssTagging(t *testing.T) {
	var hasTagged, hasUntagged bool
	for _, vm := range fakeVMs {
		if slices.Contains(vm.Tags, "pvmss") {
			hasTagged = true
		} else {
			hasUntagged = true
		}
	}
	if !hasTagged {
		t.Error("expected at least one VM tagged pvmss")
	}
	if !hasUntagged {
		t.Error("expected at least one VM without the pvmss tag")
	}
}

func nodeNames(t *testing.T) map[string]bool {
	t.Helper()
	names := make(map[string]bool, len(fakeNodes))
	for _, n := range fakeNodes {
		names[n.Name] = true
	}
	return names
}
