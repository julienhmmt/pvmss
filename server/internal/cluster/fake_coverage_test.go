package cluster_test

import (
	"context"
	"errors"
	"pvmss/server/internal/cluster"
	"slices"
	"testing"
)

func TestFake_Snapshot_DeepCopyDoesNotMutateOriginal(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-snapshot-deepcopy")
	ctx := context.Background()

	snap1, err := fake.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	if len(snap1.VMs) == 0 {
		t.Fatal("expected VMs in snapshot")
	}

	mutateFirstSnapshotVMFields(snap1)

	snap2, err := fake.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot second: %v", err)
	}

	assertSnapshotHasNoMutatedFields(t, snap2)
}

func mutateFirstSnapshotVMFields(snap cluster.Snapshot) {
	snap.VMs[0].Name = "mutated-name"
	if len(snap.VMs[0].Tags) > 0 {
		snap.VMs[0].Tags[0] = "mutated-tag"
	}

	if len(snap.VMs[0].Disks) > 0 {
		snap.VMs[0].Disks[0].Key = "mutated-key"
	}
}

func assertSnapshotHasNoMutatedFields(t *testing.T, snap cluster.Snapshot) {
	t.Helper()

	for _, vm := range snap.VMs {
		assertVMHasNoMutatedFields(t, vm)
	}
}

func assertVMHasNoMutatedFields(t *testing.T, vm cluster.VM) {
	t.Helper()

	if vm.Name == "mutated-name" {
		t.Fatal("mutating first snapshot's VM name leaked into second snapshot")
	}

	if slices.Contains(vm.Tags, "mutated-tag") {
		t.Fatal("mutating first snapshot's tags leaked into second snapshot")
	}

	if diskHasKey(vm.Disks, "mutated-key") {
		t.Fatal("mutating first snapshot's disk key leaked into second snapshot")
	}
}

func diskHasKey(disks []cluster.Disk, key string) bool {
	for _, d := range disks {
		if d.Key == key {
			return true
		}
	}

	return false
}

func TestFake_Snapshot_BootOrderAndNetworkDeepCopy(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-snapshot-bootorder")
	ctx := context.Background()

	snap, err := fake.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	mutateBootOrderAndNetworkInterfaces(snap)

	snap2, err := fake.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot second: %v", err)
	}

	assertNoBootOrderOrNetworkMutationLeaked(t, snap2)
}

func mutateBootOrderAndNetworkInterfaces(snap cluster.Snapshot) {
	for i := range snap.VMs {
		if len(snap.VMs[i].BootOrder) > 0 {
			snap.VMs[i].BootOrder[0] = "mutated-boot"
		}

		if len(snap.VMs[i].NetworkInterfaces) > 0 {
			snap.VMs[i].NetworkInterfaces[0].Bridge = "mutated-bridge"
		}
	}
}

func assertNoBootOrderOrNetworkMutationLeaked(t *testing.T, snap cluster.Snapshot) {
	t.Helper()

	for _, vm := range snap.VMs {
		assertVMBootOrderAndNetworkNotMutated(t, vm)
	}
}

func assertVMBootOrderAndNetworkNotMutated(t *testing.T, vm cluster.VM) {
	t.Helper()

	if slices.Contains(vm.BootOrder, "mutated-boot") {
		t.Fatal("mutating BootOrder leaked into second snapshot")
	}

	if anyNICBridgeIs(vm.NetworkInterfaces, "mutated-bridge") {
		t.Fatal("mutating NetworkInterface Bridge leaked into second snapshot")
	}
}

func anyNICBridgeIs(nics []cluster.NetworkInterface, bridge string) bool {
	for _, ni := range nics {
		if ni.Bridge == bridge {
			return true
		}
	}

	return false
}

func TestFake_DisplayName_NamedAndEmpty(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	named := cluster.NewFake("my-cluster")

	name, err := named.DisplayName(ctx)
	if err != nil {
		t.Fatalf("DisplayName(named): %v", err)
	}

	if name != "my-cluster" {
		t.Errorf("DisplayName = %q, want my-cluster", name)
	}

	empty := cluster.NewFake("")

	name, err = empty.DisplayName(ctx)
	if err != nil {
		t.Fatalf("DisplayName(empty): %v", err)
	}

	if name != "fake-cluster" {
		t.Errorf("DisplayName = %q, want fake-cluster", name)
	}
}

func TestFake_DisplayName_Unreachable(t *testing.T) {
	t.Parallel()

	fake := cluster.Fake{ClusterName: "offline-demo"}
	if _, err := fake.DisplayName(context.Background()); !errors.Is(err, cluster.ErrUnreachable) {
		t.Fatalf("DisplayName(offline-demo) error = %v, want ErrUnreachable", err)
	}
}

func TestFake_Snapshot_Unreachable(t *testing.T) {
	t.Parallel()

	fake := cluster.Fake{ClusterName: "offline-demo"}
	if _, err := fake.Snapshot(context.Background()); !errors.Is(err, cluster.ErrUnreachable) {
		t.Fatalf("Snapshot(offline-demo) error = %v, want ErrUnreachable", err)
	}
}

func TestFake_Snapshot_ProxmoxVersion(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-version")

	snap, err := fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	if snap.ProxmoxVersion == "" {
		t.Error("ProxmoxVersion is empty, expected a non-empty version")
	}
}

func TestFake_Authenticate_UnknownUser(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-auth-unknown")
	if _, err := fake.Authenticate(context.Background(), "ghost@pve", "password"); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("Authenticate(unknown) error = %v, want ErrNotFound", err)
	}
}

func TestFake_Authenticate_AdminIdentity(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-auth-admin")
	ctx := context.Background()

	id, err := fake.Authenticate(ctx, cluster.FakeUserAdmin, "pvmss-admin")
	if err != nil {
		t.Fatalf("Authenticate(admin): %v", err)
	}

	if !id.IsAdmin {
		t.Error("IsAdmin = false, want true for admin@pve")
	}

	if id.Pool != "" {
		t.Errorf("Pool = %q, want empty for admin", id.Pool)
	}
}

func TestFake_Authenticate_PoolUserIdentity(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-auth-pool")
	ctx := context.Background()

	id, err := fake.Authenticate(ctx, cluster.FakeUserAlice, "pvmss-alice")
	if err != nil {
		t.Fatalf("Authenticate(alice): %v", err)
	}

	if id.IsAdmin {
		t.Error("IsAdmin = true, want false for pool user")
	}

	if id.Pool != cluster.FakePoolAlice {
		t.Errorf("Pool = %q, want %q", id.Pool, cluster.FakePoolAlice)
	}
}

func TestFake_ChangePassword_UnknownUser(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-changepw-unknown")
	if err := fake.ChangePassword(context.Background(), "ghost@pve", "old", "new"); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("ChangePassword(unknown) error = %v, want ErrNotFound", err)
	}
}

func TestFake_GetCloudInitConfig_NotFound(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-ci-notfound")
	if _, err := fake.GetCloudInitConfig(context.Background(), cluster.FakeNode01, 99999); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("GetCloudInitConfig(not found) error = %v, want ErrNotFound", err)
	}
}

func TestFake_EnsureCloudInitDrive_NotFound(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-ci-drive-notfound")
	if err := fake.EnsureCloudInitDrive(context.Background(), cluster.FakeNode01, 99999); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("EnsureCloudInitDrive(not found) error = %v, want ErrNotFound", err)
	}
}

func TestFake_AttachCloudInitSnippet_NotFound(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-ci-attach-notfound")
	if err := fake.AttachCloudInitSnippet(context.Background(), cluster.FakeNode01, "local", "snippet.yaml", 99999); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("AttachCloudInitSnippet(not found) error = %v, want ErrNotFound", err)
	}
}

func TestFake_SetCloudInitPassword_NotFound(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-ci-pw-notfound")
	if err := fake.SetCloudInitPassword(context.Background(), cluster.FakeNode01, 99999, "newpass"); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("SetCloudInitPassword(not found) error = %v, want ErrNotFound", err)
	}
}

func TestFake_AddSSHKey_NotFound(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-ssh-notfound")
	if err := fake.AddSSHKey(context.Background(), cluster.FakeNode01, 99999, "root", "ssh-rsa AAA..."); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("AddSSHKey(not found) error = %v, want ErrNotFound", err)
	}
}

//nolint:paralleltest // serial: shared default fake state
func TestFake_AddSSHKey_ForcedError(t *testing.T) {
	cluster.ResetFake()
	defer cluster.ResetFake()

	forcedErr := errors.New("ssh injection failed")

	cluster.SetFakeSSHKeyError(forcedErr)
	defer cluster.SetFakeSSHKeyError(nil)

	if err := (cluster.Fake{}).AddSSHKey(context.Background(), cluster.FakeNode01, 100, "root", "ssh-rsa AAA..."); !errors.Is(err, forcedErr) {
		t.Fatalf("AddSSHKey(forced error) = %v, want %v", err, forcedErr)
	}
}

//nolint:paralleltest // serial: shared default fake state
func TestFake_PushCloudInitSnippet_ForcedError(t *testing.T) {
	cluster.ResetFake()
	defer cluster.ResetFake()

	forcedErr := errors.New("push failed")

	cluster.SetFakeCloudInitPushError(forcedErr)
	defer cluster.SetFakeCloudInitPushError(nil)

	if err := (cluster.Fake{}).PushCloudInitSnippet(context.Background(), cluster.FakeNode01, "local", "snippet.yaml", 100, "#cloud-config"); !errors.Is(err, forcedErr) {
		t.Fatalf("PushCloudInitSnippet(forced error) = %v, want %v", err, forcedErr)
	}
}

func TestFake_Action_NotFound(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-action-notfound")
	if err := fake.Action(context.Background(), cluster.FakeNode01, 99999, "start"); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("Action(not found) error = %v, want ErrNotFound", err)
	}
}

func TestFake_Action_InvalidAction(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-action-invalid")
	if err := fake.Action(context.Background(), cluster.FakeNode01, 100, "bogus-action"); !errors.Is(err, cluster.ErrInvalidAction) {
		t.Fatalf("Action(invalid) error = %v, want ErrInvalidAction", err)
	}
}

func TestFake_Action_PauseAndResume(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-pause-resume")
	ctx := context.Background()

	if err := fake.Action(ctx, cluster.FakeNode01, 100, "pause"); err != nil {
		t.Fatalf("Pause: %v", err)
	}

	snap, err := fake.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot after pause: %v", err)
	}

	var found bool

	for _, vm := range snap.VMs {
		if vm.VMID == 100 && vm.Status == cluster.VMPaused {
			found = true
		}
	}

	if !found {
		t.Fatal("VM 100 not paused after pause action")
	}

	if err := fake.Action(ctx, cluster.FakeNode01, 100, "resume"); err != nil {
		t.Fatalf("Resume: %v", err)
	}

	snap, err = fake.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot after resume: %v", err)
	}

	for _, vm := range snap.VMs {
		if vm.VMID == 100 && vm.Status != cluster.VMRunning {
			t.Fatalf("VM 100 status = %q after resume, want running", vm.Status)
		}
	}
}

func TestFake_Action_PauseOnStoppedVM(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-pause-stopped")
	if err := fake.Action(context.Background(), cluster.FakeNode01, 101, "pause"); !errors.Is(err, cluster.ErrInvalidStateTransition) {
		t.Fatalf("Pause on stopped VM error = %v, want ErrInvalidStateTransition", err)
	}
}

func TestFake_Action_ResumeOnRunningVM(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-resume-running")
	if err := fake.Action(context.Background(), cluster.FakeNode01, 100, "resume"); !errors.Is(err, cluster.ErrInvalidStateTransition) {
		t.Fatalf("Resume on running VM error = %v, want ErrInvalidStateTransition", err)
	}
}

func TestFake_Action_ResetOnRunningVM(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-reset-running")
	ctx := context.Background()

	if err := fake.Action(ctx, cluster.FakeNode01, 100, "reset"); err != nil {
		t.Fatalf("Reset on running VM: %v", err)
	}

	snap, err := fake.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot after reset: %v", err)
	}

	for _, vm := range snap.VMs {
		if vm.VMID == 100 && vm.Status != cluster.VMRunning {
			t.Fatalf("VM 100 status = %q after reset, want running", vm.Status)
		}
	}
}

func TestFake_Delete_NotFound(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-delete-notfound")
	if err := fake.Delete(context.Background(), cluster.FakeNode01, 99999); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("Delete(not found) error = %v, want ErrNotFound", err)
	}
}

func TestFake_Delete_RunningVMRejected(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-delete-running")
	if err := fake.Delete(context.Background(), cluster.FakeNode01, 100); !errors.Is(err, cluster.ErrVMRunning) {
		t.Fatalf("Delete(running) error = %v, want ErrVMRunning", err)
	}
}

func TestFake_Delete_StoppedVMSucceeds(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-delete-stopped")
	ctx := context.Background()

	before, err := fake.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot before: %v", err)
	}

	if err := fake.Delete(ctx, cluster.FakeNode01, 101); err != nil {
		t.Fatalf("Delete(stopped): %v", err)
	}

	after, err := fake.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot after: %v", err)
	}

	if len(after.VMs) != len(before.VMs)-1 {
		t.Fatalf("VM count = %d, want %d", len(after.VMs), len(before.VMs)-1)
	}

	for _, vm := range after.VMs {
		if vm.VMID == 101 {
			t.Fatal("VM 101 still present after delete")
		}
	}
}

func TestFake_Patch_NotFound(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-patch-notfound")
	if err := fake.Patch(context.Background(), cluster.FakeNode01, 99999, "new-name", "desc"); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("Patch(not found) error = %v, want ErrNotFound", err)
	}
}

func TestFake_Patch_UpdatesNameAndDescription(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-patch-update")
	ctx := context.Background()

	if err := fake.Patch(ctx, cluster.FakeNode01, 100, "renamed-vm", "new description"); err != nil {
		t.Fatalf("Patch: %v", err)
	}

	snap, err := fake.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	for _, vm := range snap.VMs {
		if vm.VMID == 100 {
			if vm.Name != "renamed-vm" {
				t.Errorf("Name = %q, want renamed-vm", vm.Name)
			}

			if vm.Description != "new description" {
				t.Errorf("Description = %q, want \"new description\"", vm.Description)
			}
		}
	}
}

func TestFake_Patch_EmptyArgsIgnored(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-patch-empty")
	ctx := context.Background()

	original, err := fake.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	var originalName, originalDesc string

	for _, vm := range original.VMs {
		if vm.VMID == 100 {
			originalName = vm.Name
			originalDesc = vm.Description
		}
	}

	if err := fake.Patch(ctx, cluster.FakeNode01, 100, "", ""); err != nil {
		t.Fatalf("Patch(empty): %v", err)
	}

	snap, err := fake.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot after empty patch: %v", err)
	}

	for _, vm := range snap.VMs {
		if vm.VMID == 100 {
			if vm.Name != originalName {
				t.Errorf("Name = %q, want %q (should be unchanged)", vm.Name, originalName)
			}

			if vm.Description != originalDesc {
				t.Errorf("Description = %q, want %q (should be unchanged)", vm.Description, originalDesc)
			}
		}
	}
}

func TestFake_AddDisk_NotFound(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-adddisk-notfound")
	if _, err := fake.AddDisk(context.Background(), cluster.FakeNode01, 99999, "scsi", "local-lvm", 10); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("AddDisk(not found) error = %v, want ErrNotFound", err)
	}
}

func TestFake_AddDisk_Success(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-adddisk-success")
	ctx := context.Background()

	key, err := fake.AddDisk(ctx, cluster.FakeNode01, 100, "scsi", "local-lvm", 20)
	if err != nil {
		t.Fatalf("AddDisk: %v", err)
	}

	if key == "" {
		t.Fatal("AddDisk returned empty key")
	}

	snap, err := fake.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	assertAddedDiskPresent(t, snap, 100, key)
}

func assertAddedDiskPresent(t *testing.T, snap cluster.Snapshot, vmid int, key string) {
	t.Helper()

	for _, vm := range snap.VMs {
		if vm.VMID != vmid {
			continue
		}

		assertDiskWithKeyPresent(t, vm.Disks, key)

		return
	}
}

func assertDiskWithKeyPresent(t *testing.T, disks []cluster.Disk, key string) {
	t.Helper()

	for _, disk := range disks {
		if disk.Key != key {
			continue
		}

		if disk.SizeGB != 20 {
			t.Errorf("disk SizeGB = %d, want 20", disk.SizeGB)
		}

		if disk.Storage != "local-lvm" {
			t.Errorf("disk Storage = %q, want local-lvm", disk.Storage)
		}

		return
	}

	t.Fatalf("added disk %q not found in snapshot", key)
}

func TestFake_ResizeDisk_NotFound(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-resizedisk-notfound")

	if err := fake.ResizeDisk(context.Background(), cluster.FakeNode01, 99999, "scsi0", 50); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("ResizeDisk(VM not found) error = %v, want ErrNotFound", err)
	}

	if err := fake.ResizeDisk(context.Background(), cluster.FakeNode01, 101, "nonexistent-disk", 50); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("ResizeDisk(disk not found) error = %v, want ErrNotFound", err)
	}
}

func TestFake_ResizeDisk_Success(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-resizedisk-success")
	ctx := context.Background()

	if err := fake.ResizeDisk(ctx, cluster.FakeNode01, 101, "scsi0", 64); err != nil {
		t.Fatalf("ResizeDisk: %v", err)
	}

	snap, err := fake.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	for _, vm := range snap.VMs {
		if vm.VMID == 101 {
			for _, disk := range vm.Disks {
				if disk.Key == "scsi0" && disk.SizeGB != 64 {
					t.Errorf("disk scsi0 SizeGB = %d, want 64", disk.SizeGB)
				}
			}
		}
	}
}

func TestFake_DeleteDisk_NotFound(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-deletedisk-notfound")

	if err := fake.DeleteDisk(context.Background(), cluster.FakeNode01, 99999, "scsi0"); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("DeleteDisk(VM not found) error = %v, want ErrNotFound", err)
	}

	if err := fake.DeleteDisk(context.Background(), cluster.FakeNode01, 101, "nonexistent-disk"); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("DeleteDisk(disk not found) error = %v, want ErrNotFound", err)
	}
}

func TestFake_DeleteDisk_Success(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-deletedisk-success")
	ctx := context.Background()

	if err := fake.DeleteDisk(ctx, cluster.FakeNode01, 101, "scsi1"); err != nil {
		t.Fatalf("DeleteDisk: %v", err)
	}

	snap, err := fake.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	for _, vm := range snap.VMs {
		if vm.VMID == 101 {
			for _, disk := range vm.Disks {
				if disk.Key == "scsi1" {
					t.Fatal("disk scsi1 still present after delete")
				}
			}
		}
	}
}

func TestFake_SetCDROM_NotFound(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-cdrom-notfound")
	if err := fake.SetCDROM(context.Background(), cluster.FakeNode01, 99999, cluster.CDROMState{State: cluster.CDROMEmpty}); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("SetCDROM(not found) error = %v, want ErrNotFound", err)
	}
}

func TestFake_SetCDROM_Success(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-cdrom-success")
	ctx := context.Background()

	cdrom := cluster.CDROMState{State: cluster.CDROMMounted, ISOVolID: "local:iso/test.iso"}
	if err := fake.SetCDROM(ctx, cluster.FakeNode01, 100, cdrom); err != nil {
		t.Fatalf("SetCDROM: %v", err)
	}

	snap, err := fake.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	for _, vm := range snap.VMs {
		if vm.VMID == 100 {
			if vm.CDROM.State != cluster.CDROMMounted {
				t.Errorf("CDROM State = %q, want mounted", vm.CDROM.State)
			}

			if vm.CDROM.ISOVolID != "local:iso/test.iso" {
				t.Errorf("CDROM ISOVolID = %q, want local:iso/test.iso", vm.CDROM.ISOVolID)
			}
		}
	}
}

func TestFake_UpdateNetwork_NotFound(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-network-notfound")
	if err := fake.UpdateNetwork(context.Background(), cluster.FakeNode01, 99999, []cluster.NetworkInterface{}); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("UpdateNetwork(not found) error = %v, want ErrNotFound", err)
	}
}

func TestFake_UpdateHardware_NotFound(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-hardware-notfound")
	if err := fake.UpdateHardware(context.Background(), cluster.FakeNode01, 99999, 1, 2, 2048, []string{}); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("UpdateHardware(not found) error = %v, want ErrNotFound", err)
	}
}

func TestFake_UpdateHardware_Success(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-hardware-success")
	ctx := context.Background()

	if err := fake.UpdateHardware(ctx, cluster.FakeNode01, 100, 2, 4, 8192, []string{"prod", "pvmss"}); err != nil {
		t.Fatalf("UpdateHardware: %v", err)
	}

	snap, err := fake.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	assertHardwareFieldsUpdated(t, snap, 100)
}

func assertHardwareFieldsUpdated(t *testing.T, snap cluster.Snapshot, vmid int) {
	t.Helper()

	for _, vm := range snap.VMs {
		if vm.VMID == vmid {
			assertVMHardwareFields(t, vm)
		}
	}
}

func assertVMHardwareFields(t *testing.T, vm cluster.VM) {
	t.Helper()

	if vm.Sockets != 2 {
		t.Errorf("Sockets = %d, want 2", vm.Sockets)
	}

	if vm.Cores != 4 {
		t.Errorf("Cores = %d, want 4", vm.Cores)
	}

	if vm.CPUCores != 8 {
		t.Errorf("CPUCores = %d, want 8", vm.CPUCores)
	}

	if vm.MemoryTotal != 8192*1024*1024 {
		t.Errorf("MemoryTotal = %d, want %d", vm.MemoryTotal, 8192*1024*1024)
	}
}

func TestFake_EnableSerial_NotFound(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-serial-notfound")
	if err := fake.EnableSerial(context.Background(), cluster.FakeNode01, 99999); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("EnableSerial(not found) error = %v, want ErrNotFound", err)
	}
}

func TestFake_EnableSerial_Success(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-serial-success")
	ctx := context.Background()

	if err := fake.EnableSerial(ctx, cluster.FakeNode01, 101); err != nil {
		t.Fatalf("EnableSerial: %v", err)
	}

	snap, err := fake.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	for _, vm := range snap.VMs {
		if vm.VMID == 101 && !vm.HasSerial {
			t.Error("HasSerial = false, want true after EnableSerial")
		}
	}
}

func TestFake_GetMetricsHistory_Success(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-metrics-history")
	ctx := context.Background()

	cases := []struct {
		name      string
		timeframe cluster.MetricsTimeframe
		wantCount int
	}{
		{name: "hour", timeframe: cluster.MetricsTimeframeHour, wantCount: 60},
		{name: "day", timeframe: cluster.MetricsTimeframeDay, wantCount: 96},
		{name: "week", timeframe: cluster.MetricsTimeframeWeek, wantCount: 168},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			samples, err := fake.GetMetricsHistory(ctx, cluster.FakeNode01, 100, tc.timeframe)
			if err != nil {
				t.Fatalf("GetMetricsHistory(%s): %v", tc.name, err)
			}

			if len(samples) != tc.wantCount {
				t.Fatalf("samples count = %d, want %d", len(samples), tc.wantCount)
			}

			assertMetricsSamplesValid(t, samples)
		})
	}
}

func assertMetricsSamplesValid(t *testing.T, samples []cluster.MetricsSample) {
	t.Helper()

	for i, s := range samples {
		assertMetricsSampleValid(t, i, s)
	}
}

func assertMetricsSampleValid(t *testing.T, i int, s cluster.MetricsSample) {
	t.Helper()

	if s.Timestamp.IsZero() {
		t.Errorf("sample[%d] has zero timestamp", i)
	}

	if s.CPUPercent < 0 || s.CPUPercent > 100 {
		t.Errorf("sample[%d] CPUPercent = %v, out of range", i, s.CPUPercent)
	}

	if s.MemoryMax <= 0 {
		t.Errorf("sample[%d] MemoryMax = %d, want positive", i, s.MemoryMax)
	}
}

func TestFake_GetMetricsCurrent_Success(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-metrics-current")
	ctx := context.Background()

	sample, err := fake.GetMetricsCurrent(ctx, cluster.FakeNode01, 100)
	if err != nil {
		t.Fatalf("GetMetricsCurrent: %v", err)
	}

	if sample.Timestamp.IsZero() {
		t.Error("Timestamp is zero")
	}

	if sample.MemoryMax <= 0 {
		t.Errorf("MemoryMax = %d, want positive", sample.MemoryMax)
	}
}

func TestFake_GetMetricsCurrent_NotFound(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-metrics-current-notfound")
	if _, err := fake.GetMetricsCurrent(context.Background(), cluster.FakeNode01, 99999); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("GetMetricsCurrent(not found) error = %v, want ErrNotFound", err)
	}
}

func TestFake_ListSnapshots_NotFound(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-listsnap-notfound")

	if _, err := fake.ListSnapshots(context.Background(), cluster.FakeNode01, 99999); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("ListSnapshots(VM not found) error = %v, want ErrNotFound", err)
	}

	if _, err := fake.ListSnapshots(context.Background(), cluster.FakeNode01, 100); err != nil {
		t.Fatalf("ListSnapshots(no snapshots yet) error = %v, want nil", err)
	}
}

func TestFake_CreateSnapshot_NotFound(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-createsnap-notfound")
	if _, err := fake.CreateSnapshot(context.Background(), cluster.FakeNode01, 99999, "snap1", "desc", false); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("CreateSnapshot(VM not found) error = %v, want ErrNotFound", err)
	}
}

func TestFake_RollbackSnapshot_NotFound(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-rollbacksnap-notfound")

	if _, err := fake.RollbackSnapshot(context.Background(), cluster.FakeNode01, 99999, "snap1"); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("RollbackSnapshot(VM not found) error = %v, want ErrNotFound", err)
	}

	if _, err := fake.RollbackSnapshot(context.Background(), cluster.FakeNode01, 100, "nonexistent-snap"); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("RollbackSnapshot(snapshot not found) error = %v, want ErrNotFound", err)
	}
}

func TestFake_DeleteSnapshot_NotFound(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-deletesnap-notfound")

	if _, err := fake.DeleteSnapshot(context.Background(), cluster.FakeNode01, 99999, "snap1"); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("DeleteSnapshot(VM not found) error = %v, want ErrNotFound", err)
	}

	if _, err := fake.DeleteSnapshot(context.Background(), cluster.FakeNode01, 100, "nonexistent-snap"); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("DeleteSnapshot(snapshot not found) error = %v, want ErrNotFound", err)
	}
}

func TestFake_Snapshot_RollbackWithVMState(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-snap-rollback-vmstate")
	ctx := context.Background()

	upid, err := fake.CreateSnapshot(ctx, cluster.FakeNode01, 101, "state-snap", "with state", true)
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	completeFakeTaskExternal(t, fake, upid)

	if err := fake.Action(ctx, cluster.FakeNode01, 101, "start"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	rollbackUPID, err := fake.RollbackSnapshot(ctx, cluster.FakeNode01, 101, "state-snap")
	if err != nil {
		t.Fatalf("RollbackSnapshot: %v", err)
	}

	completeFakeTaskExternal(t, fake, rollbackUPID)

	snap, err := fake.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot after rollback: %v", err)
	}

	for _, vm := range snap.VMs {
		if vm.VMID == 101 && vm.Status != cluster.VMRunning {
			t.Fatalf("VM 101 status = %q after rollback with vmstate, want running", vm.Status)
		}
	}
}

func TestFake_Snapshot_RollbackWithoutVMState(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-snap-rollback-no-vmstate")
	ctx := context.Background()

	upid, err := fake.CreateSnapshot(ctx, cluster.FakeNode01, 100, "no-state-snap", "no state", false)
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	completeFakeTaskExternal(t, fake, upid)

	rollbackUPID, err := fake.RollbackSnapshot(ctx, cluster.FakeNode01, 100, "no-state-snap")
	if err != nil {
		t.Fatalf("RollbackSnapshot: %v", err)
	}

	completeFakeTaskExternal(t, fake, rollbackUPID)

	snap, err := fake.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot after rollback: %v", err)
	}

	for _, vm := range snap.VMs {
		if vm.VMID == 100 && vm.Status != cluster.VMStopped {
			t.Fatalf("VM 100 status = %q after rollback without vmstate, want stopped", vm.Status)
		}
	}
}

func TestFake_GetVNCTicket_ReturnsFixedTicket(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-vnc-ticket")

	ticket, err := fake.GetVNCTicket(context.Background(), cluster.FakeNode01, 100, "test")
	if err != nil {
		t.Fatalf("GetVNCTicket: %v", err)
	}

	if ticket.Ticket == "" {
		t.Error("Ticket is empty")
	}

	if ticket.Port == 0 {
		t.Error("Port is zero")
	}
}

func TestFake_GetTermProxy_ReturnsFixedTicket(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-term-proxy")

	ticket, err := fake.GetTermProxy(context.Background(), cluster.FakeNode01, 100, "test")
	if err != nil {
		t.Fatalf("GetTermProxy: %v", err)
	}

	if ticket.Ticket == "" {
		t.Error("Ticket is empty")
	}

	if ticket.Port == 0 {
		t.Error("Port is zero")
	}
}

func TestFake_NextVMID_Monotonic(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-nextvmid-mono")
	ctx := context.Background()

	first, err := fake.NextVMID(ctx)
	if err != nil {
		t.Fatalf("NextVMID first: %v", err)
	}

	second, err := fake.NextVMID(ctx)
	if err != nil {
		t.Fatalf("NextVMID second: %v", err)
	}

	if second <= first {
		t.Fatalf("second VMID %d not greater than first %d", second, first)
	}
}

func TestFake_CreateVM_WithISO(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-createvm-iso")
	ctx := context.Background()

	vmid, err := fake.NextVMID(ctx)
	if err != nil {
		t.Fatalf("NextVMID: %v", err)
	}

	spec := cluster.VMSpec{
		VMID:             vmid,
		Node:             cluster.FakeNode01,
		Name:             "iso-vm",
		Pool:             cluster.FakePoolAlice,
		Tags:             []string{"pvmss"},
		CPUCores:         2,
		MemoryMB:         4096,
		Disk:             cluster.DiskSpec{Storage: cluster.FakeStorageLocalLVM, SizeGB: 30},
		Network:          cluster.NetworkSpec{{Bridge: cluster.FakeBridgeVMbr0, Model: string(cluster.DiskBusVirtio)}},
		ISO:              &cluster.ISOSpec{Storage: "local", File: "debian-12.iso"},
		StartAfterCreate: false,
	}

	upid, err := fake.CreateVM(ctx, spec)
	if err != nil {
		t.Fatalf("CreateVM: %v", err)
	}

	if upid == "" {
		t.Fatal("CreateVM returned empty upid")
	}

	snap, err := fake.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	var found bool

	for _, vm := range snap.VMs {
		if vm.VMID == vmid {
			found = true

			if vm.Name != "iso-vm" {
				t.Errorf("Name = %q, want iso-vm", vm.Name)
			}

			if vm.Status != cluster.VMStopped {
				t.Errorf("Status = %q, want stopped (no StartAfterCreate)", vm.Status)
			}
		}
	}

	if !found {
		t.Fatalf("created VM %d not in snapshot", vmid)
	}
}

func completeFakeTaskExternal(t *testing.T, fake cluster.Fake, upid string) {
	t.Helper()

	for range 3 {
		if _, err := fake.TaskStatus(context.Background(), upid); err != nil {
			t.Fatalf("TaskStatus: %v", err)
		}
	}
}
