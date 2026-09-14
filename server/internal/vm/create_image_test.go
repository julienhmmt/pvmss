package vm_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/vm"
	"slices"
	"strings"
	"testing"
)

// TestCreate_Image_ZeroHardwareDefaultsToImageDefaults — a cloud-image
// request with no profile and no explicit hardware defaults to
// imageDefault{CPUCores,MemoryMB,DiskGB} (1 vCPU/2048 MB/12 GB), not the
// shared technical minimum (1 vCPU/128 MB) — a cloud image needs real
// headroom to boot.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_Image_ZeroHardwareDefaultsToImageDefaults(t *testing.T) {
	fixture := newCreateFixture(t)

	req := imageRequest()
	req.CPUCores = 0
	req.MemoryMB = 0
	req.Sockets = 0
	req.Disk = vm.DiskRequest{}

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
	if created.CPUCores != 1 {
		t.Errorf("cpuCores = %d, want 1", created.CPUCores)
	}

	if created.MemoryTotal != 2048*1024*1024 {
		t.Errorf("memory = %d, want 2048 MB", created.MemoryTotal)
	}

	if created.DiskTotal != 12*1024*1024*1024 {
		t.Errorf("disk = %d, want 12 GB", created.DiskTotal)
	}
}

// TestCreate_Image_ProfileResolvesHardware — FR-009 applies to image mode
// too: a profile's catalog values (CPU/memory/disk/bus) win over any
// hardware fields the request also carries, same as template and ISO mode.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_Image_ProfileResolvesHardware(t *testing.T) {
	fixture := newCreateFixture(t)

	req := imageRequest()
	req.ProfileID = "medium"
	req.CPUCores = 32 // contradictory — must be ignored
	req.MemoryMB = 65536
	req.Disk = vm.DiskRequest{SizeGB: 2048}

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

// imageRequest is a catalog-valid cloud-image creation request (the seeded
// catalog_images row: ubuntu-24.04-server-cloudimg-amd64.qcow2 on
// pve-node-01/local).
func imageRequest() vm.CreateRequest {
	req := detailedRequest()
	req.Image = &vm.ImageRequest{
		Storage: testStorageLocal,
		File:    "ubuntu-24.04-server-cloudimg-amd64.qcow2",
		CloudInit: vm.ImageCloudInitRequest{
			User:    testUserUbuntu,
			SSHKeys: []string{"ssh-ed25519 AAAA"},
		},
	}

	return req
}

// TestCreate_Image_AppliesCloudInit — the image path delivers cloud-init
// through Proxmox's native keys (SetCloudInitConfig), then pushes the
// generated baseline as a per-VM snippet and attaches it as vendor-data
// (issue 03). The baseline state is "applied" when no cluster-wide override
// is present.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_Image_AppliesCloudInit(t *testing.T) {
	fixture := newCreateFixture(t)

	req := imageRequest()
	req.StartAfterCreate = true

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if result.CloudInitPushError != "" {
		t.Errorf("result.CloudInitPushError = %q, want empty", result.CloudInitPushError)
	}

	if result.BaselineState != "applied" {
		t.Errorf("result.BaselineState = %q, want 'applied'", result.BaselineState)
	}

	config, err := cluster.Fake{}.GetCloudInitConfig(context.Background(), cluster.FakeNode01, result.VMID)
	if err != nil {
		t.Fatalf("GetCloudInitConfig: %v", err)
	}

	if config.User != testUserUbuntu {
		t.Errorf("config.User = %q, want ubuntu", config.User)
	}

	if len(config.SSHKeys) != 1 || config.SSHKeys[0] != "ssh-ed25519 AAAA" {
		t.Errorf("config.SSHKeys = %v, want [ssh-ed25519 AAAA]", config.SSHKeys)
	}

	index := fakeCallIndexes(result.VMID, "set_cloudinit_config", "push_cloudinit_snippet", testActionAttachCloudInitSnippet, "start")

	if index["set_cloudinit_config"] == -1 {
		t.Fatal("SetCloudInitConfig not recorded")
	}

	if index["push_cloudinit_snippet"] == -1 {
		t.Error("push_cloudinit_snippet not recorded — generated baseline should be pushed")
	}

	if index[testActionAttachCloudInitSnippet] == -1 {
		t.Error("attach_cloudinit_snippet not recorded — generated baseline should be attached")
	}

	if index["start"] == -1 || index["start"] < index[testActionAttachCloudInitSnippet] {
		t.Errorf("start action %d did not come after the snippet attach %d", index["start"], index[testActionAttachCloudInitSnippet])
	}
}

// fakeCallIndexes returns the recorded-call index of each named fake action
// for vmid (the last occurrence wins), or -1 for actions never recorded.
func fakeCallIndexes(vmid int, actions ...string) map[string]int {
	index := make(map[string]int, len(actions))
	for _, action := range actions {
		index[action] = -1
	}

	for i, call := range cluster.FakeCallsFor(vmid) {
		if _, ok := index[call.Action]; ok {
			index[call.Action] = i
		}
	}

	return index
}

// snippetPushFor returns the content of the per-VM pvmss-<vmid>.yml snippet
// push recorded by the fake writer (empty when no push was recorded) and
// whether the attach call for the same filename was recorded.
func snippetPushFor(vmid int) (content string, attached bool) {
	snippetName := fmt.Sprintf("pvmss-%d.yml", vmid)

	for _, c := range cluster.FakeCallsFor(vmid) {
		if c.Action == "push_cloudinit_snippet" && c.Filename == snippetName {
			content = c.Content
		}

		if c.Action == testActionAttachCloudInitSnippet && c.Filename == snippetName {
			attached = true
		}
	}

	return content, attached
}

// TestCreate_Image_AttachesBaselineSnippetWhenPresent — when an admin has
// placed a cluster-wide pvmss-baseline.yml, its content replaces the
// generated baseline (issue 03): the merged document is pushed as
// pvmss-<vmid>.yml and attached as vendor-data. BaselineState is "override".
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_Image_AttachesBaselineSnippetWhenPresent(t *testing.T) {
	fixture := newCreateFixture(t)

	cluster.SetFakeSnippetContent(cluster.FakeNode01, testStorageLocal, "pvmss-baseline.yml", "#cloud-config\npackages:\n  - nmap\n")
	t.Cleanup(func() {
		cluster.SetFakeSnippetPresent(cluster.FakeNode01, testStorageLocal, "pvmss-baseline.yml", false)
	})

	req := imageRequest()

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if result.CloudInitPushError != "" {
		t.Errorf("result.CloudInitPushError = %q, want empty", result.CloudInitPushError)
	}

	if result.BaselineState != "override" {
		t.Errorf("result.BaselineState = %q, want 'override'", result.BaselineState)
	}

	// The per-VM snippet is pushed and attached (not the cluster-wide file).
	pushedContent, attached := snippetPushFor(result.VMID)

	if pushedContent == "" {
		t.Error("per-VM snippet push not recorded")
	}

	if !attached {
		t.Error("per-VM snippet attach not recorded")
	}

	// The override content (nmap) should be in the pushed document,
	// not the generated baseline (qemu-guest-agent).
	if !strings.Contains(pushedContent, "nmap") {
		t.Errorf("pushed snippet does not contain override content: %s", pushedContent)
	}

	if strings.Contains(pushedContent, "qemu-guest-agent") {
		t.Errorf("pushed snippet should not contain generated baseline when override present: %s", pushedContent)
	}
}

// TestCreate_Image_UserDocumentMergesWithBaseline — a user-selected
// cloud-init document is merged on top of the generated baseline (issue 04):
// the user's packages add to the baseline's (qemu-guest-agent), and the
// user's scalar values win. The merged document is the one pushed and
// attached.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_Image_UserDocumentMergesWithBaseline(t *testing.T) {
	fixture := newCreateFixture(t)

	userDoc := "#cloud-config\npackages:\n  - nmap\nruncmd:\n  - echo hello\n"
	fileID := createTestUserFile(t, fixture.store, cluster.FakeUserAlice, "dev-box", userDoc)

	req := imageRequest()
	req.CloudInitFileID = fileID

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if result.CloudInitPushError != "" {
		t.Errorf("result.CloudInitPushError = %q, want empty", result.CloudInitPushError)
	}

	if result.BaselineState != "applied" {
		t.Errorf("result.BaselineState = %q, want 'applied'", result.BaselineState)
	}

	pushedContent, _ := snippetPushFor(result.VMID)

	if pushedContent == "" {
		t.Fatal("per-VM snippet push not recorded")
	}

	// The merged document contains both the baseline's qemu-guest-agent
	// and the user's nmap — packages concatenate (issue 04).
	if !strings.Contains(pushedContent, "qemu-guest-agent") {
		t.Errorf("merged document missing baseline package qemu-guest-agent: %s", pushedContent)
	}

	if !strings.Contains(pushedContent, "nmap") {
		t.Errorf("merged document missing user package nmap: %s", pushedContent)
	}

	// The user's runcmd entry is present.
	if !strings.Contains(pushedContent, "echo hello") {
		t.Errorf("merged document missing user runcmd: %s", pushedContent)
	}
}

// TestCreate_Image_GrowsImportedDisk — import-from lands the disk at the
// source image's size (Proxmox requires the :0 target syntax), so the
// requested size is applied afterwards via ResizeDisk. Skipped when the
// request matches the image size — ResizeDisk only grows.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_Image_GrowsImportedDisk(t *testing.T) {
	fixture := newCreateFixture(t)

	// Seed a 1 GB image approval directly (the create path validates
	// against the catalog, not discovery).
	if err := fixture.store.SetImageEnabled(context.Background(), testClusterName, cluster.FakeNode01, testStorageLocal, "small.qcow2", 1024*1024*1024, true); err != nil {
		t.Fatalf("seed small image approval: %v", err)
	}

	req := detailedRequest()
	req.Disk.SizeGB = 12
	req.Image = &vm.ImageRequest{Storage: testStorageLocal, File: "small.qcow2", CloudInit: vm.ImageCloudInitRequest{User: testUserUbuntu}}

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if result.CloudInitPushError != "" {
		t.Errorf("result.CloudInitPushError = %q, want empty", result.CloudInitPushError)
	}

	var resize *cluster.FakeCall

	for _, c := range cluster.FakeCallsFor(result.VMID) {
		if c.Action == "resize_disk" {
			call := c
			resize = &call
		}
	}

	if resize == nil {
		t.Fatal("ResizeDisk not recorded — the imported disk was never grown")
	}

	if resize.DiskKey != "scsi0" || resize.SizeGB != 12 {
		t.Errorf("resize = %+v, want scsi0 grown to 12 GB", *resize)
	}
}

// TestCreate_Image_NoResizeWhenSizeMatchesImage — a request equal to the
// image's size skips the post-create resize (ResizeDisk only grows).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_Image_NoResizeWhenSizeMatchesImage(t *testing.T) {
	fixture := newCreateFixture(t)

	if err := fixture.store.SetImageEnabled(context.Background(), testClusterName, cluster.FakeNode01, testStorageLocal, "exact.qcow2", 12*1024*1024*1024, true); err != nil {
		t.Fatalf("seed exact image approval: %v", err)
	}

	req := detailedRequest()
	req.Disk.SizeGB = 12
	req.Image = &vm.ImageRequest{Storage: testStorageLocal, File: "exact.qcow2", CloudInit: vm.ImageCloudInitRequest{User: testUserUbuntu}}

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	for _, c := range cluster.FakeCallsFor(result.VMID) {
		if c.Action == "resize_disk" {
			t.Fatalf("unexpected resize for a size matching the image: %+v", c)
		}
	}
}

// TestCreate_Image_SourceMutualExclusion — the three sources are mutually
// exclusive: a request carrying more than one is rejected with
// ErrInvalidSource before any VMID is spent.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_Image_SourceMutualExclusion(t *testing.T) {
	cases := []struct {
		name string
		mut  func(req *vm.CreateRequest)
	}{
		{
			name: "image and iso",
			mut: func(req *vm.CreateRequest) {
				req.ISO = &vm.ISORequest{Storage: testStorageLocal, File: "debian-12-generic-amd64.iso"}
			},
		},
		{
			name: "image and templateId",
			mut:  func(req *vm.CreateRequest) { req.TemplateID = 9000 },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newCreateFixture(t)

			req := imageRequest()
			tc.mut(&req)

			_, err := fixture.create(t, aliceIdentity(), req)
			if !errors.Is(err, vm.ErrInvalidSource) {
				t.Fatalf("error = %v, want ErrInvalidSource", err)
			}

			for _, c := range cluster.FakeCalls() {
				if c.Action == testActionCreate {
					t.Fatalf("a VM was created despite the invalid source: %+v", c)
				}
			}
		})
	}
}

// TestCreate_Image_DiskBelowImage_RejectedBeforeVMID — a disk size below the
// cloud image is refused before a VMID is spent (Proxmox import-from grows
// the disk but never shrinks it).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_Image_DiskBelowImage_RejectedBeforeVMID(t *testing.T) {
	fixture := newCreateFixture(t)

	// Seed a 10 GB image approval directly (the create path validates
	// against the catalog, not discovery).
	if err := fixture.store.SetImageEnabled(context.Background(), testClusterName, cluster.FakeNode01, testStorageLocal, "big.qcow2", 10*1024*1024*1024, true); err != nil {
		t.Fatalf("seed big image approval: %v", err)
	}

	req := detailedRequest()
	req.Disk.SizeGB = 4
	req.Image = &vm.ImageRequest{Storage: testStorageLocal, File: "big.qcow2", CloudInit: vm.ImageCloudInitRequest{User: testUserUbuntu}}

	_, err := fixture.create(t, aliceIdentity(), req)
	if !errors.Is(err, vm.ErrDiskBelowImage) {
		t.Fatalf("error = %v, want ErrDiskBelowImage", err)
	}

	for _, c := range cluster.FakeCalls() {
		if c.Action == testActionCreate {
			t.Fatalf("a VM was created despite the undersized disk: %+v", c)
		}
	}
}

// TestCreate_Image_NotApproved — a cloud image absent from the catalog is
// rejected with ErrNotApproved before any VMID is spent.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_Image_NotApproved(t *testing.T) {
	fixture := newCreateFixture(t)

	req := detailedRequest()
	req.Image = &vm.ImageRequest{Storage: testStorageLocal, File: "unknown.qcow2", CloudInit: vm.ImageCloudInitRequest{User: testUserUbuntu}}

	_, err := fixture.create(t, aliceIdentity(), req)
	if !errors.Is(err, vm.ErrNotApproved) {
		t.Fatalf("error = %v, want ErrNotApproved", err)
	}
}

// TestCreate_Image_NoWriteTarget_SkipsBaseline — image mode delivers
// cloud-init through Proxmox's native keys. A cluster with no snippet write
// target must still create image VMs: the baseline is "not_delivered" with
// the reason, but the create succeeds and the VM starts (issue 03).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_Image_NoWriteTarget_SkipsBaseline(t *testing.T) {
	fixture := newCreateFixture(t)

	req := imageRequest()
	req.StartAfterCreate = true

	result, err := vm.Create(context.Background(), aliceIdentity(), req.Cluster, req, vm.CreateDeps{
		Store: fixture.store, Creator: fixture.fake, Pusher: fixture.fake,
		Writer: fixture.fake, FreeSpace: fixture.fake, Snippets: writeUnavailableSnippetFinder{},
		Audit: fixture.store, Log: slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if result.CloudInitPushError != "" {
		t.Errorf("result.CloudInitPushError = %q, want empty (native keys succeeded)", result.CloudInitPushError)
	}

	if result.BaselineState != "not_delivered" {
		t.Errorf("result.BaselineState = %q, want 'not_delivered'", result.BaselineState)
	}

	if result.BaselineError == "" {
		t.Error("result.BaselineError should record the reason")
	}

	index := fakeCallIndexes(result.VMID, "set_cloudinit_config", testActionAttachCloudInitSnippet, "start")

	if index["set_cloudinit_config"] == -1 {
		t.Fatal("SetCloudInitConfig not recorded")
	}

	if index[testActionAttachCloudInitSnippet] != -1 {
		t.Errorf("attach_cloudinit_snippet recorded without a write target: index %d", index[testActionAttachCloudInitSnippet])
	}

	if index["start"] == -1 {
		t.Error("start not recorded: image VM must still start without a write target")
	}
}
