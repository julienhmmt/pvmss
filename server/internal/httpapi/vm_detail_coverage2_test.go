//nolint:noctx // test scaffolding does not need real context
package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"strings"
	"testing"
)

// =============================================================================
// Unauthenticated branches for action / delete / patch handlers
// (handleAction, handleDelete, handlePatch — the Principal() error path)
// =============================================================================

// TestVmAction_Unauthenticated — POST /actions without a cookie returns 401.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVmAction_Unauthenticated(t *testing.T) {
	handler, _, _, _ := newVMDetailHandler(t)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPost, "/api/v1/vms/default/101/actions", `{"action":"start"}`, nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// TestVmDelete_Unauthenticated — DELETE without a cookie returns 401.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVmDelete_Unauthenticated(t *testing.T) {
	handler, _, _, _ := newVMDetailHandler(t)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodDelete, "/api/v1/vms/default/114", "", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// TestVmPatch_Unauthenticated — PATCH without a cookie returns 401.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVmPatch_Unauthenticated(t *testing.T) {
	handler, _, _, _ := newVMDetailHandler(t)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPatch, "/api/v1/vms/default/100", `{"name":"x"}`, nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// =============================================================================
// Invalid path (non-numeric vmid) branches — parsePath returns ok=false → 400
// =============================================================================

// detailInvalidPathRequest builds a request whose vmid path value is "abc"
// (non-numeric), so parsePath → parseIntPathValue fails → 400 invalid_request.
func detailInvalidPathRequest(method, path, body string, cookie *http.Cookie) *http.Request {
	req := detailRequest(method, path, body, cookie)
	req.SetPathValue("vmid", "abc")
	return req
}

// TestVmAction_InvalidPath — POST /actions with a non-numeric vmid returns 400.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVmAction_InvalidPath(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailInvalidPathRequest(http.MethodPost, "/api/v1/vms/default/abc/actions", `{"action":"start"}`, cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if env.Code != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", env.Code)
	}
}

// TestVmDelete_InvalidPath — DELETE with a non-numeric vmid returns 400.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVmDelete_InvalidPath(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailInvalidPathRequest(http.MethodDelete, "/api/v1/vms/default/abc", "", cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if env.Code != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", env.Code)
	}
}

// TestVmPatch_InvalidPath — PATCH with a non-numeric vmid returns 400.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVmPatch_InvalidPath(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailInvalidPathRequest(http.MethodPatch, "/api/v1/vms/default/abc", `{"name":"x"}`, cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if env.Code != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", env.Code)
	}
}

// TestVMDetail_Disk_InvalidPath — POST /disks with a non-numeric vmid returns 400.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Disk_InvalidPath(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailInvalidPathRequest(http.MethodPost, "/api/v1/vms/default/abc/disks", `{"bus":"scsi","storage":"local-lvm","sizeGB":10}`, cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if env.Code != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", env.Code)
	}
}

// TestVMDetail_CDROM_InvalidPath — PATCH /cdrom with a non-numeric vmid returns 400.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_CDROM_InvalidPath(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailInvalidPathRequest(http.MethodPatch, "/api/v1/vms/default/abc/cdrom", `{"action":"eject"}`, cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if env.Code != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", env.Code)
	}
}

// TestVMDetail_Hardware_InvalidPath — PUT /hardware with a non-numeric vmid returns 400.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Hardware_InvalidPath(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailInvalidPathRequest(http.MethodPut, "/api/v1/vms/default/abc/hardware", `{"sockets":2}`, cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if env.Code != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", env.Code)
	}
}

// TestVMDetail_EnableSerial_InvalidPath — POST /serial with a non-numeric vmid returns 400.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_EnableSerial_InvalidPath(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailInvalidPathRequest(http.MethodPost, "/api/v1/vms/default/abc/serial", "", cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if env.Code != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", env.Code)
	}
}

// TestVMDetail_Network_InvalidPath — PUT /network with a non-numeric vmid returns 400.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Network_InvalidPath(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailInvalidPathRequest(http.MethodPut, "/api/v1/vms/default/abc/network", `{"interfaces":[]}`, cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if env.Code != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", env.Code)
	}
}

// TestVMDetail_Audit_InvalidPath — GET /audit with a non-numeric vmid returns 400.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Audit_InvalidPath(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailInvalidPathRequest(http.MethodGet, "/api/v1/vms/default/abc/audit", "", cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if env.Code != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", env.Code)
	}
}

// TestVMDetail_HardwareOptions_InvalidPath — GET /hardware-options with a non-numeric vmid returns 400.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_HardwareOptions_InvalidPath(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailInvalidPathRequest(http.MethodGet, "/api/v1/vms/default/abc/hardware-options", "", cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if env.Code != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", env.Code)
	}
}

// =============================================================================
// Untagged-VM error paths for sub-handlers (Resolve → ErrNotFound → 404)
// =============================================================================

// TestVMDetail_Disk_UntaggedNotFound — disk operation on an untagged VM returns 404.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Disk_UntaggedNotFound(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailRequest(http.MethodPost, "/api/v1/vms/default/109/disks", `{"bus":"scsi","storage":"local-lvm","sizeGB":10}`, cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	if env.Code != apiCodeNotFound {
		t.Errorf("code = %q, want not_found", env.Code)
	}
}

// TestVMDetail_CDROM_UntaggedNotFound — CDROM operation on an untagged VM returns 404.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_CDROM_UntaggedNotFound(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailRequest(http.MethodPatch, "/api/v1/vms/default/109/cdrom", `{"action":"eject"}`, cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	if env.Code != apiCodeNotFound {
		t.Errorf("code = %q, want not_found", env.Code)
	}
}

// TestVMDetail_Hardware_UntaggedNotFound — hardware operation on an untagged VM returns 404.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Hardware_UntaggedNotFound(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailRequest(http.MethodPut, "/api/v1/vms/default/109/hardware", `{"sockets":2}`, cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	if env.Code != apiCodeNotFound {
		t.Errorf("code = %q, want not_found", env.Code)
	}
}

// TestVMDetail_EnableSerial_UntaggedNotFound — serial operation on an untagged VM returns 404.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_EnableSerial_UntaggedNotFound(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailRequest(http.MethodPost, "/api/v1/vms/default/109/serial", "", cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	if env.Code != apiCodeNotFound {
		t.Errorf("code = %q, want not_found", env.Code)
	}
}

// TestVMDetail_Network_UntaggedNotFound — network operation on an untagged VM returns 404.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Network_UntaggedNotFound(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailRequest(http.MethodPut, "/api/v1/vms/default/109/network", `{"interfaces":[]}`, cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	if env.Code != apiCodeNotFound {
		t.Errorf("code = %q, want not_found", env.Code)
	}
}

// TestVMDetail_Audit_UntaggedNotFound — audit on an untagged VM returns 404.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Audit_UntaggedNotFound(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailRequest(http.MethodGet, "/api/v1/vms/default/109/audit", "", cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	if env.Code != apiCodeNotFound {
		t.Errorf("code = %q, want not_found", env.Code)
	}
}

// =============================================================================
// Audit history: actor filter, admin cross-pool, pagination
// =============================================================================

// TestVMDetail_Audit_ActorFilter — the ?actor= query parameter filters entries.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Audit_ActorFilter(t *testing.T) {
	handler, authHandler, _, st := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	seedAuditEntry(t, st, "alice@pve", "default", 100, "start")
	seedAuditEntry(t, st, "bob@pve", "default", 100, "stop")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodGet, "/api/v1/vms/default/100/audit?actor=alice@pve", "", cookie))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var page struct {
		Items []struct {
			Actor string `json:"actor"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode audit page: %v", err)
	}

	if len(page.Items) == 0 {
		t.Fatal("expected at least one audit entry for alice@pve")
	}

	for _, item := range page.Items {
		if item.Actor != "alice@pve" {
			t.Errorf("actor = %q, want alice@pve (filtered)", item.Actor)
		}
	}
}

// TestVMDetail_Audit_AdminViewsAnyVM — admin can read audit for any tagged VM.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Audit_AdminViewsAnyVM(t *testing.T) {
	handler, authHandler, _, st := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	// VM 103 is in pool-bob — admin sees it.
	seedAuditEntry(t, st, "bob@pve", "default", 103, "stop")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodGet, "/api/v1/vms/default/103/audit", "", cookie))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var page struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode audit page: %v", err)
	}

	if page.Total == 0 {
		t.Fatal("expected at least one audit entry, got total=0")
	}
}

// TestVMDetail_Audit_Pagination — seeding >20 entries produces a second page.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Audit_Pagination(t *testing.T) {
	handler, authHandler, _, st := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	// Seed 25 entries so page 1 has 20 and page 2 has 5.
	for i := 0; i < 25; i++ {
		seedAuditEntry(t, st, "admin@pve", "default", 100, "start")
	}

	// Page 1
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, detailRequest(http.MethodGet, "/api/v1/vms/default/100/audit?page=1", "", cookie))
	if rec1.Code != http.StatusOK {
		t.Fatalf("page 1 status = %d, want 200: %s", rec1.Code, rec1.Body.String())
	}

	var page1 struct {
		Items    []struct{} `json:"items"`
		Total    int        `json:"total"`
		Page     int        `json:"page"`
		PageSize int        `json:"pageSize"`
	}
	if err := json.Unmarshal(rec1.Body.Bytes(), &page1); err != nil {
		t.Fatalf("decode page 1: %v", err)
	}

	if page1.Total != 25 {
		t.Errorf("page 1 total = %d, want 25", page1.Total)
	}

	if len(page1.Items) != 20 {
		t.Errorf("page 1 items = %d, want 20", len(page1.Items))
	}

	// Page 2
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, detailRequest(http.MethodGet, "/api/v1/vms/default/100/audit?page=2", "", cookie))
	if rec2.Code != http.StatusOK {
		t.Fatalf("page 2 status = %d, want 200: %s", rec2.Code, rec2.Body.String())
	}

	var page2 struct {
		Items    []struct{} `json:"items"`
		Total    int        `json:"total"`
		Page     int        `json:"page"`
		PageSize int        `json:"pageSize"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &page2); err != nil {
		t.Fatalf("decode page 2: %v", err)
	}

	if page2.Total != 25 {
		t.Errorf("page 2 total = %d, want 25", page2.Total)
	}

	if len(page2.Items) != 5 {
		t.Errorf("page 2 items = %d, want 5", len(page2.Items))
	}

	if page2.Page != 2 {
		t.Errorf("page 2 page field = %d, want 2", page2.Page)
	}
}

// =============================================================================
// Disk resize and delete success paths
// =============================================================================

// TestVMDetail_DiskResize_OwnerSuccess — owner resizes a disk on a stopped VM
// and receives the updated disk in the response (200).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_DiskResize_OwnerSuccess(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	// VM 101 is stopped, owned by alice, has scsi0 (32 GB).
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPut, "/api/v1/vms/default/101/disks/scsi0", `{"sizeGB":64}`, cookie))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var disk struct {
		Key    string `json:"key"`
		SizeGB int    `json:"sizeGB"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &disk); err != nil {
		t.Fatalf("decode disk response: %v", err)
	}

	if disk.Key != "scsi0" {
		t.Errorf("disk key = %q, want scsi0", disk.Key)
	}

	if disk.SizeGB != 64 {
		t.Errorf("disk sizeGB = %d, want 64", disk.SizeGB)
	}
}

// TestVMDetail_DiskResize_InvalidBody — malformed JSON on PUT /disks/{key} returns 400.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_DiskResize_InvalidBody(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailRequest(http.MethodPut, "/api/v1/vms/default/101/disks/scsi0", `{invalid`, cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if env.Code != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", env.Code)
	}
}

// TestVMDetail_DiskDelete_OwnerSuccess_Scsi1 — owner deletes a non-boot disk
// on a stopped VM and receives {"status":"deleted"} (200).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_DiskDelete_OwnerSuccess_Scsi1(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	// VM 101 is stopped, owned by alice, has scsi1 (10 GB, non-boot).
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodDelete, "/api/v1/vms/default/101/disks/scsi1", "", cookie))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode delete response: %v", err)
	}

	if resp.Status != testStatusDeleted {
		t.Errorf("status field = %q, want deleted", resp.Status)
	}

	calls := cluster.FakeCallsFor(101)
	var hasDelete bool
	for _, c := range calls {
		if c.Action == "delete_disk" && c.DiskKey == "scsi1" {
			hasDelete = true
		}
	}
	if !hasDelete {
		t.Errorf("fake did not record a delete_disk call for scsi1: %+v", calls)
	}
}

// =============================================================================
// Hardware exceeds limit
// =============================================================================

// TestVMDetail_Hardware_ExceedsLimit — setting cores above the gabarit max
// returns 400 with code "hardware_exceeds_limit".
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Hardware_ExceedsLimit(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	// VM 101 is stopped, owned by alice. Default maxCores is 8; request 9.
	rec, env := serveDetailError(handler, detailRequest(http.MethodPut, "/api/v1/vms/default/101/hardware", `{"cores":9}`, cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	if env.Code != "hardware_exceeds_limit" {
		t.Errorf("code = %q, want hardware_exceeds_limit", env.Code)
	}
}

// =============================================================================
// VM detail for different VM states (paused, admin viewing stopped)
// =============================================================================

// TestVMDetail_Get_PausedVm — admin can view a paused VM (VM 113, pool-shared).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Get_PausedVm(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	// VM 113 is paused, in pool-shared, tagged pvmss.
	rec, entity := serveDetail(handler, detailRequest(http.MethodGet, "/api/v1/vms/default/113", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if entity.VMID != 113 {
		t.Errorf("vmid = %d, want 113", entity.VMID)
	}

	if entity.Status != "paused" {
		t.Errorf("status = %q, want paused", entity.Status)
	}
}

// TestVMDetail_Get_AdminViewsStoppedVM — admin views a stopped VM in another
// user's pool and sees the full entity (no uptimeSeconds).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Get_AdminViewsStoppedVM(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	// VM 104 is stopped, in pool-bob, tagged pvmss.
	rec, entity := serveDetail(handler, detailRequest(http.MethodGet, "/api/v1/vms/default/104", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if entity.VMID != 104 {
		t.Errorf("vmid = %d, want 104", entity.VMID)
	}

	if entity.Status != "stopped" {
		t.Errorf("status = %q, want stopped", entity.Status)
	}

	if strings.Contains(rec.Body.String(), "uptimeSeconds") {
		t.Errorf("stopped VM response includes uptimeSeconds: %s", rec.Body.String())
	}
}

// =============================================================================
// Disk resize on a non-existent disk returns an error (not 401/403/200)
// =============================================================================

// TestVMDetail_DiskResize_NonexistentDisk — resizing a disk key that does not
// exist on the VM returns an error (disk_not_found).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_DiskResize_NonexistentDisk(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailRequest(http.MethodPut, "/api/v1/vms/default/101/disks/scsi9", `{"sizeGB":64}`, cookie))
	if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden || rec.Code == http.StatusOK {
		t.Fatalf("unexpected status %d: %s", rec.Code, rec.Body.String())
	}

	if env.Code != "disk_not_found" {
		t.Errorf("code = %q, want disk_not_found", env.Code)
	}
}

// TestVMDetail_DiskDelete_NonexistentDisk — deleting a disk key that does not
// exist returns disk_not_found.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_DiskDelete_NonexistentDisk(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailRequest(http.MethodDelete, "/api/v1/vms/default/101/disks/scsi9", "", cookie))
	if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden || rec.Code == http.StatusOK {
		t.Fatalf("unexpected status %d: %s", rec.Code, rec.Body.String())
	}

	if env.Code != "disk_not_found" {
		t.Errorf("code = %q, want disk_not_found", env.Code)
	}
}

// TestVMDetail_DiskDelete_NonOwnerForbidden — non-owner disk delete returns 403.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_DiskDelete_NonOwnerForbidden(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := bobCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailRequest(http.MethodDelete, "/api/v1/vms/default/101/disks/scsi1", "", cookie))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}

	if env.Code != apiCodeForbidden {
		t.Errorf("code = %q, want forbidden", env.Code)
	}
}

// TestVMDetail_DiskResize_NonOwnerForbidden — non-owner disk resize returns 403.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_DiskResize_NonOwnerForbidden(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := bobCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailRequest(http.MethodPut, "/api/v1/vms/default/101/disks/scsi0", `{"sizeGB":64}`, cookie))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}

	if env.Code != apiCodeForbidden {
		t.Errorf("code = %q, want forbidden", env.Code)
	}
}

// TestVMDetail_DiskResize_DiskSizeNotGreater — resizing to a size ≤ current
// returns 400 disk_size_not_greater.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_DiskResize_DiskSizeNotGreater(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	// VM 101 scsi0 is 32 GB; requesting 32 (not greater) → error.
	rec, env := serveDetailError(handler, detailRequest(http.MethodPut, "/api/v1/vms/default/101/disks/scsi0", `{"sizeGB":32}`, cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	if env.Code != "disk_size_not_greater" {
		t.Errorf("code = %q, want disk_size_not_greater", env.Code)
	}
}

// TestVMDetail_DiskResize_ExceedsLimit — resizing beyond MaxDiskPerVMGB (500)
// returns 400 disk_size_exceeds_limit.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_DiskResize_ExceedsLimit(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailRequest(http.MethodPut, "/api/v1/vms/default/101/disks/scsi0", `{"sizeGB":999}`, cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	if env.Code != "disk_size_exceeds_limit" {
		t.Errorf("code = %q, want disk_size_exceeds_limit", env.Code)
	}
}

// TestVMDetail_DiskDelete_RunningVMRejected — deleting a disk on a running VM
// returns 400 vm_not_stopped.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_DiskDelete_RunningVMRejected(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	// VM 101 is stopped with disks scsi0 and scsi1. Start it first.
	startRec := httptest.NewRecorder()
	handler.ServeHTTP(startRec, detailRequest(http.MethodPost, "/api/v1/vms/default/101/actions", `{"action":"start"}`, cookie))
	if startRec.Code != http.StatusOK {
		t.Fatalf("start VM: status = %d, want 200: %s", startRec.Code, startRec.Body.String())
	}

	// Now try to delete scsi1 on the running VM → vm_not_stopped.
	rec, env := serveDetailError(handler, detailRequest(http.MethodDelete, "/api/v1/vms/default/101/disks/scsi1", "", cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	if env.Code != "vm_not_stopped" {
		t.Errorf("code = %q, want vm_not_stopped", env.Code)
	}
}
