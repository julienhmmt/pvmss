package httpapi_test

import (
	"encoding/json"
	"net/http"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/vm"
	"testing"
)

//nolint:paralleltest // serial: shared fake dataset + global var mutation
func TestVMDetail_BootCDROM_OwnerBootsStoppedVM(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	originalPoll := vm.BootPollInterval
	vm.BootPollInterval = 1

	t.Cleanup(func() { vm.BootPollInterval = originalPoll })

	rec, _ := serveDetail(handler, detailRequest(http.MethodPost, "/api/v1/vms/default/101/boot-cdrom", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Status != "accepted" {
		t.Errorf("status = %q, want accepted", resp.Status)
	}

	assertBootSequence(t)
	assertVMBootedFromCDROM(t)
}

// assertBootSequence checks the fake call trail: CD-first boot order, start,
// then the original order restored.
func assertBootSequence(t *testing.T) {
	t.Helper()

	calls := cluster.FakeCallsFor(101)
	if len(calls) != 3 {
		t.Fatalf("fake calls = %d, want 3 (set_boot_order, start, set_boot_order)", len(calls))
	}

	if calls[0].Action != "set_boot_order" || len(calls[0].BootOrder) != 2 || calls[0].BootOrder[0] != "ide2" {
		t.Errorf("first call = %+v, want set_boot_order CD-first", calls[0])
	}

	if calls[1].Action != "start" {
		t.Errorf("second call = %q, want start", calls[1].Action)
	}

	if calls[2].Action != "set_boot_order" || len(calls[2].BootOrder) != 0 {
		t.Errorf("restore call = %+v, want empty boot order", calls[2])
	}
}

// assertVMBootedFromCDROM checks the VM ended up running with its original
// (empty) boot order restored.
func assertVMBootedFromCDROM(t *testing.T) {
	t.Helper()

	snap, err := (cluster.Fake{}).Snapshot(t.Context())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	for _, testVM := range snap.VMs {
		if testVM.VMID != 101 {
			continue
		}

		if testVM.Status != "running" {
			t.Errorf("status = %q, want running", testVM.Status)
		}

		if len(testVM.BootOrder) != 0 {
			t.Errorf("BootOrder = %v, want restored to empty", testVM.BootOrder)
		}
	}
}

//nolint:paralleltest // serial: shared fake dataset
func TestVMDetail_BootCDROM_RunningRejected(t *testing.T) {
	// Mount an ISO on running VM 100 before the handler snapshots the dataset,
	// so the status guard is what rejects (not the CD-ROM check).
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)

	if err := (cluster.Fake{}).SetCDROM(t.Context(), cluster.FakeNode01, 100, cluster.CDROMState{State: cluster.CDROMMounted, ISOVolID: "local:iso/debian-12-generic-amd64.iso"}); err != nil {
		t.Fatalf("SetCDROM: %v", err)
	}

	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, _ := serveDetailError(handler, detailRequest(http.MethodPost, "/api/v1/vms/default/100/boot-cdrom", "", cookie))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusConflict, rec.Body.String())
	}

	assertAPIError(t, rec.Body.Bytes(), "vm_running")
}

//nolint:paralleltest // serial: shared fake dataset
func TestVMDetail_BootCDROM_NoISORejected(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	// VM 124 is stopped and alice's, but has no CD-ROM mounted.
	rec, _ := serveDetailError(handler, detailRequest(http.MethodPost, "/api/v1/vms/default/124/boot-cdrom", "", cookie))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusConflict, rec.Body.String())
	}

	assertAPIError(t, rec.Body.Bytes(), "no_cdrom")
}

//nolint:paralleltest // serial: shared fake dataset
func TestVMDetail_BootCDROM_NonOwnerRejected(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := bobCookie(t, authHandler)

	rec, _ := serveDetailError(handler, detailRequest(http.MethodPost, "/api/v1/vms/default/101/boot-cdrom", "", cookie))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}

	assertAPIError(t, rec.Body.Bytes(), apiCodeForbidden)
}

//nolint:paralleltest // serial: shared fake dataset
func TestVMDetail_BootCDROM_MethodNotAllowed(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, _ := serveDetailError(handler, detailRequest(http.MethodGet, "/api/v1/vms/default/101/boot-cdrom", "", cookie))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
