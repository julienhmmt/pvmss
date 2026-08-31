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
	"slices"
	"testing"
)

// createWithServices runs vm.Create with a policy service backed by the given
// projection, so name-uniqueness and capacity checks see the live inventory.
func createWithServices(t *testing.T, fixture createFixture, actor auth.Identity, req vm.CreateRequest, service *policy.Policy) (vm.CreateResult, error) {
	t.Helper()

	return vm.Create(context.Background(), actor, req.Cluster, req, vm.CreateDeps{
		Store:     fixture.store,
		Creator:   fixture.fake,
		Pusher:    fixture.fake,
		Writer:    fixture.fake,
		FreeSpace: fixture.fake,
		Snippets:  fixture.fake,
		Audit:     fixture.store,
		Log:       slog.New(slog.DiscardHandler),
		Services:  []*policy.Policy{service},
	})
}

// policyWithProjection builds a policy service backed by a projection of the
// fake's current snapshot, so PoolHasName and capacity checks see live VMs.
func policyWithProjection(t *testing.T, fixture createFixture) *policy.Policy {
	t.Helper()

	snapshot, err := fixture.fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	index := inventory.BuildIndex(snapshot)

	return policy.New(fixture.store, inventory.NewProjectionFromIndex(&index), fixture.fake)
}

// --- US5/issue-05 (a): VMID collision retry ---

// TestCreate_VMIDCollisionRetry_SucceedsOnSecondAttempt — D5c: a CreateVM
// that returns ErrVMIDTaken once then succeeds produces a single VM with a
// different VMID than the first attempt, and no error reaches the client.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_VMIDCollisionRetry_SucceedsOnSecondAttempt(t *testing.T) {
	fixture := newCreateFixture(t)

	// Inject one collision; the second CreateVM call succeeds.
	cluster.SetFakeCreateError(cluster.ErrVMIDTaken, 1)

	result, err := fixture.create(t, aliceIdentity(), detailedRequest())
	if err != nil {
		t.Fatalf("Create: %v, want nil (collision should retry)", err)
	}

	if result.UPID == "" {
		t.Fatalf("no UPID returned")
	}

	// Exactly one VM should exist in the snapshot.
	snap, err := fixture.fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	idx := slices.IndexFunc(snap.VMs, func(v cluster.VM) bool { return v.VMID == result.VMID })
	if idx < 0 {
		t.Fatalf("created VM %d not in snapshot", result.VMID)
	}

	// Only one create call should have landed (the retry used a fresh VMID).
	createCalls := 0

	for _, c := range cluster.FakeCalls() {
		if c.Action == "create" {
			createCalls++
		}
	}

	if createCalls != 1 {
		t.Errorf("create calls = %d, want 1 (the collision was rejected, retry succeeded)", createCalls)
	}
}

// TestCreate_VMIDCollisionRetry_ExhaustsAfterThreeAttempts — D5c: three
// consecutive collisions produce ErrClusterCreate, not an infinite loop.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_VMIDCollisionRetry_ExhaustsAfterThreeAttempts(t *testing.T) {
	fixture := newCreateFixture(t)

	// Inject unlimited collisions (count=0 = unlimited).
	cluster.SetFakeCreateError(cluster.ErrVMIDTaken, 0)

	_, err := fixture.create(t, aliceIdentity(), detailedRequest())
	if !errors.Is(err, vm.ErrClusterCreate) {
		t.Fatalf("error = %v, want ErrClusterCreate", err)
	}

	if !errors.Is(err, cluster.ErrVMIDTaken) {
		t.Fatalf("error should wrap ErrVMIDTaken: %v", err)
	}
}

// --- US5/issue-05 (b): Rollback on failed task ---

// TestCreate_RollbackOnFailedTask_PurgesHalfMadeVM — D5a: when the create
// task fails, the half-made VM is purged (best-effort Delete) so it does not
// consume the user's quota. The original error is what reaches the client
// (via CloudInitPushError for the ISO path).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_RollbackOnFailedTask_PurgesHalfMadeVM(t *testing.T) {
	fixture := newCreateFixture(t)

	// Inject a task error on the next registered task.
	cluster.SetFakeTaskError("storage full")

	// Use a cloud-init template so the ISO path waits for the task.
	tmplID := createTestTemplate(t, fixture.store)

	req := detailedRequest()
	req.CloudInitTemplateID = tmplID

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v, want nil (task error is surfaced via CloudInitPushError)", err)
	}

	if result.CloudInitPushError == "" {
		t.Fatalf("CloudInitPushError = empty, want the task error message")
	}

	// The half-made VM should have been purged — it must not appear in the
	// snapshot.
	snap, err := fixture.fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	idx := slices.IndexFunc(snap.VMs, func(v cluster.VM) bool { return v.VMID == result.VMID })
	if idx >= 0 {
		t.Fatalf("half-made VM %d still in snapshot after rollback", result.VMID)
	}

	// A rollback audit entry should have been recorded.
	entries, err := fixture.store.QueryAudit(context.Background())
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}

	foundRollback := false

	for _, e := range entries {
		if e.Action == "vm_create_rollback" && e.VMID != nil && *e.VMID == result.VMID {
			foundRollback = true
			break
		}
	}

	if !foundRollback {
		t.Errorf("no vm_create_rollback audit entry for VMID %d", result.VMID)
	}
}

// TestCreate_RollbackOnFailedCloneTask_PurgesHalfMadeVM — D5a: when the clone
// task fails, the half-made VM is purged. The clone path always waits for the
// task, so a task error triggers rollback regardless of cloud-init.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_RollbackOnFailedCloneTask_PurgesHalfMadeVM(t *testing.T) {
	fixture := newCreateFixture(t)

	cluster.SetFakeTaskError("clone: disk allocation failed")

	req := templateRequest(9000)

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v, want nil (task error is surfaced via CloudInitPushError)", err)
	}

	if result.CloudInitPushError == "" {
		t.Fatalf("CloudInitPushError = empty, want the task error message")
	}

	snap, err := fixture.fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	idx := slices.IndexFunc(snap.VMs, func(v cluster.VM) bool { return v.VMID == result.VMID })
	if idx >= 0 {
		t.Fatalf("half-made clone %d still in snapshot after rollback", result.VMID)
	}
}

// --- US5/issue-05 (c): Name uniqueness by pool ---

// TestCreate_NameUniqueness_RejectsDuplicateInSamePool — D5b: a name already
// used by a VM in the actor's pool is rejected with ErrNameTaken before any
// VMID is consumed.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_NameUniqueness_RejectsDuplicateInSamePool(t *testing.T) {
	fixture := newCreateFixture(t)

	// First, create a VM named "web-prod" in Alice's pool.
	result, err := fixture.create(t, aliceIdentity(), detailedRequest())
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}

	// Build a projection that sees the first VM, so the name check finds it.
	service := policyWithProjection(t, fixture)

	// Now try to create another VM with the same name in Alice's pool.
	req := detailedRequest()
	req.Name = result.Name

	_, err = createWithServices(t, fixture, aliceIdentity(), req, service)
	if !errors.Is(err, vm.ErrNameTaken) {
		t.Fatalf("error = %v, want ErrNameTaken", err)
	}

	// No VMID should have been consumed for the rejected request.
	if calls := cluster.FakeCalls(); len(calls) == 0 {
		t.Fatalf("expected at least one call from the first create")
	}
}

// TestCreate_NameUniqueness_AllowsSameNameInDifferentPool — D5b: the same
// name in a different pool is accepted — the uniqueness is per-pool, not
// global, so two tenants can each have a "web-prod".
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_NameUniqueness_AllowsSameNameInDifferentPool(t *testing.T) {
	fixture := newCreateFixture(t)

	// Create a VM named "web-prod" in Alice's pool.
	aliceReq := detailedRequest()
	aliceReq.Name = "web-prod"

	_, err := fixture.create(t, aliceIdentity(), aliceReq)
	if err != nil {
		t.Fatalf("Alice Create: %v", err)
	}

	// Build a projection that sees Alice's VM.
	service := policyWithProjection(t, fixture)

	// Bob tries the same name in his own pool — must succeed.
	bobReq := detailedRequest()
	bobReq.Name = "web-prod"

	_, err = createWithServices(t, fixture, bobIdentity(), bobReq, service)
	if err != nil {
		t.Fatalf("Bob Create with same name in different pool: %v, want nil", err)
	}
}
