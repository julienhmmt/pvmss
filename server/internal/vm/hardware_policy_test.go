package vm_test

import (
	"context"
	"errors"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/vm"
	"testing"
)

//nolint:paralleltest // serial: mutates global fake cluster state
func TestUpdateHardware_NodeCapacityExcludesResizedVM(t *testing.T) {
	cluster.ResetFake()

	index := diskTestIndex(t, 105, cluster.VMStopped)
	st := cloudInitStore(t)
	service := policy.New(st, inventory.NewProjectionFromIndex(index), cluster.Fake{})

	capacity, err := service.NodeCapacity(context.Background(), testClusterName, cluster.FakeNode02)
	if err != nil {
		t.Fatalf("NodeCapacity: %v", err)
	}

	capacity.MaxVCPUs = capacity.UsedVCPUs
	if err := service.SetNodeCapacity(context.Background(), testClusterName, cluster.FakeNode02, capacity); err != nil {
		t.Fatalf("SetNodeCapacity: %v", err)
	}

	deps := hardwareDependencies(index, bobIdentity(), 105)
	deps.Policy = service
	machine := index.ByVMID[105]

	cores := machine.Cores
	if cores < 1 {
		cores = machine.CPUCores
	}

	if err := vm.UpdateHardware(context.Background(), deps, vm.HardwarePatch{Cores: &cores}); err != nil {
		t.Fatalf("same VM growth should be evaluated after self exclusion: %v", err)
	}

	cluster.ResetFake()

	index = diskTestIndex(t, 105, cluster.VMStopped)
	deps.Index = index

	deps.Policy = service
	if err := vm.UpdateHardware(context.Background(), deps, vm.HardwarePatch{Cores: new(8)}); !errors.Is(err, policy.ErrNodeCapacityExceeded) {
		t.Fatalf("capacity error = %v, want ErrNodeCapacityExceeded", err)
	}

	if calls := cluster.FakeCallsFor(105); len(calls) != 0 {
		t.Fatalf("capacity rejection reached cluster: %+v", calls)
	}
}
