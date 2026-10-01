package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var errSessionStore = errors.New("session store down")

// brokenSessionRepository fails every write, as a database outage would.
type brokenSessionRepository struct{}

func (brokenSessionRepository) CreateSession(context.Context, auth.SessionRecord) error {
	return errSessionStore
}

func (brokenSessionRepository) FindSession(context.Context, []byte) (auth.SessionRecord, error) {
	return auth.SessionRecord{}, errSessionStore
}

func (brokenSessionRepository) TouchSession(context.Context, []byte, time.Time) error {
	return errSessionStore
}

func (brokenSessionRepository) DeleteSession(context.Context, []byte) error {
	return errSessionStore
}

func newAuthWithBrokenSessions(t *testing.T) *httpapi.Auth {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte("pvmss-local-admin"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	sessions, err := auth.NewSessionManager(brokenSessionRepository{}, "a-session-secret-with-at-least-thirty-two-bytes", false)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}

	return httpapi.NewAuth(cluster.Fake{}, sessions, string(hash), testLogger(t))
}

// A session store outage must surface as a 500, never as a login that looks
// successful but carries no session.
//
//nolint:paralleltest // serial: shared fake authentication state
func TestAuth_LoginFailsClosedWhenSessionStoreIsDown(t *testing.T) {
	handler := newAuthWithBrokenSessions(t)

	rec := serveJSON(handler.AdminLogin, "/api/v1/auth/admin-login", `{"password":"pvmss-local-admin"}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500: %s", rec.Code, rec.Body.String())
	}

	if len(rec.Result().Cookies()) != 0 {
		t.Fatalf("cookies = %v, want none when the session could not be stored", rec.Result().Cookies())
	}
}

//nolint:paralleltest // serial: shared fake authentication state
func TestAuth_LogoutReportsRevocationFailure(t *testing.T) {
	handler := newAuthWithBrokenSessions(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "some-session"}) //nolint:gosec // test fixture cookie

	rec := httptest.NewRecorder()
	handler.Logout(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500: %s", rec.Code, rec.Body.String())
	}
}
