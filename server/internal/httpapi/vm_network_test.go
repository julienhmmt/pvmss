package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"testing"
)

//nolint:paralleltest // serial: shared fake cluster dataset
func TestVMDetail_Network_ChangesApprovedBridge(t *testing.T) {
	cluster.ResetFake()

	handler, authHandler, _, _ := newVMDetailHandler(t)
	request := detailRequest(http.MethodPut, "/api/v1/vms/default/101/network", `{"interfaces":[{"index":0,"bridge":"vmbr1","model":"virtio"}]}`, aliceCookie(t, authHandler))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	assertSingleFakeCall(t, 101, "update_network")
}

//nolint:paralleltest // serial: shared fake cluster dataset
func TestVMDetail_Network_RejectsForgedGuestFields(t *testing.T) {
	cluster.ResetFake()

	handler, authHandler, _, _ := newVMDetailHandler(t)
	request := detailRequest(http.MethodPut, "/api/v1/vms/default/101/network", `{"interfaces":[{"index":0,"bridge":"vmbr1","model":"virtio","mac":"00:00:00:00:00:00"}]}`, aliceCookie(t, authHandler))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}

	if calls := cluster.FakeCallsFor(101); len(calls) != 0 {
		t.Fatalf("fake calls = %+v, want none", calls)
	}
}
