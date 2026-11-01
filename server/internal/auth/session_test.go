//nolint:noctx,goconst // test scaffolding does not need real context; identity literal reused across cases
package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/auth"
	"sync"
	"testing"
	"time"
)

type sessionRepository struct {
	mutex    sync.Mutex
	sessions map[string]auth.SessionRecord
	touches  int
}

func newSessionRepository() *sessionRepository {
	return &sessionRepository{sessions: make(map[string]auth.SessionRecord)}
}

func (r *sessionRepository) CreateSession(_ context.Context, session auth.SessionRecord) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.sessions[string(session.Hash)] = session

	return nil
}

func (r *sessionRepository) FindSession(_ context.Context, hash []byte) (auth.SessionRecord, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	session, ok := r.sessions[string(hash)]
	if !ok {
		return auth.SessionRecord{}, errors.New("session not found")
	}

	return session, nil
}

func (r *sessionRepository) TouchSession(_ context.Context, hash []byte, expiresAt time.Time) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	session, ok := r.sessions[string(hash)]
	if !ok {
		return errors.New("session not found")
	}

	session.ExpiresAt = expiresAt
	r.sessions[string(hash)] = session
	r.touches++

	return nil
}

func (r *sessionRepository) DeleteSession(_ context.Context, hash []byte) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	delete(r.sessions, string(hash))

	return nil
}

func (r *sessionRepository) DeleteOtherSessions(_ context.Context, cluster, username string, keep []byte) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for hash, session := range r.sessions {
		if session.Identity.Cluster == cluster && session.Identity.Username == username && hash != string(keep) {
			delete(r.sessions, hash)
		}
	}

	return nil
}

func (r *sessionRepository) expireAll() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for hash, session := range r.sessions {
		session.ExpiresAt = time.Now().Add(-time.Second)
		r.sessions[hash] = session
	}
}

func (r *sessionRepository) ageAll(d time.Duration) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for hash, session := range r.sessions {
		session.CreatedAt = session.CreatedAt.Add(-d)
		r.sessions[hash] = session
	}
}

//nolint:paralleltest // serial: shared session repository fixture
func TestSessionManager_RoundTrip(t *testing.T) {
	manager, err := auth.NewSessionManager(newSessionRepository(), "a-session-secret-with-at-least-thirty-two-bytes", false)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}

	identity := auth.Identity{Username: "alice@pve"}

	recorder := httptest.NewRecorder()
	if err := manager.SetCookie(context.Background(), recorder, identity); err != nil {
		t.Fatalf("SetCookie: %v", err)
	}

	cookie := recorder.Result().Cookies()[0]
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(cookie)

	got, err := manager.Resolve(context.Background(), request)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if got != identity {
		t.Fatalf("identity = %+v, want %+v", got, identity)
	}
}

//nolint:paralleltest // serial: shared session repository fixture
func TestSessionManager_ResolveRejectsUnknownAndExpiredCookies(t *testing.T) {
	repository := newSessionRepository()

	manager, err := auth.NewSessionManager(repository, "a-session-secret-with-at-least-thirty-two-bytes", false)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}

	recorder := httptest.NewRecorder()
	if err := manager.SetCookie(context.Background(), recorder, auth.Identity{Username: "alice@pve"}); err != nil {
		t.Fatalf("SetCookie: %v", err)
	}

	expiredCookie := recorder.Result().Cookies()[0]

	repository.expireAll()

	cases := []struct {
		name   string
		cookie *http.Cookie
	}{
		{name: "unknown", cookie: &http.Cookie{Name: auth.SessionCookieName, Value: "unknown-session-value"}}, //nolint:gosec // test fixture cookie
		{name: "expired", cookie: expiredCookie},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.AddCookie(tc.cookie)

			if _, err := manager.Resolve(context.Background(), request); err == nil {
				t.Fatal("expected invalid session error")
			}
		})
	}
}

// Regresses the fix for the original stateless signed-cookie session: a
// signature-only cookie stays valid after logout until its embedded expiry.
// A DB-backed session must reject the same raw cookie value immediately once
// its row is deleted.
//
//nolint:paralleltest // serial: shared session repository fixture
func TestSessionManager_LogoutRevokesSessionImmediately(t *testing.T) {
	manager, err := auth.NewSessionManager(newSessionRepository(), "a-session-secret-with-at-least-thirty-two-bytes", false)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}

	recorder := httptest.NewRecorder()
	if err := manager.SetCookie(context.Background(), recorder, auth.Identity{Username: "alice@pve"}); err != nil {
		t.Fatalf("SetCookie: %v", err)
	}

	cookie := recorder.Result().Cookies()[0]

	logoutRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	logoutRequest.AddCookie(cookie)

	if err := manager.Logout(context.Background(), httptest.NewRecorder(), logoutRequest); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	replay := httptest.NewRequest(http.MethodGet, "/", nil)
	replay.AddCookie(cookie)

	if _, err := manager.Resolve(context.Background(), replay); err == nil {
		t.Fatal("expected revoked session to be rejected on replay")
	}
}

// Sliding expiry alone would let an actively used session live forever, and
// keep the admin flag it was issued with. An absolute lifetime bounds both.
//
//nolint:paralleltest // serial: shared session repository fixture
func TestSessionManager_RejectsSessionPastAbsoluteLifetime(t *testing.T) {
	repository := newSessionRepository()

	manager, err := auth.NewSessionManager(repository, "a-session-secret-with-at-least-thirty-two-bytes", false)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}

	recorder := httptest.NewRecorder()
	if err := manager.SetCookie(context.Background(), recorder, auth.Identity{Username: "alice@pve"}); err != nil {
		t.Fatalf("SetCookie: %v", err)
	}

	cookie := recorder.Result().Cookies()[0]
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(cookie)

	repository.ageAll(auth.MaxSessionAge - time.Minute)

	if _, err := manager.Resolve(context.Background(), request); err != nil {
		t.Fatalf("session just under the absolute lifetime must resolve: %v", err)
	}

	repository.ageAll(2 * time.Minute)

	if _, err := manager.Resolve(context.Background(), request); err == nil {
		t.Fatal("Resolve accepted a session past the absolute lifetime")
	}

	if _, err := manager.CSRFToken(context.Background(), request); err == nil {
		t.Fatal("CSRFToken accepted a session past the absolute lifetime")
	}
}

func (r *sessionRepository) setExpiresIn(d time.Duration) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for hash, session := range r.sessions {
		session.ExpiresAt = time.Now().Add(d)
		r.sessions[hash] = session
	}
}

func (r *sessionRepository) touchCount() int {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return r.touches
}

// Every API call resolves the session; sliding the expiry on each one turned
// every request (console and status polling included) into a database write.
//
//nolint:paralleltest // serial: shared session repository fixture
func TestSessionManager_ResolveSlidesExpiryOnlyOnceStale(t *testing.T) {
	repository := newSessionRepository()

	manager, err := auth.NewSessionManager(repository, "a-session-secret-with-at-least-thirty-two-bytes", false)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}

	recorder := httptest.NewRecorder()
	if err := manager.SetCookie(context.Background(), recorder, auth.Identity{Username: "alice@pve"}); err != nil {
		t.Fatalf("SetCookie: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(recorder.Result().Cookies()[0])

	for range 3 {
		if _, err := manager.Resolve(context.Background(), request); err != nil {
			t.Fatalf("Resolve: %v", err)
		}
	}

	if got := repository.touchCount(); got != 0 {
		t.Fatalf("touches after resolving a fresh session = %d, want 0", got)
	}

	repository.setExpiresIn(7 * time.Hour)

	if _, err := manager.Resolve(context.Background(), request); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if got := repository.touchCount(); got != 1 {
		t.Fatalf("touches after the expiry aged an hour = %d, want 1", got)
	}
}

// The browser must keep the cookies for the session's absolute lifetime: the
// server slides the 8h idle expiry on use, so a cookie that dies 8h after
// login would sign out an active user early.
//
//nolint:paralleltest // serial: shared session repository fixture
func TestSessionManager_CookiesLastTheAbsoluteLifetime(t *testing.T) {
	manager, err := auth.NewSessionManager(newSessionRepository(), "a-session-secret-with-at-least-thirty-two-bytes", false)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}

	recorder := httptest.NewRecorder()
	if err := manager.SetCookie(context.Background(), recorder, auth.Identity{Username: "alice@pve"}); err != nil {
		t.Fatalf("SetCookie: %v", err)
	}

	floor := time.Now().Add(auth.MaxSessionAge - time.Minute)
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Expires.Before(floor) {
			t.Fatalf("cookie %s expires %v, want about %v after login", cookie.Name, cookie.Expires, auth.MaxSessionAge)
		}
	}
}

func cookieByName(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}

	return nil
}

// issueSession logs identity in and returns the session cookie plus the CSRF
// cookie that SetCookie wrote.
func issueSession(t *testing.T, manager *auth.SessionManager, identity auth.Identity) (*http.Cookie, *http.Cookie) {
	t.Helper()

	recorder := httptest.NewRecorder()
	if err := manager.SetCookie(context.Background(), recorder, identity); err != nil {
		t.Fatalf("SetCookie: %v", err)
	}

	sessionCookie := cookieByName(recorder.Result().Cookies(), auth.SessionCookieName)
	csrfCookie := cookieByName(recorder.Result().Cookies(), auth.CSRFCookieName)

	if sessionCookie == nil || csrfCookie == nil {
		t.Fatalf("SetCookie wrote %v, want session+csrf cookies", recorder.Result().Cookies())
	}

	return sessionCookie, csrfCookie
}

// After a password change the user keeps only the session that made the
// change: every other session for the same cluster+username must die, while
// other usernames and other clusters are untouched.
func TestSessionManager_RevokeOthers(t *testing.T) {
	t.Parallel()

	manager, err := auth.NewSessionManager(newSessionRepository(), "a-session-secret-with-at-least-thirty-two-bytes", false)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}

	alice := auth.Identity{Username: "alice@pve", Cluster: "default"}

	current, _ := issueSession(t, manager, alice)
	other, _ := issueSession(t, manager, alice)
	bobSession, _ := issueSession(t, manager, auth.Identity{Username: "bob@pve", Cluster: "default"})
	elsewhere, _ := issueSession(t, manager, auth.Identity{Username: "alice@pve", Cluster: "remote"})

	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request.AddCookie(current)

	if err := manager.RevokeOthers(context.Background(), request, alice); err != nil {
		t.Fatalf("RevokeOthers: %v", err)
	}

	t.Run("keeps the requesting session", func(t *testing.T) {
		t.Parallel()

		kept := httptest.NewRequest(http.MethodGet, "/", nil)
		kept.AddCookie(current)

		if _, err := manager.Resolve(context.Background(), kept); err != nil {
			t.Fatal("the session that requested revocation was revoked")
		}
	})

	t.Run("revokes the user's other sessions", func(t *testing.T) {
		t.Parallel()

		revoked := httptest.NewRequest(http.MethodGet, "/", nil)
		revoked.AddCookie(other)

		if _, err := manager.Resolve(context.Background(), revoked); err == nil {
			t.Fatal("a second session of the same user still resolves")
		}
	})

	t.Run("leaves other users and clusters alone", func(t *testing.T) {
		t.Parallel()

		for _, cookie := range []*http.Cookie{bobSession, elsewhere} {
			check := httptest.NewRequest(http.MethodGet, "/", nil)
			check.AddCookie(cookie)

			if _, err := manager.Resolve(context.Background(), check); err != nil {
				t.Fatalf("session %q was revoked but is out of scope", cookie.Value[:8])
			}
		}
	})
}

// A request without a session cookie cannot name the session to keep, so the
// call fails instead of deleting blindly.
func TestSessionManager_RevokeOthers_NoCookie(t *testing.T) {
	t.Parallel()

	manager, err := auth.NewSessionManager(newSessionRepository(), "a-session-secret-with-at-least-thirty-two-bytes", false)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/", nil)
	if err := manager.RevokeOthers(context.Background(), request, auth.Identity{Username: "alice@pve"}); err == nil {
		t.Fatal("RevokeOthers without a session cookie = nil, want error")
	}
}

// A browser that lost only its CSRF cookie (cleared site data, expired tab)
// gets it re-issued from the persisted session state instead of being forced
// to log in again.
func TestSessionManager_EnsureCSRFCookie_ReissuesFromSession(t *testing.T) {
	t.Parallel()

	manager, err := auth.NewSessionManager(newSessionRepository(), "a-session-secret-with-at-least-thirty-two-bytes", false)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}

	sessionCookie, csrfCookie := issueSession(t, manager, auth.Identity{Username: "alice@pve"})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(sessionCookie)

	if err := manager.EnsureCSRFCookie(context.Background(), recorder, request); err != nil {
		t.Fatalf("EnsureCSRFCookie: %v", err)
	}

	reissued := cookieByName(recorder.Result().Cookies(), auth.CSRFCookieName)
	if reissued == nil {
		t.Fatal("no CSRF cookie re-issued for a valid session")
	}

	if reissued.Value != csrfCookie.Value {
		t.Fatalf("reissued token = %q, want the persisted %q", reissued.Value, csrfCookie.Value)
	}

	if floor := time.Now().Add(auth.MaxSessionAge - time.Minute); reissued.Expires.Before(floor) {
		t.Fatalf("reissued cookie expires %v, want about %v", reissued.Expires, auth.MaxSessionAge)
	}
}

// With the CSRF cookie still present there is nothing to repair: the handler
// must not touch the response.
func TestSessionManager_EnsureCSRFCookie_NoopWhenPresent(t *testing.T) {
	t.Parallel()

	manager, err := auth.NewSessionManager(newSessionRepository(), "a-session-secret-with-at-least-thirty-two-bytes", false)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}

	sessionCookie, csrfCookie := issueSession(t, manager, auth.Identity{Username: "alice@pve"})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(sessionCookie)
	request.AddCookie(csrfCookie)

	if err := manager.EnsureCSRFCookie(context.Background(), recorder, request); err != nil {
		t.Fatalf("EnsureCSRFCookie: %v", err)
	}

	if cookies := recorder.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("wrote %d cookies with CSRF already present, want 0", len(cookies))
	}
}

// Without a valid session there is no persisted token to re-issue: both the
// missing-cookie and the expired-session cases reject as unauthenticated.
func TestSessionManager_EnsureCSRFCookie_RejectsWithoutSession(t *testing.T) {
	t.Parallel()

	repository := newSessionRepository()

	manager, err := auth.NewSessionManager(repository, "a-session-secret-with-at-least-thirty-two-bytes", false)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}

	sessionCookie, _ := issueSession(t, manager, auth.Identity{Username: "alice@pve"})
	expiredCookie := &http.Cookie{Name: sessionCookie.Name, Value: sessionCookie.Value, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode}

	repository.expireAll()

	cases := []struct {
		name   string
		cookie *http.Cookie
	}{
		{name: "no session cookie", cookie: nil},
		{name: "expired session", cookie: expiredCookie},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.cookie != nil {
				request.AddCookie(tc.cookie)
			}

			err := manager.EnsureCSRFCookie(context.Background(), httptest.NewRecorder(), request)
			if !errors.Is(err, auth.ErrUnauthenticated) {
				t.Fatalf("EnsureCSRFCookie = %v, want ErrUnauthenticated", err)
			}
		})
	}
}

// Only the bootstrap provider owns local-account management; every other
// provider, including DB-backed admins, must answer false.
func TestIdentity_Owner(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		provider auth.Provider
		want     bool
	}{
		{name: "bootstrap is owner", provider: auth.ProviderBootstrap, want: true},
		{name: "pve is not owner", provider: auth.ProviderPVE, want: false},
		{name: "local admin is not owner", provider: auth.ProviderLocal, want: false},
		{name: "empty provider is not owner", provider: "", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := (auth.Identity{Provider: tc.provider}).Owner(); got != tc.want {
				t.Errorf("Owner() = %t, want %t", got, tc.want)
			}
		})
	}
}
