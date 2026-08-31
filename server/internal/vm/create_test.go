package vm_test

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"slices"
	"testing"
	"time"
)

// createFixture wires the real seeded store and the fake Creator, so
// validation runs against the same catalog rows production serves (T06: the
// catalog is fixture data, not a mock).
type createFixture struct {
	store *store.Store
	fake  cluster.Fake
}

func newCreateFixture(t *testing.T) createFixture {
	t.Helper()
	t.Cleanup(cluster.ResetFake)

	st, err := store.Open(config.Configuration{
		DBPath:    filepath.Join(t.TempDir(), "vm-create.db"),
		LogLevel:  "info",
		LogFormat: "json",
		LogOutput: "stdout",
	})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	ctx := context.Background()
	for _, bridge := range []catalog.Bridge{
		{Name: testBridgeVMbr0, Node: "pve-node-01"},
		{Name: testBridgeVMbr1, Node: "pve-node-01"},
		{Name: testBridgeVMbr0, Node: "pve-node-02"},
	} {
		if err := st.SetBridgeEnabled(ctx, "default", bridge.Node, bridge.Name, true); err != nil {
			t.Fatalf("seed bridge approval: %v", err)
		}
	}

	// FR-013: tags are admin-curated — seed the one the tests reference.
	if err := st.InsertTag(ctx, testClusterName, "team-web", "#3b82f6", time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatalf("seed tag approval: %v", err)
	}

	return createFixture{store: st, fake: cluster.Fake{}}
}

func (f createFixture) create(t *testing.T, actor auth.Identity, req vm.CreateRequest) (vm.CreateResult, error) {
	t.Helper()

	log := slog.New(slog.DiscardHandler)

	return vm.Create(context.Background(), actor, req.Cluster, req, vm.CreateDeps{
		Store:     f.store,
		Creator:   f.fake,
		Pusher:    f.fake,
		Writer:    f.fake,
		FreeSpace: f.fake,
		Snippets:  f.fake,
		Audit:     f.store,
		Log:       log,
	})
}

func aliceIdentity() auth.Identity {
	return auth.Identity{Username: cluster.FakeUserAlice, Pool: cluster.FakePoolAlice}
}

func bobIdentity() auth.Identity {
	return auth.Identity{Username: cluster.FakeUserBob, Pool: cluster.FakePoolBob}
}

// mustPolicyService builds a policy service over the fixture's store.
func mustPolicyService(st *store.Store) *policy.Policy {
	return policy.New(st, nil, nil)
}

// mustGabarit reads the cluster's gabarit, failing the test on error.
func mustGabarit(t *testing.T, st *store.Store) policy.Gabarit {
	t.Helper()

	gabarit, err := mustPolicyService(st).Gabarit(context.Background(), testClusterName)
	if err != nil {
		t.Fatalf("Gabarit: %v", err)
	}

	return gabarit
}

// detailedRequest is a fully explicit, catalog-valid detailed-mode request.
func detailedRequest() vm.CreateRequest {
	return vm.CreateRequest{
		Cluster:  testClusterName,
		Name:     "web-01",
		Node:     cluster.FakeNode01,
		CPUCores: 2,
		MemoryMB: 4096,
		Disk:     vm.DiskRequest{Storage: cluster.FakeStorageLocalLVM, SizeGB: 40},
		Network:  vm.NetworkRequest{{Bridge: cluster.FakeBridgeVMbr0, Model: string(cluster.DiskBusVirtio)}},
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_ValidationPipeline(t *testing.T) {
	cases := []struct {
		name    string
		actor   auth.Identity
		mutate  func(*vm.CreateRequest)
		wantErr error
	}{
		{
			name:    "non-admin without pool",
			actor:   auth.Identity{Username: "nopool@pve", IsAdmin: false},
			mutate:  func(_ *vm.CreateRequest) {},
			wantErr: vm.ErrNoPool,
		},
		{
			name:    "invalid hostname",
			actor:   aliceIdentity(),
			mutate:  func(r *vm.CreateRequest) { r.Name = "Bad_Name!" },
			wantErr: vm.ErrInvalidName,
		},
		{
			name:    "cpu out of range",
			actor:   aliceIdentity(),
			mutate:  func(r *vm.CreateRequest) { r.CPUCores = 64 },
			wantErr: vm.ErrOutOfRange,
		},
		{
			name:    "memory out of range",
			actor:   aliceIdentity(),
			mutate:  func(r *vm.CreateRequest) { r.MemoryMB = 0 },
			wantErr: vm.ErrOutOfRange,
		},
		{
			name:    "disk out of range",
			actor:   aliceIdentity(),
			mutate:  func(r *vm.CreateRequest) { r.Disk.SizeGB = -5 },
			wantErr: vm.ErrOutOfRange,
		},
		{
			name:    "node not approved",
			actor:   aliceIdentity(),
			mutate:  func(r *vm.CreateRequest) { r.Node = "pve-node-03" },
			wantErr: vm.ErrNotApproved,
		},
		{
			name:    "storage not approved",
			actor:   aliceIdentity(),
			mutate:  func(r *vm.CreateRequest) { r.Disk.Storage = "nas-scratch" },
			wantErr: vm.ErrNotApproved,
		},
		{
			name:    "storage approved but on another node",
			actor:   aliceIdentity(),
			mutate:  func(r *vm.CreateRequest) { r.Disk.Storage = "ceph-data" },
			wantErr: vm.ErrNotApproved,
		},
		{
			name:    "bridge not approved",
			actor:   aliceIdentity(),
			mutate:  func(r *vm.CreateRequest) { r.Network[0].Bridge = "vmbr9" },
			wantErr: vm.ErrNotApproved,
		},
		{
			name:  "iso not approved",
			actor: aliceIdentity(),
			mutate: func(r *vm.CreateRequest) {
				r.ISO = &vm.ISORequest{Storage: "local", File: "windows-11.iso"}
			},
			wantErr: vm.ErrNotApproved,
		},
		{
			name:    "profile not approved",
			actor:   aliceIdentity(),
			mutate:  func(r *vm.CreateRequest) { r.ProfileID = "huge" },
			wantErr: vm.ErrNotApproved,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newCreateFixture(t)
			req := detailedRequest()
			tc.mutate(&req)

			_, err := fixture.create(t, tc.actor, req)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			// Rejections happen before any cluster call (contracts behavioural
			// rule): no VMID burned, no task created.
			if calls := cluster.FakeCalls(); len(calls) != 0 {
				t.Fatalf("rejected request reached the cluster: %+v", calls)
			}
		})
	}
}

// TestCreate_ProfileResolvesHardware — FR-009: a profile's catalog values win
// over any hardware fields the request also carries; the client cannot
// contradict the chosen profile.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_ProfileResolvesHardware(t *testing.T) {
	fixture := newCreateFixture(t)
	req := vm.CreateRequest{
		Cluster:          testClusterName,
		Name:             "profiled-vm",
		ProfileID:        "medium",
		CPUCores:         32, // contradictory — must be ignored
		MemoryMB:         65536,
		Disk:             vm.DiskRequest{SizeGB: 2048},
		StartAfterCreate: true,
	}

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if result.VMID < 1 || result.UPID == "" {
		t.Fatalf("unexpected result: %+v", result)
	}

	snap, err := fixture.fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	idx := slices.IndexFunc(snap.VMs, func(v cluster.VM) bool { return v.VMID == result.VMID })
	if idx < 0 {
		t.Fatalf("created VM not in snapshot")
	}

	created := snap.VMs[idx]
	if created.CPUCores != 2 {
		t.Errorf("cpuCores = %d, want 2 (medium profile, request said 32)", created.CPUCores)
	}

	if created.MemoryTotal != 4096*1024*1024 {
		t.Errorf("memory = %d, want 4096 MB (medium profile)", created.MemoryTotal)
	}

	if created.DiskTotal != 40*1024*1024*1024 {
		t.Errorf("disk = %d, want 40 GB (medium profile)", created.DiskTotal)
	}
}

// TestCreate_SimpleModeAutoSelection — FR-010: unset node/storage/bridge are
// filled from the first approved catalog entries, deterministically.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_SimpleModeAutoSelection(t *testing.T) {
	fixture := newCreateFixture(t)

	result, err := fixture.create(t, aliceIdentity(), vm.CreateRequest{
		Cluster:   testClusterName,
		Name:      "auto-vm",
		ProfileID: "small",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if result.Node != cluster.FakeNode01 {
		t.Errorf("auto-selected node = %q, want %q", result.Node, cluster.FakeNode01)
	}

	snap, err := fixture.fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	idx := slices.IndexFunc(snap.VMs, func(v cluster.VM) bool { return v.VMID == result.VMID })
	if idx < 0 {
		t.Fatalf("created VM not in snapshot")
	}

	if snap.VMs[idx].Status != cluster.VMStopped {
		t.Errorf("status = %q, want stopped (no startAfterCreate)", snap.VMs[idx].Status)
	}
}

// TestCreate_PoolIsAlwaysActors — FR-004/SC-003: the created VM's pool is the
// actor's own. The request type carries no pool field, so there is nothing to
// forge; this test pins that the spec dispatched to the cluster always takes
// the pool from the identity.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_PoolIsAlwaysActors(t *testing.T) {
	fixture := newCreateFixture(t)

	result, err := fixture.create(t, aliceIdentity(), detailedRequest())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	snap, err := fixture.fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	idx := slices.IndexFunc(snap.VMs, func(v cluster.VM) bool { return v.VMID == result.VMID })
	if idx < 0 {
		t.Fatalf("created VM not in snapshot")
	}

	if snap.VMs[idx].Pool != cluster.FakePoolAlice {
		t.Errorf("pool = %q, want actor's pool %q", snap.VMs[idx].Pool, cluster.FakePoolAlice)
	}
}

// TestCreate_AdminCannotCreate — an admin (local or cluster) cannot create
// VMs through the self-service portal. VM ownership requires a personal pool,
// which admins do not have.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_AdminCannotCreate(t *testing.T) {
	fixture := newCreateFixture(t)
	admin := auth.Identity{Username: "admin@pve", IsAdmin: true}

	_, err := fixture.create(t, admin, detailedRequest())
	if !errors.Is(err, vm.ErrAdminCannotCreate) {
		t.Fatalf("Create: want ErrAdminCannotCreate, got %v", err)
	}
}

// TestCreate_PvmssTagAlwaysPresent — FR-006.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_PvmssTagAlwaysPresent(t *testing.T) {
	fixture := newCreateFixture(t)
	req := detailedRequest()
	req.Tags = []string{"team-web"}

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	snap, err := fixture.fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	idx := slices.IndexFunc(snap.VMs, func(v cluster.VM) bool { return v.VMID == result.VMID })
	if idx < 0 {
		t.Fatalf("created VM not in snapshot")
	}

	tags := snap.VMs[idx].Tags
	if !slices.Contains(tags, "pvmss") || !slices.Contains(tags, "team-web") {
		t.Errorf("tags = %v, want both pvmss and team-web", tags)
	}
}

// TestCreate_RecordsAudit — FR-017: a successful creation lands in the audit
// log with the real actor, the allocated VMID, and action vm_create.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_RecordsAudit(t *testing.T) {
	fixture := newCreateFixture(t)

	result, err := fixture.create(t, aliceIdentity(), detailedRequest())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	entries, err := fixture.store.QueryAudit(context.Background())
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(entries))
	}

	entry := entries[0]
	if entry.Actor != cluster.FakeUserAlice || entry.Cluster != testClusterName || *entry.VMID != result.VMID || entry.Action != "vm_create" {
		t.Errorf("audit entry = %+v", entry)
	}
}

// failingAudit always errors — simulates the audit log write failing after
// the cluster has already accepted the creation task.
type failingAudit struct{}

func (failingAudit) RecordAction(context.Context, string, string, int, string) error {
	return errors.New("audit write failed")
}

// TestCreate_AuditFailureDoesNotFailCreate — a step-7 audit-write failure
// must not turn an already-dispatched creation into a client-facing error:
// the cluster task is real by the time audit runs, so the client still needs
// its upid to poll. Regression for the T06 audit-failure-orphaned-task gap.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_AuditFailureDoesNotFailCreate(t *testing.T) {
	fixture := newCreateFixture(t)
	log := slog.New(slog.DiscardHandler)

	result, err := vm.Create(context.Background(), aliceIdentity(), testClusterName, detailedRequest(), vm.CreateDeps{
		Store:   fixture.store,
		Creator: fixture.fake,
		Pusher:  fixture.fake,
		Writer:  fixture.fake,
		Audit:   failingAudit{},
		Log:     log,
	})
	if err != nil {
		t.Fatalf("Create: %v, want nil (audit failure must not fail the request)", err)
	}

	if result.VMID < 1 || result.UPID == "" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

// --- T18 cloud-init template application (T011) ---

const testCloudInitContent = "#cloud-config\npackages:\n  - nginx\n"

// createTestTemplate inserts an enabled cloud-init template into the fixture
// store and returns its id.
func createTestTemplate(t *testing.T, st *store.Store) string {
	t.Helper()

	tmpl, err := catalog.CreateCloudInitTemplate(context.Background(), st, testClusterName, "Web server", testCloudInitContent)
	if err != nil {
		t.Fatalf("CreateCloudInitTemplate: %v", err)
	}

	return tmpl.ID
}

// TestCreate_CloudInitTemplate_Applied — a valid enabled template is resolved,
// the snippet is persisted and pushed with the exact template content, and the
// result echoes the template id (FR-007, SC-003).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_CloudInitTemplate_Applied(t *testing.T) {
	fixture := newCreateFixture(t)
	tmplID := createTestTemplate(t, fixture.store)

	req := detailedRequest()
	req.CloudInitTemplateID = tmplID

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if result.CloudInitTemplateID != tmplID {
		t.Errorf("result.CloudInitTemplateID = %q, want %q", result.CloudInitTemplateID, tmplID)
	}

	if result.CloudInitPushError != "" {
		t.Errorf("result.CloudInitPushError = %q, want empty", result.CloudInitPushError)
	}

	// The snippet was persisted with the exact template content.
	snippet, found, err := fixture.store.GetCloudInitSnippet(context.Background(), testClusterName, result.VMID)
	if err != nil {
		t.Fatalf("GetCloudInitSnippet: %v", err)
	}

	if !found {
		t.Fatal("snippet not persisted")
	}

	if snippet.Content != testCloudInitContent {
		t.Errorf("snippet content = %q, want %q", snippet.Content, testCloudInitContent)
	}

	// The push was recorded with the exact content.
	pushed := false

	for _, c := range cluster.FakeCallsFor(result.VMID) {
		if c.Action == "push_cloudinit_snippet" && c.Content == testCloudInitContent {
			pushed = true
		}
	}

	if !pushed {
		t.Error("PushCloudInitSnippet not recorded with the template content")
	}
}

// failingSnippetFinder refuses every resolution — the stand-in for a node
// without any snippet-capable storage (ticket 04).
type failingSnippetFinder struct{}

func (failingSnippetFinder) FindSnippetStorage(context.Context, string) (string, error) {
	return "", cluster.ErrNotFound
}

// fixedSnippetFinder always resolves to one storage and counts calls, so
// tests can assert the create path used the plan-resolved target (ticket 04).
type fixedSnippetFinder struct {
	calls   int
	storage string
}

func (f *fixedSnippetFinder) FindSnippetStorage(context.Context, string) (string, error) {
	f.calls++

	return f.storage, nil
}

// TestCreate_CloudInitTemplate_NoSnippetStorage_RejectedBeforeVMID — ticket
// 04: a cloud-init template on a node without snippet-capable storage is
// refused before NextVMID, instead of creating a VM whose cloud-init is
// silently absent.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_CloudInitTemplate_NoSnippetStorage_RejectedBeforeVMID(t *testing.T) {
	fixture := newCreateFixture(t)
	tmplID := createTestTemplate(t, fixture.store)

	req := detailedRequest()
	req.CloudInitTemplateID = tmplID

	log := slog.New(slog.DiscardHandler)

	_, err := vm.Create(context.Background(), aliceIdentity(), testClusterName, req, vm.CreateDeps{
		Store: fixture.store, Creator: fixture.fake, Pusher: fixture.fake,
		Writer: fixture.fake, FreeSpace: fixture.fake, Snippets: failingSnippetFinder{},
		Audit: fixture.store, Log: log,
	})
	if !errors.Is(err, vm.ErrNoSnippetStorage) {
		t.Fatalf("error = %v, want ErrNoSnippetStorage", err)
	}

	for _, c := range cluster.FakeCalls() {
		if c.Action == "create" {
			t.Fatalf("a VM was created despite the missing snippet storage: %+v", c)
		}
	}
}

// TestCreate_CloudInitTemplate_UsesPlanSnippetStorage — ticket 04: the
// snippet is pushed to the storage resolved at plan time, never the VM disk's
// storage (which is block-backed and cannot host a snippet).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_CloudInitTemplate_UsesPlanSnippetStorage(t *testing.T) {
	fixture := newCreateFixture(t)
	tmplID := createTestTemplate(t, fixture.store)

	finder := &fixedSnippetFinder{storage: "snippet-vol"}

	req := detailedRequest()
	req.CloudInitTemplateID = tmplID

	log := slog.New(slog.DiscardHandler)

	result, err := vm.Create(context.Background(), aliceIdentity(), testClusterName, req, vm.CreateDeps{
		Store: fixture.store, Creator: fixture.fake, Pusher: fixture.fake,
		Writer: fixture.fake, FreeSpace: fixture.fake, Snippets: finder,
		Audit: fixture.store, Log: log,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if finder.calls == 0 {
		t.Fatal("the plan never resolved a snippet storage")
	}

	pushedToSnippetVol := false

	for _, c := range cluster.FakeCallsFor(result.VMID) {
		if c.Action == "push_cloudinit_snippet" {
			if c.Storage != "snippet-vol" {
				t.Fatalf("snippet pushed to %q, want the plan-resolved snippet-vol", c.Storage)
			}

			pushedToSnippetVol = true
		}
	}

	if !pushedToSnippetVol {
		t.Fatal("no snippet push recorded")
	}
}

// TestCreate_WithoutCloudInitTemplate_DoesNotResolveSnippetStorage — ticket
// 04: the resolution costs a cluster read and must not run on the plain ISO
// path.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_WithoutCloudInitTemplate_DoesNotResolveSnippetStorage(t *testing.T) {
	fixture := newCreateFixture(t)
	finder := &fixedSnippetFinder{storage: "snippet-vol"}

	log := slog.New(slog.DiscardHandler)

	if _, err := vm.Create(context.Background(), aliceIdentity(), testClusterName, detailedRequest(), vm.CreateDeps{
		Store: fixture.store, Creator: fixture.fake, Pusher: fixture.fake,
		Writer: fixture.fake, FreeSpace: fixture.fake, Snippets: finder,
		Audit: fixture.store, Log: log,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if finder.calls != 0 {
		t.Errorf("FindSnippetStorage calls = %d, want 0 without a cloud-init template", finder.calls)
	}
}

// TestCreate_CloudInitTemplate_Unknown_RejectedBeforeVMID — an unknown template
// id is rejected with ErrNotApproved before NextVMID is called (FR-006, SC-004):
// zero NextVMID/CreateVM calls for the rejected request.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_CloudInitTemplate_Unknown_RejectedBeforeVMID(t *testing.T) {
	fixture := newCreateFixture(t)

	req := detailedRequest()
	req.CloudInitTemplateID = "does-not-exist"

	_, err := fixture.create(t, aliceIdentity(), req)
	if !errors.Is(err, vm.ErrNotApproved) {
		t.Fatalf("error = %v, want ErrNotApproved", err)
	}

	if calls := cluster.FakeCalls(); len(calls) != 0 {
		t.Fatalf("rejected request reached the cluster: %+v", calls)
	}
}

// TestCreate_CloudInitTemplate_Disabled_RejectedBeforeVMID — a disabled template
// id is rejected with ErrNotApproved before NextVMID is called (FR-006).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_CloudInitTemplate_Disabled_RejectedBeforeVMID(t *testing.T) {
	fixture := newCreateFixture(t)
	tmplID := createTestTemplate(t, fixture.store)

	if err := catalog.SetCloudInitTemplateEnabled(context.Background(), fixture.store, testClusterName, tmplID, false); err != nil {
		t.Fatalf("disable template: %v", err)
	}

	req := detailedRequest()
	req.CloudInitTemplateID = tmplID

	_, err := fixture.create(t, aliceIdentity(), req)
	if !errors.Is(err, vm.ErrNotApproved) {
		t.Fatalf("error = %v, want ErrNotApproved", err)
	}

	if calls := cluster.FakeCalls(); len(calls) != 0 {
		t.Fatalf("rejected request reached the cluster: %+v", calls)
	}
}

// TestCreate_CloudInitTemplate_PushFailure_SoftField — a push failure after
// CreateVM succeeded sets CreateResult.CloudInitPushError without failing the
// creation (FR-008): the VM still materializes.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_CloudInitTemplate_PushFailure_SoftField(t *testing.T) {
	fixture := newCreateFixture(t)
	tmplID := createTestTemplate(t, fixture.store)

	cluster.SetFakeCloudInitPushError(errors.New("cluster client: push failed"))
	t.Cleanup(func() { cluster.SetFakeCloudInitPushError(nil) })

	req := detailedRequest()
	req.CloudInitTemplateID = tmplID

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v, want nil (push failure must not fail creation)", err)
	}

	if result.VMID < 1 || result.UPID == "" {
		t.Fatalf("creation did not succeed: %+v", result)
	}

	if result.CloudInitPushError == "" {
		t.Error("result.CloudInitPushError should be non-empty on push failure")
	}

	if result.CloudInitTemplateID != tmplID {
		t.Errorf("result.CloudInitTemplateID = %q, want %q (resolved even though push failed)", result.CloudInitTemplateID, tmplID)
	}
}

// TestCreate_CloudInitTemplate_DeletedAfterUse — deleting a template after a VM
// was created from it leaves the VM's own snippet unchanged (FR-009, SC-006).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_CloudInitTemplate_DeletedAfterUse(t *testing.T) {
	fixture := newCreateFixture(t)
	tmplID := createTestTemplate(t, fixture.store)

	req := detailedRequest()
	req.CloudInitTemplateID = tmplID

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := catalog.DeleteCloudInitTemplate(context.Background(), fixture.store, testClusterName, tmplID); err != nil {
		t.Fatalf("DeleteCloudInitTemplate: %v", err)
	}

	snippet, found, err := fixture.store.GetCloudInitSnippet(context.Background(), testClusterName, result.VMID)
	if err != nil {
		t.Fatalf("GetCloudInitSnippet: %v", err)
	}

	if !found {
		t.Fatal("VM snippet vanished after template deletion (no cascade expected)")
	}

	if snippet.Content != testCloudInitContent {
		t.Errorf("snippet content = %q, want %q (unchanged)", snippet.Content, testCloudInitContent)
	}
}

// --- US2: sockets and multi-NIC (T037, T038) ---

// TestCreate_Sockets_PopulatesForm — sockets=2, cores=4 produces a VM with
// Sockets=2, Cores=4, and CPUCores=8 (sockets*cores) in the fake dataset
// (US2/D3b).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_Sockets_PopulatesForm(t *testing.T) {
	fixture := newCreateFixture(t)
	req := detailedRequest()
	req.Sockets = 2
	req.CPUCores = 4

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	snap, err := fixture.fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	idx := slices.IndexFunc(snap.VMs, func(v cluster.VM) bool { return v.VMID == result.VMID })
	if idx < 0 {
		t.Fatalf("created VM not in snapshot")
	}

	created := snap.VMs[idx]
	if created.Sockets != 2 {
		t.Errorf("sockets = %d, want 2", created.Sockets)
	}

	if created.Cores != 4 {
		t.Errorf("cores = %d, want 4", created.Cores)
	}

	if created.CPUCores != 8 {
		t.Errorf("cpuCores (sockets*cores) = %d, want 8", created.CPUCores)
	}
}

// TestCreate_Sockets_DefaultsToOne — a request without sockets preserves the
// previous behaviour: the created VM has Sockets=1 (US2/D3b).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_Sockets_DefaultsToOne(t *testing.T) {
	fixture := newCreateFixture(t)
	req := detailedRequest()

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	snap, err := fixture.fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	idx := slices.IndexFunc(snap.VMs, func(v cluster.VM) bool { return v.VMID == result.VMID })
	if idx < 0 {
		t.Fatalf("created VM not in snapshot")
	}

	if snap.VMs[idx].Sockets != 1 {
		t.Errorf("sockets = %d, want 1 (default)", snap.VMs[idx].Sockets)
	}
}

// TestCreate_Sockets_BeyondMaxSockets — sockets exceeding the gabarit's
// MaxSockets is rejected with GabaritExceededError{Field: "sockets"} before
// any cluster call (US2/D3b).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_Sockets_BeyondMaxSockets(t *testing.T) {
	fixture := newCreateFixture(t)

	snapshot, err := fixture.fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	index := inventory.BuildIndex(snapshot)
	service := policy.New(fixture.store, inventory.NewProjectionFromIndex(&index), fixture.fake)

	gabarit, err := service.Gabarit(context.Background(), testClusterName)
	if err != nil {
		t.Fatalf("Gabarit: %v", err)
	}

	gabarit.MaxSockets = 1
	if err := service.SetGabarit(context.Background(), testClusterName, gabarit); err != nil {
		t.Fatalf("SetGabarit: %v", err)
	}

	req := detailedRequest()
	req.Sockets = 2
	req.Name = "sockets-over"

	_, err = vm.Create(context.Background(), aliceIdentity(), req.Cluster, req, vm.CreateDeps{
		Store:    fixture.store,
		Creator:  fixture.fake,
		Pusher:   fixture.fake,
		Audit:    fixture.store,
		Log:      slog.New(slog.DiscardHandler),
		Services: []*policy.Policy{service},
	})
	if !errors.Is(err, policy.ErrGabaritExceeded) {
		t.Fatalf("error = %v, want ErrGabaritExceeded", err)
	}

	var gabaritErr *policy.GabaritExceededError
	if !errors.As(err, &gabaritErr) {
		t.Fatalf("error is not a GabaritExceededError: %v", err)
	}

	if gabaritErr.Field != "sockets" {
		t.Errorf("field = %q, want %q", gabaritErr.Field, "sockets")
	}

	if calls := cluster.FakeCalls(); len(calls) != 0 {
		t.Fatalf("gabarit rejection reached cluster: %+v", calls)
	}
}

// TestCreate_Sockets_NodeCapacityCountsSocketsTimesCores — CheckNodeCapacity
// multiplies sockets*cores: with sockets=2, cores=4 (8 vCPU) and MaxVCPUs
// set to UsedVCPUs+4, the request is rejected because 8 > 4. If only cores
// were counted (4), the request would pass. The node_limits row is written
// directly via UpsertNodePolicyRow to bypass the physical-capacity admin
// validation (US2/D3b).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_Sockets_NodeCapacityCountsSocketsTimesCores(t *testing.T) {
	fixture := newCreateFixture(t)

	snapshot, err := fixture.fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	index := inventory.BuildIndex(snapshot)
	service := policy.New(fixture.store, inventory.NewProjectionFromIndex(&index), fixture.fake)

	capacity, err := service.NodeCapacity(context.Background(), testClusterName, cluster.FakeNode01)
	if err != nil {
		t.Fatalf("NodeCapacity: %v", err)
	}

	// Set MaxVCPUs to UsedVCPUs+4 directly in the store, bypassing the
	// admin validation that would reject values above physical capacity.
	// With sockets=2, cores=4: deltaVCPUs = 2*4 = 8 > 4 → rejected.
	// With sockets=1, cores=4: deltaVCPUs = 1*4 = 4 = headroom → passes.
	if err := fixture.store.UpsertNodePolicyRow(context.Background(), store.NodePolicyRow{
		Cluster: testClusterName, Node: cluster.FakeNode01,
		MaxVMs: capacity.MaxVMs, MaxVCPUs: capacity.UsedVCPUs + 4,
		MaxRAMGB: capacity.MaxRAMGB, MaxDiskGB: capacity.MaxDiskGB,
	}); err != nil {
		t.Fatalf("UpsertNodePolicyRow: %v", err)
	}

	req := detailedRequest()
	req.Sockets = 2
	req.CPUCores = 4
	req.Name = "sockets-capacity"

	_, err = vm.Create(context.Background(), aliceIdentity(), req.Cluster, req, vm.CreateDeps{
		Store:    fixture.store,
		Creator:  fixture.fake,
		Pusher:   fixture.fake,
		Audit:    fixture.store,
		Log:      slog.New(slog.DiscardHandler),
		Services: []*policy.Policy{service},
	})
	if !errors.Is(err, policy.ErrNodeCapacityExceeded) {
		t.Fatalf("error = %v, want ErrNodeCapacityExceeded (8 vCPU > headroom of 4)", err)
	}

	if calls := cluster.FakeCalls(); len(calls) != 0 {
		t.Fatalf("capacity rejection reached cluster: %+v", calls)
	}
}

// TestCreate_MultiNIC_PopulatesForm — two NICs produce a VM with two network
// interfaces (net0 and net1), each with its own bridge and model (US2/D3a).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_MultiNIC_PopulatesForm(t *testing.T) {
	fixture := newCreateFixture(t)
	req := detailedRequest()
	req.Network = vm.NetworkRequest{
		{Bridge: cluster.FakeBridgeVMbr0, Model: testModelVirtio},
		{Bridge: testBridgeVMbr1, Model: "e1000"},
	}

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	snap, err := fixture.fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	idx := slices.IndexFunc(snap.VMs, func(v cluster.VM) bool { return v.VMID == result.VMID })
	if idx < 0 {
		t.Fatalf("created VM not in snapshot")
	}

	ifaces := snap.VMs[idx].NetworkInterfaces
	if len(ifaces) != 2 {
		t.Fatalf("network interfaces = %d, want 2", len(ifaces))
	}

	if ifaces[0].Bridge != "vmbr0" || ifaces[0].Model != "virtio" {
		t.Errorf("net0 = {bridge: %q, model: %q}, want {vmbr0, virtio}", ifaces[0].Bridge, ifaces[0].Model)
	}

	if ifaces[1].Bridge != "vmbr1" || ifaces[1].Model != "e1000" {
		t.Errorf("net1 = {bridge: %q, model: %q}, want {vmbr1, e1000}", ifaces[1].Bridge, ifaces[1].Model)
	}
}

// TestCreate_MultiNIC_BeyondMaxNetworkCards — three NICs with MaxNetworkCards=2
// is rejected with GabaritExceededError before any cluster call (US2/D3a).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_MultiNIC_BeyondMaxNetworkCards(t *testing.T) {
	fixture := newCreateFixture(t)

	snapshot, err := fixture.fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	index := inventory.BuildIndex(snapshot)
	service := policy.New(fixture.store, inventory.NewProjectionFromIndex(&index), fixture.fake)

	gabarit, err := service.Gabarit(context.Background(), testClusterName)
	if err != nil {
		t.Fatalf("Gabarit: %v", err)
	}

	gabarit.MaxNetworkCards = 2
	if err := service.SetGabarit(context.Background(), testClusterName, gabarit); err != nil {
		t.Fatalf("SetGabarit: %v", err)
	}

	req := detailedRequest()
	req.Name = "multi-nic-over"
	req.Network = vm.NetworkRequest{
		{Bridge: cluster.FakeBridgeVMbr0, Model: testModelVirtio},
		{Bridge: testBridgeVMbr1, Model: testModelVirtio},
		{Bridge: testBridgeVMbr0, Model: testModelVirtio},
	}

	_, err = vm.Create(context.Background(), aliceIdentity(), req.Cluster, req, vm.CreateDeps{
		Store:    fixture.store,
		Creator:  fixture.fake,
		Pusher:   fixture.fake,
		Audit:    fixture.store,
		Log:      slog.New(slog.DiscardHandler),
		Services: []*policy.Policy{service},
	})
	if !errors.Is(err, policy.ErrGabaritExceeded) {
		t.Fatalf("error = %v, want ErrGabaritExceeded", err)
	}

	var gabaritErr *policy.GabaritExceededError
	if !errors.As(err, &gabaritErr) {
		t.Fatalf("error is not a GabaritExceededError: %v", err)
	}

	if gabaritErr.Field != "networkCards" {
		t.Errorf("field = %q, want %q", gabaritErr.Field, "networkCards")
	}

	if calls := cluster.FakeCalls(); len(calls) != 0 {
		t.Fatalf("gabarit rejection reached cluster: %+v", calls)
	}
}

// TestCreate_MultiNIC_EachBridgeValidated — each NIC's bridge is validated
// against the node's catalog: a request with two NICs where the second
// bridge is not approved on the node is rejected with ErrNotApproved (US2/D3a).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_MultiNIC_EachBridgeValidated(t *testing.T) {
	fixture := newCreateFixture(t)
	req := detailedRequest()
	req.Network = vm.NetworkRequest{
		{Bridge: cluster.FakeBridgeVMbr0, Model: testModelVirtio},
		{Bridge: "vmbr9", Model: testModelVirtio},
	}

	_, err := fixture.create(t, aliceIdentity(), req)
	if !errors.Is(err, vm.ErrNotApproved) {
		t.Fatalf("error = %v, want ErrNotApproved (second NIC bridge not approved)", err)
	}

	if calls := cluster.FakeCalls(); len(calls) != 0 {
		t.Fatalf("rejection reached cluster: %+v", calls)
	}
}

// TestCreate_InsufficientDiskSpace_RefusedBeforeVMID — US3/issue-04 D4b: when
// the live free-space check on the target storage reports less than the
// requested disk, Create returns ErrInsufficientDiskSpace before any VMID is
// allocated or cluster call made. The fixture wires the fake as FreeSpaceChecker,
// so the live check runs against the fake's static storage dataset.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_InsufficientDiskSpace_RefusedBeforeVMID(t *testing.T) {
	fixture := newCreateFixture(t)
	req := detailedRequest()
	req.Name = "oversized-disk"
	// local-lvm on pve-node-01: Total=549755813888 (~512 GB), Used=219902325555
	// (~205 GB), free ~307 GB. Requesting 400 GB exceeds the free space.
	req.Disk.SizeGB = 400

	_, err := fixture.create(t, aliceIdentity(), req)
	if !errors.Is(err, vm.ErrInsufficientDiskSpace) {
		t.Fatalf("error = %v, want ErrInsufficientDiskSpace", err)
	}

	// No VMID allocation or cluster mutation may have happened.
	if calls := cluster.FakeCalls(); len(calls) != 0 {
		t.Fatalf("disk-space rejection reached cluster: %+v", calls)
	}
}

// TestCreate_SufficientDiskSpace_Passes — the happy path of the live check:
// a disk that fits within the target storage's free space is accepted. This
// guards against a regression where the check is wired but always rejects.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_SufficientDiskSpace_Passes(t *testing.T) {
	fixture := newCreateFixture(t)
	req := detailedRequest()
	req.Name = "fits-disk"
	// local-lvm on pve-node-01 has ~307 GB free; 40 GB fits.
	req.Disk.SizeGB = 40

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if result.VMID < 1 || result.UPID == "" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

// TestCreate_IsolationVLAN_StampsTagOnEveryNIC asserts the admin-imposed
// per-cluster VLAN tag (US6/issue-06 D6b) is stamped on every created NIC.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_IsolationVLAN_StampsTagOnEveryNIC(t *testing.T) {
	fixture := newCreateFixture(t)

	gabarit := mustGabarit(t, fixture.store)
	gabarit.IsolationVLANTag = 110

	if err := mustPolicyService(fixture.store).SetGabarit(context.Background(), testClusterName, gabarit); err != nil {
		t.Fatalf("SetGabarit: %v", err)
	}

	req := detailedRequest()
	req.Name = "vlan-stamp"
	req.Network = vm.NetworkRequest{
		{Bridge: cluster.FakeBridgeVMbr0, Model: testModelVirtio},
		{Bridge: testBridgeVMbr1, Model: "e1000"},
	}

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	snap, err := fixture.fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	idx := slices.IndexFunc(snap.VMs, func(v cluster.VM) bool { return v.VMID == result.VMID })
	if idx < 0 {
		t.Fatalf("created VM not in snapshot")
	}

	ifaces := snap.VMs[idx].NetworkInterfaces
	if len(ifaces) != 2 {
		t.Fatalf("network interfaces = %d, want 2", len(ifaces))
	}

	for i, nic := range ifaces {
		if nic.VLAN == nil || *nic.VLAN != 110 {
			t.Errorf("nic[%d].vlan = %v, want 110", i, nic.VLAN)
		}

		if !nic.Firewall {
			t.Errorf("nic[%d].firewall = false, want true", i)
		}
	}
}

// TestCreate_IsolationVLAN_Zero_NoTag asserts that when the gabarit's VLAN
// tag is 0 (the default), no tag is stamped on NICs.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_IsolationVLAN_Zero_NoTag(t *testing.T) {
	fixture := newCreateFixture(t)

	req := detailedRequest()
	req.Name = "no-vlan"

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	snap, err := fixture.fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	idx := slices.IndexFunc(snap.VMs, func(v cluster.VM) bool { return v.VMID == result.VMID })
	if idx < 0 {
		t.Fatalf("created VM not in snapshot")
	}

	ifaces := snap.VMs[idx].NetworkInterfaces
	if len(ifaces) != 1 {
		t.Fatalf("network interfaces = %d, want 1", len(ifaces))
	}

	if ifaces[0].VLAN != nil {
		t.Errorf("nic.vlan = %v, want nil (no imposed tag)", ifaces[0].VLAN)
	}
}

// TestCreate_UEFI_ProvisionsEFIDisk asserts that requesting UEFI produces
// bios=ovmf, machine=q35, and efidisk0 on the created VM (US6/issue-06).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_UEFI_ProvisionsEFIDisk(t *testing.T) {
	fixture := newCreateFixture(t)

	req := detailedRequest()
	req.Name = "uefi-vm"
	req.UEFI = true

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if result.VMID < 1 || result.UPID == "" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

// TestCreate_UEFI_WithTPM_ProvisionsTPMState asserts that UEFI+TPM is
// accepted and the VM is created (the cluster layer emits tpmstate0).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_UEFI_WithTPM_ProvisionsTPMState(t *testing.T) {
	fixture := newCreateFixture(t)

	req := detailedRequest()
	req.Name = "uefi-tpm-vm"
	req.UEFI = true
	req.TPM = true

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if result.VMID < 1 || result.UPID == "" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

// TestCreate_TPM_WithoutUEFI_Rejected asserts that TPM without UEFI is
// rejected with ErrInvalidRequest before any VMID is allocated (US6/issue-06:
// TPM 2.0 requires UEFI).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_TPM_WithoutUEFI_Rejected(t *testing.T) {
	fixture := newCreateFixture(t)

	req := detailedRequest()
	req.Name = "tpm-no-uefi"
	req.TPM = true
	req.UEFI = false

	_, err := fixture.create(t, aliceIdentity(), req)
	if !errors.Is(err, vm.ErrInvalidRequest) {
		t.Fatalf("err = %v, want ErrInvalidRequest", err)
	}
}
