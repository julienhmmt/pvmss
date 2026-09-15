package vm_test

import (
	"context"
	"errors"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/vm"
	"slices"
	"testing"
)

// TestRetrofitToSeaBIOS_HappyPath - a stopped UEFI VM with no TPM state and no
// Secure Boot is switched to SeaBIOS: bios/machine/efidisk are cleared, an
// audit row is recorded, and the inventory refreshes.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestRetrofitToSeaBIOS_HappyPath(t *testing.T) {
	fixture := newCreateFixture(t)

	// Create a UEFI VM (explicit uefi=true), stopped.
	uefi := true
	req := imageRequest()
	req.UEFI = &uefi
	req.StartAfterCreate = false

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Confirm it is UEFI before retrofit.
	fw, err := fixture.fake.ReadFirmwareConfig(context.Background(), cluster.FakeNode01, result.VMID)
	if err != nil {
		t.Fatalf("ReadFirmwareConfig before: %v", err)
	}

	if fw.BIOS != testBIOSOVMF || !fw.HasEFIDisk {
		t.Fatalf("precondition: VM is not UEFI: %+v", fw)
	}

	// Build a fresh index from the fake snapshot (the retrofit resolves
	// against the index, not the live cluster).
	snap, err := fixture.fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	index := inventory.BuildIndexForCluster(testClusterName, snap)

	err = vm.RetrofitToSeaBIOS(context.Background(), vm.RetrofitDependencies{
		Index:       &index,
		Actor:       aliceIdentity(),
		ClusterName: testClusterName,
		VMID:        result.VMID,
		Writer:      fixture.fake,
		Audit:       fixture.store,
		Refresher:   noopRefresher{},
	})
	if err != nil {
		t.Fatalf("RetrofitToSeaBIOS: %v", err)
	}

	// Confirm it is SeaBIOS after retrofit.
	fw, err = fixture.fake.ReadFirmwareConfig(context.Background(), cluster.FakeNode01, result.VMID)
	if err != nil {
		t.Fatalf("ReadFirmwareConfig after: %v", err)
	}

	if fw.BIOS == testBIOSOVMF || fw.HasEFIDisk {
		t.Errorf("after retrofit: %+v, want SeaBIOS", fw)
	}

	// Audit row recorded.
	calls := cluster.FakeCallsFor(result.VMID)
	if !slices.ContainsFunc(calls, func(c cluster.FakeCall) bool { return c.Action == "retrofit_seabios" }) {
		t.Errorf("retrofit_seabios audit/call not recorded: %+v", calls)
	}
}

// TestRetrofitToSeaBIOS_RefusesTPM - a VM with TPM state is refused before
// any configuration change. The writer is not touched.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestRetrofitToSeaBIOS_RefusesTPM(t *testing.T) {
	fixture := newCreateFixture(t)

	// Create a UEFI VM with TPM.
	uefi := true
	req := imageRequest()
	req.UEFI = &uefi
	req.TPM = true
	req.StartAfterCreate = false

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	snap, _ := fixture.fake.Snapshot(context.Background())
	index := inventory.BuildIndexForCluster(testClusterName, snap)

	err = vm.RetrofitToSeaBIOS(context.Background(), vm.RetrofitDependencies{
		Index:       &index,
		Actor:       aliceIdentity(),
		ClusterName: testClusterName,
		VMID:        result.VMID,
		Writer:      fixture.fake,
		Audit:       fixture.store,
		Refresher:   noopRefresher{},
	})
	if !errors.Is(err, vm.ErrRetrofitRefused) {
		t.Fatalf("error = %v, want ErrRetrofitRefused", err)
	}

	// No retrofit_seabios call recorded.
	for _, c := range cluster.FakeCallsFor(result.VMID) {
		if c.Action == "retrofit_seabios" {
			t.Errorf("retrofit_seabios was called despite refusal: %+v", c)
		}
	}
}

// TestRetrofitToSeaBIOS_RefusesSecureBoot - a VM with Secure Boot is refused
// before any configuration change.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestRetrofitToSeaBIOS_RefusesSecureBoot(t *testing.T) {
	fixture := newCreateFixture(t)

	uefi := true
	req := imageRequest()
	req.UEFI = &uefi
	req.SecureBoot = true
	req.StartAfterCreate = false

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	snap, _ := fixture.fake.Snapshot(context.Background())
	index := inventory.BuildIndexForCluster(testClusterName, snap)

	err = vm.RetrofitToSeaBIOS(context.Background(), vm.RetrofitDependencies{
		Index:       &index,
		Actor:       aliceIdentity(),
		ClusterName: testClusterName,
		VMID:        result.VMID,
		Writer:      fixture.fake,
		Audit:       fixture.store,
		Refresher:   noopRefresher{},
	})
	if !errors.Is(err, vm.ErrRetrofitRefused) {
		t.Fatalf("error = %v, want ErrRetrofitRefused", err)
	}
}

// TestRetrofitToSeaBIOS_RunningRequiresConfirmation - a running UEFI VM
// without confirm=true returns ErrRetrofitRequiresConfirmation and changes
// nothing.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestRetrofitToSeaBIOS_RunningRequiresConfirmation(t *testing.T) {
	fixture := newCreateFixture(t)

	uefi := true
	req := imageRequest()
	req.UEFI = &uefi
	req.StartAfterCreate = true // running

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	snap, _ := fixture.fake.Snapshot(context.Background())
	index := inventory.BuildIndexForCluster(testClusterName, snap)

	err = vm.RetrofitToSeaBIOS(context.Background(), vm.RetrofitDependencies{
		Index:       &index,
		Actor:       aliceIdentity(),
		ClusterName: testClusterName,
		VMID:        result.VMID,
		Writer:      fixture.fake,
		Audit:       fixture.store,
		Refresher:   noopRefresher{},
		Confirm:     false,
	})
	if !errors.Is(err, vm.ErrRetrofitRequiresConfirmation) {
		t.Fatalf("error = %v, want ErrRetrofitRequiresConfirmation", err)
	}

	// No stop or retrofit call recorded.
	for _, c := range cluster.FakeCallsFor(result.VMID) {
		if c.Action == actionStop || c.Action == "retrofit_seabios" {
			t.Errorf("writer was touched despite no confirmation: %+v", c)
		}
	}
}

// TestRetrofitToSeaBIOS_RunningWithConfirmStopsAndStarts - a running UEFI VM
// with confirm=true is stopped, retrofitted, and started again.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestRetrofitToSeaBIOS_RunningWithConfirmStopsAndStarts(t *testing.T) {
	fixture := newCreateFixture(t)

	uefi := true
	req := imageRequest()
	req.UEFI = &uefi
	req.StartAfterCreate = true // running

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	snap, _ := fixture.fake.Snapshot(context.Background())
	index := inventory.BuildIndexForCluster(testClusterName, snap)

	err = vm.RetrofitToSeaBIOS(context.Background(), vm.RetrofitDependencies{
		Index:       &index,
		Actor:       aliceIdentity(),
		ClusterName: testClusterName,
		VMID:        result.VMID,
		Writer:      fixture.fake,
		Audit:       fixture.store,
		Refresher:   noopRefresher{},
		Confirm:     true,
	})
	if err != nil {
		t.Fatalf("RetrofitToSeaBIOS: %v", err)
	}

	calls := cluster.FakeCallsFor(result.VMID)

	actions := make([]string, 0, len(calls))

	for _, c := range calls {
		actions = append(actions, c.Action)
	}

	// stop before retrofit_seabios, and a start after retrofit_seabios.
	// (The create path also recorded a start at index ~4 - we want the one
	// the retrofit issued, which must come after the retrofit call.)
	stopIdx := slices.Index(actions, actionStop)

	retrofitIdx := slices.Index(actions, "retrofit_seabios")

	if stopIdx < 0 || retrofitIdx < 0 {
		t.Fatalf("missing stop or retrofit in %v", actions)
	}

	if stopIdx > retrofitIdx {
		t.Errorf("stop=%d should come before retrofit=%d: %v", stopIdx, retrofitIdx, actions)
	}

	// Find the first start after the retrofit.
	startIdx := -1

	for i := range actions[retrofitIdx+1:] {
		if actions[retrofitIdx+1+i] == "start" {
			startIdx = retrofitIdx + 1 + i

			break
		}
	}

	if startIdx < 0 {
		t.Fatalf("no start recorded after the retrofit: %v", actions)
	}
}
