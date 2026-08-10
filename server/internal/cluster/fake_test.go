//nolint:wsl_v5 // fake tests group state setup, call assertions, and reset checks
package cluster

import (
	"context"
	"errors"
	"slices"
	"testing"
)

// White-box: these invariants concern the raw dataset literals, not the
// public Client contract (that is contract_test.go's job).

//nolint:paralleltest // serial: shared fake dataset
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

//nolint:paralleltest // serial: shared fake dataset
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

//nolint:paralleltest // serial: shared fake dataset
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

//nolint:paralleltest // serial: shared fake dataset
func TestFakeDataset_UniqueVMIDs(t *testing.T) {
	seen := make(map[int]bool, len(fakeVMs))
	for _, vm := range fakeVMs {
		if seen[vm.VMID] {
			t.Errorf("duplicate VMID %d", vm.VMID)
		}

		seen[vm.VMID] = true
	}
}

//nolint:paralleltest // serial: shared fake dataset
func TestFakeDataset_MixedVMStatuses(t *testing.T) {
	var hasRunning, hasStopped bool

	for _, vm := range fakeVMs {
		switch vm.Status {
		case VMRunning:
			hasRunning = true
		case VMStopped:
			hasStopped = true
		case VMPaused:
			// paused VMs are not relevant to this invariant
		}
	}

	if !hasRunning {
		t.Error("expected at least one running VM")
	}

	if !hasStopped {
		t.Error("expected at least one stopped VM")
	}
}

//nolint:paralleltest // serial: shared fake dataset
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

//nolint:paralleltest // serial: shared fake cloud-init state
func TestFakeCloudInit_CallOrderAndFailureReset(t *testing.T) {
	ResetFake()
	defer ResetFake()
	config := CloudInitConfig{User: FakeCloudInitUser, IPMode: CloudInitIPModeDHCP}
	if err := (Fake{}).SetCloudInitConfig(context.Background(), FakeNode01, 101, config); err != nil {
		t.Fatalf("SetCloudInitConfig: %v", err)
	}
	if err := (Fake{}).PushCloudInitSnippet(context.Background(), FakeNode01, FakeSnippetStorage, "pvmss-101.yml", 101, "#cloud-config\n"); err != nil {
		t.Fatalf("PushCloudInitSnippet: %v", err)
	}
	calls := FakeCallsFor(101)
	if len(calls) != 3 || calls[0].Action != "ensure_cloudinit_drive" || calls[1].Action != "set_cloudinit_config" || calls[2].Action != "push_cloudinit_snippet" {
		t.Fatalf("calls = %+v", calls)
	}
	pushErr := errors.New("push failed")
	SetFakeCloudInitPushError(pushErr)
	if err := (Fake{}).PushCloudInitSnippet(context.Background(), FakeNode01, FakeSnippetStorage, "pvmss-101.yml", 101, ""); !errors.Is(err, pushErr) {
		t.Fatalf("push err = %v, want %v", err, pushErr)
	}
	ResetFake()
	if len(FakeCalls()) != 0 {
		t.Fatalf("calls after reset = %+v", FakeCalls())
	}
	if _, err := (Fake{}).GetCloudInitConfig(context.Background(), FakeNode01, 101); err != nil {
		t.Fatalf("config after reset: %v", err)
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
