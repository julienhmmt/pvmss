//nolint:noctx // test scaffolding does not need real context
package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMCreateCatalog_WithPolicyReturnsLimits(t *testing.T) {
	handler, authHandler, _ := newVMCreateHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/vm-create/catalog", nil)
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	handler.ServeCatalog(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMCreate_InvalidJSON(t *testing.T) {
	handler, authHandler, _ := newVMCreateHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/vms", strings.NewReader(`{invalid`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMCreate_EmptyBody(t *testing.T) {
	handler, authHandler, _ := newVMCreateHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/vms", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}
