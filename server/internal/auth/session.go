// Package auth resolves authenticated identities from browser sessions and API tokens.
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	// SessionCookieName is the HttpOnly browser session cookie.
	SessionCookieName = "pvmss_session"
	sessionTTL        = 8 * time.Hour
	minimumSecretSize = 32
	sessionTokenBytes = 32
)

var (
	// ErrUnauthenticated indicates an absent, malformed, expired, or revoked credential.
	ErrUnauthenticated = errors.New("unauthenticated")
	// ErrForbidden indicates an authenticated identity without the required privilege.
	ErrForbidden = errors.New("forbidden")
)

// Identity is the principal resolved for an API request.
type Identity struct {
	Username string `json:"username"`
	// Pool is the tenancy anchor owning this user's VMs (PD00: one pool per
	// user). Empty for the local admin and for a cluster admin with no
	// personal pool.
	Pool               string `json:"pool"`
	IsAdmin            bool   `json:"isAdmin"`
	Cluster            string `json:"cluster"`
	ClusterDisplayName string `json:"clusterDisplayName"`
}

// SessionRecord is a persisted, revocable browser session.
type SessionRecord struct {
	Hash      []byte
	Identity  Identity
	ExpiresAt time.Time
	CreatedAt time.Time
}

// SessionRepository is the persistence required to issue, resolve, and revoke sessions.
type SessionRepository interface {
	CreateSession(ctx context.Context, session SessionRecord) error
	FindSession(ctx context.Context, hash []byte) (SessionRecord, error)
	TouchSession(ctx context.Context, hash []byte, expiresAt time.Time) error
	DeleteSession(ctx context.Context, hash []byte) error
}

// SessionManager issues opaque, database-backed browser sessions with sliding
// expiry. Sessions are trivially revocable: logout deletes the row, so a
// cleared cookie can never be replayed (plan.md: SQLite session over JWT,
// chosen specifically because a self-contained signed token cannot be
// revoked before its own expiry).
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

// SetCookie issues a new revocable session for identity and writes its cookie.
func (m *SessionManager) SetCookie(ctx context.Context, w http.ResponseWriter, identity Identity) error {
	raw, err := randomHex(sessionTokenBytes)
	if err != nil {
		return fmt.Errorf("generate session token: %w", err)
	}

	expires := time.Now().Add(sessionTTL)

	session := SessionRecord{Hash: m.hash(raw), Identity: identity, ExpiresAt: expires, CreatedAt: time.Now().UTC()}
	if err := m.repository.CreateSession(ctx, session); err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	http.SetCookie(w, m.cookie(raw, expires))

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
	if err != nil || !session.ExpiresAt.After(time.Now()) {
		return Identity{}, ErrUnauthenticated
	}

	if err := m.repository.TouchSession(ctx, hash, time.Now().Add(sessionTTL)); err != nil {
		return Identity{}, fmt.Errorf("slide session expiry: %w", err)
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

	return nil
}

func (m *SessionManager) cookie(raw string, expires time.Time) *http.Cookie {
	return &http.Cookie{Name: SessionCookieName, Value: raw, Path: "/", Expires: expires, HttpOnly: true, Secure: m.isSecure, SameSite: http.SameSiteStrictMode} //nolint:gosec // Secure is intentionally conditional for dev HTTP mode
}

// hash keys the at-rest session lookup with the application secret so a
// database dump alone cannot be used to mint valid session tokens.
func (m *SessionManager) hash(raw string) []byte {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(raw))

	return mac.Sum(nil)
}
