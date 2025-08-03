package security

import (
	"github.com/alexedwards/scs/v2"
	"net/http"
	"pvmss/logger"
	"pvmss/state"
)

// GetSession retrieves the session from the request context
func GetSession(r *http.Request) *scs.SessionManager {
	log := logger.Get().With().
		Str("function", "GetSession").
		Str("path", r.URL.Path).
		Logger()

	stateManager := state.GetGlobalState()
	if stateManager == nil {
		log.Error().Msg("Global state manager is nil in GetSession")
		return nil
	}

	sessionManager := stateManager.GetSessionManager()
	if sessionManager == nil {
		log.Error().Msg("Session manager is nil in GetSession")
		return nil
	}
	return sessionManager
}
