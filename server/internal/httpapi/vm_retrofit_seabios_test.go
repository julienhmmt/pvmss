package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"testing"
)

// TestVMDetail_RetrofitSeaBIOS_NonAdminForbidden — the retrofit action is
// admin-only; a non-admin owner gets 403 and
// the writer is not touched.
//
//nolint:paralleltest,dupl // serial: shared fake cluster dataset; standard POST-403 pattern
func TestVMDetail_RetrofitSeaBIOS_NonAdminForbidden(t *testing.T) {
	cluster.ResetFake()

	handler, authHandler, _, _ := newVMDetailHandler(t)
	request := detailRequest(http.MethodPost, "/api/v1/vms/default/100/retrofit-seabios", "{}", aliceCookie(t, authHandler))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}

	assertAPIError(t, recorder.Body.Bytes(), apiCodeForbidden)

	if calls := cluster.FakeCallsFor(100); len(calls) != 0 {
		t.Fatalf("expected no fake calls for non-admin, got %+v", calls)
	}
}

// TestVMDetail_RetrofitSeaBIOS_Unauthenticated — the retrofit endpoint
// requires auth.
//
//nolint:paralleltest // serial: shared fake cluster dataset
func TestVMDetail_RetrofitSeaBIOS_Unauthenticated(t *testing.T) {
	cluster.ResetFake()

	handler, _, _, _ := newVMDetailHandler(t)
	request := detailRequest(http.MethodPost, "/api/v1/vms/default/100/retrofit-seabios", "{}", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
	}
}

// TestVMDetail_RetrofitSeaBIOS_MethodNotAllowed — non-POST methods are rejected.
//
//nolint:paralleltest // serial: shared fake cluster dataset
func TestVMDetail_RetrofitSeaBIOS_MethodNotAllowed(t *testing.T) {
	cluster.ResetFake()

	handler, authHandler, _, _ := newVMDetailHandler(t)
	request := detailRequest(http.MethodGet, "/api/v1/vms/default/100/retrofit-seabios", "", adminCookie(t, authHandler))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusMethodNotAllowed, recorder.Body.String())
	}
}
