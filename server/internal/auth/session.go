// Package auth resolves authenticated identities from browser sessions and API tokens.
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	// SessionCookieName is the HttpOnly browser session cookie.
	SessionCookieName = "pvmss_session"
	// CSRFCookieName is the non-HttpOnly CSRF token cookie sent with every session.
	CSRFCookieName = "pvmss_csrf"
	sessionTTL     = 8 * time.Hour
	// touchInterval is the minimum age of a session's expiry before Resolve
	// slides it again, so reads do not become writes.
	touchInterval = 5 * time.Minute
	// MaxSessionAge is the absolute lifetime of a session, whatever its sliding
	// expiry says. It bounds how long a stale identity (for example a revoked
	// admin flag) can outlive the login that produced it.
	MaxSessionAge     = 24 * time.Hour
	minimumSecretSize = 32
	sessionTokenBytes = 32
	csrfTokenBytes    = 32
)

var (
	// ErrUnauthenticated indicates an absent, malformed, expired, or revoked credential.
	ErrUnauthenticated = errors.New("unauthenticated")
	// ErrForbidden indicates an authenticated identity without the required privilege.
	ErrForbidden = errors.New("forbidden")
)

// Provider identifies where an identity's credentials came from.
type Provider string

const (
	// ProviderPVE is a Proxmox VE cluster account (pve or pam realm).
	ProviderPVE Provider = "pve"
	// ProviderLocal is a DB-backed PVMSS administrator account.
	ProviderLocal Provider = "local"
	// ProviderBootstrap is the env-seeded administrator (ADMIN_PASSWORD_HASH).
	// It is the immutable owner: only it may manage local accounts.
	ProviderBootstrap Provider = "bootstrap"
)

// Identity is the principal resolved for an API request.
type Identity struct {
	Username string `json:"username"`
	// DisplayName is the human-friendly form of Username: the local part with
	// the optional pvmss- pool prefix and realm stripped. It is used by the UI
	// in welcome messages and user chips; Username remains the canonical value
	// for Proxmox API calls.
	DisplayName string `json:"displayName"`
	// Pool is the tenancy anchor owning this user's VMs (one pool per user). Empty for the local admin and for a cluster admin with no
	// personal pool.
	Pool               string `json:"pool"`
	IsAdmin            bool   `json:"isAdmin"`
	Cluster            string `json:"cluster"`
	ClusterDisplayName string `json:"clusterDisplayName"`
	// Provider records the credential source. Old sessions default to "pve"
	// through the migration backfill.
	Provider Provider `json:"provider"`
	// IsOwner is derived from Provider on every resolution (never persisted):
	// the bootstrap administrator owns local account management.
	IsOwner bool `json:"isOwner"`
}

// Owner reports whether the identity may manage local administrator accounts.
func (i Identity) Owner() bool {
	return i.Provider == ProviderBootstrap
}

// SessionRecord is a persisted, revocable browser session.
type SessionRecord struct {
	Hash      []byte
	Identity  Identity
	CSRFToken string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// SessionRepository is the persistence required to issue, resolve, and revoke sessions.
type SessionRepository interface {
	CreateSession(ctx context.Context, session SessionRecord) error
	FindSession(ctx context.Context, hash []byte) (SessionRecord, error)
	TouchSession(ctx context.Context, hash []byte, expiresAt time.Time) error
	DeleteSession(ctx context.Context, hash []byte) error
	DeleteOtherSessions(ctx context.Context, cluster, username string, keep []byte) error
}

// SessionManager issues opaque, database-backed browser sessions with sliding
// expiry. Sessions are trivially revocable: logout deletes the row, so a
// cleared cookie can never be replayed (SQLite session over JWT, chosen specifically because a
// self-contained signed token cannot be revoked before its own expiry).
type SessionManager struct {
	repository SessionRepository
	secret     []byte
	isSecure   bool
}

// NewSessionManager builds a manager from a 32-byte minimum application secret,
// used to key the at-rest hash of session tokens.
func NewSessionManager(repository SessionRepository, secret string, secure bool) (*SessionManager, error) {
	if len(secret) < minimumSecretSize {
		return nil, fmt.Errorf("session secret must be at least %d bytes", minimumSecretSize)
	}

	return &SessionManager{repository: repository, secret: []byte(secret), isSecure: secure}, nil
}

// SetCookie issues a new revocable session for identity and writes both the
// HttpOnly session cookie and the readable CSRF token cookie.
func (m *SessionManager) SetCookie(ctx context.Context, w http.ResponseWriter, identity Identity) error {
	raw, err := randomHex(sessionTokenBytes)
	if err != nil {
		return fmt.Errorf("generate session token: %w", err)
	}

	csrfToken, err := randomHex(csrfTokenBytes)
	if err != nil {
		return fmt.Errorf("generate csrf token: %w", err)
	}

	expires := time.Now().Add(sessionTTL)

	session := SessionRecord{Hash: m.hash(raw), Identity: identity, CSRFToken: csrfToken, ExpiresAt: expires, CreatedAt: time.Now().UTC()}
	if err := m.repository.CreateSession(ctx, session); err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	http.SetCookie(w, m.cookie(raw, expires))
	http.SetCookie(w, m.csrfCookie(csrfToken, expires))

	return nil
}

// Resolve returns the identity attached to a valid, unexpired session cookie,
// sliding its expiry forward on each successful use.
func (m *SessionManager) Resolve(ctx context.Context, r *http.Request) (Identity, error) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return Identity{}, ErrUnauthenticated
	}

	hash := m.hash(cookie.Value)

	session, err := m.repository.FindSession(ctx, hash)
	if err != nil || !usable(session, time.Now()) {
		return Identity{}, ErrUnauthenticated
	}

	if now := time.Now(); session.ExpiresAt.Before(now.Add(sessionTTL - touchInterval)) {
		if err := m.repository.TouchSession(ctx, hash, now.Add(sessionTTL)); err != nil {
			return Identity{}, fmt.Errorf("slide session expiry: %w", err)
		}
	}

	return session.Identity, nil
}

// Logout revokes the session tied to the request's cookie, if any, and clears it.
func (m *SessionManager) Logout(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	if cookie, err := r.Cookie(SessionCookieName); err == nil {
		if err := m.repository.DeleteSession(ctx, m.hash(cookie.Value)); err != nil {
			return fmt.Errorf("revoke session: %w", err)
		}
	}

	http.SetCookie(w, &http.Cookie{Name: SessionCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: m.isSecure, SameSite: http.SameSiteStrictMode}) //nolint:gosec // Secure is intentionally conditional for dev HTTP mode
	//nolint:gosec // CSRF cookie must remain readable by the client, so it is not HttpOnly; Secure and SameSite match the session cookie.
	http.SetCookie(w, &http.Cookie{Name: CSRFCookieName, Value: "", Path: "/", MaxAge: -1, Secure: m.isSecure, SameSite: http.SameSiteStrictMode})

	return nil
}

// RevokeOthers ends every session of identity except the one tied to the
// request's cookie. Used after a password change.
func (m *SessionManager) RevokeOthers(ctx context.Context, r *http.Request, identity Identity) error {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return fmt.Errorf("read session cookie: %w", err)
	}

	if err := m.repository.DeleteOtherSessions(ctx, identity.Cluster, identity.Username, m.hash(cookie.Value)); err != nil {
		return fmt.Errorf("revoke other sessions: %w", err)
	}

	return nil
}

// EnsureCSRFCookie sets the CSRF cookie again when the browser lost it while
// its session is still valid. Without it every write fails with
// invalid_csrf_token until the user logs in again.
func (m *SessionManager) EnsureCSRFCookie(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	if _, err := r.Cookie(CSRFCookieName); err == nil {
		return nil
	}

	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return ErrUnauthenticated
	}

	session, err := m.repository.FindSession(ctx, m.hash(cookie.Value))
	if err != nil || !usable(session, time.Now()) {
		return ErrUnauthenticated
	}

	http.SetCookie(w, m.csrfCookie(session.CSRFToken, session.ExpiresAt))

	return nil
}

func (m *SessionManager) cookie(raw string, expires time.Time) *http.Cookie {
	return &http.Cookie{Name: SessionCookieName, Value: raw, Path: "/", Expires: expires, HttpOnly: true, Secure: m.isSecure, SameSite: http.SameSiteStrictMode} //nolint:gosec // Secure is intentionally conditional for dev HTTP mode
}

func (m *SessionManager) csrfCookie(value string, expires time.Time) *http.Cookie {
	return &http.Cookie{Name: CSRFCookieName, Value: value, Path: "/", Expires: expires, Secure: m.isSecure, SameSite: http.SameSiteStrictMode} //nolint:gosec // Secure is intentionally conditional for dev HTTP mode
}

// CSRFToken returns the persisted token for the request's session cookie.
// It is used by CSRF middleware to validate the X-CSRF-Token header against
// the server-side session state.
func (m *SessionManager) CSRFToken(ctx context.Context, r *http.Request) (string, error) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return "", ErrUnauthenticated
	}

	hash := m.hash(cookie.Value)

	session, err := m.repository.FindSession(ctx, hash)
	if err != nil || !usable(session, time.Now()) {
		return "", ErrUnauthenticated
	}

	return session.CSRFToken, nil
}

// usable reports whether a stored session is inside both its sliding expiry
// and its absolute lifetime.
func usable(session SessionRecord, now time.Time) bool {
	return session.ExpiresAt.After(now) && now.Before(session.CreatedAt.Add(MaxSessionAge))
}

// hash keys the at-rest session lookup with the application secret so a
// database dump alone cannot be used to mint valid session tokens.
func (m *SessionManager) hash(raw string) []byte {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(raw))

	return mac.Sum(nil)
}

func randomHex(size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}

	return hex.EncodeToString(bytes), nil
}
