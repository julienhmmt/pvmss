package vm_test

import (
	"context"
	"errors"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/vm"
	"testing"
)

//nolint:paralleltest // serial: shared fake cluster dataset
func TestUpdateHardware_RestartsForResourceChanges(t *testing.T) {
	cluster.ResetFake()

	// T001b: the fake now rejects stop on an already-stopped VM. VM 101 is
	// stopped in the pristine dataset, but the test exercises the restart
	// flow for a running VM — start it first so the fake dataset matches the
	// index's running status.
	if err := (cluster.Fake{}).Action(context.Background(), cluster.FakeNode01, 101, "start"); err != nil {
		t.Fatalf("start VM 101 for test setup: %v", err)
	}

	deps := hardwareDependencies(diskTestIndex(t, 101, cluster.VMRunning), aliceIdentity(), 101)

	patch := vm.HardwarePatch{Cores: new(4)}
	if err := vm.UpdateHardware(context.Background(), deps, patch); err != nil {
		t.Fatalf("UpdateHardware: %v", err)
	}

	calls := cluster.FakeCallsFor(101)
	// The setup "start" is call 0; UpdateHardware's stop/update/start are 1-3.
	if len(calls) != 4 || calls[1].Action != "stop" || calls[2].Action != "update_hardware" || calls[3].Action != "start" {
		t.Fatalf("calls = %+v, want setup-start/stop/update_hardware/start", calls)
	}
}

//nolint:paralleltest // serial: shared fake cluster dataset
func TestUpdateHardware_TagsOnlyStaysLive(t *testing.T) {
	cluster.ResetFake()

	deps := hardwareDependencies(diskTestIndex(t, 101, cluster.VMRunning), aliceIdentity(), 101)

	patch := vm.HardwarePatch{Tags: &[]string{testPvmssTag, "updated"}}
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
		Index: index, Actor: actor, ClusterName: testClusterName, VMID: vmid, Writer: cluster.Fake{}, Gabarit: policy.DefaultGabarit(), Audit: noopAudit{}, Refresher: noopRefresher{},
	}
}
