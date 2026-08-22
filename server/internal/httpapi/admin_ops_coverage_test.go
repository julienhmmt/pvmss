//nolint:noctx,paralleltest // HTTP coverage tests use shared fake and session fixtures
package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// --- admin_ops.go coverage ---

// TestAdminOpsCoverage_Audit_Unauthenticated_Returns401 exercises the
// RequireAdmin guard on GET /admin/audit with no cookie at all.
func TestAdminOpsCoverage_Audit_Unauthenticated_Returns401(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)

	rec := opsGet(t, ops, auth, nil, "/api/v1/admin/audit")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// TestAdminOpsCoverage_Audit_InvalidVMID_Returns400 covers the parseAuditFilter
// branch where vmid is present but not an integer.
func TestAdminOpsCoverage_Audit_InvalidVMID_Returns400(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)
	cookie := adminCookie(t, auth)

	rec := opsGet(t, ops, auth, cookie, "/api/v1/admin/audit?vmid=abc")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body["code"] != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", body["code"])
	}
}

// TestAdminOpsCoverage_Audit_InvalidFrom_Returns400 covers the parseAuditFilter
// branch where the from timestamp is malformed.
func TestAdminOpsCoverage_Audit_InvalidFrom_Returns400(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)
	cookie := adminCookie(t, auth)

	rec := opsGet(t, ops, auth, cookie, "/api/v1/admin/audit?from=not-a-timestamp")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

// TestAdminOpsCoverage_Audit_InvalidTo_Returns400 covers the parseAuditFilter
// branch where the to timestamp is malformed.
func TestAdminOpsCoverage_Audit_InvalidTo_Returns400(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)
	cookie := adminCookie(t, auth)

	rec := opsGet(t, ops, auth, cookie, "/api/v1/admin/audit?to=not-a-timestamp")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

// TestAdminOpsCoverage_Audit_ValidTimeFilters_ReturnsEntries covers the
// parseAuditFilter happy path where from/to are valid RFC3339 timestamps and
// vmid is a valid integer.
func TestAdminOpsCoverage_Audit_ValidTimeFilters_ReturnsEntries(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)
	cookie := adminCookie(t, auth)

	rec := opsGet(t, ops, auth, cookie,
		"/api/v1/admin/audit?vmid=101&from=2000-01-01T00:00:00Z&to=2100-01-01T00:00:00Z&cluster=default&actor=alice@pve&page=1&pageSize=10")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var page auditPageDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if page.PageSize != 10 || page.Page != 1 {
		t.Errorf("pagination = page=%d pageSize=%d, want 1/10", page.Page, page.PageSize)
	}
}

// TestAdminOpsCoverage_Dashboard_Unauthenticated_Returns401 exercises the
// RequireAdmin guard on GET /admin/dashboard with no cookie.
func TestAdminOpsCoverage_Dashboard_Unauthenticated_Returns401(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)

	rec := opsGet(t, ops, auth, nil, "/api/v1/admin/dashboard")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// TestAdminOpsCoverage_Dashboard_InventoryNotReady_Returns503 covers the
// ServeDashboard branch where the projection has not been populated yet
// (Load returns nil).
func TestAdminOpsCoverage_Dashboard_InventoryNotReady_Returns503(t *testing.T) {
	authHandler := newAuthHandler(t)
	st := auditAdminStore(t)
	emptyProjection := inventory.NewProjection()
	logger := slog.New(slog.DiscardHandler)
	ops := httpapi.NewAdminOps(authHandler, st, cluster.Fake{}, emptyProjection, "0.4.0-test", logger)

	cookie := adminCookie(t, authHandler)
	rec := opsGet(t, ops, authHandler, cookie, "/api/v1/admin/dashboard")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body["code"] != "inventory_not_ready" {
		t.Errorf("code = %q, want inventory_not_ready", body["code"])
	}
}

// TestAdminOpsCoverage_DBExport_Unauthenticated_Returns401 exercises the
// RequireAdmin guard on GET /admin/db/export with no cookie.
func TestAdminOpsCoverage_DBExport_Unauthenticated_Returns401(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)

	rec := opsGet(t, ops, auth, nil, "/api/v1/admin/db/export")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// TestAdminOpsCoverage_DBImport_Unauthenticated_Returns401 exercises the
// RequireAdmin guard on POST /admin/db/import with no cookie.
func TestAdminOpsCoverage_DBImport_Unauthenticated_Returns401(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)

	rec := opsPost(t, ops, auth, nil, "/api/v1/admin/db/import", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// TestAdminOpsCoverage_DBImport_NonMultipartBody_Returns400 covers the
// ServeDBImport branch where the body is not a valid multipart form.
func TestAdminOpsCoverage_DBImport_NonMultipartBody_Returns400(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)
	cookie := adminCookie(t, auth)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/db/import", strings.NewReader("not multipart"))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	opsMux(ops, auth).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body["code"] != "invalid_upload" {
		t.Errorf("code = %q, want invalid_upload", body["code"])
	}
}

// TestAdminOpsCoverage_DBImport_MissingFileField_Returns400 covers the
// ServeDBImport branch where the multipart form parses but has no "file" field.
func TestAdminOpsCoverage_DBImport_MissingFileField_Returns400(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)
	cookie := adminCookie(t, auth)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("notfile", "value")
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/db/import", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	opsMux(ops, auth).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var errBody map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if errBody["code"] != "invalid_upload" {
		t.Errorf("code = %q, want invalid_upload", errBody["code"])
	}
}

// TestAdminOpsCoverage_DBImportConfirm_Unauthenticated_Returns401 exercises
// the RequireAdmin guard on POST /admin/db/import/confirm with no cookie.
func TestAdminOpsCoverage_DBImportConfirm_Unauthenticated_Returns401(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)

	rec := opsPost(t, ops, auth, nil, "/api/v1/admin/db/import/confirm", `{"stagingToken":"x"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// TestAdminOpsCoverage_DBImportConfirm_InvalidJSON_Returns400 covers the
// ServeDBImportConfirm branch where the request body is not valid JSON.
func TestAdminOpsCoverage_DBImportConfirm_InvalidJSON_Returns400(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)
	cookie := adminCookie(t, auth)

	rec := opsPost(t, ops, auth, cookie, "/api/v1/admin/db/import/confirm", "{bad json")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body["code"] != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", body["code"])
	}
}

// TestAdminOpsCoverage_DBImportConfirm_EmptyToken_Returns400 covers the
// ServeDBImportConfirm branch where stagingToken is present but empty.
func TestAdminOpsCoverage_DBImportConfirm_EmptyToken_Returns400(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)
	cookie := adminCookie(t, auth)

	rec := opsPost(t, ops, auth, cookie, "/api/v1/admin/db/import/confirm", `{"stagingToken":""}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body["code"] != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", body["code"])
	}
}

// TestAdminOpsCoverage_AppInfo_Unauthenticated_Returns401 exercises the
// RequireAdmin guard on GET /admin/appinfo with no cookie.
func TestAdminOpsCoverage_AppInfo_Unauthenticated_Returns401(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)

	rec := opsGet(t, ops, auth, nil, "/api/v1/admin/appinfo")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// TestAdminOpsCoverage_AppInfo_NoProjection_ReturnsEmptyClusters covers the
// ServeAppInfo branch where the projection is nil — the clusters slice is
// empty but the endpoint still returns 200 with config fields.
func TestAdminOpsCoverage_AppInfo_NoProjection_ReturnsEmptyClusters(t *testing.T) {
	t.Setenv("SESSION_SECRET", strings.Repeat("s", 32))

	authHandler := newAuthHandler(t)
	st := auditAdminStore(t)
	emptyProjection := inventory.NewProjection()
	logger := slog.New(slog.DiscardHandler)
	ops := httpapi.NewAdminOps(authHandler, st, cluster.Fake{}, emptyProjection, "0.4.0-test", logger)

	cookie := adminCookie(t, authHandler)
	rec := opsGet(t, ops, authHandler, cookie, "/api/v1/admin/appinfo")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var info appInfoDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(info.Clusters) != 0 {
		t.Errorf("clusters = %d, want 0 (no projection loaded)", len(info.Clusters))
	}

	if info.Version != "0.4.0-test" {
		t.Errorf("version = %q, want 0.4.0-test", info.Version)
	}
}

// --- admin_clusters.go coverage ---

// TestAdminClustersCoverage_Unauthenticated_Returns401 exercises the
// RequireAdmin guard on every /admin/clusters/* endpoint with no cookie.
func TestAdminClustersCoverage_Unauthenticated_Returns401(t *testing.T) {
	fixture := newAdminClusterFixture(t)

	cases := []struct {
		name       string
		method     http.HandlerFunc
		httpMethod string
		path       string
		pathName   string
		body       string
	}{
		{"list", fixture.handler.ServeList, http.MethodGet, adminClustersPath, "", ""},
		{"create", fixture.handler.ServeCreate, http.MethodPost, adminClustersPath, "", `{"name":"x","url":"u","tokenId":"t","tokenSecret":"s"}`},
		{"update", fixture.handler.ServeUpdate, http.MethodPut, "/api/v1/admin/clusters/secondary", crossSecondaryCluster, `{"url":"u","tokenId":"t"}`},
		{"test", fixture.handler.ServeTest, http.MethodPost, "/api/v1/admin/clusters/secondary/test", crossSecondaryCluster, ""},
		{"oidc", fixture.handler.ServeOIDC, http.MethodPost, "/api/v1/admin/clusters/secondary/oidc", crossSecondaryCluster, `{"enabled":true}`},
		{"delete", fixture.handler.ServeDelete, http.MethodDelete, "/api/v1/admin/clusters/secondary", crossSecondaryCluster, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := adminClusterRequest(t, fixture, nil, clusterRequestSpec{
				Method: tc.method, HTTPMethod: tc.httpMethod, Path: tc.path, Name: tc.pathName, Body: tc.body,
			})
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
			}
		})
	}
}

// TestAdminClustersCoverage_ListMethodNotAllowed_Returns405 covers the
// ServeList branch that rejects non-GET methods.
func TestAdminClustersCoverage_ListMethodNotAllowed_Returns405(t *testing.T) {
	fixture := newAdminClusterFixture(t)
	cookie := adminClusterCookie(t, fixture.auth)

	rec := adminClusterRequest(t, fixture, cookie, clusterRequestSpec{
		Method: fixture.handler.ServeList, HTTPMethod: http.MethodPost, Path: adminClustersPath, Name: "", Body: "",
	})
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusMethodNotAllowed, rec.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body["code"] != "method_not_allowed" {
		t.Errorf("code = %q, want method_not_allowed", body["code"])
	}
}

// TestAdminClustersCoverage_CreateInvalidJSON_Returns400 covers the ServeCreate
// branch where the request body is not valid JSON.
func TestAdminClustersCoverage_CreateInvalidJSON_Returns400(t *testing.T) {
	fixture := newAdminClusterFixture(t)
	cookie := adminClusterCookie(t, fixture.auth)

	rec := adminClusterRequest(t, fixture, cookie, clusterRequestSpec{
		Method: fixture.handler.ServeCreate, HTTPMethod: http.MethodPost, Path: adminClustersPath, Name: "", Body: "{bad json",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body["code"] != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", body["code"])
	}
}

// TestAdminClustersCoverage_UpdateInvalidJSON_Returns400 covers the ServeUpdate
// branch where the request body is not valid JSON.
func TestAdminClustersCoverage_UpdateInvalidJSON_Returns400(t *testing.T) {
	fixture := newAdminClusterFixture(t)
	cookie := adminClusterCookie(t, fixture.auth)

	rec := adminClusterRequest(t, fixture, cookie, clusterRequestSpec{
		Method: fixture.handler.ServeUpdate, HTTPMethod: http.MethodPut,
		Path: "/api/v1/admin/clusters/secondary", Name: crossSecondaryCluster, Body: "{bad json",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body["code"] != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", body["code"])
	}
}

// TestAdminClustersCoverage_OIDCInvalidJSON_Returns400 covers the ServeOIDC
// branch where the request body is not valid JSON.
func TestAdminClustersCoverage_OIDCInvalidJSON_Returns400(t *testing.T) {
	fixture := newAdminClusterFixture(t)
	cookie := adminClusterCookie(t, fixture.auth)

	rec := adminClusterRequest(t, fixture, cookie, clusterRequestSpec{
		Method: fixture.handler.ServeOIDC, HTTPMethod: http.MethodPost,
		Path: "/api/v1/admin/clusters/secondary/oidc", Name: crossSecondaryCluster, Body: "{bad json",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body["code"] != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", body["code"])
	}
}

// TestAdminClustersCoverage_TestUnknownCluster_Returns404 covers the ServeTest
// branch where GetCluster fails for an unknown cluster name.
func TestAdminClustersCoverage_TestUnknownCluster_Returns404(t *testing.T) {
	fixture := newAdminClusterFixture(t)
	cookie := adminClusterCookie(t, fixture.auth)

	rec := adminClusterRequest(t, fixture, cookie, clusterRequestSpec{
		Method: fixture.handler.ServeTest, HTTPMethod: http.MethodPost,
		Path: "/api/v1/admin/clusters/nonexistent/test", Name: "nonexistent", Body: "",
	})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

// TestAdminClustersCoverage_DeleteUnknownCluster_Returns404 covers the
// ServeDelete branch where the cluster does not exist.
func TestAdminClustersCoverage_DeleteUnknownCluster_Returns404(t *testing.T) {
	fixture := newAdminClusterFixture(t)
	cookie := adminClusterCookie(t, fixture.auth)

	rec := adminClusterRequest(t, fixture, cookie, clusterRequestSpec{
		Method: fixture.handler.ServeDelete, HTTPMethod: http.MethodDelete,
		Path: "/api/v1/admin/clusters/nonexistent", Name: "nonexistent", Body: "",
	})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

// --- admin_pools.go coverage ---

// TestAdminPoolsCoverage_Unauthenticated_Returns401 exercises the adminActor
// guard (which returns 401, not 403, for missing identity) on every pools
// endpoint.
func TestAdminPoolsCoverage_Unauthenticated_Returns401(t *testing.T) {
	handler, _ := newAdminPoolsHandler(t)

	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"list", http.MethodGet, "/api/v1/admin/pools", ""},
		{"create", http.MethodPost, "/api/v1/admin/pools", `{"name":"x"}`},
		{"delete", http.MethodDelete, "/api/v1/admin/pools/x", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := adminPoolsRequest(t, handlerForMethod(handler, tc.method), tc.method, tc.path, nil, tc.body)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
			}

			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode: %v", err)
			}

			if body["code"] != "unauthenticated" {
				t.Errorf("code = %q, want unauthenticated", body["code"])
			}
		})
	}
}

// TestAdminPoolsCoverage_CreateInvalidJSON_Returns400 covers the ServeCreate
// branch where the request body is not valid JSON.
func TestAdminPoolsCoverage_CreateInvalidJSON_Returns400(t *testing.T) {
	handler, authHandler := newAdminPoolsHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminPoolsRequest(t, handler.ServeCreate, http.MethodPost, "/api/v1/admin/pools", cookie, "{bad json")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body["code"] != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", body["code"])
	}
}

// TestAdminPoolsCoverage_DeleteEmptyName_Returns400 covers the ServeDelete
// branch where the path name is empty.
func TestAdminPoolsCoverage_DeleteEmptyName_Returns400(t *testing.T) {
	handler, authHandler := newAdminPoolsHandler(t)
	cookie := adminCookie(t, authHandler)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/pools/", nil)
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	handler.ServeDelete(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body["code"] != "invalid_pool_name" {
		t.Errorf("code = %q, want invalid_pool_name", body["code"])
	}
}

// TestAdminPoolsCoverage_CreateUnknownCluster_Returns404 covers the ServeCreate
// branch where clientFor cannot resolve the requested cluster. Uses a
// registry-backed handler whose registry does not contain the requested name.
func TestAdminPoolsCoverage_CreateUnknownCluster_Returns404(t *testing.T) {
	handler, authHandler := newAdminPoolsRegistryHandler(t)
	cookie := adminClusterCookie(t, authHandler)

	rec := adminPoolsRequest(t, handler.ServeCreate, http.MethodPost,
		"/api/v1/admin/pools?cluster=nonexistent", cookie, `{"name":"newteam"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body["code"] != "cluster_not_found" {
		t.Errorf("code = %q, want cluster_not_found", body["code"])
	}
}

// TestAdminPoolsCoverage_ListUnknownCluster_Returns404 covers the ServeList
// branch where clientFor cannot resolve the requested cluster.
func TestAdminPoolsCoverage_ListUnknownCluster_Returns404(t *testing.T) {
	handler, authHandler := newAdminPoolsRegistryHandler(t)
	cookie := adminClusterCookie(t, authHandler)

	rec := adminPoolsRequest(t, handler.ServeList, http.MethodGet,
		"/api/v1/admin/pools?cluster=nonexistent", cookie, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

// TestAdminPoolsCoverage_DeleteUnknownCluster_Returns404 covers the ServeDelete
// branch where clientFor cannot resolve the requested cluster.
func TestAdminPoolsCoverage_DeleteUnknownCluster_Returns404(t *testing.T) {
	handler, authHandler := newAdminPoolsRegistryHandler(t)
	cookie := adminClusterCookie(t, authHandler)

	rec := adminPoolsRequest(t, handler.ServeDelete, http.MethodDelete,
		"/api/v1/admin/pools/pvmss-x?cluster=nonexistent", cookie, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

// newAdminPoolsRegistryHandler builds a registry-backed AdminPools handler
// whose ClientProvider only knows the "default" cluster, so requests for any
// other cluster name hit the cluster_not_found path.
func newAdminPoolsRegistryHandler(t *testing.T) (*httpapi.AdminPools, *httpapi.Auth) {
	t.Helper()
	t.Cleanup(cluster.ResetFake)

	const secret = "pools-registry-test-secret-32-bytes!!"
	st, err := store.Open(config.Configuration{
		DBPath:        filepath.Join(t.TempDir(), "pools-registry.db"),
		ClusterSource: cluster.SourceFake,
		SessionSecret: secret,
		LogLevel:      snapshotTestLogLevel,
		LogFormat:     snapshotTestLogFormat,
		LogOutput:     snapshotTestLogOutput,
	})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	rows, err := st.ListClusters(context.Background())
	if err != nil {
		t.Fatalf("ListClusters: %v", err)
	}
	registry, err := cluster.NewRegistry("fake", rows)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	sessions, err := auth.NewSessionManager(st, secret, false)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("admin-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword: %v", err)
	}
	logger := slog.New(slog.DiscardHandler)
	authHandler := httpapi.NewAuthWithRegistry(registry, st, sessions, string(hash), auth.NewTokenService(st), logger)

	fake := cluster.Fake{}
	snapshot, err := fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	index := inventory.BuildIndex(snapshot)
	projection := inventory.NewProjectionFromIndex(&index)
	worker := inventory.NewWorker(fake, projection, 0, logger)

	handler := httpapi.NewAdminPoolsWithRegistry(httpapi.AdminPoolsRegistryDeps{
		Auth:       authHandler,
		Clients:    registry,
		Source:     inventory.NewRegistry(registry, 0, logger),
		Projection: projection,
		Writer:     fake,
		Audit:      st,
		Refresher:  worker,
		Store:      st,
		Log:        logger,
	})

	return handler, authHandler
}

// --- admin_policy.go coverage ---

// TestAdminPolicyCoverage_Unauthenticated_Returns401 exercises the RequireAdmin
// guard on GET and PUT /admin/policy with no cookie.
func TestAdminPolicyCoverage_Unauthenticated_Returns401(t *testing.T) {
	policyHandler, authHandler := newPolicyHandler(t)
	mux := policyMux(policyHandler, authHandler)

	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{"get", http.MethodGet, "/api/v1/admin/policy?cluster=default"},
		{"put", http.MethodPut, "/api/v1/admin/policy"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
			}
		})
	}
}

// TestAdminPolicyCoverage_GetMethodNotAllowed_Returns405 covers the ServePolicy
// branch that rejects non-GET methods. The handler is invoked directly (not via
// a method-prefixed mux route) so the handler's own method check is exercised.
func TestAdminPolicyCoverage_GetMethodNotAllowed_Returns405(t *testing.T) {
	policyHandler, authHandler := newPolicyHandler(t)
	cookie := adminCookie(t, authHandler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/policy?cluster=default", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	authHandler.RequireAdmin(http.HandlerFunc(policyHandler.ServePolicy)).ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusMethodNotAllowed, rec.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body["code"] != "method_not_allowed" {
		t.Errorf("code = %q, want method_not_allowed", body["code"])
	}
}

// TestAdminPolicyCoverage_PutMethodNotAllowed_Returns405 covers the
// ServePolicyUpdate branch that rejects non-PUT methods. The handler is
// invoked directly so the handler's own method check is exercised.
func TestAdminPolicyCoverage_PutMethodNotAllowed_Returns405(t *testing.T) {
	policyHandler, authHandler := newPolicyHandler(t)
	cookie := adminCookie(t, authHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/policy", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	authHandler.RequireAdmin(http.HandlerFunc(policyHandler.ServePolicyUpdate)).ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusMethodNotAllowed, rec.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body["code"] != "method_not_allowed" {
		t.Errorf("code = %q, want method_not_allowed", body["code"])
	}
}

// TestAdminPolicyCoverage_PutInvalidJSON_Returns400 covers the ServePolicyUpdate
// branch where the request body is not valid JSON.
func TestAdminPolicyCoverage_PutInvalidJSON_Returns400(t *testing.T) {
	policyHandler, authHandler := newPolicyHandler(t)
	mux := policyMux(policyHandler, authHandler)
	cookie := adminCookie(t, authHandler)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/policy", strings.NewReader("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body["code"] != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", body["code"])
	}
}

// TestAdminPolicyCoverage_PutClusterRequired_Returns400 covers the
// ServePolicyUpdate branch where multiple clusters are configured but the
// request body omits the cluster field — ResolveClusterValue returns
// ErrClusterRequired.
func TestAdminPolicyCoverage_PutClusterRequired_Returns400(t *testing.T) {
	st := newAdminStore(t)
	authHandler := newAuthHandler(t)
	fake := cluster.Fake{}
	snapshot, err := fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	index := inventory.BuildIndex(snapshot)
	service := policy.New(st, inventory.NewProjectionFromIndex(&index), fake)
	lister := fakeClusterListerProvider{names: []string{"default", "secondary"}}
	policyHandler := httpapi.NewAdminPolicyWithRegistry(authHandler, service, lister, slog.New(slog.DiscardHandler))

	mux := http.NewServeMux()
	mux.Handle("PUT /api/v1/admin/policy", authHandler.RequireAdmin(http.HandlerFunc(policyHandler.ServePolicyUpdate)))

	cookie := adminCookie(t, authHandler)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/policy", strings.NewReader(`{"gabarit":{"maxCores":4}}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body["code"] != "cluster_required" {
		t.Errorf("code = %q, want cluster_required", body["code"])
	}
}

// TestAdminPolicyCoverage_GetClusterRequired_Returns400 covers the ServePolicy
// branch where multiple clusters are configured but the query parameter is
// omitted.
func TestAdminPolicyCoverage_GetClusterRequired_Returns400(t *testing.T) {
	st := newAdminStore(t)
	authHandler := newAuthHandler(t)
	fake := cluster.Fake{}
	snapshot, err := fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	index := inventory.BuildIndex(snapshot)
	service := policy.New(st, inventory.NewProjectionFromIndex(&index), fake)
	lister := fakeClusterListerProvider{names: []string{"default", "secondary"}}
	policyHandler := httpapi.NewAdminPolicyWithRegistry(authHandler, service, lister, slog.New(slog.DiscardHandler))

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/admin/policy", authHandler.RequireAdmin(http.HandlerFunc(policyHandler.ServePolicy)))

	cookie := adminCookie(t, authHandler)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/policy", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body["code"] != "cluster_required" {
		t.Errorf("code = %q, want cluster_required", body["code"])
	}
}
