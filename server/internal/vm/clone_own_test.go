package vm_test

import (
	"context"
	"errors"
	"log/slog"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/vm"
	"strings"
	"testing"
)

// cloneFixture builds the create fixture plus an index of the demo VMs and a
// policy service, which is everything CloneOwn reads.
type cloneFixture struct {
	createFixture
	index   *inventory.Index
	service *policy.Policy
}

func newCloneFixture(t *testing.T) cloneFixture {
	t.Helper()

	fixture := newCreateFixture(t)

	snapshot, err := fixture.fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	index := inventory.BuildIndex(snapshot)

	service := policy.New(fixture.store, inventory.NewProjectionFromIndex(&index), fixture.fake)

	return cloneFixture{createFixture: fixture, index: &index, service: service}
}

func (f cloneFixture) deps() vm.CreateDeps {
	return vm.CreateDeps{
		Store:    f.store,
		Creator:  f.fake,
		Writer:   f.fake,
		Audit:    f.store,
		Log:      slog.New(slog.DiscardHandler),
		Services: []*policy.Policy{f.service},
	}
}

// aliceSource returns one of alice's VMs, the clone source.
func (f cloneFixture) aliceSource(t *testing.T) cluster.VM {
	t.Helper()

	vms := f.index.ByPool[cluster.FakePoolAlice]
	if len(vms) == 0 {
		t.Fatal("demo fixtures have no VM in alice's pool")
	}

	return vms[0]
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCloneOwn_FullCloneOnSourceNode(t *testing.T) {
	fixture := newCloneFixture(t)
	source := fixture.aliceSource(t)

	result, err := vm.CloneOwn(context.Background(), aliceIdentity(), fixture.index, testClusterName, source.VMID, "copy-of-source", fixture.deps())
	if err != nil {
		t.Fatalf("CloneOwn: %v", err)
	}

	if result.VMID == 0 || result.UPID == "" || result.Name != "copy-of-source" || result.Node != source.Node {
		t.Errorf("result = %+v, source node %s", result, source.Node)
	}

	entries, err := fixture.store.QueryAudit(context.Background())
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}

	if len(entries) != 1 || entries[0].Action != "vm_clone" || *entries[0].VMID != result.VMID {
		t.Errorf("audit entries = %+v, want one vm_clone for vmid %d", entries, result.VMID)
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCloneOwn_AdminRefused(t *testing.T) {
	fixture := newCloneFixture(t)
	source := fixture.aliceSource(t)
	admin := auth.Identity{Username: "admin", IsAdmin: true}

	_, err := vm.CloneOwn(context.Background(), admin, fixture.index, testClusterName, source.VMID, "copy", fixture.deps())
	if !errors.Is(err, vm.ErrAdminCannotCreate) {
		t.Fatalf("admin clone: err = %v, want ErrAdminCannotCreate", err)
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCloneOwn_ForeignVMForbidden(t *testing.T) {
	fixture := newCloneFixture(t)
	source := fixture.aliceSource(t)

	_, err := vm.CloneOwn(context.Background(), bobIdentity(), fixture.index, testClusterName, source.VMID, "copy", fixture.deps())
	if !errors.Is(err, vm.ErrForbidden) {
		t.Fatalf("cross-pool clone: err = %v, want ErrForbidden", err)
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCloneOwn_UnknownVMNotFound(t *testing.T) {
	fixture := newCloneFixture(t)

	_, err := vm.CloneOwn(context.Background(), aliceIdentity(), fixture.index, testClusterName, 999999, "copy", fixture.deps())
	if !errors.Is(err, vm.ErrNotFound) {
		t.Fatalf("unknown vmid: err = %v, want ErrNotFound", err)
	}
}

// A NIC on a bridge that is no longer approved refuses the clone before any
// VMID is spent, and the error names the bridge.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCloneOwn_UnapprovedBridgeRefused(t *testing.T) {
	fixture := newCloneFixture(t)
	source := fixture.aliceSource(t)

	if len(source.NetworkInterfaces) == 0 {
		t.Fatal("demo source VM has no NIC to check")
	}

	bridge := source.NetworkInterfaces[0].Bridge
	if err := fixture.store.SetBridgeEnabled(context.Background(), testClusterName, source.Node, bridge, false); err != nil {
		t.Fatalf("disable bridge approval: %v", err)
	}

	_, err := vm.CloneOwn(context.Background(), aliceIdentity(), fixture.index, testClusterName, source.VMID, "copy", fixture.deps())
	if !errors.Is(err, vm.ErrNotApproved) || !strings.Contains(err.Error(), bridge) {
		t.Fatalf("unapproved bridge: err = %v, want ErrNotApproved naming %q", err, bridge)
	}

	if calls := cluster.FakeCalls(); len(calls) != 0 {
		t.Errorf("refused clone still dispatched: %v", calls)
	}
}

// The copy's name is subject to the same per-pool uniqueness as create.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCloneOwn_NameTakenInPool(t *testing.T) {
	fixture := newCloneFixture(t)
	source := fixture.aliceSource(t)

	_, err := vm.CloneOwn(context.Background(), aliceIdentity(), fixture.index, testClusterName, source.VMID, source.Name, fixture.deps())
	if !errors.Is(err, vm.ErrNameTaken) {
		t.Fatalf("clone onto own name: err = %v, want ErrNameTaken", err)
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCloneOwn_InvalidNameRejected(t *testing.T) {
	fixture := newCloneFixture(t)
	source := fixture.aliceSource(t)

	_, err := vm.CloneOwn(context.Background(), aliceIdentity(), fixture.index, testClusterName, source.VMID, "Not A Hostname", fixture.deps())
	if !errors.Is(err, vm.ErrInvalidName) {
		t.Fatalf("invalid name: err = %v, want ErrInvalidName", err)
	}
}
