package vm_test

import (
	"context"
	"errors"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/vm"
	"testing"
)

//nolint:paralleltest // serial: shared fake cluster dataset
func TestUpdateHardware_RestartsForResourceChanges(t *testing.T) {
	cluster.ResetFake()

	deps := hardwareDependencies(diskTestIndex(t, 101, cluster.VMRunning), aliceIdentity(), 101)

	patch := vm.HardwarePatch{Cores: new(4)}
	if err := vm.UpdateHardware(context.Background(), deps, patch); err != nil {
		t.Fatalf("UpdateHardware: %v", err)
	}

	calls := cluster.FakeCallsFor(101)
	if len(calls) != 3 || calls[0].Action != "stop" || calls[1].Action != "update_hardware" || calls[2].Action != "start" {
		t.Fatalf("calls = %+v, want stop/update_hardware/start", calls)
	}
}

//nolint:paralleltest // serial: shared fake cluster dataset
func TestUpdateHardware_TagsOnlyStaysLive(t *testing.T) {
	cluster.ResetFake()

	deps := hardwareDependencies(diskTestIndex(t, 101, cluster.VMRunning), aliceIdentity(), 101)

	patch := vm.HardwarePatch{Tags: &[]string{"pvmss", "updated"}}
	if err := vm.UpdateHardware(context.Background(), deps, patch); err != nil {
		t.Fatalf("UpdateHardware: %v", err)
	}

	calls := cluster.FakeCallsFor(101)
	if len(calls) != 1 || calls[0].Action != "update_hardware" {
		t.Fatalf("calls = %+v, want one update_hardware call", calls)
	}
}

//nolint:paralleltest // serial: shared fake cluster dataset
func TestUpdateHardware_RejectsBoundBeforeWriter(t *testing.T) {
	cluster.ResetFake()

	deps := hardwareDependencies(diskTestIndex(t, 101, cluster.VMStopped), aliceIdentity(), 101)
	if err := vm.UpdateHardware(context.Background(), deps, vm.HardwarePatch{Cores: new(9)}); !errors.Is(err, vm.ErrHardwareExceedsLimit) {
		t.Fatalf("err = %v, want ErrHardwareExceedsLimit", err)
	}

	if calls := cluster.FakeCallsFor(101); len(calls) != 0 {
		t.Fatalf("fake calls = %+v, want none", calls)
	}
}

//nolint:paralleltest // serial: shared fake cluster dataset
func TestUpdateHardware_RejectsNonOwner(t *testing.T) {
	cluster.ResetFake()

	deps := hardwareDependencies(diskTestIndex(t, 101, cluster.VMStopped), bobIdentity(), 101)
	if err := vm.UpdateHardware(context.Background(), deps, vm.HardwarePatch{Cores: new(4)}); !errors.Is(err, vm.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}

	if calls := cluster.FakeCallsFor(101); len(calls) != 0 {
		t.Fatalf("fake calls = %+v, want none", calls)
	}
}

func hardwareDependencies(index *inventory.Index, actor auth.Identity, vmid int) vm.HardwareDependencies {
	return vm.HardwareDependencies{
		Index: index, Actor: actor, ClusterName: testClusterName, VMID: vmid, Writer: cluster.Fake{}, Limits: config.DefaultVMLimits(), Audit: noopAudit{}, Refresher: noopRefresher{},
	}
}
