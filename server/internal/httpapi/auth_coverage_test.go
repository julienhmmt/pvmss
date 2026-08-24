//nolint:noctx // test scaffolding does not need real context
package httpapi_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"strings"
	"testing"
)

func authErrorResponse(t *testing.T, rec *httptest.ResponseRecorder) (string, string) {
	t.Helper()

	var body struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}

	return body.Code, body.Message
}

// authAuthenticatedRequest logs in as alice, builds a POST request with the
// provided path and body, attaches the session cookie, and invokes do. Shared
// by the auth coverage tests to avoid duplicated login+request setup (dupl).
func authAuthenticatedRequest(t *testing.T, path, body string, do func(*httpapi.Auth, http.ResponseWriter, *http.Request)) *httptest.ResponseRecorder {
	t.Helper()

	handler := newAuthHandler(t)
	login := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice"}`)
	cookie := login.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	do(handler, rec, req)

	return rec
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_Login_MethodNotAllowed(t *testing.T) {
	handler := newAuthHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/login", nil)
	rec := httptest.NewRecorder()
	handler.Login(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}

	code, _ := authErrorResponse(t, rec)
	if code != apiCodeMethodNotAllowed {
		t.Errorf("code = %q, want method_not_allowed", code)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_Login_InvalidJSON(t *testing.T) {
	handler := newAuthHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.Login(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	code, _ := authErrorResponse(t, rec)
	if code != apiCodeInvalidRequest {
		t.Errorf("code = %q, want invalid_request", code)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_Login_EmptyUsernameAndPassword(t *testing.T) {
	handler := newAuthHandler(t)

	rec := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"","password":""}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	code, _ := authErrorResponse(t, rec)
	if code != apiCodeInvalidRequest {
		t.Errorf("code = %q, want invalid_request", code)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_Login_UnknownCluster(t *testing.T) {
	handler, _ := newAuthHandlerWithStore(t)

	rec := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice","cluster":"nonexistent"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	code, _ := authErrorResponse(t, rec)
	if code != "invalid_cluster" {
		t.Errorf("code = %q, want invalid_cluster", code)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_AdminLogin_MethodNotAllowed(t *testing.T) {
	handler := newAuthHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/admin-login", nil)
	rec := httptest.NewRecorder()
	handler.AdminLogin(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_AdminLogin_InvalidJSON(t *testing.T) {
	handler := newAuthHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/admin-login", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.AdminLogin(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_AdminLogin_EmptyPassword(t *testing.T) {
	handler := newAuthHandler(t)

	rec := serveJSON(handler.AdminLogin, "/api/v1/auth/admin-login", `{"password":""}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_AdminLogin_WrongPassword(t *testing.T) {
	handler := newAuthHandler(t)

	rec := serveJSON(handler.AdminLogin, "/api/v1/auth/admin-login", `{"password":"wrong-admin-password"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_Me_Unauthenticated(t *testing.T) {
	handler := newAuthHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	rec := httptest.NewRecorder()
	handler.Me(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	code, _ := authErrorResponse(t, rec)
	if code != "unauthenticated" {
		t.Errorf("code = %q, want unauthenticated", code)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_Logout_MethodNotAllowed(t *testing.T) {
	handler := newAuthHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/logout", nil)
	rec := httptest.NewRecorder()
	handler.Logout(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_Logout_NoSessionStill204(t *testing.T) {
	handler := newAuthHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	rec := httptest.NewRecorder()
	handler.Logout(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_CreateToken_Unauthenticated(t *testing.T) {
	handler := newAuthHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", strings.NewReader(`{"label":"x","scope":"read"}`))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.CreateToken(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_CreateToken_InvalidJSON(t *testing.T) {
	rec := authAuthenticatedRequest(t, "/api/v1/auth/tokens", "{bad", func(h *httpapi.Auth, rec http.ResponseWriter, req *http.Request) {
		h.CreateToken(rec, req)
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	assertAPIError(t, rec.Body.Bytes(), apiCodeInvalidRequest)
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_ListTokens_Unauthenticated(t *testing.T) {
	handler := newAuthHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/tokens", nil)
	rec := httptest.NewRecorder()
	handler.ListTokens(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_RevokeToken_Unauthenticated(t *testing.T) {
	handler := newAuthHandler(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/tokens/abc", nil)
	req.SetPathValue("id", "abc")

	rec := httptest.NewRecorder()
	handler.RevokeToken(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_ChangePassword_Unauthenticated(t *testing.T) {
	handler := newAuthHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password", strings.NewReader(`{"oldPassword":"x","newPassword":"newpass12"}`))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ChangePassword(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_ChangePassword_InvalidJSON(t *testing.T) {
	rec := authAuthenticatedRequest(t, "/api/v1/auth/password", "{bad", func(h *httpapi.Auth, rec http.ResponseWriter, req *http.Request) {
		h.ChangePassword(rec, req)
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	assertAPIError(t, rec.Body.Bytes(), apiCodeInvalidRequest)
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_ChangePassword_WrongOldPassword(t *testing.T) {
	rec := authAuthenticatedRequest(t, "/api/v1/auth/password", `{"oldPassword":"wrong-old","newPassword":"newpass12"}`, func(h *httpapi.Auth, rec http.ResponseWriter, req *http.Request) {
		h.ChangePassword(rec, req)
	})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_ServeClusters_MethodNotAllowed(t *testing.T) {
	handler := newAuthHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/clusters", nil)
	rec := httptest.NewRecorder()
	handler.ServeClusters(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_ServeClusters_NoStoreReturnsDefault(t *testing.T) {
	handler := newAuthHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/clusters", nil)
	rec := httptest.NewRecorder()
	handler.ServeClusters(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var clusters []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &clusters); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(clusters) != 1 || clusters[0].Name != auditTestCluster {
		t.Fatalf("clusters = %+v, want [{default}]", clusters)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_ServeClusters_WithStore(t *testing.T) {
	handler, _ := newAuthHandlerWithStore(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/clusters", nil)
	rec := httptest.NewRecorder()
	handler.ServeClusters(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var clusters []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &clusters); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(clusters) == 0 {
		t.Fatal("expected at least one cluster from store")
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_OIDC_InvalidJSON(t *testing.T) {
	handler := newAuthHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/oidc", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.OIDC(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_OIDC_NoStoreReturns501(t *testing.T) {
	handler := newAuthHandler(t)

	rec := serveJSON(handler.OIDC, "/api/v1/auth/oidc", `{"cluster":"default"}`)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotImplemented)
	}

	code, _ := authErrorResponse(t, rec)
	if code != "not_implemented" {
		t.Errorf("code = %q, want not_implemented", code)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_OIDC_WithStoreDisabledClusterReturns404(t *testing.T) {
	handler, _ := newAuthHandlerWithStore(t)

	rec := serveJSON(handler.OIDC, "/api/v1/auth/oidc", `{"cluster":"default"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	code, _ := authErrorResponse(t, rec)
	if code != apiCodeNotFound {
		t.Errorf("code = %q, want not_found", code)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_Principal_MalformedCookie(t *testing.T) {
	handler := newAuthHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	//nolint:gosec // G124: intentionally insecure test cookie for malformed value handling
	req.AddCookie(&http.Cookie{Name: "pvmss_session", Value: "malformed-cookie-value"})

	_, err := handler.Principal(req)
	if err == nil {
		t.Fatal("expected error for malformed cookie")
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_Principal_NoCookieNoBearer(t *testing.T) {
	handler := newAuthHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vms/default/100", nil)

	_, err := handler.Principal(req)
	if err == nil {
		t.Fatal("expected error for no cookie and no bearer")
	}

	if !errors.Is(err, auth.ErrUnauthenticated) {
		t.Errorf("err = %v, want auth.ErrUnauthenticated", err)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_Principal_BearerWithInvalidToken(t *testing.T) {
	handler := newAuthHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vms/default/100", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-value")

	_, err := handler.Principal(req)
	if err == nil {
		t.Fatal("expected error for invalid bearer token")
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_Require_Unauthenticated(t *testing.T) {
	handler := newAuthHandler(t)
	called := false
	guarded := handler.Require(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vms/default/100", nil)
	rec := httptest.NewRecorder()
	guarded.ServeHTTP(rec, req)

	if called {
		t.Fatal("handler should not be called for unauthenticated request")
	}

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_RequireAdmin_Unauthenticated(t *testing.T) {
	handler := newAuthHandler(t)
	called := false
	guarded := handler.RequireAdmin(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/nodes", nil)
	rec := httptest.NewRecorder()
	guarded.ServeHTTP(rec, req)

	if called {
		t.Fatal("handler should not be called for unauthenticated request")
	}

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_RequireAdmin_NonAdminForbidden(t *testing.T) {
	handler := newAuthHandler(t)
	called := false
	guarded := handler.RequireAdmin(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	login := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice"}`)
	cookie := login.Result().Cookies()[0]
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/nodes", nil)
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	guarded.ServeHTTP(rec, req)

	if called {
		t.Fatal("handler should not be called for non-admin request")
	}

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_Login_PoolPrefixRetry(t *testing.T) {
	handler := newAuthHandler(t)

	rec := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var identity auth.Identity
	if err := json.Unmarshal(rec.Body.Bytes(), &identity); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if identity.Username != cluster.FakeUserAlice {
		t.Errorf("username = %q, want %q", identity.Username, cluster.FakeUserAlice)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_Login_UnknownUser(t *testing.T) {
	handler := newAuthHandler(t)

	rec := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"nobody","password":"whatever"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_Require_AuthenticatedPasses(t *testing.T) {
	handler := newAuthHandler(t)
	called := false
	guarded := handler.Require(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true

		w.WriteHeader(http.StatusOK)
	}))
	login := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice"}`)
	cookie := login.Result().Cookies()[0]
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vms/default/100", nil)
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	guarded.ServeHTTP(rec, req)

	if !called {
		t.Fatal("handler should be called for authenticated request")
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_RequireAdmin_AdminPasses(t *testing.T) {
	handler := newAuthHandler(t)
	called := false
	guarded := handler.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true

		w.WriteHeader(http.StatusOK)
	}))
	cookie := adminCookie(t, handler)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/nodes", nil)
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	guarded.ServeHTTP(rec, req)

	if !called {
		t.Fatal("handler should be called for admin request")
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_Me_WithStoreAndNoDisplayName(t *testing.T) {
	handler, _ := newAuthHandlerWithStore(t)
	login := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice","cluster":"default"}`)
	cookie := login.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	handler.Me(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var identity auth.Identity
	if err := json.Unmarshal(rec.Body.Bytes(), &identity); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if identity.Username != cluster.FakeUserAlice {
		t.Errorf("username = %q, want %q", identity.Username, cluster.FakeUserAlice)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_ChangePassword_WithRegistrySucceeds(t *testing.T) {
	handler, _ := newAuthHandlerWithStore(t)
	login := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice","cluster":"default"}`)
	cookie := login.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password", strings.NewReader(`{"oldPassword":"pvmss-alice","newPassword":"newpass12"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	handler.ChangePassword(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}

	t.Cleanup(func() {
		restore := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password", strings.NewReader(`{"oldPassword":"newpass12","newPassword":"pvmss-alice"}`))
		restore.Header.Set("Content-Type", "application/json")
		restore.AddCookie(cookie)
		handler.ChangePassword(httptest.NewRecorder(), restore)
	})
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_CreateToken_EmptyLabelReturns400(t *testing.T) {
	rec := authAuthenticatedRequest(t, "/api/v1/auth/tokens", `{"label":"","scope":"read"}`, func(h *httpapi.Auth, rec http.ResponseWriter, req *http.Request) {
		h.CreateToken(rec, req)
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_RevokeToken_NotFound(t *testing.T) {
	handler := newAuthHandler(t)
	login := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice"}`)
	cookie := login.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/tokens/nonexistent-id", nil)
	req.SetPathValue("id", "nonexistent-id")
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	handler.RevokeToken(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_Login_UnknownFieldRejected(t *testing.T) {
	handler := newAuthHandler(t)

	rec := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice","extra":"field"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (unknown field rejected)", rec.Code, http.StatusBadRequest)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_AdminLogin_UnknownFieldRejected(t *testing.T) {
	handler := newAuthHandler(t)

	rec := serveJSON(handler.AdminLogin, "/api/v1/auth/admin-login", `{"password":"pvmss-local-admin","extra":"field"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (unknown field rejected)", rec.Code, http.StatusBadRequest)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_Login_MultipleJSONValues(t *testing.T) {
	handler := newAuthHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"alice","password":"pvmss-alice"}{"second":"object"}`))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.Login(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (multiple JSON values rejected)", rec.Code, http.StatusBadRequest)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_Login_PreservesClusterInIdentity(t *testing.T) {
	handler, _ := newAuthHandlerWithStore(t)

	rec := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice","cluster":"default"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var identity auth.Identity
	if err := json.Unmarshal(rec.Body.Bytes(), &identity); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if identity.Cluster != auditTestCluster {
		t.Errorf("cluster = %q, want default", identity.Cluster)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_Logout_WithSessionSucceeds(t *testing.T) {
	handler := newAuthHandler(t)
	login := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice"}`)
	cookie := login.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	handler.Logout(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_ListTokens_WithSessionReturnsList(t *testing.T) {
	handler := newAuthHandler(t)
	login := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice"}`)
	cookie := login.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/tokens", nil)
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	handler.ListTokens(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthCoverage_Principal_BearerTokenAuthenticates(t *testing.T) {
	handler := newAuthHandler(t)
	login := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice"}`)
	cookie := login.Result().Cookies()[0]

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", strings.NewReader(`{"label":"test","scope":"read"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(cookie)

	createRec := httptest.NewRecorder()
	handler.CreateToken(createRec, createReq)

	var created struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode token: %v", err)
	}

	bearerReq := httptest.NewRequest(http.MethodGet, "/api/v1/vms/default/100", nil)
	bearerReq.Header.Set("Authorization", "Bearer "+created.Value)

	identity, err := handler.Principal(bearerReq)
	if err != nil {
		t.Fatalf("Principal: %v", err)
	}

	if identity.Username != cluster.FakeUserAlice {
		t.Errorf("username = %q, want %q", identity.Username, cluster.FakeUserAlice)
	}
}
