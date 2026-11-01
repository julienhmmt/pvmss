//nolint:noctx // test scaffolding does not need real context
package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/auth"
	"strings"
	"testing"
	"time"
)

const aliceName = "alice"

var aliceIdentity = auth.Identity{Username: aliceName, Pool: "pvmss-alice"}

var errRepository = errors.New("repository failure")

// failingSessionRepository returns errRepository from every call, driving the
// error branches of the SessionManager methods under test.
type failingSessionRepository struct{}

func (failingSessionRepository) CreateSession(context.Context, auth.SessionRecord) error {
	return errRepository
}

func (failingSessionRepository) FindSession(context.Context, []byte) (auth.SessionRecord, error) {
	return auth.SessionRecord{}, errRepository
}

func (failingSessionRepository) TouchSession(context.Context, []byte, time.Time) error {
	return errRepository
}

func (failingSessionRepository) DeleteSession(context.Context, []byte) error {
	return errRepository
}

func (failingSessionRepository) DeleteOtherSessions(context.Context, string, string, []byte) error {
	return errRepository
}

func newManager(t *testing.T, repository auth.SessionRepository) *auth.SessionManager {
	t.Helper()

	manager, err := auth.NewSessionManager(repository, "a-session-secret-with-at-least-thirty-two-bytes", false)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}

	return manager
}

func TestNewSessionManager_SecretSize(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		secret  string
		wantErr bool
	}{
		{name: "empty", secret: "", wantErr: true},
		{name: "31 bytes", secret: strings.Repeat("x", 31), wantErr: true},
		{name: "32 bytes", secret: strings.Repeat("x", 32), wantErr: false},
		{name: "64 bytes", secret: strings.Repeat("x", 64), wantErr: false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := auth.NewSessionManager(newSessionRepository(), tt.secret, false)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewSessionManager(%d bytes) error = %v, wantErr %v", len(tt.secret), err, tt.wantErr)
			}
		})
	}
}

//nolint:paralleltest // serial: writes a real session through SetCookie
func TestSessionManager_SetCookie_RepositoryFailure(t *testing.T) {
	manager := newManager(t, failingSessionRepository{})

	err := manager.SetCookie(context.Background(), httptest.NewRecorder(), aliceIdentity)
	if err == nil || !strings.Contains(err.Error(), "create session") {
		t.Fatalf("SetCookie error = %v, want wrapped create-session failure", err)
	}
}

//nolint:paralleltest // serial: shared repository fixture
func TestSessionManager_Resolve_TouchFailureReturnsError(t *testing.T) {
	repository := newSessionRepository()
	manager := newManager(t, repository)

	recorder := httptest.NewRecorder()
	if err := manager.SetCookie(context.Background(), recorder, aliceIdentity); err != nil {
		t.Fatalf("SetCookie: %v", err)
	}

	cookie := recorder.Result().Cookies()[0]

	// Shrink the expiry past the touch interval so Resolve must slide it.
	repository.setExpiresIn(7 * time.Hour)

	failing := &touchFailingRepository{sessionRepository: repository}
	stale := newManager(t, failing)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(cookie)

	_, err := stale.Resolve(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "slide session expiry") {
		t.Fatalf("Resolve error = %v, want wrapped slide-expiry failure", err)
	}
}

// touchFailingRepository fails only TouchSession; reads go through the real
// in-memory fixture.
type touchFailingRepository struct {
	*sessionRepository
}

func (r *touchFailingRepository) TouchSession(context.Context, []byte, time.Time) error {
	return errRepository
}

//nolint:paralleltest // serial: reads cookie state on requests
func TestSessionManager_RevokeOthers_MissingCookie(t *testing.T) {
	manager := newManager(t, newSessionRepository())

	request := httptest.NewRequest(http.MethodGet, "/", nil)

	err := manager.RevokeOthers(context.Background(), request, aliceIdentity)
	if err == nil || !strings.Contains(err.Error(), "read session cookie") {
		t.Fatalf("RevokeOthers error = %v, want read-cookie failure", err)
	}
}

//nolint:paralleltest // serial: shared repository fixture
func TestSessionManager_RevokeOthers_RepositoryFailure(t *testing.T) {
	repository := newSessionRepository()
	manager := newManager(t, repository)

	recorder := httptest.NewRecorder()
	if err := manager.SetCookie(context.Background(), recorder, aliceIdentity); err != nil {
		t.Fatalf("SetCookie: %v", err)
	}

	cookie := recorder.Result().Cookies()[0]

	failing := newManager(t, failingSessionRepository{})

	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request.AddCookie(cookie)

	err := failing.RevokeOthers(context.Background(), request, aliceIdentity)
	if err == nil || !strings.Contains(err.Error(), "revoke other sessions") {
		t.Fatalf("RevokeOthers error = %v, want wrapped revoke failure", err)
	}
}

//nolint:paralleltest // serial: reads cookie state on requests
func TestSessionManager_CSRFToken_Unauthenticated(t *testing.T) {
	manager := newManager(t, newSessionRepository())

	cases := []struct {
		name   string
		cookie *http.Cookie
	}{
		{name: "no cookie", cookie: nil},
		{name: "unknown session", cookie: &http.Cookie{Name: auth.SessionCookieName, Value: "unknown-session-value"}}, //nolint:gosec // test fixture cookie
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.cookie != nil {
				request.AddCookie(tt.cookie)
			}

			_, err := manager.CSRFToken(context.Background(), request)
			if !errors.Is(err, auth.ErrUnauthenticated) {
				t.Fatalf("CSRFToken error = %v, want ErrUnauthenticated", err)
			}
		})
	}
}

//nolint:paralleltest // serial: reads cookie state on requests
func TestSessionManager_CSRFToken_RepositoryErrorIsUnauthenticated(t *testing.T) {
	manager := newManager(t, failingSessionRepository{})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "any-value"}) //nolint:gosec // test fixture cookie

	_, err := manager.CSRFToken(context.Background(), request)
	if !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("CSRFToken error = %v, want ErrUnauthenticated", err)
	}
}

//nolint:paralleltest // serial: reads cookie state on requests
func TestSessionManager_Logout_WithoutCookieClearsCookies(t *testing.T) {
	manager := newManager(t, newSessionRepository())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	if err := manager.Logout(context.Background(), recorder, request); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	cookies := recorder.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("cleared cookies = %d, want 2 (session + csrf)", len(cookies))
	}

	for _, cookie := range cookies {
		if cookie.MaxAge != -1 || cookie.Value != "" {
			t.Fatalf("cookie %s not cleared: %+v", cookie.Name, cookie)
		}
	}
}
