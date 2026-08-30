//nolint:noctx // test scaffolding does not need real context
package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"strings"
	"testing"
	"time"
)

// vmLiveStatusResponse mirrors the GET /vms/:cluster/:vmid/status 200 body.
type vmLiveStatusResponse struct {
	Status string `json:"status"`
	Lock   string `json:"lock,omitempty"`
	Uptime int64  `json:"uptime"`
}

// statusBatchResponse mirrors the POST /vms/status 200 body — a bare array
// per ticket 01b.
type statusBatchResponse = []statusBatchItem

type statusBatchItem struct {
	Cluster string `json:"cluster"`
	VMID    int    `json:"vmid"`
	Status  string `json:"status"`
	Lock    string `json:"lock,omitempty"`
	Uptime  int64  `json:"uptime"`
}

// =============================================================================
// GET /vms/:cluster/:vmid/status — live status read (ticket 01b)
// =============================================================================

//nolint:paralleltest // serial: shared fake authentication state
func TestVMDetail_Status_OwnerSeesLiveStatus(t *testing.T) {
	handler, auth, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, auth)

	req := detailRequest(http.MethodGet, "/api/v1/vms/default/100/status", "", cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body vmLiveStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body.Status != "running" {
		t.Errorf("status = %q, want %q", body.Status, "running")
	}
}

//nolint:paralleltest // serial: shared fake authentication state
func TestVMDetail_Status_ReflectsActionImmediately(t *testing.T) {
	handler, auth, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, auth)

	// VM 100 is running in the fake dataset. Stop it via the action endpoint,
	// then immediately read live status — it should be "stopped" without
	// waiting for the 30s inventory tick.
	actionReq := detailRequest(http.MethodPost, "/api/v1/vms/default/100/actions",
		`{"action":"stop"}`, cookie)
	actionRec := httptest.NewRecorder()
	handler.ServeHTTP(actionRec, actionReq)

	if actionRec.Code != http.StatusOK {
		t.Fatalf("action status = %d, want %d", actionRec.Code, http.StatusOK)
	}

	statusReq := detailRequest(http.MethodGet, "/api/v1/vms/default/100/status", "", cookie)
	statusRec := httptest.NewRecorder()
	handler.ServeHTTP(statusRec, statusReq)

	if statusRec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", statusRec.Code, http.StatusOK)
	}

	var body vmLiveStatusResponse
	if err := json.Unmarshal(statusRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body.Status != "stopped" {
		t.Errorf("live status = %q, want %q (should reflect the stop immediately)", body.Status, "stopped")
	}
}

//nolint:paralleltest // serial: shared fake authentication state
func TestVMDetail_Status_ForbiddenForNonOwner(t *testing.T) {
	handler, auth, _, _ := newVMDetailHandler(t)
	// Bob does not own VM 100 (alice does).
	cookie := bobCookie(t, auth)

	req := detailRequest(http.MethodGet, "/api/v1/vms/default/100/status", "", cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

//nolint:paralleltest // serial: shared fake authentication state
func TestVMDetail_Status_NotFoundForUnknownVM(t *testing.T) {
	handler, auth, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, auth)

	req := detailRequest(http.MethodGet, "/api/v1/vms/default/99999/status", "", cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// =============================================================================
// POST /vms/status — batch live status read (ticket 01b)
// =============================================================================

//nolint:paralleltest // serial: shared fake authentication state
func TestVMStatusBatch_ReturnsLiveStatusForMultipleVMs(t *testing.T) {
	t.Cleanup(cluster.ResetFake)

	snap, err := (cluster.Fake{}).Snapshot(t.Context())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	projection := buildProjectionWithIndex(t, snap, time.Now())
	authHandler := newAuthHandler(t)

	handler := httpapi.NewVMStatusBatchSingle(projection, authHandler, cluster.Fake{}, testLogger(t))

	cookie := aliceCookie(t, authHandler)

	body := `[{"cluster":"default","vmid":100},{"cluster":"default","vmid":101}]`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vms/status", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp statusBatchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp) != 2 {
		t.Fatalf("expected 2 results, got %d", len(resp))
	}

	for _, item := range resp {
		if item.Status == "" {
			t.Errorf("VM %d: empty status", item.VMID)
		}
	}
}

//nolint:paralleltest // serial: shared fake authentication state
func TestVMStatusBatch_EmptyTargetsRejected(t *testing.T) {
	t.Cleanup(cluster.ResetFake)

	authHandler := newAuthHandler(t)
	handler := httpapi.NewVMStatusBatchSingle(nil, authHandler, cluster.Fake{}, testLogger(t))

	cookie := aliceCookie(t, authHandler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/vms/status", strings.NewReader(`[]`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

//nolint:paralleltest // serial: shared fake authentication state
func TestVMStatusBatch_UnknownVMOmitted(t *testing.T) {
	t.Cleanup(cluster.ResetFake)

	snap, err := (cluster.Fake{}).Snapshot(t.Context())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	projection := buildProjectionWithIndex(t, snap, time.Now())
	authHandler := newAuthHandler(t)

	handler := httpapi.NewVMStatusBatchSingle(projection, authHandler, cluster.Fake{}, testLogger(t))

	cookie := aliceCookie(t, authHandler)

	// VM 100 exists, 99999 does not — the unknown one should be omitted, not
	// cause a failure.
	body := `[{"cluster":"default","vmid":100},{"cluster":"default","vmid":99999}]`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vms/status", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp statusBatchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp) != 1 {
		t.Fatalf("expected 1 result (unknown VM omitted), got %d", len(resp))
	}

	if resp[0].VMID != 100 {
		t.Errorf("VMID = %d, want 100", resp[0].VMID)
	}
}

//nolint:paralleltest // serial: shared fake authentication state
func TestVMStatusBatch_RequiresAuth(t *testing.T) {
	t.Cleanup(cluster.ResetFake)

	authHandler := newAuthHandler(t)
	handler := httpapi.NewVMStatusBatchSingle(nil, authHandler, cluster.Fake{}, testLogger(t))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/vms/status", strings.NewReader(`[{"cluster":"default","vmid":100}]`))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
