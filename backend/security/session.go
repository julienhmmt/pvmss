package security

import (
	"net/http"

	"github.com/alexedwards/scs/v2"
	"pvmss/logger"
)

// GetSession retrieves the session from the request context
func GetSession(r *http.Request) *scs.SessionManager {
	if r == nil {
		return nil
	}

	log := logger.Get().With().
		Str("function", "GetSession").
		Str("path", r.URL.Path).
		Logger()

	// First try to get from context (in case it was already set by middleware)
	if sessionManager, ok := r.Context().Value(sessionManagerKey).(*scs.SessionManager); ok && sessionManager != nil {
		return sessionManager
	}

	// No global fallback: if not present in context, consider no session available
	log.Debug().Msg("Session manager not found in request context in GetSession")
	return nil
}
