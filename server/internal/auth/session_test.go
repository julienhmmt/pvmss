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
