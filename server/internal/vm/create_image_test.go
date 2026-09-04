package vm_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"pvmss/server/internal/cluster"
	"pvmss/server/internal/vm"
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
		Storage: "local",
		File:    "ubuntu-24.04-server-cloudimg-amd64.qcow2",
		CloudInit: vm.ImageCloudInitRequest{
			User:    "ubuntu",
			SSHKeys: []string{"ssh-ed25519 AAAA"},
		},
	}

	return req
}

// TestCreate_Image_AppliesCloudInit — the image path delivers cloud-init
// through Proxmox's native keys (SetCloudInitConfig — the REST API cannot
// write a per-VM snippet file), skips the baseline snippet when the admin
// has not placed one, and only then starts the VM.
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

	config, err := cluster.Fake{}.GetCloudInitConfig(context.Background(), cluster.FakeNode01, result.VMID)
	if err != nil {
		t.Fatalf("GetCloudInitConfig: %v", err)
	}

	if config.User != "ubuntu" {
		t.Errorf("config.User = %q, want ubuntu", config.User)
	}

	if len(config.SSHKeys) != 1 || config.SSHKeys[0] != "ssh-ed25519 AAAA" {
		t.Errorf("config.SSHKeys = %v, want [ssh-ed25519 AAAA]", config.SSHKeys)
	}

	setConfigIndex, attachIndex, startIndex := -1, -1, -1

	for i, c := range cluster.FakeCallsFor(result.VMID) {
		switch c.Action {
		case "set_cloudinit_config":
			setConfigIndex = i
		case "attach_cloudinit_snippet":
			attachIndex = i
		case "start":
			startIndex = i
		}
	}

	if setConfigIndex == -1 {
		t.Fatal("SetCloudInitConfig not recorded")
	}

	if attachIndex != -1 {
		t.Errorf("attach_cloudinit_snippet recorded despite no baseline snippet present: index %d", attachIndex)
	}

	if startIndex == -1 || startIndex < setConfigIndex {
		t.Errorf("start action %d did not come after the cloud-init config set %d", startIndex, setConfigIndex)
	}
}

// TestCreate_Image_AttachesBaselineSnippetWhenPresent — when an admin has
// placed the fixed baseline snippet, image-mode create attaches it after the
// native-key config, on top of (not instead of) ciuser/sshkeys.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_Image_AttachesBaselineSnippetWhenPresent(t *testing.T) {
	fixture := newCreateFixture(t)

	cluster.SetFakeSnippetPresent(cluster.FakeNode01, "local", "pvmss-baseline.yml", true)
	t.Cleanup(func() { cluster.SetFakeSnippetPresent(cluster.FakeNode01, "local", "pvmss-baseline.yml", false) })

	req := imageRequest()

	result, err := fixture.create(t, aliceIdentity(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if result.CloudInitPushError != "" {
		t.Errorf("result.CloudInitPushError = %q, want empty", result.CloudInitPushError)
	}

	found := false

	for _, c := range cluster.FakeCallsFor(result.VMID) {
		if c.Action == "attach_cloudinit_snippet" && c.Filename == "pvmss-baseline.yml" {
			found = true
		}
	}

	if !found {
		t.Fatal("baseline snippet attach not recorded")
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
	if err := fixture.store.SetImageEnabled(context.Background(), testClusterName, cluster.FakeNode01, "local", "small.qcow2", 1024*1024*1024, true); err != nil {
		t.Fatalf("seed small image approval: %v", err)
	}

	req := detailedRequest()
	req.Disk.SizeGB = 12
	req.Image = &vm.ImageRequest{Storage: "local", File: "small.qcow2", CloudInit: vm.ImageCloudInitRequest{User: "ubuntu"}}

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

	if err := fixture.store.SetImageEnabled(context.Background(), testClusterName, cluster.FakeNode01, "local", "exact.qcow2", 12*1024*1024*1024, true); err != nil {
		t.Fatalf("seed exact image approval: %v", err)
	}

	req := detailedRequest()
	req.Disk.SizeGB = 12
	req.Image = &vm.ImageRequest{Storage: "local", File: "exact.qcow2", CloudInit: vm.ImageCloudInitRequest{User: "ubuntu"}}

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
				req.ISO = &vm.ISORequest{Storage: "local", File: "debian-12-generic-amd64.iso"}
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
				if c.Action == "create" {
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
	if err := fixture.store.SetImageEnabled(context.Background(), testClusterName, cluster.FakeNode01, "local", "big.qcow2", 10*1024*1024*1024, true); err != nil {
		t.Fatalf("seed big image approval: %v", err)
	}

	req := detailedRequest()
	req.Disk.SizeGB = 4
	req.Image = &vm.ImageRequest{Storage: "local", File: "big.qcow2", CloudInit: vm.ImageCloudInitRequest{User: "ubuntu"}}

	_, err := fixture.create(t, aliceIdentity(), req)
	if !errors.Is(err, vm.ErrDiskBelowImage) {
		t.Fatalf("error = %v, want ErrDiskBelowImage", err)
	}

	for _, c := range cluster.FakeCalls() {
		if c.Action == "create" {
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
	req.Image = &vm.ImageRequest{Storage: "local", File: "unknown.qcow2", CloudInit: vm.ImageCloudInitRequest{User: "ubuntu"}}

	_, err := fixture.create(t, aliceIdentity(), req)
	if !errors.Is(err, vm.ErrNotApproved) {
		t.Fatalf("error = %v, want ErrNotApproved", err)
	}
}
