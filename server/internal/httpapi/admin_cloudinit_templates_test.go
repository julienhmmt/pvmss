//nolint:noctx // test scaffolding does not need real context
package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/httpapi"
	"strings"
	"testing"
)

// cloudInitTemplateDTO is the admin response shape for one template.
type cloudInitTemplateDTO struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Content string `json:"content"`
	Enabled bool   `json:"enabled"`
}

// cloudInitTemplatesMux wires the five new admin cloud-init-template routes
// through RequireAdmin — the real guard the 403 tests must exercise. Kept local
// so the shared adminMux does not reference handler methods before T006 lands.
func cloudInitTemplatesMux(handler *httpapi.AdminCatalog, auth *httpapi.Auth) *http.ServeMux {
	mux := http.NewServeMux()
	guard := auth.RequireAdmin
	mux.Handle("GET /api/v1/admin/cloudinit-templates", guard(http.HandlerFunc(handler.ServeCloudInitTemplates)))
	mux.Handle("POST /api/v1/admin/cloudinit-templates", guard(http.HandlerFunc(handler.ServeCloudInitTemplateCreate)))
	mux.Handle("PUT /api/v1/admin/cloudinit-templates/{id}", guard(http.HandlerFunc(handler.ServeCloudInitTemplateUpdate)))
	mux.Handle("DELETE /api/v1/admin/cloudinit-templates/{id}", guard(http.HandlerFunc(handler.ServeCloudInitTemplateDelete)))
	mux.Handle("POST /api/v1/admin/cloudinit-templates/{id}/toggle", guard(http.HandlerFunc(handler.ServeCloudInitTemplateToggle)))

	return mux
}

func citGet(t *testing.T, handler *httpapi.AdminCatalog, auth *httpapi.Auth, cookie *http.Cookie, path string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()
	cloudInitTemplatesMux(handler, auth).ServeHTTP(rec, req)

	return rec
}

func citPost(t *testing.T, handler *httpapi.AdminCatalog, auth *httpapi.Auth, cookie *http.Cookie, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	if cookie != nil {
		req.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()
	cloudInitTemplatesMux(handler, auth).ServeHTTP(rec, req)

	return rec
}

func citPut(t *testing.T, handler *httpapi.AdminCatalog, auth *httpapi.Auth, cookie *http.Cookie, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	if cookie != nil {
		req.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()
	cloudInitTemplatesMux(handler, auth).ServeHTTP(rec, req)

	return rec
}

func citDelete(t *testing.T, handler *httpapi.AdminCatalog, auth *httpapi.Auth, cookie *http.Cookie, path string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodDelete, path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()
	cloudInitTemplatesMux(handler, auth).ServeHTTP(rec, req)

	return rec
}

// TestAdminCloudInitTemplates_ListAsAdmin — GET returns an empty list
// initially (no seed data, spec.md Assumptions), then a created template
// including disabled ones.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCloudInitTemplates_ListAsAdmin(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := citGet(t, handler, authHandler, cookie, "/api/v1/admin/cloudinit-templates?cluster=default")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var list []cloudInitTemplateDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}

	if len(list) != 0 {
		t.Fatalf("expected empty list initially, got %d", len(list))
	}

	// Create one, disable it, confirm it still appears in the admin list.
	create := citPost(t, handler, authHandler, cookie, "/api/v1/admin/cloudinit-templates",
		`{"cluster":"default","label":"Web server","content":"#cloud-config\npackages:\n  - nginx\n"}`)
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", create.Code, create.Body.String())
	}

	toggle := citPost(t, handler, authHandler, cookie, "/api/v1/admin/cloudinit-templates/web-server/toggle",
		`{"cluster":"default","enabled":false}`)
	if toggle.Code != http.StatusOK {
		t.Fatalf("toggle status = %d: %s", toggle.Code, toggle.Body.String())
	}

	rec = citGet(t, handler, authHandler, cookie, "/api/v1/admin/cloudinit-templates?cluster=default")
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}

	if len(list) != 1 || list[0].ID != "web-server" || list[0].Enabled {
		t.Fatalf("expected one disabled web-server, got %+v", list)
	}
}

// TestAdminCloudInitTemplates_NonAdmin_Returns403 — every endpoint returns 403
// for a non-admin identity (FR-010, SC-005).
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCloudInitTemplates_NonAdmin_Returns403(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	alice := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	cases := []struct {
		name string
		rec  *httptest.ResponseRecorder
	}{
		{"list", citGet(t, handler, authHandler, alice, "/api/v1/admin/cloudinit-templates?cluster=default")},
		{"create", citPost(t, handler, authHandler, alice, "/api/v1/admin/cloudinit-templates", `{"cluster":"default","label":"x","content":"#cloud-config\n"}`)},
		{"update", citPut(t, handler, authHandler, alice, "/api/v1/admin/cloudinit-templates/x", `{"cluster":"default","label":"x","content":"#cloud-config\n"}`)},
		{testActionDelete, citDelete(t, handler, authHandler, alice, "/api/v1/admin/cloudinit-templates/x?cluster=default")},
		{"toggle", citPost(t, handler, authHandler, alice, "/api/v1/admin/cloudinit-templates/x/toggle", `{"cluster":"default","enabled":false}`)},
	}
	for _, tc := range cases {
		if tc.rec.Code != http.StatusForbidden {
			t.Errorf("%s: status = %d, want %d", tc.name, tc.rec.Code, http.StatusForbidden)
		}
	}
}

// TestAdminCloudInitTemplates_Create — POST creates a template, returns 201
// with the derived slug and enabled=true.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCloudInitTemplates_Create(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := citPost(t, handler, authHandler, cookie, "/api/v1/admin/cloudinit-templates",
		`{"cluster":"default","label":"Web server","content":"#cloud-config\npackages:\n  - nginx\n"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var tmpl cloudInitTemplateDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &tmpl); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if tmpl.ID != "web-server" || !tmpl.Enabled || tmpl.Content == "" {
		t.Errorf("unexpected template: %+v", tmpl)
	}
}

// TestAdminCloudInitTemplates_CreateDuplicate_Returns409 — slug collision.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCloudInitTemplates_CreateDuplicate_Returns409(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	body := `{"cluster":"default","label":"Web server","content":"#cloud-config\n"}`
	if rec := citPost(t, handler, authHandler, cookie, "/api/v1/admin/cloudinit-templates", body); rec.Code != http.StatusCreated {
		t.Fatalf("first create status = %d: %s", rec.Code, rec.Body.String())
	}

	rec := citPost(t, handler, authHandler, cookie, "/api/v1/admin/cloudinit-templates", body)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusConflict, rec.Body.String())
	}

	assertAPIError(t, rec.Body.Bytes(), "duplicate_template")
}

// TestAdminCloudInitTemplates_CreateInvalidContent_Returns400 — content not
// starting with #cloud-config is rejected (FR-003, reused T08 validation).
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCloudInitTemplates_CreateInvalidContent_Returns400(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := citPost(t, handler, authHandler, cookie, "/api/v1/admin/cloudinit-templates",
		`{"cluster":"default","label":"Bad","content":"packages:\n  - nginx\n"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	assertAPIError(t, rec.Body.Bytes(), "invalid_content")
}

// TestAdminCloudInitTemplates_Update — PUT changes label and content.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCloudInitTemplates_Update(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	if rec := citPost(t, handler, authHandler, cookie, "/api/v1/admin/cloudinit-templates",
		`{"cluster":"default","label":"Web server","content":"#cloud-config\npackages:\n  - nginx\n"}`); rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", rec.Code, rec.Body.String())
	}

	rec := citPut(t, handler, authHandler, cookie, "/api/v1/admin/cloudinit-templates/web-server",
		`{"cluster":"default","label":"Web server (nginx + certbot)","content":"#cloud-config\npackages:\n  - nginx\n  - certbot\n"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d: %s", rec.Code, rec.Body.String())
	}

	var tmpl cloudInitTemplateDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &tmpl); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if tmpl.Label != "Web server (nginx + certbot)" || !strings.Contains(tmpl.Content, "certbot") {
		t.Errorf("unexpected template: %+v", tmpl)
	}
}

// TestAdminCloudInitTemplates_UpdateNotFound_Returns404.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCloudInitTemplates_UpdateNotFound_Returns404(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := citPut(t, handler, authHandler, cookie, "/api/v1/admin/cloudinit-templates/nonexistent",
		`{"cluster":"default","label":"x","content":"#cloud-config\n"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// TestAdminCloudInitTemplates_Delete — DELETE removes the template.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCloudInitTemplates_Delete(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	if rec := citPost(t, handler, authHandler, cookie, "/api/v1/admin/cloudinit-templates",
		`{"cluster":"default","label":"Web server","content":"#cloud-config\n"}`); rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", rec.Code, rec.Body.String())
	}

	rec := citDelete(t, handler, authHandler, cookie, "/api/v1/admin/cloudinit-templates/web-server?cluster=default")
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d: %s", rec.Code, rec.Body.String())
	}

	var status statusDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if status.Status != testStatusDeleted {
		t.Errorf("status = %q, want deleted", status.Status)
	}

	list := citGet(t, handler, authHandler, cookie, "/api/v1/admin/cloudinit-templates?cluster=default")

	var arr []cloudInitTemplateDTO
	if err := json.Unmarshal(list.Body.Bytes(), &arr); err != nil {
		t.Fatalf("decode list: %v", err)
	}

	if len(arr) != 0 {
		t.Errorf("expected empty list after delete, got %d", len(arr))
	}
}

// TestAdminCloudInitTemplates_Toggle — toggle disables then re-enables.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCloudInitTemplates_Toggle(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	if rec := citPost(t, handler, authHandler, cookie, "/api/v1/admin/cloudinit-templates",
		`{"cluster":"default","label":"Web server","content":"#cloud-config\n"}`); rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", rec.Code, rec.Body.String())
	}

	rec := citPost(t, handler, authHandler, cookie, "/api/v1/admin/cloudinit-templates/web-server/toggle",
		`{"cluster":"default","enabled":false}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle status = %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body.ID != "web-server" || body.Enabled {
		t.Errorf("unexpected toggle response: %+v", body)
	}
}
