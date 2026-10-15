package httpapi_test

import (
	"context"
	"net/http"
	"testing"
)

// When the audit tables are unavailable the admin audit endpoints answer 500
// with the generic message rather than leaking the database error.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminOps_AuditEndpointsFailClosedWhenTablesAreGone(t *testing.T) {
	ops, authHandler, st := newAdminOpsHandler(t)
	cookie := adminCookie(t, authHandler)

	for _, table := range []string{"audit_config", "audit_log"} {
		if _, err := st.DB().ExecContext(context.Background(), "DROP TABLE "+table); err != nil {
			t.Fatalf("drop %s: %v", table, err)
		}
	}

	for _, path := range []string{
		"/api/v1/admin/audit",
		"/api/v1/admin/audit/config",
		"/api/v1/admin/audit/prune-preview?retention_days=30",
	} {
		t.Run(path, func(t *testing.T) {
			rec := opsGet(t, ops, authHandler, cookie, path)
			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminOps_AuditConfigUpdateFailsClosedWhenTableIsGone(t *testing.T) {
	ops, authHandler, st := newAdminOpsHandler(t)
	cookie := adminCookie(t, authHandler)

	if _, err := st.DB().ExecContext(context.Background(), "DROP TABLE audit_config"); err != nil {
		t.Fatalf("drop audit_config: %v", err)
	}

	rec := opsPut(t, ops, authHandler, cookie, "/api/v1/admin/audit/config", `{"retentionDays":30}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500: %s", rec.Code, rec.Body.String())
	}
}
