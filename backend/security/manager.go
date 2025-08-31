package security

import (
	"context"

	"github.com/alexedwards/scs/v2"
)

// SessionManager wraps the scs.SessionManager to provide a consistent, application-specific
// interface for session management. While it currently acts as a direct proxy, it offers
// a central point for future customization or extension of session-related behavior
// without modifying code throughout the application.
type SessionManager struct {
	*scs.SessionManager
}

// NewSessionManager creates a new SessionManager from an underlying scs.SessionManager.
// This constructor is used during application initialization to set up the session
// management component.
func NewSessionManager(sm *scs.SessionManager) *SessionManager {
	return &SessionManager{SessionManager: sm}
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
