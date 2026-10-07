//nolint:noctx // test scaffolding does not need real context
package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"strings"
	"testing"
)

type fakeClusterListerProvider struct {
	names []string
}

func (f fakeClusterListerProvider) List() []string { return f.names }
func (f fakeClusterListerProvider) Client(_ string) (cluster.Client, error) {
	return cluster.Fake{}, nil
}

func newAdminTagsMultiClusterHandler(t *testing.T) (*httpapi.AdminCatalog, *httpapi.Auth) {
	t.Helper()
	t.Cleanup(cluster.ResetFake)

	authHandler := newAuthHandler(t)
	st := newAdminStore(t)
	fake := cluster.Fake{}
	snap, _ := fake.Snapshot(context.Background())
	idx := inventory.BuildIndex(snap)
	projection := inventory.NewProjectionFromIndex(&idx)
	registry := fakeClusterListerProvider{names: []string{auditTestCluster, crossSecondaryCluster}}
	adminCatalog := httpapi.NewAdminCatalogWithRegistry(authHandler, st, registry, projection, testLogger(t))

	return adminCatalog, authHandler
}

func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(testWriter{t}, nil))
}

func adminTagsMux(handler *httpapi.AdminCatalog, auth *httpapi.Auth) *http.ServeMux {
	mux := http.NewServeMux()
	guard := auth.RequireAdmin
	mux.Handle("GET /api/v1/admin/tags", guard(http.HandlerFunc(handler.ServeTags)))
	mux.Handle("POST /api/v1/admin/tags", guard(http.HandlerFunc(handler.ServeTagCreate)))
	mux.Handle("PUT /api/v1/admin/tags/{name}/color", guard(http.HandlerFunc(handler.ServeTagColor)))
	mux.Handle("DELETE /api/v1/admin/tags/{name}", guard(http.HandlerFunc(handler.ServeTagDelete)))

	return mux
}

func tagsRequest(method, path, body string, cookie *http.Cookie) (*http.Request, *httptest.ResponseRecorder) {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}

	if cookie != nil {
		req.AddCookie(cookie)
	}

	return req, httptest.NewRecorder()
}

func serveTags(mux *http.ServeMux, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	return rec
}

func decodeAdminErrorCode(t *testing.T, body []byte) string {
	t.Helper()

	var env apiErrorEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode error body: %v", err)
	}

	return env.Code
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestAdminTags_ListClusterRequired_Returns400(t *testing.T) {
	handler, authHandler := newAdminTagsMultiClusterHandler(t)
	cookie := adminCookie(t, authHandler)

	req, _ := tagsRequest(http.MethodGet, "/api/v1/admin/tags", "", cookie)
	rec := serveTags(adminTagsMux(handler, authHandler), req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	if code := decodeAdminErrorCode(t, rec.Body.Bytes()); code != apiCodeClusterRequired {
		t.Errorf("code = %q, want cluster_required", code)
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestAdminTags_CreateInvalidBody_Returns400(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	req, _ := tagsRequest(http.MethodPost, "/api/v1/admin/tags", `{invalid json`, cookie)
	rec := serveTags(adminTagsMux(handler, authHandler), req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if code := decodeAdminErrorCode(t, rec.Body.Bytes()); code != apiCodeInvalidRequest {
		t.Errorf("code = %q, want invalid_request", code)
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestAdminTags_CreateClusterRequired_Returns400(t *testing.T) {
	handler, authHandler := newAdminTagsMultiClusterHandler(t)
	cookie := adminCookie(t, authHandler)

	req, _ := tagsRequest(http.MethodPost, "/api/v1/admin/tags", `{"name":"newtag","color":"#000000"}`, cookie)
	rec := serveTags(adminTagsMux(handler, authHandler), req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	if code := decodeAdminErrorCode(t, rec.Body.Bytes()); code != apiCodeClusterRequired {
		t.Errorf("code = %q, want cluster_required", code)
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestAdminTags_ColorNotFound_Returns404(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	req, _ := tagsRequest(http.MethodPut, "/api/v1/admin/tags/nonexistent/color", `{"cluster":"default","color":"#ff0000"}`, cookie)
	rec := serveTags(adminTagsMux(handler, authHandler), req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}

	if code := decodeAdminErrorCode(t, rec.Body.Bytes()); code != apiCodeNotFound {
		t.Errorf("code = %q, want not_found", code)
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestAdminTags_ColorEmptyColor_Returns400(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	adminPost(t, handler, authHandler, cookie, "/api/v1/admin/tags",
		`{"cluster":"default","name":"colortest","color":"#000000"}`)

	req, _ := tagsRequest(http.MethodPut, "/api/v1/admin/tags/colortest/color", `{"cluster":"default","color":""}`, cookie)
	rec := serveTags(adminTagsMux(handler, authHandler), req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	if code := decodeAdminErrorCode(t, rec.Body.Bytes()); code != "invalid_tag_color" {
		t.Errorf("code = %q, want invalid_tag_color", code)
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestAdminTags_ColorInvalidBody_Returns400(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	req, _ := tagsRequest(http.MethodPut, "/api/v1/admin/tags/pvmss/color", `{invalid`, cookie)
	rec := serveTags(adminTagsMux(handler, authHandler), req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if code := decodeAdminErrorCode(t, rec.Body.Bytes()); code != apiCodeInvalidRequest {
		t.Errorf("code = %q, want invalid_request", code)
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestAdminTags_ColorClusterRequired_Returns400(t *testing.T) {
	handler, authHandler := newAdminTagsMultiClusterHandler(t)
	cookie := adminCookie(t, authHandler)

	req, _ := tagsRequest(http.MethodPut, "/api/v1/admin/tags/pvmss/color", `{"color":"#ff0000"}`, cookie)
	rec := serveTags(adminTagsMux(handler, authHandler), req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	if code := decodeAdminErrorCode(t, rec.Body.Bytes()); code != apiCodeClusterRequired {
		t.Errorf("code = %q, want cluster_required", code)
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestAdminTags_DeleteNotFound_Returns404(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	req, _ := tagsRequest(http.MethodDelete, "/api/v1/admin/tags/nonexistent?cluster=default", "", cookie)
	rec := serveTags(adminTagsMux(handler, authHandler), req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}

	if code := decodeAdminErrorCode(t, rec.Body.Bytes()); code != apiCodeNotFound {
		t.Errorf("code = %q, want not_found", code)
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestAdminTags_DeleteClusterRequired_Returns400(t *testing.T) {
	handler, authHandler := newAdminTagsMultiClusterHandler(t)
	cookie := adminCookie(t, authHandler)

	req, _ := tagsRequest(http.MethodDelete, "/api/v1/admin/tags/temp", "", cookie)
	rec := serveTags(adminTagsMux(handler, authHandler), req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	if code := decodeAdminErrorCode(t, rec.Body.Bytes()); code != apiCodeClusterRequired {
		t.Errorf("code = %q, want cluster_required", code)
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestAdminTags_ListWithExplicitCluster_ReturnsTags(t *testing.T) {
	handler, authHandler := newAdminTagsMultiClusterHandler(t)
	cookie := adminCookie(t, authHandler)

	req, _ := tagsRequest(http.MethodGet, "/api/v1/admin/tags?cluster=default", "", cookie)
	rec := serveTags(adminTagsMux(handler, authHandler), req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var tags []adminTagDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &tags); err != nil {
		t.Fatalf("decode tags: %v", err)
	}

	found := false

	for _, tag := range tags {
		if tag.Name == extraPvmssTag {
			found = true
		}
	}

	if !found {
		t.Error("pvmss tag not found in multi-cluster list")
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestAdminTags_CreateWithExplicitCluster_Succeeds(t *testing.T) {
	handler, authHandler := newAdminTagsMultiClusterHandler(t)
	cookie := adminCookie(t, authHandler)

	req, _ := tagsRequest(http.MethodPost, "/api/v1/admin/tags", `{"cluster":"default","name":"multitag","color":"#00ff00"}`, cookie)
	rec := serveTags(adminTagsMux(handler, authHandler), req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var tag adminTagDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &tag); err != nil {
		t.Fatalf("decode tag: %v", err)
	}

	if tag.Name != "multitag" {
		t.Errorf("tag.Name = %q, want multitag", tag.Name)
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestAdminTags_DeleteWithExplicitCluster_Succeeds(t *testing.T) {
	handler, authHandler := newAdminTagsMultiClusterHandler(t)
	cookie := adminCookie(t, authHandler)

	adminPost(t, handler, authHandler, cookie, "/api/v1/admin/tags",
		`{"cluster":"default","name":"deltest","color":"#000000"}`)

	req, _ := tagsRequest(http.MethodDelete, "/api/v1/admin/tags/deltest?cluster=default", "", cookie)
	rec := serveTags(adminTagsMux(handler, authHandler), req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var status statusDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}

	if status.Status != testStatusDeleted {
		t.Errorf("status = %q, want deleted", status.Status)
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestAdminTags_ColorUpdateNonExistentCluster_ReturnsTags(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	adminPost(t, handler, authHandler, cookie, "/api/v1/admin/tags",
		`{"cluster":"default","name":"colortag","color":"#000000"}`)

	req, _ := tagsRequest(http.MethodPut, "/api/v1/admin/tags/colortag/color", `{"cluster":"default","color":"#aabbcc"}`, cookie)
	rec := serveTags(adminTagsMux(handler, authHandler), req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var tag adminTagDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &tag); err != nil {
		t.Fatalf("decode tag: %v", err)
	}

	if tag.Color != "#aabbcc" {
		t.Errorf("tag.Color = %q, want #aabbcc", tag.Color)
	}
}
