package vm_test

import (
	"context"
	"errors"
	"log/slog"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"slices"
	"testing"
)

// templateRequest is a fully explicit, catalog-valid detailed-mode request
// that clones from an approved Proxmox template (US2/issue-02). The fixture
// (newCreateFixture) approves VMID 9000 on pve-node-02 (cloud-init capable,
// 8 GB disk) and VMID 9001 on pve-node-02 (not cloud-init capable, 2 GB
// disk).
const discoveryNode01 = "pve-node-01"

func templateRequest(templateVMID int) vm.CreateRequest {
	return vm.CreateRequest{
		Cluster:    testClusterName,
		Name:       "clone-01",
		TemplateID: templateVMID,
		CPUCores:   2,
		MemoryMB:   4096,
		Disk:       vm.DiskRequest{SizeGB: 20},
		Network:    vm.NetworkRequest{{Bridge: cluster.FakeBridgeVMbr0, Model: testModelVirtio}},
	}
}

// findCloneCall returns the recorded fake clone call for a VMID, or nil.
func findCloneCall(vmid int) *cluster.FakeCall {
	for _, c := range cluster.FakeCalls() {
		if c.VMID == vmid && c.Action == "clone" {
			return &c
		}
	}

	return nil
}

// TestCreate_TemplateClone_RejectsMutualExclusion — US2/issue-02 D2a: a
// request carrying both an ISO and a templateId is rejected with
// ErrInvalidSource before any VMID is allocated.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_TemplateClone_RejectsMutualExclusion(t *testing.T) {
	fixture := newCreateFixture(t)
	req := templateRequest(9000)
	req.ISO = &vm.ISORequest{Storage: cluster.FakeStorageLocal, File: "debian-12-generic-amd64.iso"}

	_, err := fixture.create(t, aliceIdentity(), req)
	if !errors.Is(err, vm.ErrInvalidSource) {
		t.Fatalf("error = %v, want ErrInvalidSource", err)
	}

	if calls := cluster.FakeCalls(); len(calls) != 0 {
		t.Fatalf("rejected request reached the cluster: %+v", calls)
	}
}

// TestCreate_TemplateClone_RejectsUnknownTemplate — US2/issue-02: an
// unapproved template VMID is rejected with ErrNotApproved before any VMID
// is allocated.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_TemplateClone_RejectsUnknownTemplate(t *testing.T) {
	fixture := newCreateFixture(t)
	req := templateRequest(9999)

	_, err := fixture.create(t, aliceIdentity(), req)
	if !errors.Is(err, vm.ErrNotApproved) {
		t.Fatalf("error = %v, want ErrNotApproved", err)
	}

	if calls := cluster.FakeCalls(); len(calls) != 0 {
		t.Fatalf("rejected request reached the cluster: %+v", calls)
	}
}

// TestCreate_TemplateClone_RejectsDisabledTemplate — US2/issue-02: a
// disabled template is rejected with ErrNotApproved before any VMID is
// allocated.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_TemplateClone_RejectsDisabledTemplate(t *testing.T) {
	fixture := newCreateFixture(t)

	if err := fixture.store.SetTemplateEnabled(context.Background(), testClusterName, 9000, false); err != nil {
		t.Fatalf("disable template: %v", err)
	}

	req := templateRequest(9000)

	_, err := fixture.create(t, aliceIdentity(), req)
	if !errors.Is(err, vm.ErrNotApproved) {
		t.Fatalf("error = %v, want ErrNotApproved", err)
	}

	if calls := cluster.FakeCalls(); len(calls) != 0 {
		t.Fatalf("rejected request reached the cluster: %+v", calls)
	}
}

// TestCreate_TemplateClone_RejectsDiskReduction — US2/issue-02 D2c: a
// requested disk size smaller than the template's disk is rejected with
// ErrDiskReduction before any VMID is allocated.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_TemplateClone_RejectsDiskReduction(t *testing.T) {
	fixture := newCreateFixture(t)
	req := templateRequest(9000)
	req.Disk.SizeGB = 4 // template disk is 8 GB

	_, err := fixture.create(t, aliceIdentity(), req)
	if !errors.Is(err, vm.ErrDiskReduction) {
		t.Fatalf("error = %v, want ErrDiskReduction", err)
	}

	if calls := cluster.FakeCalls(); len(calls) != 0 {
		t.Fatalf("rejected request reached the cluster: %+v", calls)
	}
}

// TestCreate_TemplateClone_ProfileDiskBelowTemplateStillRejected — US2/issue-02
// D2c: a forged request carrying both a profileId (whose DiskGB is smaller
// than the template's disk) and a templateId must still be rejected with
// ErrDiskReduction. The reduction check runs after planCreate (which applies
// the profile's DiskGB), so the profile override cannot bypass the guard.
// Without the post-planCreate check, the profile's DiskGB=4 would silently
// override the request's SizeGB and the reduction would go undetected until
// the clone's resize step (which only enlarges, never reduces).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_TemplateClone_ProfileDiskBelowTemplateStillRejected(t *testing.T) {
	fixture := newCreateFixture(t)

	// Insert a profile with DiskGB=4, below template 9000's 8 GB disk.
	if err := fixture.store.InsertProfile(context.Background(), testClusterName, "tiny",
		store.ProfileValues{Label: "Tiny (1 vCPU, 1 GB, 4 GB)", CPUCores: 1, MemoryMB: 1024, DiskGB: 4, Bus: "scsi"}); err != nil {
		t.Fatalf("InsertProfile: %v", err)
	}

	req := templateRequest(9000)
	req.ProfileID = "tiny"
	req.Disk.SizeGB = 20 // above template — would pass the pre-plan check

	_, err := fixture.create(t, aliceIdentity(), req)
	if !errors.Is(err, vm.ErrDiskReduction) {
		t.Fatalf("error = %v, want ErrDiskReduction (profile DiskGB=4 < template 8 GB)", err)
	}

	if calls := cluster.FakeCalls(); len(calls) != 0 {
		t.Fatalf("rejected request reached the cluster: %+v", calls)
	}
}

// TestCreate_TemplateClone_OverridesNodeToTemplateNode — US2/issue-02 D2b:
// a forged request that names a different node is overridden — the clone
// stays on the template's node.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_TemplateClone_OverridesNodeToTemplateNode(t *testing.T) {
	fixture := newCreateFixture(t)
	req := templateRequest(9000)
	req.Node = cluster.FakeNode01 // template is on pve-node-02

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// The clone must land on the template's node (pve-node-02), not the
	// client-supplied pve-node-01.
	if result.Node != cluster.FakeNode02 {
		t.Errorf("result.Node = %q, want %q (template's node)", result.Node, cluster.FakeNode02)
	}

	call := findCloneCall(result.VMID)
	if call == nil {
		t.Fatalf("no clone call recorded for VMID %d", result.VMID)
	}

	if call.Node != cluster.FakeNode02 {
		t.Errorf("clone call Node = %q, want %q", call.Node, cluster.FakeNode02)
	}
}

// TestCreate_TemplateClone_CloudInitTemplateForcesFullClone — US2/issue-02
// §5: a cloud-init capable template is always cloned fully (lvmthin cannot
// linked-clone an imported disk).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_TemplateClone_CloudInitTemplateForcesFullClone(t *testing.T) {
	fixture := newCreateFixture(t)
	req := templateRequest(9000) // VMID 9000 is cloud-init capable

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if result.UPID == "" {
		t.Fatalf("no UPID returned")
	}

	// The cloned VM should appear in the snapshot after the task completes
	// (waitCreateTask polls until TaskOK).
	snap, err := fixture.fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	idx := slices.IndexFunc(snap.VMs, func(v cluster.VM) bool { return v.VMID == result.VMID })
	if idx < 0 {
		t.Fatalf("cloned VM %d not in snapshot", result.VMID)
	}
}

// TestCreate_TemplateClone_NonCloudInitTemplateAllowsLinkedClone — US2/issue-02
// §5: a non-cloud-init template with matching storage allows a linked clone.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_TemplateClone_NonCloudInitTemplateAllowsLinkedClone(t *testing.T) {
	fixture := newCreateFixture(t)
	req := templateRequest(9001) // VMID 9001 is not cloud-init capable, disk on "local"

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if result.UPID == "" {
		t.Fatalf("no UPID returned")
	}

	if result.Node != cluster.FakeNode02 {
		t.Errorf("result.Node = %q, want %q", result.Node, cluster.FakeNode02)
	}
}

// TestCreate_TemplateClone_DiskEnlargementAfterTask — US2/issue-02 D2c:
// a requested disk size larger than the template's disk is applied after
// the clone task completes (via Writer.ResizeDisk).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_TemplateClone_DiskEnlargementAfterTask(t *testing.T) {
	fixture := newCreateFixture(t)
	req := templateRequest(9000) // template disk is 8 GB
	req.Disk.SizeGB = 20         // request 20 GB → enlarge after clone

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// The clone must have completed (waitCreateTask polled until TaskOK)
	// and the VM must be in the snapshot.
	snap, err := fixture.fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	idx := slices.IndexFunc(snap.VMs, func(v cluster.VM) bool { return v.VMID == result.VMID })
	if idx < 0 {
		t.Fatalf("cloned VM %d not in snapshot after task wait", result.VMID)
	}
}

// TestCreate_TemplateClone_NoDiskSizeUsesTemplateSize — US2/issue-02: a
// request with no explicit disk size (SizeGB=0) defaults to the template's
// disk size. The clone succeeds, no resize is invoked (the plan disk equals
// the template disk), and no out-of-range error is raised.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_TemplateClone_NoDiskSizeUsesTemplateSize(t *testing.T) {
	fixture := newCreateFixture(t)
	req := templateRequest(9000)
	req.Disk.SizeGB = 0 // omit → default to template's 8 GB

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if result.UPID == "" {
		t.Fatalf("no UPID returned")
	}

	if result.CloudInitPushError != "" {
		t.Errorf("CloudInitPushError = %q, want empty", result.CloudInitPushError)
	}

	// The cloned VM must exist in the snapshot (task completed).
	snap, err := fixture.fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	idx := slices.IndexFunc(snap.VMs, func(v cluster.VM) bool { return v.VMID == result.VMID })
	if idx < 0 {
		t.Fatalf("cloned VM %d not in snapshot", result.VMID)
	}

	// No resize should have been called — the plan disk equals the template
	// disk (8 GB), so applyPostCloneConfig skips the resize step.
	for _, c := range cluster.FakeCalls() {
		if c.VMID == result.VMID && c.Action == "resize" {
			t.Errorf("unexpected resize call for VMID %d (disk should equal template size)", result.VMID)
		}
	}
}

// TestCreate_TemplateClone_SimpleModeMinimalRequest — the simple-mode
// wizard sends only cluster, name, templateId, and startAfterCreate (no
// cpuCores, memoryMB, sockets, disk, or network). The clone must inherit
// the template's hardware: checkTechnicalRange must not reject the zero
// values, and UpdateHardware must not be called (which would shrink the
// clone to the minimum). Regression for the HTTP 400 out_of_range that
// blocked template-based creation from the web UI.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_TemplateClone_SimpleModeMinimalRequest(t *testing.T) {
	fixture := newCreateFixture(t)
	req := vm.CreateRequest{
		Cluster:          testClusterName,
		Name:             "clone-01",
		TemplateID:       9000,
		StartAfterCreate: true,
	}

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if result.UPID == "" {
		t.Fatalf("no UPID returned")
	}

	if result.Node != cluster.FakeNode02 {
		t.Errorf("result.Node = %q, want %q", result.Node, cluster.FakeNode02)
	}

	// The clone must not have been hit by a hardware override — the
	// template's CPU/memory are inherited, not shrunk to the minimum.
	for _, c := range cluster.FakeCalls() {
		if c.VMID == result.VMID && c.Action == actionUpdateHW {
			t.Errorf("unexpected update_hardware call for VMID %d (simple mode should inherit template hardware)", result.VMID)
		}
	}

	// The pvmss tag must still be stamped via SetTags — without it, the
	// clone is invisible to PVMSS (FR-006: Resolve returns ErrNotFound).
	snap, _ := fixture.fake.Snapshot(context.Background())

	idx := slices.IndexFunc(snap.VMs, func(v cluster.VM) bool { return v.VMID == result.VMID })
	if idx < 0 {
		t.Fatalf("cloned VM %d not in snapshot", result.VMID)
	}

	if !slices.Contains(snap.VMs[idx].Tags, "pvmss") {
		t.Errorf("cloned VM %d tags = %v, want pvmss tag present (FR-006)", result.VMID, snap.VMs[idx].Tags)
	}
}

// TestCreate_TemplateClone_CloudInitAppliedAfterTask — lifecycle-04: when
// a cloud-init template is requested alongside a Proxmox template clone,
// the cloud-init snippet is attached after the clone task completes.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_TemplateClone_CloudInitAppliedAfterTask(t *testing.T) {
	fixture := newCreateFixture(t)
	tmplID := createTestTemplate(t, fixture.store)

	req := templateRequest(9000)
	req.CloudInitTemplateID = tmplID

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// The clone must have completed before cloud-init was attached — the
	// VM exists in the snapshot (materialized by the fake's onComplete
	// callback on the third poll).
	snap, err := fixture.fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	idx := slices.IndexFunc(snap.VMs, func(v cluster.VM) bool { return v.VMID == result.VMID })
	if idx < 0 {
		t.Fatalf("cloned VM %d not in snapshot — cloud-init may have been attached before task completion", result.VMID)
	}

	// The cloud-init snippet should be persisted.
	snippet, found, err := fixture.store.GetCloudInitSnippet(context.Background(), testClusterName, result.VMID)
	if err != nil {
		t.Fatalf("GetCloudInitSnippet: %v", err)
	}

	if !found {
		t.Errorf("cloud-init snippet not persisted for VMID %d", result.VMID)
	}

	if snippet.Content != testCloudInitContent {
		t.Errorf("snippet content mismatch")
	}

	if result.CloudInitTemplateID != tmplID {
		t.Errorf("result.CloudInitTemplateID = %q, want %q", result.CloudInitTemplateID, tmplID)
	}
}

// TestCreate_TemplateClone_StartAfterCreateWithCloudInit — lifecycle-04:
// when a cloud-init template is requested with StartAfterCreate, the VM is
// not started in the clone task (StartAfterCreate is forced off for the
// clone); the VM is started explicitly after cloud-init attachment.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_TemplateClone_StartAfterCreateWithCloudInit(t *testing.T) {
	fixture := newCreateFixture(t)
	tmplID := createTestTemplate(t, fixture.store)

	req := templateRequest(9000)
	req.CloudInitTemplateID = tmplID
	req.StartAfterCreate = true

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if result.CloudInitPushError != "" {
		t.Errorf("CloudInitPushError = %q, want empty", result.CloudInitPushError)
	}

	// The VM should be in the snapshot and started (the explicit start
	// after cloud-init attachment runs via Writer.Action).
	snap, err := fixture.fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	idx := slices.IndexFunc(snap.VMs, func(v cluster.VM) bool { return v.VMID == result.VMID })
	if idx < 0 {
		t.Fatalf("cloned VM %d not in snapshot", result.VMID)
	}

	// The fake's Action("start") transitions the VM to running.
	if snap.VMs[idx].Status != cluster.VMRunning {
		t.Errorf("VM status = %v, want VMRunning (started after cloud-init)", snap.VMs[idx].Status)
	}
}

// TestCreate_TemplateClone_AuditRecorded — FR-017: a successful clone is
// recorded in the audit log.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_TemplateClone_AuditRecorded(t *testing.T) {
	fixture := newCreateFixture(t)
	req := templateRequest(9000)

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	entries, err := fixture.store.QueryAudit(context.Background())
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}

	if len(entries) == 0 {
		t.Fatalf("no audit entries for VMID %d", result.VMID)
	}

	entry := entries[0]
	if entry.Actor != cluster.FakeUserAlice || entry.Cluster != testClusterName || *entry.VMID != result.VMID || entry.Action != "vm_create" {
		t.Errorf("audit entry = %+v", entry)
	}
}

// TestCreate_TemplateClone_CatalogExposesTemplates — US2/issue-02: the
// catalog query returns the approved templates from the seed.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_TemplateClone_CatalogExposesTemplates(t *testing.T) {
	fixture := newCreateFixture(t)

	templates, err := catalog.Templates(context.Background(), fixture.store, testClusterName)
	if err != nil {
		t.Fatalf("catalog.Templates: %v", err)
	}

	if len(templates) != 2 {
		t.Fatalf("expected 2 seeded templates, got %d", len(templates))
	}

	// VMID 9000 — cloud-init capable, 8 GB disk, on pve-node-02.
	tmpl9000, err := catalog.FindTemplate(templates, 9000)
	if err != nil {
		t.Fatalf("FindTemplate(9000): %v", err)
	}

	if tmpl9000.Node != cluster.FakeNode02 {
		t.Errorf("template 9000 node = %q, want %q", tmpl9000.Node, cluster.FakeNode02)
	}

	if !tmpl9000.CloudInitCapable {
		t.Errorf("template 9000 should be cloud-init capable")
	}

	if tmpl9000.DiskSizeGB != 8 {
		t.Errorf("template 9000 disk size = %d, want 8", tmpl9000.DiskSizeGB)
	}

	// VMID 9001 — not cloud-init capable, 2 GB disk, on pve-node-02.
	tmpl9001, err := catalog.FindTemplate(templates, 9001)
	if err != nil {
		t.Fatalf("FindTemplate(9001): %v", err)
	}

	if tmpl9001.CloudInitCapable {
		t.Errorf("template 9001 should not be cloud-init capable")
	}

	if tmpl9001.DiskSizeGB != 2 {
		t.Errorf("template 9001 disk size = %d, want 2", tmpl9001.DiskSizeGB)
	}
}

// TestCreate_TemplateClone_PoolPropagation — FR-004: the cloned VM must
// land in the actor's personal pool, not an arbitrary or empty pool. The
// clone call must carry the pool, and the materialized VM must have it.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_TemplateClone_PoolPropagation(t *testing.T) {
	fixture := newCreateFixture(t)
	req := templateRequest(9000)

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	call := findCloneCall(result.VMID)
	if call == nil {
		t.Fatalf("no clone call recorded for VMID %d", result.VMID)
	}

	if call.Pool != cluster.FakePoolAlice {
		t.Errorf("clone call Pool = %q, want %q", call.Pool, cluster.FakePoolAlice)
	}

	snap, err := fixture.fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	idx := slices.IndexFunc(snap.VMs, func(v cluster.VM) bool { return v.VMID == result.VMID })
	if idx < 0 {
		t.Fatalf("cloned VM %d not in snapshot", result.VMID)
	}

	if snap.VMs[idx].Pool != cluster.FakePoolAlice {
		t.Errorf("cloned VM Pool = %q, want %q", snap.VMs[idx].Pool, cluster.FakePoolAlice)
	}
}

// goneTemplateClient simulates a template deleted in Proxmox after approval
// (issue 02): discovery no longer reports it.
type goneTemplateClient struct {
	cluster.Fake
}

func (goneTemplateClient) TemplateByVMID(_ context.Context, _ int) (cluster.TemplateVM, error) {
	return cluster.TemplateVM{}, cluster.ErrNotFound
}

// migratedTemplateClient simulates a template migrated to another node after
// approval (issue 02): discovery reports pve-node-01, the stored row says
// pve-node-02.
type migratedTemplateClient struct {
	cluster.Fake
}

func (migratedTemplateClient) TemplateByVMID(_ context.Context, _ int) (cluster.TemplateVM, error) {
	return cluster.TemplateVM{
		VMID: 9000, Node: discoveryNode01, Name: "debian-12-cloud", CloudInitCapable: true,
		DiskStorage: "local-lvm", DiskSizeGB: 8, DiskBus: string(cluster.DiskBusSCSI),
	}, nil
}

// unreadableAtCloneClient simulates a template whose config became unreadable
// between approval and clone time (issue 03): the discovered node is kept,
// the disk fields fall back to the stored row.
type unreadableAtCloneClient struct {
	cluster.Fake
}

func (unreadableAtCloneClient) TemplateByVMID(_ context.Context, _ int) (cluster.TemplateVM, error) {
	return cluster.TemplateVM{VMID: 9000, Node: discoveryNode01, Name: "debian-12-cloud", DiskUnreadable: true}, nil
}

// createWithTemplates runs the clone path with an explicit TemplateReader
// (the clone-time freshness backstop's seam).
func createWithTemplates(t *testing.T, fixture createFixture, templates interface {
	TemplateByVMID(ctx context.Context, vmid int) (cluster.TemplateVM, error)
}, req vm.CreateRequest,
) (vm.CreateResult, error) {
	t.Helper()

	log := slog.New(slog.DiscardHandler)

	return vm.Create(context.Background(), aliceIdentity(), req.Cluster, req, vm.CreateDeps{
		Store:     fixture.store,
		Creator:   fixture.fake,
		Pusher:    fixture.fake,
		Writer:    fixture.fake,
		FreeSpace: fixture.fake,
		Snippets:  fixture.fake,
		Audit:     fixture.store,
		Log:       log,
		Templates: templates,
	})
}

// TestCreate_TemplateClone_DeletedTemplateFailsFast — a template deleted in
// Proxmox after approval fails the create fast (ErrNotApproved) before a
// VMID is spent (issue 02/T17).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_TemplateClone_DeletedTemplateFailsFast(t *testing.T) {
	fixture := newCreateFixture(t)

	cluster.ResetFake()

	_, err := createWithTemplates(t, fixture, goneTemplateClient{}, templateRequest(9000))
	if !errors.Is(err, vm.ErrNotApproved) {
		t.Fatalf("err = %v, want ErrNotApproved", err)
	}

	if findCloneCall(9001) != nil {
		t.Error("no clone may be dispatched for a deleted template")
	}
}

// TestCreate_TemplateClone_ClonesOnDiscoveredNode — a template migrated since
// approval is cloned on its new node: discovery wins on values at clone time,
// the stored row keeps only the approval role (T17).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_TemplateClone_ClonesOnDiscoveredNode(t *testing.T) {
	fixture := newCreateFixture(t)

	cluster.ResetFake()

	result, err := createWithTemplates(t, fixture, migratedTemplateClient{}, templateRequest(9000))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if result.Node != discoveryNode01 {
		t.Errorf("clone node = %q, want the discovered node", result.Node)
	}

	call := findCloneCall(result.VMID)
	if call == nil {
		t.Fatal("clone call not recorded")
	}

	if call.Node != discoveryNode01 {
		t.Errorf("clone POST node = %q, want pve-node-01", call.Node)
	}
}

// TestCreate_TemplateClone_UnreadableFallsBackToStoredDisk — when the config
// is unreadable at clone time, the discovered node is kept and the stored
// disk fields (validated at approval) drive the resize floor (T17).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_TemplateClone_UnreadableFallsBackToStoredDisk(t *testing.T) {
	fixture := newCreateFixture(t)

	cluster.ResetFake()

	result, err := createWithTemplates(t, fixture, unreadableAtCloneClient{}, templateRequest(9000))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if result.Node != discoveryNode01 {
		t.Errorf("clone node = %q, want the discovered node", result.Node)
	}

	if findCloneCall(result.VMID) == nil {
		t.Error("clone call not recorded")
	}
}
