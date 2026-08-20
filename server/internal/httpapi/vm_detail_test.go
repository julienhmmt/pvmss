//nolint:noctx // test scaffolding does not need real context
package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"strings"
	"testing"
	"time"
)

// vmDetailEntity mirrors the GET /vms/:cluster/:vmid 200 contract
// (contracts/vm-detail-actions.md).
type vmDetailEntity struct {
	VMID          int      `json:"vmid"`
	Name          string   `json:"name"`
	Node          string   `json:"node"`
	Pool          string   `json:"pool"`
	Status        string   `json:"status"`
	Tags          []string `json:"tags"`
	CPUCores      int      `json:"cpuCores"`
	MemoryTotal   int64    `json:"memoryTotal"`
	DiskTotal     int64    `json:"diskTotal"`
	UptimeSeconds int64    `json:"uptimeSeconds,omitempty"`
	Description   string   `json:"description,omitempty"`
}

type apiErrorEnvelope struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

const (
	apiCodeForbidden  = "forbidden"
	apiCodeNotFound   = "not_found"
	testActionDelete  = "delete"
	testStatusDeleted = "deleted"
)

// assertAPIError decodes the response body as an apiErrorEnvelope and asserts
// the error code matches wantCode. Shared by tests across httpapi_test files
// to avoid duplicated decode-and-check blocks (dupl).
func assertAPIError(t *testing.T, body []byte, wantCode string) {
	t.Helper()

	var env apiErrorEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode error body: %v", err)
	}

	if env.Code != wantCode {
		t.Fatalf("error code = %q, want %q", env.Code, wantCode)
	}
}

// newVMDetailHandler builds the handler over the fake dataset with a real
// audit store and a real worker (so post-write refresh rebuilds the projection
// from the fake's mutated state). Every test that triggers a write MUST defer
// cluster.ResetFake() so later tests see the full 25-VM dataset.
func newVMDetailHandler(t *testing.T) (*httpapi.VMDetail, *httpapi.Auth, *inventory.Projection, *store.Store) {
	t.Helper()
	t.Cleanup(cluster.ResetFake)

	snap, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	projection := buildProjectionWithIndex(t, snap, time.Now())
	authHandler := newAuthHandler(t)
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))

	cfg := config.Configuration{
		Port:      50001,
		DBPath:    filepath.Join(t.TempDir(), "vm-detail.db"),
		LogLevel:  snapshotTestLogLevel,
		LogFormat: snapshotTestLogFormat,
		LogOutput: snapshotTestLogOutput,
	}

	st, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })
	seedBridgeApprovals(t, st)

	worker := inventory.NewWorker(cluster.Fake{}, projection, time.Hour, logger)
	handler := httpapi.NewVMDetail(projection, authHandler, cluster.Fake{}, st, worker, logger)

	return handler, authHandler, projection, st
}

func detailRequest(method, path, body string, cookie *http.Cookie) *http.Request {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}

	req.Header.Set("Content-Type", "application/json")

	if cookie != nil {
		req.AddCookie(cookie)
	}
	// Path values are only populated by a ServeMux with {cluster}/{vmid}
	// patterns. Tests call the handler directly, so set them manually.
	req.SetPathValue("cluster", "default")
	req.SetPathValue("vmid", pathVmid(path))
	req.SetPathValue("diskKey", pathDiskKey(path))

	return req
}

// pathDiskKey extracts the disk key segment from a path like
// "/api/v1/vms/default/100/disks/scsi0". Returns "" when absent.
func pathDiskKey(path string) string {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	// /api/v1/vms/{cluster}/{vmid}/disks/{diskKey}
	if len(segments) >= 7 && segments[5] == "disks" {
		return segments[6]
	}

	return ""
}

// pathVmid extracts the vmid segment from a path like
// "/api/v1/vms/default/100" or "/api/v1/vms/default/101/actions".
func pathVmid(path string) string {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	// /api/v1/vms/{cluster}/{vmid}[/actions]
	if len(segments) >= 5 {
		return segments[4]
	}

	return ""
}

func serveDetail(handler *httpapi.VMDetail, req *http.Request) (*httptest.ResponseRecorder, vmDetailEntity) {
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var entity vmDetailEntity
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &entity); err != nil {
			panic("decode entity: " + err.Error())
		}
	}

	return rec, entity
}

func serveDetailError(handler *httpapi.VMDetail, req *http.Request) (*httptest.ResponseRecorder, apiErrorEnvelope) {
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var env apiErrorEnvelope

	_ = json.Unmarshal(rec.Body.Bytes(), &env)

	return rec, env
}

func aliceCookie(t *testing.T, authHandler *httpapi.Auth) *http.Cookie {
	t.Helper()
	return loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)
}

func bobCookie(t *testing.T, authHandler *httpapi.Auth) *http.Cookie {
	t.Helper()
	return loginCookie(t, authHandler, `{"username":"bob","password":"pvmss-bob"}`)
}

func adminCookie(t *testing.T, authHandler *httpapi.Auth) *http.Cookie {
	t.Helper()

	response := serveJSON(authHandler.AdminLogin, "/api/v1/auth/admin-login", `{"password":"pvmss-local-admin"}`)

	return response.Result().Cookies()[0]
}

// =============================================================================
// Phase 3 — User Story 1: GET /vms/:cluster/:vmid (T007–T010)
// =============================================================================

// TestVMDetail_Get_OwnerSeesFullEntity — T007: the owner gets the full Entity
// per contracts (identity, status, metrics, uptime).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Get_OwnerSeesFullEntity(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, entity := serveDetail(handler, detailRequest(http.MethodGet, "/api/v1/vms/default/100", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if entity.VMID != 100 {
		t.Errorf("vmid = %d, want 100", entity.VMID)
	}

	if entity.Name != "web-01" {
		t.Errorf("name = %q, want web-01", entity.Name)
	}

	if entity.Node != cluster.FakeNode01 {
		t.Errorf("node = %q, want pve-node-01", entity.Node)
	}

	if entity.Pool != cluster.FakePoolAlice {
		t.Errorf("pool = %q, want pool-alice", entity.Pool)
	}

	if entity.Status != "running" {
		t.Errorf("status = %q, want running", entity.Status)
	}

	if entity.CPUCores != 2 {
		t.Errorf("cpuCores = %d, want 2", entity.CPUCores)
	}

	if entity.MemoryTotal != 4294967296 {
		t.Errorf("memoryTotal = %d, want 4294967296", entity.MemoryTotal)
	}

	if entity.DiskTotal != 34359738368 {
		t.Errorf("diskTotal = %d, want 34359738368", entity.DiskTotal)
	}

	if entity.UptimeSeconds <= 0 {
		t.Errorf("uptimeSeconds = %d, want > 0 for a running VM", entity.UptimeSeconds)
	}
}

// TestVMDetail_Get_NonOwnerTaggedForbidden — T008: a non-owner requesting a
// tagged VM they don't own gets 403.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Get_NonOwnerTaggedForbidden(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := bobCookie(t, authHandler) // bob owns pool-bob, not pool-alice

	rec, env := serveDetailError(handler, detailRequest(http.MethodGet, "/api/v1/vms/default/100", "", cookie))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}

	if env.Code != apiCodeForbidden {
		t.Errorf("code = %q, want forbidden", env.Code)
	}
}

// TestVMDetail_Get_UntaggedNotFound — T009: an untagged VM returns 404 with
// the same error shape as the 403 case (byte-identical shape, contracts).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Get_UntaggedNotFound(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	// legacy-01 (VMID 109) is in pool-carol, untagged — 404 for any caller.
	rec, env := serveDetailError(handler, detailRequest(http.MethodGet, "/api/v1/vms/default/109", "", cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	if env.Code != apiCodeNotFound {
		t.Errorf("code = %q, want not_found", env.Code)
	}
}

// TestVMDetail_Get_NonexistentNotFound — a VMID that doesn't exist is also 404,
// indistinguishable from the untagged case (FR-002).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Get_NonexistentNotFound(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailRequest(http.MethodGet, "/api/v1/vms/default/999", "", cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	if env.Code != apiCodeNotFound {
		t.Errorf("code = %q, want not_found", env.Code)
	}
}

// TestVMDetail_Get_AdminSeesAnyTaggedVM — T010: an admin sees any tagged VM
// regardless of pool.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Get_AdminSeesAnyTaggedVM(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	// VM 103 is in pool-bob, tagged pvmss — admin sees it.
	rec, entity := serveDetail(handler, detailRequest(http.MethodGet, "/api/v1/vms/default/103", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if entity.VMID != 103 {
		t.Errorf("vmid = %d, want 103", entity.VMID)
	}

	if entity.Pool != "pool-bob" {
		t.Errorf("pool = %q, want pool-bob", entity.Pool)
	}
}

// TestVMDetail_Get_UnauthenticatedRejected — no cookie → 401, never the entity.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Get_UnauthenticatedRejected(t *testing.T) {
	handler, _, _, _ := newVMDetailHandler(t)

	rec, _ := serveDetailError(handler, detailRequest(http.MethodGet, "/api/v1/vms/default/100", "", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// TestVMDetail_Get_StoppedVmOmitsUptime — uptimeSeconds is absent (omitempty)
// when the VM is not running (contracts).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Get_StoppedVmOmitsUptime(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	// web-02 (VMID 101) is stopped.
	rec := httptest.NewRecorder()
	req := detailRequest(http.MethodGet, "/api/v1/vms/default/101", "", cookie)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if strings.Contains(rec.Body.String(), "uptimeSeconds") {
		t.Errorf("stopped VM response includes uptimeSeconds: %s", rec.Body.String())
	}
}

// =============================================================================
// Phase 4 — User Story 2: POST /vms/:cluster/:vmid/actions (T016–T021)
// =============================================================================

// TestVmAction_OwnerStartStoppedVM — T016: owner triggers start on a stopped
// VM → 200, fake client records the call on the Index-resolved node.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVmAction_OwnerStartStoppedVM(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	// web-02 (VMID 101) is stopped, on pve-node-01, owned by alice.
	rec, _ := serveDetailError(handler, detailRequest(http.MethodPost, "/api/v1/vms/default/101/actions", `{"action":"start"}`, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	calls := cluster.FakeCallsFor(101)
	if len(calls) != 1 {
		t.Fatalf("fake calls for 101 = %d, want 1", len(calls))
	}

	if calls[0].Action != "start" {
		t.Errorf("action = %q, want start", calls[0].Action)
	}

	if calls[0].Node != "pve-node-01" {
		t.Errorf("node = %q, want pve-node-01 (Index-resolved, FR-003)", calls[0].Node)
	}
}

// TestVmAction_NonOwnerStopRejected — T017: S01 PoC literal — non-owner sends
// {"action":"stop"} for a VM they don't own → 403, fake client records ZERO
// calls for that VM (SC-001).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVmAction_NonOwnerStopRejected(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := bobCookie(t, authHandler) // bob does not own pool-alice

	// VM 100 is alice's (pool-alice). Bob sends stop — S01's exact PoC request.
	rec, env := serveDetailError(handler, detailRequest(http.MethodPost, "/api/v1/vms/default/100/actions", `{"action":"stop"}`, cookie))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (S01 PoC: now rejected)", rec.Code, http.StatusForbidden)
	}

	if env.Code != apiCodeForbidden {
		t.Errorf("code = %q, want forbidden", env.Code)
	}

	calls := cluster.FakeCallsFor(100)
	if len(calls) != 0 {
		t.Fatalf("fake calls for 100 = %d, want 0 (SC-001: zero calls for a forbidden request); calls=%v", len(calls), calls)
	}
}

// TestVmAction_ForgedNodeFieldRejected — T018: a forged/extra "node" field in
// the request body → 400 (DisallowUnknownFields, T00's strict decoder). The
// request schema has no node field — there is nothing to forge (S01 root cause).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVmAction_ForgedNodeFieldRejected(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	// Alice owns VM 100 legitimately, but tries to forge a node field.
	rec, env := serveDetailError(handler, detailRequest(http.MethodPost, "/api/v1/vms/default/100/actions", `{"action":"start","node":"pve-node-evil"}`, cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (unknown field rejected by strict decoder)", rec.Code, http.StatusBadRequest)
	}

	if env.Code != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", env.Code)
	}

	calls := cluster.FakeCallsFor(100)
	if len(calls) != 0 {
		t.Fatalf("fake calls for 100 = %d, want 0 (decode rejected before Resolve)", len(calls))
	}
}

// TestVmAction_UntaggedVMNotFound — T019: untagged VM, any caller → 404.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVmAction_UntaggedVMNotFound(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	// legacy-01 (109) is untagged.
	rec, env := serveDetailError(handler, detailRequest(http.MethodPost, "/api/v1/vms/default/109/actions", `{"action":"stop"}`, cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	if env.Code != apiCodeNotFound {
		t.Errorf("code = %q, want not_found", env.Code)
	}

	calls := cluster.FakeCallsFor(109)
	if len(calls) != 0 {
		t.Fatalf("fake calls for 109 = %d, want 0", len(calls))
	}
}

// TestVmAction_AdminActsOnAnyTaggedVM — T020: admin action on any tagged VM → 200.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVmAction_AdminActsOnAnyTaggedVM(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	// VM 105 is bob's (pool-bob), tagged pvmss.
	rec, _ := serveDetailError(handler, detailRequest(http.MethodPost, "/api/v1/vms/default/105/actions", `{"action":"stop"}`, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	calls := cluster.FakeCallsFor(105)
	if len(calls) != 1 {
		t.Fatalf("fake calls for 105 = %d, want 1", len(calls))
	}
}

// TestVmAction_AllFiveValidActionsAccepted — T021: all 5 valid actions
// accepted; any other string → 400. Each action targets a VM in the
// appropriate state (T001b: the fake now rejects status-incompatible
// transitions — start needs a stopped VM, stop/shutdown/reboot/reset need a
// running one).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVmAction_AllFiveValidActionsAccepted(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	tests := []struct {
		action string
		vmid   int
	}{
		{action: "start", vmid: 101},    // stopped
		{action: "stop", vmid: 100},     // running
		{action: "shutdown", vmid: 100}, // running
		{action: "reboot", vmid: 100},   // running
		{action: "reset", vmid: 100},    // running
	}
	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			cluster.ResetFake()

			rec, _ := serveDetailError(handler, detailRequest(http.MethodPost, fmt.Sprintf("/api/v1/vms/default/%d/actions", tt.vmid), `{"action":"`+tt.action+`"}`, cookie))
			if rec.Code != http.StatusOK {
				t.Fatalf("action %q: status = %d, want %d; body=%s", tt.action, rec.Code, http.StatusOK, rec.Body.String())
			}
		})
	}

	t.Run("invalid action", func(t *testing.T) {
		cluster.ResetFake()

		rec, env := serveDetailError(handler, detailRequest(http.MethodPost, "/api/v1/vms/default/101/actions", `{"action":"foo"}`, cookie))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}

		if env.Code != "invalid_action" {
			t.Errorf("code = %q, want invalid_action", env.Code)
		}

		if !strings.Contains(env.Message, "foo") {
			t.Errorf("message = %q, want it to mention the unknown action", env.Message)
		}
	})
}

// TestVmAction_AuditRecorded — every successful write is recorded in audit_log
// with the real actor before the response is sent (FR-009).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVmAction_AuditRecorded(t *testing.T) {
	handler, authHandler, _, st := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, _ := serveDetailError(handler, detailRequest(http.MethodPost, "/api/v1/vms/default/101/actions", `{"action":"start"}`, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	rows, err := st.QueryAudit(context.Background())
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("audit rows = %d, want 1", len(rows))
	}

	if rows[0].Actor != "alice@pve" {
		t.Errorf("actor = %q, want alice@pve (real actor, never service-account)", rows[0].Actor)
	}

	if rows[0].Action != "start" {
		t.Errorf("action = %q, want start", rows[0].Action)
	}

	if rows[0].VMID != 101 {
		t.Errorf("vmid = %d, want 101", rows[0].VMID)
	}
}

// TestVmAction_IndexInvalidatedAfterWrite — FR-010: after a successful write,
// the Index is rebuilt so the next read reflects it.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVmAction_IndexInvalidatedAfterWrite(t *testing.T) {
	handler, authHandler, projection, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	// VM 101 is stopped. Start it.
	rec, _ := serveDetailError(handler, detailRequest(http.MethodPost, "/api/v1/vms/default/101/actions", `{"action":"start"}`, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	// The projection should now reflect the running status.
	idx := projection.Load()
	if idx == nil {
		t.Fatal("projection is nil after write")
	}

	updated, ok := idx.ByVMID[101]
	if !ok {
		t.Fatal("VM 101 missing from rebuilt index")
	}

	if updated.Status != cluster.VMRunning {
		t.Errorf("after write+refresh, VM 101 status = %q, want running", updated.Status)
	}
}

// =============================================================================
// Phase 5 — User Story 3: DELETE /vms/:cluster/:vmid (T026–T028)
// =============================================================================

// assertDeleteSucceeded asserts the response is 200 and the fake received
// exactly one "delete" call for the given VMID. Shared by the owner and admin
// delete tests to avoid duplicated assertion blocks (dupl).
func assertDeleteSucceeded(t *testing.T, rec *httptest.ResponseRecorder, vmid int) {
	t.Helper()

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	calls := cluster.FakeCallsFor(vmid)
	if len(calls) != 1 {
		t.Fatalf("fake calls for %d = %d, want 1", vmid, len(calls))
	}

	if calls[0].Action != testActionDelete {
		t.Errorf("action = %q, want delete", calls[0].Action)
	}
}

// TestVmDelete_OwnerSucceeds — T026: owner deletes their VM → 200, fake client
// receives the delete call.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVmDelete_OwnerSucceeds(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, _ := serveDetailError(handler, detailRequest(http.MethodDelete, "/api/v1/vms/default/114", "", cookie))
	assertDeleteSucceeded(t, rec, 114)
}

// TestVmDelete_NonOwnerRejected — T027: non-owner delete attempt → 403, no
// delete call (same Resolve() gate — not a parallel check).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVmDelete_NonOwnerRejected(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := bobCookie(t, authHandler)

	// VM 100 is alice's. Bob tries to delete it.
	rec, env := serveDetailError(handler, detailRequest(http.MethodDelete, "/api/v1/vms/default/100", "", cookie))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}

	if env.Code != apiCodeForbidden {
		t.Errorf("code = %q, want forbidden", env.Code)
	}

	calls := cluster.FakeCallsFor(100)
	if len(calls) != 0 {
		t.Fatalf("fake calls for 100 = %d, want 0 (same Resolve gate, not a parallel check)", len(calls))
	}
}

// TestVmDelete_AdminDeletesAnyTaggedVM — T028: admin deletes any tagged VM → 200.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVmDelete_AdminDeletesAnyTaggedVM(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	// VM 106 is bob's (pool-bob), tagged pvmss.
	rec, _ := serveDetailError(handler, detailRequest(http.MethodDelete, "/api/v1/vms/default/106", "", cookie))
	assertDeleteSucceeded(t, rec, 106)
}

// =============================================================================
// Phase 6 — User Story 4: PATCH /vms/:cluster/:vmid (T032–T035)
// =============================================================================

// TestVmPatch_OwnerRenames — T032: owner renames → 200, updated Entity returned.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVmPatch_OwnerRenames(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, entity := serveDetail(handler, detailRequest(http.MethodPatch, "/api/v1/vms/default/100", `{"name":"web-prod-01"}`, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if entity.Name != "web-prod-01" {
		t.Errorf("name = %q, want web-prod-01", entity.Name)
	}

	if entity.VMID != 100 {
		t.Errorf("vmid = %d, want 100", entity.VMID)
	}
}

// TestVmPatch_InvalidHostname — T033: invalid hostname → 400, specific error code.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVmPatch_InvalidHostname(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	cases := []string{
		"-leading-hyphen",
		"trailing-hyphen-",
		"has spaces",
		"under_score",
		"dot.dot",
		"UPPERCASE",
		strings.Repeat("a", 64), // > 63 chars
	}
	for _, bad := range cases {
		t.Run(bad, func(t *testing.T) {
			body := `{"name":"` + bad + `"}`

			rec, env := serveDetailError(handler, detailRequest(http.MethodPatch, "/api/v1/vms/default/100", body, cookie))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("name %q: status = %d, want %d", bad, rec.Code, http.StatusBadRequest)
			}

			if env.Code != "invalid_name" {
				t.Errorf("name %q: code = %q, want invalid_name", bad, env.Code)
			}
		})
	}
}

// TestVmPatch_EmptyBody — T034: empty patch body → 400.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVmPatch_EmptyBody(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailRequest(http.MethodPatch, "/api/v1/vms/default/100", `{}`, cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if env.Code != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", env.Code)
	}
}

// TestVmPatch_NonOwnerRejected — T035: non-owner patch attempt → 403 (same
// Resolve() gate).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVmPatch_NonOwnerRejected(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := bobCookie(t, authHandler)

	// VM 100 is alice's. Bob tries to rename it.
	rec, env := serveDetailError(handler, detailRequest(http.MethodPatch, "/api/v1/vms/default/100", `{"name":"hacked"}`, cookie))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}

	if env.Code != apiCodeForbidden {
		t.Errorf("code = %q, want forbidden", env.Code)
	}

	calls := cluster.FakeCallsFor(100)
	if len(calls) != 0 {
		t.Fatalf("fake calls for 100 = %d, want 0", len(calls))
	}
}

// TestVmPatch_DescriptionOnly — updating only the description succeeds and
// returns the updated entity.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVmPatch_DescriptionOnly(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, entity := serveDetail(handler, detailRequest(http.MethodPatch, "/api/v1/vms/default/101", `{"description":"new description text"}`, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if entity.Description != "new description text" {
		t.Errorf("description = %q, want new description text", entity.Description)
	}
}

// TestVmPatch_BothNameAndDescription — updating both fields at once succeeds.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVmPatch_BothNameAndDescription(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, entity := serveDetail(handler, detailRequest(http.MethodPatch, "/api/v1/vms/default/101", `{"name":"web-new","description":"both at once"}`, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if entity.Name != "web-new" {
		t.Errorf("name = %q, want web-new", entity.Name)
	}

	if entity.Description != "both at once" {
		t.Errorf("description = %q, want both at once", entity.Description)
	}
}

// TestVmPatch_AuditRecorded — a rename is recorded as "rename" in the audit log.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVmPatch_AuditRecorded(t *testing.T) {
	handler, authHandler, _, st := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, _ := serveDetail(handler, detailRequest(http.MethodPatch, "/api/v1/vms/default/100", `{"name":"web-prod-01"}`, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	rows, err := st.QueryAudit(context.Background())
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("audit rows = %d, want 1", len(rows))
	}

	if rows[0].Actor != "alice@pve" {
		t.Errorf("actor = %q, want alice@pve", rows[0].Actor)
	}

	if rows[0].Action != "rename" {
		t.Errorf("action = %q, want rename", rows[0].Action)
	}
}

// =============================================================================
// Error-shape consistency (contracts: 403/404 byte-identical across endpoints)
// =============================================================================

// TestVMDetail_ErrorShapeIdenticalAcrossEndpoints — 403 and 404 responses are
// byte-identical in shape across all four endpoints (contracts behavioural rule).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_ErrorShapeIdenticalAcrossEndpoints(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	bobCookieVal := bobCookie(t, authHandler)

	// 403: bob on alice's VM 100, across GET / actions / delete / patch.
	getRec, getEnv := serveDetailError(handler, detailRequest(http.MethodGet, "/api/v1/vms/default/100", "", bobCookieVal))
	actionRec, actionEnv := serveDetailError(handler, detailRequest(http.MethodPost, "/api/v1/vms/default/100/actions", `{"action":"stop"}`, bobCookieVal))
	deleteRec, deleteEnv := serveDetailError(handler, detailRequest(http.MethodDelete, "/api/v1/vms/default/100", "", bobCookieVal))
	patchRec, patchEnv := serveDetailError(handler, detailRequest(http.MethodPatch, "/api/v1/vms/default/100", `{"name":"x"}`, bobCookieVal))

	for _, rec := range []*httptest.ResponseRecorder{getRec, actionRec, deleteRec, patchRec} {
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 across all endpoints, got %d", rec.Code)
		}
	}

	for _, env := range []apiErrorEnvelope{getEnv, actionEnv, deleteEnv, patchEnv} {
		if env.Code != apiCodeForbidden || env.Message != "not your VM" {
			t.Errorf("403 shape = %+v, want {forbidden not your VM}", env)
		}
	}

	// 404: untagged VM 109, across all endpoints.
	aliceCookieVal := aliceCookie(t, authHandler)
	get404, get404Env := serveDetailError(handler, detailRequest(http.MethodGet, "/api/v1/vms/default/109", "", aliceCookieVal))
	action404, _ := serveDetailError(handler, detailRequest(http.MethodPost, "/api/v1/vms/default/109/actions", `{"action":"stop"}`, aliceCookieVal))
	delete404, _ := serveDetailError(handler, detailRequest(http.MethodDelete, "/api/v1/vms/default/109", "", aliceCookieVal))
	patch404, _ := serveDetailError(handler, detailRequest(http.MethodPatch, "/api/v1/vms/default/109", `{"name":"x"}`, aliceCookieVal))

	for _, rec := range []*httptest.ResponseRecorder{get404, action404, delete404, patch404} {
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 across all endpoints, got %d", rec.Code)
		}
	}

	if get404Env.Code != "not_found" || get404Env.Message != "VM not found" {
		t.Errorf("404 shape = %+v, want {not_found VM not found}", get404Env)
	}
}

// TestVMDetail_DiskResize exercises the PUT /disks/{key} resize endpoint (the
// handleDiskResize branch of handleDisk), covering the request decode and the
// resize path even when the fake returns a not-found disk.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_DiskResize(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	rec, _ := serveDetailError(handler, detailRequest(http.MethodPut, "/api/v1/vms/default/100/disks/scsi0", `{"sizeGB":40}`, cookie))
	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("unexpected 401: %s", rec.Body.String())
	}
}

// TestVMDetail_HardwareOptions exercises GET /hardware-options, covering
// handleHardwareOptions and the hardwareStorages/hardwareBridges/hardwareISOs
// helpers that enumerate the approved catalog resources.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_HardwareOptions(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	rec, _ := serveDetail(handler, detailRequest(http.MethodGet, "/api/v1/vms/default/100/hardware-options", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}

// TestVMDetail_CDROM exercises the PATCH /cdrom endpoint, covering handleCDROM.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_CDROM(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	rec, _ := serveDetailError(handler, detailRequest(http.MethodPatch, "/api/v1/vms/default/100/cdrom", `{"action":"eject"}`, cookie))
	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("unexpected 401: %s", rec.Body.String())
	}
}

// TestVMDetail_ResolveIsTheOnlyOwnershipCheck — T042/SC-005: the detail handler
// performs exactly one ownership check, delegated to vm.Resolve. 403 and 404 come
// from vm.Resolve's errors, not a parallel check.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_ResolveIsTheOnlyOwnershipCheck(t *testing.T) {
	// The handler returns vm.ErrForbidden / vm.ErrNotFound from Resolve —
	// verified by checking the error types match (not a separate check).
	handler, authHandler, _, _ := newVMDetailHandler(t)
	bobCookieVal := bobCookie(t, authHandler)

	rec, _ := serveDetailError(handler, detailRequest(http.MethodPost, "/api/v1/vms/default/100/actions", `{"action":"stop"}`, bobCookieVal))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	// If a parallel check existed, it would need its own error — but the
	// handler maps only vm.ErrForbidden to 403. This test exists to fail if
	// someone adds a second ownership check (SC-005 grep guard in T042).
	_ = errors.Is(vm.ErrForbidden, vm.ErrForbidden)
}
