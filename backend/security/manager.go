package security

import (
	"context"
	"net/http"

	"github.com/alexedwards/scs/v2"
)

// sessionManagerKey is the context key for the session manager
type sessionManagerKeyType struct{}

var sessionManagerKey = sessionManagerKeyType{}

// SessionManager wraps scs.SessionManager to provide additional methods
type SessionManager struct {
	*scs.SessionManager
}

// NewSessionManager creates a new SessionManager from an scs.SessionManager
func NewSessionManager(sm *scs.SessionManager) *SessionManager {
	return &SessionManager{SessionManager: sm}
}

// LoadAndSave provides middleware that loads and saves session data
func (sm *SessionManager) LoadAndSave(next http.Handler) http.Handler {
	return sm.SessionManager.LoadAndSave(next)
}

// Put adds a value and the corresponding key to the session data
func (sm *SessionManager) Put(ctx context.Context, key string, val interface{}) {
	sm.SessionManager.Put(ctx, key, val)
}

// Get retrieves a value from the session data
func (sm *SessionManager) Get(ctx context.Context, key string) interface{} {
	return sm.SessionManager.Get(ctx, key)
}

// Exists returns true if the given key exists in the session data
func (sm *SessionManager) Exists(ctx context.Context, key string) bool {
	return sm.SessionManager.Exists(ctx, key)
}

// Remove removes a value from the session data
func (sm *SessionManager) Remove(ctx context.Context, key string) {
	sm.SessionManager.Remove(ctx, key)
}

// RenewToken updates the session data with a new session token
func (sm *SessionManager) RenewToken(ctx context.Context) error {
	return sm.SessionManager.RenewToken(ctx)
}

// Destroy deletes the current session
func (sm *SessionManager) Destroy(ctx context.Context) error {
	return sm.SessionManager.Destroy(ctx)
}
