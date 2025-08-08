package security

import (
	"net/http"

	"github.com/alexedwards/scs/v2"
	"pvmss/logger"
	"pvmss/state"
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

	// Fall back to global state
	stateManager := state.GetGlobalState()
	if stateManager == nil {
		log.Debug().Msg("Global state manager is nil in GetSession")
		return nil
	}

	sessionManager := stateManager.GetSessionManager()
	if sessionManager == nil {
		log.Debug().Msg("Session manager is nil in GetSession")
		return nil
	}

	return sessionManager
}
