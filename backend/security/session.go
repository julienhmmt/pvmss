package security

import (
	"context"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"pvmss/logger"
)

// sessionContextKey is an unexported type used as a context key for the session manager.
// Using an unexported type prevents collisions with context keys defined in other packages.
type sessionContextKey struct{}

// WithSessionManager returns a new context with the provided scs.SessionManager.
func WithSessionManager(ctx context.Context, sm *scs.SessionManager) context.Context {
	return context.WithValue(ctx, sessionContextKey{}, sm)
}

// GetSession retrieves the scs.SessionManager from the request context.
// It provides a safe way to access the session manager, returning nil if it's not found.
func GetSession(r *http.Request) *scs.SessionManager {
	if r == nil {
		logger.Get().Debug().Msg("GetSession called with nil request")
		return nil
	}
	sm, _ := r.Context().Value(sessionContextKey{}).(*scs.SessionManager)
	if sm == nil {
		logger.Get().Debug().Str("path", r.URL.Path).Msg("Session manager not found in request context")
	}
	return sm
}
