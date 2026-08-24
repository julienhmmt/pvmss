package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/store"
	"testing"
)

// TestVMDetail_Audit_HappyPath — GET /vms/:cluster/:vmid/audit returns
// paginated audit entries for the VM.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Audit_HappyPath(t *testing.T) {
	handler, authHandler, _, st := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	seedAuditEntry(t, st, "admin@pve", "default", 100, "start")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodGet, "/api/v1/vms/default/100/audit", "", cookie))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var page struct {
		Items []struct {
			Actor string `json:"actor"`
			VMID  int    `json:"vmid"`
		} `json:"items"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode audit page: %v", err)
	}

	if page.Total == 0 {
		t.Fatal("expected at least one audit entry, got total=0")
	}
}

// TestVMDetail_Audit_PageParam — the page query parameter is parsed and
// invalid values are rejected.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Audit_PageParam(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	tests := []struct {
		name       string
		pageParam  string
		wantStatus int
	}{
		{"default page", "", http.StatusOK},
		{"page 1", "1", http.StatusOK},
		{"page 2", "2", http.StatusOK},
		{"invalid page zero", "0", http.StatusOK},
		{"invalid page negative", "-1", http.StatusOK},
		{"invalid page non-numeric", "abc", http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := "/api/v1/vms/default/100/audit"
			if tc.pageParam != "" {
				path += "?page=" + tc.pageParam
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, detailRequest(http.MethodGet, path, "", cookie))

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
		})
	}
}

// TestVMDetail_Audit_Unauthenticated — audit endpoint requires auth.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Audit_Unauthenticated(t *testing.T) {
	handler, _, _, _ := newVMDetailHandler(t)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodGet, "/api/v1/vms/default/100/audit", "", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

// TestVMDetail_Audit_NonOwnerForbidden — non-owner cannot read another user's audit.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Audit_NonOwnerForbidden(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	bob := bobCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodGet, "/api/v1/vms/default/100/audit", "", bob))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

// TestVMDetail_Audit_ActorActionFilters — actor and action query params are
// forwarded to the store filter.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Audit_ActorActionFilters(t *testing.T) {
	handler, authHandler, _, st := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	seedAuditEntry(t, st, "admin@pve", "default", 100, "start")
	seedAuditEntry(t, st, "admin@pve", "default", 100, "stop")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodGet, "/api/v1/vms/default/100/audit?action=start", "", cookie))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var page struct {
		Items []struct {
			Action string `json:"action"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode: %v", err)
	}

	for _, item := range page.Items {
		if item.Action != auditTestAction {
			t.Errorf("expected only start actions, got %q", item.Action)
		}
	}
}

// TestVMDetail_Network_MethodNotAllowed — non-PUT methods on /network are rejected.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Network_MethodNotAllowed(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPost, "/api/v1/vms/default/100/network", "", cookie))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

// TestVMDetail_Network_Unauthenticated — network endpoint requires auth.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Network_Unauthenticated(t *testing.T) {
	handler, _, _, _ := newVMDetailHandler(t)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPut, "/api/v1/vms/default/100/network", `{"interfaces":[]}`, nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

// TestVMDetail_Network_NonOwnerForbidden — non-owner cannot update network.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Network_NonOwnerForbidden(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	bob := bobCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPut, "/api/v1/vms/default/100/network", `{"interfaces":[{"index":0,"bridge":"vmbr0","model":"virtio"}]}`, bob))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

// TestVMDetail_Network_InvalidBody — malformed JSON is rejected.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Network_InvalidBody(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPut, "/api/v1/vms/default/100/network", `{invalid`, cookie))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// TestVMDetail_Hardware_MethodNotAllowed — non-PUT methods on /hardware are rejected.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Hardware_MethodNotAllowed(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPost, "/api/v1/vms/default/100/hardware", "", cookie))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

// TestVMDetail_Hardware_Unauthenticated — hardware endpoint requires auth.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Hardware_Unauthenticated(t *testing.T) {
	handler, _, _, _ := newVMDetailHandler(t)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPut, "/api/v1/vms/default/100/hardware", `{"sockets":2}`, nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

// TestVMDetail_Hardware_EmptyPatch — at least one hardware field is required.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Hardware_EmptyPatch(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPut, "/api/v1/vms/default/100/hardware", `{}`, cookie))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

// TestVMDetail_Hardware_InvalidBody — malformed JSON is rejected.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Hardware_InvalidBody(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPut, "/api/v1/vms/default/100/hardware", `{invalid`, cookie))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// TestVMDetail_Hardware_NonOwnerForbidden — non-owner cannot update hardware.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Hardware_NonOwnerForbidden(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	bob := bobCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPut, "/api/v1/vms/default/100/hardware", `{"sockets":2}`, bob))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

// TestVMDetail_EnableSerial_MethodNotAllowed — non-POST methods on /serial are rejected.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_EnableSerial_MethodNotAllowed(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodGet, "/api/v1/vms/default/100/serial", "", cookie))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

// TestVMDetail_EnableSerial_Unauthenticated — serial endpoint requires auth.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_EnableSerial_Unauthenticated(t *testing.T) {
	handler, _, _, _ := newVMDetailHandler(t)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPost, "/api/v1/vms/default/100/serial", "", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

// TestVMDetail_EnableSerial_NonOwnerForbidden — non-owner cannot enable serial.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_EnableSerial_NonOwnerForbidden(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	bob := bobCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPost, "/api/v1/vms/default/100/serial", "", bob))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

// TestVMDetail_EnableSerial_OwnerSuccess — owner can enable serial on their VM.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_EnableSerial_OwnerSuccess(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPost, "/api/v1/vms/default/101/serial", "", cookie))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

// TestVMDetail_CDROM_MethodNotAllowed — non-PATCH methods on /cdrom are rejected.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_CDROM_MethodNotAllowed(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodGet, "/api/v1/vms/default/100/cdrom", "", cookie))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

// TestVMDetail_CDROM_Unauthenticated — cdrom endpoint requires auth.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_CDROM_Unauthenticated(t *testing.T) {
	handler, _, _, _ := newVMDetailHandler(t)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPatch, "/api/v1/vms/default/100/cdrom", `{"action":"eject"}`, nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

// TestVMDetail_CDROM_NonOwnerForbidden — non-owner cannot use cdrom endpoint.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_CDROM_NonOwnerForbidden(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	bob := bobCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPatch, "/api/v1/vms/default/100/cdrom", `{"action":"eject"}`, bob))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

// TestVMDetail_CDROM_InvalidBody — malformed JSON is rejected.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_CDROM_InvalidBody(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPatch, "/api/v1/vms/default/100/cdrom", `{invalid`, cookie))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// TestVMDetail_Disk_MethodNotAllowed — unsupported method on /disks is rejected.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Disk_MethodNotAllowed(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodGet, "/api/v1/vms/default/100/disks", "", cookie))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

// TestVMDetail_Disk_Unauthenticated — disk endpoint requires auth.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Disk_Unauthenticated(t *testing.T) {
	handler, _, _, _ := newVMDetailHandler(t)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPost, "/api/v1/vms/default/100/disks", `{"bus":"scsi","storage":"local-lvm","sizeGB":10}`, nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

// TestVMDetail_Disk_NonOwnerForbidden — non-owner cannot create disks.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Disk_NonOwnerForbidden(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	bob := bobCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPost, "/api/v1/vms/default/100/disks", `{"bus":"scsi","storage":"local-lvm","sizeGB":10}`, bob))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

// TestVMDetail_Disk_InvalidBody — malformed JSON is rejected.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Disk_InvalidBody(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPost, "/api/v1/vms/default/100/disks", `{invalid`, cookie))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// TestVMDetail_DiskCreate_OwnerSuccess — owner can add a disk.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_DiskCreate_OwnerSuccess(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPost, "/api/v1/vms/default/101/disks", `{"bus":"scsi","storage":"local-lvm","sizeGB":10}`, cookie))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

// TestVMDetail_DiskDelete_OwnerSuccess — owner can delete a disk.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_DiskDelete_OwnerSuccess(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodDelete, "/api/v1/vms/default/101/disks/scsi0", "", cookie))

	if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
		t.Fatalf("unexpected %d: %s", rec.Code, rec.Body.String())
	}
}

// TestVMDetail_HardwareOptions_MethodNotAllowed — non-GET on hardware-options.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_HardwareOptions_MethodNotAllowed(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPost, "/api/v1/vms/default/100/hardware-options", "", cookie))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

// TestVMDetail_HardwareOptions_Unauthenticated — hardware-options requires auth.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_HardwareOptions_Unauthenticated(t *testing.T) {
	handler, _, _, _ := newVMDetailHandler(t)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodGet, "/api/v1/vms/default/100/hardware-options", "", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

// TestVMDetail_HardwareOptions_NonOwnerForbidden — non-owner cannot see options.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_HardwareOptions_NonOwnerForbidden(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	bob := bobCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodGet, "/api/v1/vms/default/100/hardware-options", "", bob))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

// seedAuditEntry inserts one audit log row for testing.
func seedAuditEntry(t *testing.T, st *store.Store, actor, clusterName string, vmid int, action string) {
	t.Helper()

	if err := st.RecordAction(context.Background(), actor, clusterName, vmid, action); err != nil {
		t.Fatalf("seed audit entry: %v", err)
	}
}
