package security

import (
	"context"
	"net/http"
)

// sessionContextKey is an unexported type used as a context key for the session manager.
// Using an unexported type prevents collisions with context keys defined in other packages.
type sessionContextKey struct{}

// WithSessionManager returns a new context with the provided SessionManager.
func WithSessionManager(ctx context.Context, sm *SessionManager) context.Context {
	return context.WithValue(ctx, sessionContextKey{}, sm)
}

// GetSession retrieves the SessionManager from the request context.
// It provides a safe way to access the session manager, returning nil if it's not found.
func GetSession(r *http.Request) *SessionManager {
	if r == nil {
		return nil
	}
	sm, _ := r.Context().Value(sessionContextKey{}).(*SessionManager)
	return sm
}
