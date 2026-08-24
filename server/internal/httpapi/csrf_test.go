package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/httpapi"
	"strings"
	"testing"
)

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestCSRF_MissingTokenReturns403(t *testing.T) {
	handler := newAuthHandler(t)
	mux := newCSRFRouter(t, handler)

	session, _ := loginCSRF(t, handler, `{"username":"alice","password":"pvmss-alice"}`)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/tokens", strings.NewReader(`{"label":"test","scope":"read"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(session)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}

	var body struct{ Code string }
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.Code != "invalid_csrf_token" {
		t.Fatalf("code = %q, want invalid_csrf_token", body.Code)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestCSRF_WrongTokenReturns403(t *testing.T) {
	handler := newAuthHandler(t)
	mux := newCSRFRouter(t, handler)

	session, csrfCookie := loginCSRF(t, handler, `{"username":"alice","password":"pvmss-alice"}`)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/tokens", strings.NewReader(`{"label":"test","scope":"read"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(session)
	req.AddCookie(csrfCookie)
	req.Header.Set("X-CSRF-Token", "wrong-token")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestCSRF_ValidTokenPasses(t *testing.T) {
	handler := newAuthHandler(t)
	mux := newCSRFRouter(t, handler)

	session, csrfCookie := loginCSRF(t, handler, `{"username":"alice","password":"pvmss-alice"}`)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/tokens", strings.NewReader(`{"label":"test","scope":"read"}`))
	req.Header.Set("Content-Type", "application/json")
	setCSRF(req, session, csrfCookie)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestCSRF_GetRequestsAreNotBlocked(t *testing.T) {
	handler := newAuthHandler(t)
	mux := newCSRFRouter(t, handler)

	session, _ := loginCSRF(t, handler, `{"username":"alice","password":"pvmss-alice"}`)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(session)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func newCSRFRouter(t *testing.T, handler *httpapi.Auth) http.Handler {
	t.Helper()

	return httpapi.NewRouter(httpapi.RouterConfig{
		Health:         http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }),
		ClusterNodes:   http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }),
		ClusterRefresh: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }),
		VMs:            http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }),
		VMDetail:       http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }),
		Auth:           handler,
		Log:            slog.New(slog.DiscardHandler),
	})
}

func loginCSRF(t *testing.T, handler *httpapi.Auth, body string) (session, csrf *http.Cookie) {
	t.Helper()

	response := serveJSON(handler.Login, "/api/v1/auth/login", body)
	if response.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}

	return findCSRFCookies(t, response)
}

func findCSRFCookies(t *testing.T, response *httptest.ResponseRecorder) (session, csrf *http.Cookie) {
	t.Helper()

	for _, c := range response.Result().Cookies() {
		switch c.Name {
		case auth.SessionCookieName:
			session = c
		case auth.CSRFCookieName:
			csrf = c
		}
	}

	if session == nil {
		t.Fatal("session cookie not found")
	}

	if csrf == nil {
		t.Fatal("csrf cookie not found")
	}

	return session, csrf
}

func setCSRF(req *http.Request, session, csrf *http.Cookie) {
	req.AddCookie(session)
	req.AddCookie(csrf)
	req.Header.Set("X-CSRF-Token", csrf.Value)
}
