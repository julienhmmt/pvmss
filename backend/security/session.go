package security

import (
	"net/http"

	"github.com/alexedwards/scs/v2"
)

// GetSession retrieves the session from the request context
func GetSession(r *http.Request) *scs.SessionManager {
	if r == nil {
		return nil
	}

	// First try to get from context (in case it was already set by middleware)
	if sessionManager, ok := r.Context().Value(sessionManagerKey).(*scs.SessionManager); ok && sessionManager != nil {
		return sessionManager
	}

	// No global fallback: if not present in context, consider no session available
	return nil
}

// GetSessionWrapped returns the wrapped SessionManager for convenience.
// It constructs the wrapper around the underlying scs manager if present.
func GetSessionWrapped(r *http.Request) *SessionManager {
	if sm := GetSession(r); sm != nil {
		return NewSessionManager(sm)
	}
	return nil
}
