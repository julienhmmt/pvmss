package vm_test

import (
	"context"
	"fmt"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/vm"
	"testing"
)

// uncappedPolicy returns a policy service whose cluster gabarit is all zeros -
// the "no cap" state the admin policy page documents.
func uncappedPolicy(t *testing.T, index *inventory.Index) *policy.Policy {
	t.Helper()

	service := policy.New(cloudInitStore(t), inventory.NewProjectionFromIndex(index), cluster.Fake{})
	if err := service.SetGabarit(context.Background(), testClusterName, policy.Gabarit{}); err != nil {
		t.Fatalf("SetGabarit: %v", err)
	}

	return service
}

//nolint:paralleltest // serial: shared fake cluster dataset
func TestUpdateHardware_ZeroGabaritIsUncapped(t *testing.T) {
	cluster.ResetFake()

	index := diskTestIndex(t, 101, cluster.VMStopped)
	service := uncappedPolicy(t, index)

	deps := hardwareDependencies(index, aliceIdentity(), 101)
	deps.Policy = service

	cores := 64 // beyond the shipped gabarit, under an uncapped ceiling
	if err := vm.UpdateHardware(context.Background(), deps, vm.HardwarePatch{Cores: &cores}); err != nil {
		t.Fatalf("UpdateHardware: %v", err)
	}
}

//nolint:paralleltest // serial: shared fake cluster dataset
func TestUpdateNetwork_ZeroGabaritIsUncapped(t *testing.T) {
	cluster.ResetFake()

	index := diskTestIndex(t, 101, cluster.VMRunning)
	service := uncappedPolicy(t, index)

	deps := networkDependencies(index, aliceIdentity(), 101)
	deps.Policy = service

	interfaces := make([]cluster.NetworkInterface, policy.DefaultGabarit().MaxNetworkCards+2)
	for i := range interfaces {
		interfaces[i] = cluster.NetworkInterface{Index: i, Bridge: testBridgeVMbr0, Model: testModelVirtio}
	}

	result, err := vm.UpdateNetwork(context.Background(), deps, interfaces)
	if err != nil {
		t.Fatalf("UpdateNetwork: %v", err)
	}
	if len(result) != len(interfaces) {
		t.Fatalf("interfaces = %d, want %d", len(result), len(interfaces))
	}
}

//nolint:paralleltest // serial: shared fake snapshot registry
func TestCreateSnapshot_ZeroGabaritIsUncapped(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)

	// More snapshots than the shipped cap: the uncapped gabarit must not stop
	// the create.
	for i := range policy.DefaultGabarit().MaxSnapshots + 2 {
		seedSnapshot(t, fmt.Sprintf("snap-%d", i))
	}

	index := testSnapshotIndex(t, func(snap *cluster.Snapshot) {
		for i := range snap.VMs {
			if snap.VMs[i].VMID == 101 {
				snap.VMs[i].Status = cluster.VMRunning
			}
		}
	})
	service := uncappedPolicy(t, index)

	deps := snapshotDependencies(index, 101, policy.Gabarit{})
	deps.Policy = service

	if _, err := vm.CreateSnapshot(context.Background(), deps, "one-more", "", false); err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}
}

//nolint:paralleltest // serial: shared fake cluster dataset
func TestDiskWrites_ZeroGabaritIsUncapped(t *testing.T) {
	cluster.ResetFake()

	index := diskTestIndex(t, 101, cluster.VMStopped)
	service := uncappedPolicy(t, index)

	deps := diskDependencies(index, aliceIdentity(), 101)
	deps.Policy = service

	sizeGB := policy.DefaultGabarit().MaxDiskPerVMGB * 2 // beyond the shipped cap
	if _, err := vm.AddDisk(context.Background(), deps, cluster.DiskBusSCSI, "local-lvm", sizeGB); err != nil {
		t.Fatalf("AddDisk: %v", err)
	}
	if err := vm.ResizeDisk(context.Background(), deps, "scsi1", sizeGB); err != nil {
		t.Fatalf("ResizeDisk: %v", err)
	}
}
