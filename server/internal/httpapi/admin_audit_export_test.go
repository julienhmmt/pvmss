package httpapi_test

import (
	"encoding/csv"
	"net/http"
	"strings"
	"testing"
)

// TestAdminAuditExport_AsAdmin_ReturnsCSV - GET /admin/audit/export streams
// every matching row as CSV, newest first, with a header row.
//
//nolint:paralleltest // serial: shared database fixture
func TestAdminAuditExport_AsAdmin_ReturnsCSV(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)
	cookie := adminCookie(t, auth)

	rec := opsGet(t, ops, auth, cookie, "/api/v1/admin/audit/export")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("content-type = %q, want text/csv", ct)
	}

	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, `attachment; filename="pvmss-audit`) {
		t.Errorf("content-disposition = %q", cd)
	}

	if rec.Header().Get("X-PVMSS-Truncated") != "" {
		t.Error("truncated header set on a small export")
	}

	rows, err := csv.NewReader(rec.Body).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}

	if len(rows) != 3 {
		t.Fatalf("rows = %d, want header + 2", len(rows))
	}

	if strings.Join(rows[0], ",") != "timestamp,actor,action,cluster,vmid,target_type,target_id,detail,ip,severity" {
		t.Errorf("header = %v", rows[0])
	}

	if rows[1][1] != auditTestActor || rows[1][2] != auditTestAction {
		t.Errorf("first row = %v, want alice@pve start (newest first)", rows[1])
	}
}

// TestAdminAuditExport_FilterApplies - the same filters as /admin/audit.
//
//nolint:paralleltest // serial: shared database fixture
func TestAdminAuditExport_FilterApplies(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)
	cookie := adminCookie(t, auth)

	rec := opsGet(t, ops, auth, cookie, "/api/v1/admin/audit/export?action=start")

	rows, err := csv.NewReader(rec.Body).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("rows = %d, want header + 1", len(rows))
	}
}

// TestAdminAuditExport_AsNonAdmin_Returns403.
//
//nolint:paralleltest // serial: shared database fixture
func TestAdminAuditExport_AsNonAdmin_Returns403(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)
	aliceCookie := loginCookie(t, auth, `{"username":"alice","password":"pvmss-alice"}`)

	rec := opsGet(t, ops, auth, aliceCookie, "/api/v1/admin/audit/export")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}
