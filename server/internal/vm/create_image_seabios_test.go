package vm_test

import (
	"context"
	"errors"
	"log/slog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/vm"
	"slices"
	"testing"
)

// TestCreate_Image_SeaBIOSDefault — image mode defaults to SeaBIOS
// (cloud-image-console issue 02): a request that omits uefi creates a VM with
// no efidisk0 and no bios=ovmf, so the graphical tab shows the guest's real
// text console instead of an uninitialised framebuffer.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_Image_SeaBIOSDefault(t *testing.T) {
	fixture := newCreateFixture(t)

	req := imageRequest()
	// Omit uefi entirely — the default must be SeaBIOS.
	req.UEFI = nil

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
	if created.BIOS == testBIOSOVMF {
		t.Errorf("BIOS = %q, want empty (SeaBIOS) for image mode default", created.BIOS)
	}

	if created.EFIDisk {
		t.Errorf("EFIDisk = true, want false for SeaBIOS image-mode VM")
	}
}

// TestCreate_Image_UEFIExplicitHonored — a request that sends uefi=true in
// image mode still creates a UEFI VM (the checkbox stays re-tickable; TPM and
// Secure Boot keep their "requires UEFI" behaviour).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_Image_UEFIExplicitHonored(t *testing.T) {
	fixture := newCreateFixture(t)

	uefi := true
	req := imageRequest()
	req.UEFI = &uefi

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
	if created.BIOS != testBIOSOVMF {
		t.Errorf("BIOS = %q, want ovmf for explicit uefi=true", created.BIOS)
	}

	if !created.EFIDisk {
		t.Errorf("EFIDisk = false, want true for UEFI image-mode VM")
	}
}

// TestCreate_Image_UEFIExplicitFalseHonored — a request that sends uefi=false
// explicitly is honored (same as the default, but the explicit path must not
// be flipped by the image-mode default).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_Image_UEFIExplicitFalseHonored(t *testing.T) {
	fixture := newCreateFixture(t)

	uefi := false
	req := imageRequest()
	req.UEFI = &uefi

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
	if created.BIOS == testBIOSOVMF {
		t.Errorf("BIOS = %q, want empty (SeaBIOS) for explicit uefi=false", created.BIOS)
	}

	if created.EFIDisk {
		t.Errorf("EFIDisk = true, want false for explicit uefi=false")
	}
}

// TestCreate_Image_TPMRequiresUEFIStillRejected — the image-mode SeaBIOS
// default does not weaken checkUEFICompat: TPM without UEFI is still rejected
// (TPM 2.0 requires UEFI). A user who re-ticks UEFI can still get TPM.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_Image_TPMRequiresUEFIStillRejected(t *testing.T) {
	fixture := newCreateFixture(t)

	req := imageRequest()
	req.UEFI = nil // image-mode default → SeaBIOS
	req.TPM = true

	_, err := fixture.create(t, aliceIdentity(), req)
	if !errors.Is(err, vm.ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest (tpm requires uefi)", err)
	}

	for _, c := range cluster.FakeCalls() {
		if c.Action == testActionCreate {
			t.Fatalf("a VM was created despite TPM-without-UEFI: %+v", c)
		}
	}
}

// TestCreate_Image_NoWriteTarget_SeaBIOSDefault — the SeaBIOS default holds
// even when the cluster has no snippet write target (the baseline is skipped,
// but the firmware decision is independent of snippet delivery).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_Image_NoWriteTarget_SeaBIOSDefault(t *testing.T) {
	fixture := newCreateFixture(t)

	req := imageRequest()
	req.UEFI = nil
	req.StartAfterCreate = true

	result, err := vm.Create(context.Background(), aliceIdentity(), req.Cluster, req, vm.CreateDeps{
		Store: fixture.store, Creator: fixture.fake, Pusher: fixture.fake,
		Writer: fixture.fake, FreeSpace: fixture.fake, Snippets: writeUnavailableSnippetFinder{},
		Audit: fixture.store, Log: slog.New(slog.DiscardHandler),
	})
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
	if created.BIOS == testBIOSOVMF {
		t.Errorf("BIOS = %q, want SeaBIOS even without a write target", created.BIOS)
	}
}

// TestCreate_Image_StampedWithPvmssImageTag — image-mode VMs carry the
// pvmss-image tag alongside the mandatory pvmss tag, so the console can
// default to the readable text tab (cloud-image-console issue 06).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_Image_StampedWithPvmssImageTag(t *testing.T) {
	fixture := newCreateFixture(t)

	req := imageRequest()
	req.UEFI = nil

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
	if !slices.Contains(created.Tags, "pvmss") {
		t.Errorf("pvmss tag missing: %v", created.Tags)
	}

	if !slices.Contains(created.Tags, "pvmss-image") {
		t.Errorf("pvmss-image tag missing: %v", created.Tags)
	}
}

// TestCreate_NonImage_NoPvmssImageTag — the pvmss-image tag is image-mode
// only; an ISO VM gets pvmss but not pvmss-image.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_NonImage_NoPvmssImageTag(t *testing.T) {
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

	created := snap.VMs[idx]
	if !slices.Contains(created.Tags, "pvmss") {
		t.Errorf("pvmss tag missing: %v", created.Tags)
	}

	if slices.Contains(created.Tags, "pvmss-image") {
		t.Errorf("pvmss-image tag appeared on a non-image VM: %v", created.Tags)
	}
}
