package vm_test

import (
	"context"
	"errors"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/vm"
	"slices"
	"testing"
	"time"
)

//nolint:paralleltest // serial: shared fake cluster dataset
func TestUpdateHardware_RestartsForResourceChanges(t *testing.T) {
	cluster.ResetFake()

	// The fake now rejects stop on an already-stopped VM. VM 101 is
	// stopped in the pristine dataset, but the test exercises the restart
	// flow for a running VM - start it first so the fake dataset matches the
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
	// The setup "start" is call 0; UpdateHardware's shutdown/update/start are 1-3.
	if len(calls) != 4 || calls[1].Action != actionShutdown || calls[2].Action != actionUpdateHW || calls[3].Action != actionStart {
		t.Fatalf("calls = %+v, want setup-start/shutdown/update_hardware/start", calls)
	}
}

// slowStopWriter is the fake writer whose guest takes a few polls to stop
// after a shutdown, or never stops when stopAfter < 0.
type slowStopWriter struct {
	cluster.Fake
	stopAfter int
	polls     *int
	log       *[]string
}

func (w slowStopWriter) Action(_ context.Context, _ string, _ int, action string) error {
	*w.log = append(*w.log, action)
	return nil
}

func (w slowStopWriter) UpdateHardware(context.Context, string, int, int, int, int, []string) error {
	*w.log = append(*w.log, actionUpdateHW)
	return nil
}

func (w slowStopWriter) VMStatus(context.Context, string, int) (cluster.VMLiveStatus, error) {
	*w.polls++
	if w.stopAfter >= 0 && *w.polls > w.stopAfter {
		*w.log = append(*w.log, "observed-stopped")
		return cluster.VMLiveStatus{Status: cluster.VMStopped}, nil
	}

	return cluster.VMLiveStatus{Status: cluster.VMRunning}, nil
}

//nolint:paralleltest // mutates the package stop-wait timings
func TestUpdateHardware_StartsOnlyAfterGuestStopped(t *testing.T) {
	cluster.ResetFake()
	vm.SetStopWaitForTest(t, time.Millisecond, time.Second)

	var log []string

	polls := 0
	deps := hardwareDependencies(diskTestIndex(t, 101, cluster.VMRunning), aliceIdentity(), 101)
	deps.Writer = slowStopWriter{stopAfter: 3, polls: &polls, log: &log}

	if err := vm.UpdateHardware(context.Background(), deps, vm.HardwarePatch{Cores: new(4)}); err != nil {
		t.Fatalf("UpdateHardware: %v", err)
	}

	want := []string{actionShutdown, "observed-stopped", actionUpdateHW, actionStart}
	if !slices.Equal(log, want) {
		t.Fatalf("sequence = %v, want %v", log, want)
	}
}

//nolint:paralleltest // mutates the package stop-wait timings
func TestUpdateHardware_GuestNeverStopsChangesNothing(t *testing.T) {
	cluster.ResetFake()
	vm.SetStopWaitForTest(t, time.Millisecond, 20*time.Millisecond)

	var log []string

	polls := 0
	deps := hardwareDependencies(diskTestIndex(t, 101, cluster.VMRunning), aliceIdentity(), 101)
	deps.Writer = slowStopWriter{stopAfter: -1, polls: &polls, log: &log}

	err := vm.UpdateHardware(context.Background(), deps, vm.HardwarePatch{Cores: new(4)})
	if !errors.Is(err, vm.ErrShutdownTimeout) {
		t.Fatalf("err = %v, want ErrShutdownTimeout", err)
	}

	if !slices.Equal(log, []string{actionShutdown}) {
		t.Fatalf("sequence = %v, want only the shutdown request (no power cut, no config)", log)
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
	if len(calls) != 1 || calls[0].Action != actionUpdateHW {
		t.Fatalf("calls = %+v, want one update_hardware call", calls)
	}
}

//nolint:paralleltest // serial: shared fake cluster dataset
func TestUpdateHardware_UnknownTagRejected(t *testing.T) {
	cluster.ResetFake()

	deps := hardwareDependencies(diskTestIndex(t, 101, cluster.VMStopped), aliceIdentity(), 101)

	patch := vm.HardwarePatch{Tags: &[]string{"forged-tag"}}
	if err := vm.UpdateHardware(context.Background(), deps, patch); !errors.Is(err, vm.ErrNotApproved) {
		t.Fatalf("err = %v, want ErrNotApproved", err)
	}

	if calls := cluster.FakeCallsFor(101); len(calls) != 0 {
		t.Fatalf("fake calls = %+v, want none", calls)
	}
}

//nolint:paralleltest // serial: shared fake cluster dataset
func TestUpdateHardware_PvmssTagAlwaysRetained(t *testing.T) {
	cluster.ResetFake()

	deps := hardwareDependencies(diskTestIndex(t, 101, cluster.VMRunning), aliceIdentity(), 101)

	// A patch that omits pvmss must not strip the mandatory tag:
	// a VM without it is invisible to every PVMSS endpoint.
	patch := vm.HardwarePatch{Tags: &[]string{"updated"}}
	if err := vm.UpdateHardware(context.Background(), deps, patch); err != nil {
		t.Fatalf("UpdateHardware: %v", err)
	}

	calls := cluster.FakeCallsFor(101)
	if len(calls) != 1 || calls[0].Action != actionUpdateHW {
		t.Fatalf("calls = %+v, want one update_hardware call", calls)
	}

	snap, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	for _, testVM := range snap.VMs {
		if testVM.VMID == 101 && !slices.Contains(testVM.Tags, testPvmssTag) {
			t.Fatalf("tags = %v, want pvmss retained", testVM.Tags)
		}
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
		AllowedTags: []string{testPvmssTag, "updated"},
	}
}
