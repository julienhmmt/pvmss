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
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"slices"
	"testing"
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
		{Name: "vmbr0", Node: "pve-node-01"},
		{Name: "vmbr1", Node: "pve-node-01"},
		{Name: "vmbr0", Node: "pve-node-02"},
	} {
		if err := st.SetBridgeEnabled(ctx, "default", bridge.Node, bridge.Name, true); err != nil {
			t.Fatalf("seed bridge approval: %v", err)
		}
	}

	return createFixture{store: st, fake: cluster.Fake{}}
}

func (f createFixture) create(t *testing.T, actor auth.Identity, req vm.CreateRequest) (vm.CreateResult, error) {
	t.Helper()

	log := slog.New(slog.DiscardHandler)

	return vm.Create(context.Background(), actor, req.Cluster, req, vm.CreateDeps{
		Store:   f.store,
		Creator: f.fake,
		Pusher:  f.fake,
		Audit:   f.store,
		Log:     log,
	})
}

func aliceIdentity() auth.Identity {
	return auth.Identity{Username: cluster.FakeUserAlice, Pool: cluster.FakePoolAlice}
}

func bobIdentity() auth.Identity {
	return auth.Identity{Username: cluster.FakeUserBob, Pool: cluster.FakePoolBob}
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
		Network:  vm.NetworkRequest{Bridge: cluster.FakeBridgeVMbr0, Model: string(cluster.DiskBusVirtio)},
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
			name:    "identity without pool",
			actor:   auth.Identity{Username: "admin@pve", IsAdmin: true},
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
			mutate:  func(r *vm.CreateRequest) { r.Network.Bridge = "vmbr9" },
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
	if entry.Actor != cluster.FakeUserAlice || entry.Cluster != testClusterName || entry.VMID != result.VMID || entry.Action != "vm_create" {
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
