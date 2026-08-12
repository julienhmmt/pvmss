package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"testing"
)

//nolint:paralleltest // serial: shared fake cluster dataset
func TestVMDetail_Hardware_TagsOnlyDoesNotRestart(t *testing.T) {
	cluster.ResetFake()

	handler, authHandler, _, _ := newVMDetailHandler(t)
	request := detailRequest(http.MethodPut, "/api/v1/vms/default/101/hardware", `{"tags":["pvmss","updated"]}`, aliceCookie(t, authHandler))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	assertSingleFakeCall(t, 101, "update_hardware")
}

func assertSingleFakeCall(t *testing.T, vmid int, action string) {
	t.Helper()

	calls := cluster.FakeCallsFor(vmid)
	if len(calls) != 1 || calls[0].Action != action {
		t.Fatalf("calls = %+v, want one %s call", calls, action)
	}
}

//nolint:paralleltest // serial: shared fake cluster dataset
func TestVMDetail_Hardware_RejectsEmptyPatch(t *testing.T) {
	cluster.ResetFake()

	handler, authHandler, _, _ := newVMDetailHandler(t)
	request := detailRequest(http.MethodPut, "/api/v1/vms/default/101/hardware", `{}`, aliceCookie(t, authHandler))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}

	assertAPIError(t, recorder.Body.Bytes(), "empty_patch")
}
