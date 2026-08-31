package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
	"slices"
	"testing"
	"time"
)

//nolint:paralleltest // serial: shared fake cluster dataset
func TestVMDetail_Hardware_TagsOnlyDoesNotRestart(t *testing.T) {
	cluster.ResetFake()

	handler, authHandler, _, st := newVMDetailHandler(t)
	seedHardwareTestTag(t, st)
	request := detailRequest(http.MethodPut, "/api/v1/vms/default/101/hardware", `{"tags":["pvmss","updated"]}`, aliceCookie(t, authHandler))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	assertSingleFakeCall(t, 101, "update_hardware")
}

//nolint:paralleltest // serial: shared fake cluster dataset
func TestVMDetail_Hardware_UnknownTagRejected(t *testing.T) {
	cluster.ResetFake()

	handler, authHandler, _, _ := newVMDetailHandler(t)
	request := detailRequest(http.MethodPut, "/api/v1/vms/default/101/hardware", `{"tags":["forged-tag"]}`, aliceCookie(t, authHandler))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}

	assertAPIError(t, recorder.Body.Bytes(), "not_approved")
}

// seedHardwareTestTag seeds the "updated" tag the hardware tests reference
// (FR-013: tags outside the admin catalog are rejected).
func seedHardwareTestTag(t *testing.T, st *store.Store) {
	t.Helper()

	if err := st.InsertTag(context.Background(), "default", "updated", "#10b981", time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatalf("seed tag approval: %v", err)
	}
}

//nolint:paralleltest // serial: shared fake cluster dataset
func TestVMDetail_HardwareOptions_TagsExcludeProtected(t *testing.T) {
	cluster.ResetFake()

	handler, authHandler, _, st := newVMDetailHandler(t)
	seedHardwareTestTag(t, st)
	cookie := adminCookie(t, authHandler)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, detailRequest(http.MethodGet, "/api/v1/vms/default/101/hardware-options", "", cookie))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var body struct {
		Tags []struct {
			Name  string `json:"name"`
			Color string `json:"color"`
		} `json:"tags"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode hardware options: %v", err)
	}

	names := make([]string, 0, len(body.Tags))
	for _, tag := range body.Tags {
		names = append(names, tag.Name)
	}

	// The picker offers admin tags but never the protected pvmss tag.
	if !slices.Contains(names, "updated") {
		t.Errorf("tags = %v, want updated offered", names)
	}

	if slices.Contains(names, "pvmss") {
		t.Errorf("tags = %v, want pvmss excluded", names)
	}
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
