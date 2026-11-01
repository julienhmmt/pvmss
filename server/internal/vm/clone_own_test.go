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
		Store:     f.store,
		Creator:   f.fake,
		Writer:    f.fake,
		FreeSpace: f.fake,
		Audit:     f.store,
		Log:       slog.New(slog.DiscardHandler),
		Services:  []*policy.Policy{f.service},
	}
}

// aliceSource returns one of alice's VMs with a disk and a NIC, the clone source.
// The demo seed gives disks and NICs to only a few VMs.
func (f cloneFixture) aliceSource(t *testing.T) cluster.VM {
	t.Helper()

	for _, candidate := range f.index.ByPool[cluster.FakePoolAlice] {
		if len(candidate.Disks) > 0 && len(candidate.NetworkInterfaces) > 0 {
			return candidate
		}
	}

	t.Fatal("demo fixtures have no alice VM with a disk and a NIC")

	return cluster.VM{}
}

// cloneMutatedSource replaces the indexed source with a copy changed by mutate,
// then clones it. Refusals are checked before dispatch, so FakeCalls stays empty.
func (f cloneFixture) cloneMutatedSource(t *testing.T, mutate func(*cluster.VM), deps vm.CreateDeps) error {
	t.Helper()

	source := f.aliceSource(t)
	mutate(&source)
	f.index.ByVMID[source.VMID] = source

	cluster.ClearFakeCalls()

	_, err := vm.CloneOwn(context.Background(), aliceIdentity(), f.index, testClusterName, source.VMID, "gate-copy", deps)

	if calls := cluster.FakeCalls(); len(calls) != 0 {
		t.Errorf("refused clone still dispatched: %v", calls)
	}

	return err
}

// zeroFreeSpace reports every storage as empty.
type zeroFreeSpace struct{}

func (zeroFreeSpace) StorageFreeSpace(context.Context, string, string) (int64, error) {
	return 0, nil
}

// failingTaskCreator reports every clone task as failed, the definite-failure path.
type failingTaskCreator struct {
	cluster.Fake
}

func (failingTaskCreator) TaskStatus(_ context.Context, upid string) (cluster.TaskStatus, error) {
	return cluster.TaskStatus{UPID: upid, State: cluster.TaskError, ExitMessage: "clone exploded"}, nil
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

// Gate: the source node must be approved on the target cluster.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCloneOwn_GateUnapprovedNode(t *testing.T) {
	fixture := newCloneFixture(t)

	err := fixture.cloneMutatedSource(t, func(v *cluster.VM) { v.Node = "ghost-node" }, fixture.deps())
	if !errors.Is(err, vm.ErrNotApproved) {
		t.Fatalf("unapproved node: err = %v, want ErrNotApproved", err)
	}
}

// Gate: every disk's storage must be approved on the source node.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCloneOwn_GateUnapprovedStorage(t *testing.T) {
	fixture := newCloneFixture(t)

	err := fixture.cloneMutatedSource(t, func(v *cluster.VM) {
		disks := append([]cluster.Disk(nil), v.Disks...)
		disks[0].Storage = "ghost-storage"
		v.Disks = disks
	}, fixture.deps())
	if !errors.Is(err, vm.ErrNotApproved) || !strings.Contains(err.Error(), "ghost-storage") {
		t.Fatalf("unapproved storage: err = %v, want ErrNotApproved naming storage", err)
	}
}

// Gate: a NIC model outside the server's whitelist is refused.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCloneOwn_GateNetworkModel(t *testing.T) {
	fixture := newCloneFixture(t)

	err := fixture.cloneMutatedSource(t, func(v *cluster.VM) {
		nics := append([]cluster.NetworkInterface(nil), v.NetworkInterfaces...)
		nics[0].Model = "bogus-nic"
		v.NetworkInterfaces = nics
	}, fixture.deps())
	if !errors.Is(err, vm.ErrNotApproved) || !strings.Contains(err.Error(), "bogus-nic") {
		t.Fatalf("unknown NIC model: err = %v, want ErrNotApproved naming the model", err)
	}
}

// Gate: a NIC VLAN that differs from the cluster's isolation tag is refused.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCloneOwn_GateIsolationVLAN(t *testing.T) {
	fixture := newCloneFixture(t)

	err := fixture.cloneMutatedSource(t, func(v *cluster.VM) {
		nics := append([]cluster.NetworkInterface(nil), v.NetworkInterfaces...)
		tag := 999
		nics[0].VLAN = &tag
		v.NetworkInterfaces = nics
	}, fixture.deps())
	if !errors.Is(err, vm.ErrNotApproved) || !strings.Contains(err.Error(), "999") {
		t.Fatalf("foreign VLAN tag: err = %v, want ErrNotApproved naming the tag", err)
	}
}

// Gate: the gabarit ceiling applies to the source's current hardware.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCloneOwn_GateGabarit(t *testing.T) {
	fixture := newCloneFixture(t)

	gabarit := policy.DefaultGabarit()

	gabarit.MaxDiskPerVMGB = 1
	if err := fixture.service.SetPolicy(context.Background(), testClusterName, gabarit, 100); err != nil {
		t.Fatalf("SetPolicy: %v", err)
	}

	err := fixture.cloneMutatedSource(t, func(*cluster.VM) {}, fixture.deps())
	if !errors.Is(err, policy.ErrGabaritExceeded) {
		t.Fatalf("gabarit disk ceiling: err = %v, want ErrGabaritExceeded", err)
	}
}

// Gate: a target storage without room for the full copy is refused.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCloneOwn_GateDiskSpace(t *testing.T) {
	fixture := newCloneFixture(t)

	deps := fixture.deps()
	deps.FreeSpace = zeroFreeSpace{}

	err := fixture.cloneMutatedSource(t, func(*cluster.VM) {}, deps)
	if !errors.Is(err, vm.ErrInsufficientDiskSpace) {
		t.Fatalf("no free space: err = %v, want ErrInsufficientDiskSpace", err)
	}
}

// A failed clone task releases the name reservation, so the same name can be
// tried again. Matches create: any wait error releases the reservation.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCloneOwn_FailedTaskReleasesReservation(t *testing.T) {
	fixture := newCloneFixture(t)
	source := fixture.aliceSource(t)

	deps := fixture.deps()
	deps.Creator = failingTaskCreator{}

	_, err := vm.CloneOwn(context.Background(), aliceIdentity(), fixture.index, testClusterName, source.VMID, "retry-me", deps)
	if !errors.Is(err, vm.ErrClusterCreate) {
		t.Fatalf("failed task: err = %v, want ErrClusterCreate", err)
	}

	_, err = vm.CloneOwn(context.Background(), aliceIdentity(), fixture.index, testClusterName, source.VMID, "retry-me", deps)
	if errors.Is(err, vm.ErrNameTaken) {
		t.Fatalf("name still reserved after failed task: %v", err)
	}
}

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
