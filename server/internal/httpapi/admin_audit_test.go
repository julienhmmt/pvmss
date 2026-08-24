//nolint:noctx // test scaffolding does not need real context
package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"strings"
	"testing"
)

// Test fixture constants — centralizing them keeps goconst below threshold
// across the httpapi test package.
const (
	auditTestActor             = "alice@pve"
	auditTestAction            = "start"
	auditTestCluster           = "default"
	testNodePVE01              = "pve-node-01"
	adminPoolsPath             = "/api/v1/admin/pools"
	adminClustersSecondaryPath = "/api/v1/admin/clusters/secondary"
	adminClustersOIDCPath      = "/api/v1/admin/clusters/secondary/oidc"
	testOpList                 = "list"
	testOpCreate               = "create"
	testOpUpdate               = "update"
	testOpTest                 = "test"
	testOpOIDC                 = "oidc"
)

// auditAdminStore opens a fully-migrated store and seeds it with two audit
// rows: a VM start (T05) and a cloud-init snippet edit (T08), both by alice.
func auditAdminStore(t *testing.T) *store.Store {
	t.Helper()
	cfg := config.Configuration{
		Port:      50001,
		DBPath:    filepath.Join(t.TempDir(), "audit-httpapi.db"),
		LogLevel:  snapshotTestLogLevel,
		LogFormat: snapshotTestLogFormat,
		LogOutput: snapshotTestLogOutput,
	}

	st, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	ctx := context.Background()
	if err := st.RecordAction(ctx, auditTestActor, auditTestCluster, 101, "edit_cloudinit_snippet"); err != nil {
		t.Fatalf("seed edit_cloudinit_snippet: %v", err)
	}

	if err := st.RecordAction(ctx, auditTestActor, auditTestCluster, 101, auditTestAction); err != nil {
		t.Fatalf("seed start: %v", err)
	}

	return st
}

// newAdminOpsHandler builds an AdminOps handler with a real store (seeded
// with audit rows), the fake cluster client, and a projection populated from
// the fake snapshot. Returns the handler, auth, and store for direct
// inspection.
func newAdminOpsHandler(t *testing.T) (*httpapi.AdminOps, *httpapi.Auth, *store.Store) {
	t.Helper()
	t.Cleanup(cluster.ResetFake)

	authHandler := newAuthHandler(t)
	st := auditAdminStore(t)
	fake := cluster.Fake{}
	snap, _ := fake.Snapshot(context.Background())
	idx := inventory.BuildIndex(snap)
	projection := inventory.NewProjectionFromIndex(&idx)
	logger := slog.New(slog.DiscardHandler)
	ops := httpapi.NewAdminOps(authHandler, st, fake, projection, "0.4.0-dev-test", logger)

	return ops, authHandler, st
}

// opsMux builds a ServeMux with the admin ops routes wired through
// RequireAdmin, plus the public version route outside the guard.
func opsMux(ops *httpapi.AdminOps, auth *httpapi.Auth) *http.ServeMux {
	mux := http.NewServeMux()
	guard := auth.RequireAdmin
	mux.Handle("GET /api/v1/admin/audit", guard(http.HandlerFunc(ops.ServeAudit)))
	mux.Handle("GET /api/v1/admin/dashboard", guard(http.HandlerFunc(ops.ServeDashboard)))
	mux.Handle("GET /api/v1/admin/db/export", guard(http.HandlerFunc(ops.ServeDBExport)))
	mux.Handle("POST /api/v1/admin/db/import", guard(http.HandlerFunc(ops.ServeDBImport)))
	mux.Handle("POST /api/v1/admin/db/import/confirm", guard(http.HandlerFunc(ops.ServeDBImportConfirm)))
	mux.Handle("GET /api/v1/admin/appinfo", guard(http.HandlerFunc(ops.ServeAppInfo)))
	mux.Handle("GET /api/v1/public/version", http.HandlerFunc(ops.ServePublicVersion))

	return mux
}

func opsGet(t *testing.T, ops *httpapi.AdminOps, auth *httpapi.Auth, cookie *http.Cookie, path string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()
	opsMux(ops, auth).ServeHTTP(rec, req)

	return rec
}

func opsPost(t *testing.T, ops *httpapi.AdminOps, auth *httpapi.Auth, cookie *http.Cookie, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	if cookie != nil {
		req.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()
	opsMux(ops, auth).ServeHTTP(rec, req)

	return rec
}

// --- audit handler tests ---

type auditEntryDTO struct {
	ID        int64  `json:"id"`
	Actor     string `json:"actor"`
	Cluster   string `json:"cluster"`
	VMID      int    `json:"vmid"`
	Action    string `json:"action"`
	Timestamp string `json:"timestamp"`
}

type auditPageDTO struct {
	Items    []auditEntryDTO `json:"items"`
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"pageSize"`
}

// TestAdminAudit_AsAdminNoFilter_ReturnsEntriesAndPagination — T013: GET
// /admin/audit as admin with no filter returns both seeded entries, most
// recent first, with the pagination envelope populated.
//
//nolint:paralleltest // serial: shared database fixture
func TestAdminAudit_AsAdminNoFilter_ReturnsEntriesAndPagination(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)
	cookie := adminCookie(t, auth)

	rec := opsGet(t, ops, auth, cookie, "/api/v1/admin/audit")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var page auditPageDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if page.Total != 2 {
		t.Errorf("total = %d, want 2", page.Total)
	}

	if len(page.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(page.Items))
	}
	// Most recent first: start was inserted after edit_cloudinit_snippet.
	if page.Items[0].Action != auditTestAction {
		t.Errorf("first item action = %q, want start (most recent first)", page.Items[0].Action)
	}

	if page.Items[0].Actor != auditTestActor {
		t.Errorf("first item actor = %q, want alice@pve (real username)", page.Items[0].Actor)
	}
}

// TestAdminAudit_FilterByAction — T014: GET /admin/audit?action=start returns
// only the matching entry.
//
//nolint:paralleltest // serial: shared database fixture
func TestAdminAudit_FilterByAction(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)
	cookie := adminCookie(t, auth)

	rec := opsGet(t, ops, auth, cookie, "/api/v1/admin/audit?action=start")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var page auditPageDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(page.Items) != 1 || page.Items[0].Action != auditTestAction {
		t.Errorf("items = %+v, want one start entry", page.Items)
	}
}

// TestAdminAudit_AsNonAdmin_Returns403 — T015: GET /admin/audit as non-admin
// returns 403 (FR-016).
//
//nolint:paralleltest // serial: shared database fixture
func TestAdminAudit_AsNonAdmin_Returns403(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)
	aliceCookie := loginCookie(t, auth, `{"username":"alice","password":"pvmss-alice"}`)

	rec := opsGet(t, ops, auth, aliceCookie, "/api/v1/admin/audit")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// TestAdminAudit_PageSizeOverMaximum_Returns400 — T016: pageSize beyond the
// configured maximum returns 400 page_size_too_large.
//
//nolint:paralleltest // serial: shared database fixture
func TestAdminAudit_PageSizeOverMaximum_Returns400(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)
	cookie := adminCookie(t, auth)

	rec := opsGet(t, ops, auth, cookie, "/api/v1/admin/audit?pageSize=9999")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	assertAPIError(t, rec.Body.Bytes(), apiCodePageSizeTooLarge)
}
