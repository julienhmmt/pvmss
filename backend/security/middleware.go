package security

import (
	"net/http"

	"github.com/alexedwards/scs/v2"
	"pvmss/logger"
	"pvmss/state"
)

// CSRFMiddleware provides CSRF protection.
func CSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logger.Get().With().Str("middleware", "CSRFMiddleware").Logger()

		// Skip CSRF check for safe methods
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions || r.Method == http.MethodTrace {
			next.ServeHTTP(w, r)
			return
		}

		// Get session
		session := GetSession(r)
		if session == nil {
			log.Error().Msg("No session available for CSRF validation")
			http.Error(w, "Session not available", http.StatusInternalServerError)
			return
		}

		// For all other requests (POST, PUT, etc.), validate the token.
		if !ValidateCSRFToken(r) {
			log.Warn().Msg("CSRF token validation failed")
			http.Error(w, "Invalid CSRF Token", http.StatusForbidden)
			return
		}
		log.Debug().Msg("CSRF token validated successfully")

		next.ServeHTTP(w, r)
	})
}

// GetSession retrieves the session from the request context
func GetSession(r *http.Request) *scs.SessionManager {
	stateManager := state.GetGlobalState()
	if stateManager == nil {
		return nil
	}
	return stateManager.GetSessionManager()
}
