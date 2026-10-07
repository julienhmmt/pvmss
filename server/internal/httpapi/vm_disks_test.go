package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"testing"
)

//nolint:paralleltest // serial: shared fake cluster dataset
func TestVMDetail_DiskDelete_ProtectsBootDiskBeforeClusterCall(t *testing.T) {
	cluster.ResetFake()

	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)
	request := detailRequest(http.MethodDelete, "/api/v1/vms/default/101/disks/scsi0", "", cookie)
	request.SetPathValue("diskKey", "scsi0")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}

	assertAPIError(t, recorder.Body.Bytes(), "boot_disk_protected")

	if calls := cluster.FakeCallsFor(101); len(calls) != 0 {
		t.Fatalf("fake calls = %+v, want none", calls)
	}
}

//nolint:paralleltest // serial: shared fake cluster dataset
func TestVMDetail_DiskAdd_UsesApprovedStorage(t *testing.T) {
	cluster.ResetFake()

	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)
	recorder := httptest.NewRecorder()
	request := detailRequest(http.MethodPost, "/api/v1/vms/default/101/disks", `{"bus":"scsi","storage":"local-lvm","sizeGB":10}`, cookie)
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	calls := cluster.FakeCallsFor(101)
	if len(calls) != 1 || calls[0].Action != "add_disk" {
		t.Fatalf("fake calls = %+v, want one add_disk call", calls)
	}
}
