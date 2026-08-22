package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"testing"
)

// TestVMDetail_Serial_EnablesPortOnExistingVM — POST /serial on a VM created
// without a serial port (dataset VM 101) provisions serial0, returns 200 with
// the refreshed entity reporting hasSerial=true, and records one enable_serial
// fake call.
//
//nolint:paralleltest // serial: shared fake cluster dataset
func TestVMDetail_Serial_EnablesPortOnExistingVM(t *testing.T) {
	cluster.ResetFake()

	handler, authHandler, _, _ := newVMDetailHandler(t)
	request := detailRequest(http.MethodPost, "/api/v1/vms/default/101/serial", "{}", aliceCookie(t, authHandler))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var entity vmDetailEntityWithSerial
	if err := json.Unmarshal(recorder.Body.Bytes(), &entity); err != nil {
		t.Fatalf("decode entity: %v", err)
	}

	if !entity.HasSerial {
		t.Fatalf("entity.HasSerial = false after enable, want true")
	}

	assertSingleFakeCall(t, 101, "enable_serial")
}

// TestVMDetail_Serial_NonOwnerForbidden — an actor who does not own the VM
// (bob, pool-bob) gets 403 and the writer is not touched.
//
//nolint:paralleltest // serial: shared fake cluster dataset
func TestVMDetail_Serial_NonOwnerForbidden(t *testing.T) {
	cluster.ResetFake()

	handler, authHandler, _, _ := newVMDetailHandler(t)
	request := detailRequest(http.MethodPost, "/api/v1/vms/default/100/serial", "{}", bobCookie(t, authHandler))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}

	assertAPIError(t, recorder.Body.Bytes(), apiCodeForbidden)

	if calls := cluster.FakeCallsFor(100); len(calls) != 0 {
		t.Fatalf("expected no fake calls for non-owner, got %+v", calls)
	}
}

// vmDetailEntityWithSerial extends the shared DTO mirror with the hasSerial
// field added by this feature.
type vmDetailEntityWithSerial struct {
	VMID      int    `json:"vmid"`
	HasSerial bool   `json:"hasSerial"`
	Name      string `json:"name"`
}
