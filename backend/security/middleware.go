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
		log := logger.Get().With().
			Str("middleware", "CSRFMiddleware").
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Logger()

		// Skip CSRF for health endpoints
		if r.URL.Path == "/health" || r.URL.Path == "/api/health" {
			log.Debug().Msg("Skipping CSRF check for health endpoint")
			next.ServeHTTP(w, r)
			return
		}

		// Skip CSRF check for safe methods
		safeMethods := map[string]bool{
			http.MethodGet:     true,
			http.MethodHead:    true,
			http.MethodOptions: true,
			http.MethodTrace:   true,
		}
		if safeMethods[r.Method] {
			log.Debug().Msg("Skipping CSRF check for safe method")
			next.ServeHTTP(w, r)
			return
		}

		// Get session manager
		sessionManager := GetSession(r)
		if sessionManager == nil {
			log.Error().Msg("Session manager not available for CSRF validation")
			http.Error(w, "Session not available", http.StatusInternalServerError)
			return
		}

		// Check for valid session
		sessionToken, err := r.Cookie(sessionManager.Cookie.Name)
		if err != nil || sessionToken == nil || sessionToken.Value == "" {
			log.Warn().Msg("No active session found for CSRF validation")
			http.Error(w, "No active session", http.StatusUnauthorized)
			return
		}

		// Validate CSRF token
		if !ValidateCSRFToken(r) {
			log.Warn().
				Str("remote_addr", r.RemoteAddr).
				Str("user_agent", r.UserAgent()).
				Msg("CSRF token validation failed")
			http.Error(w, "Invalid CSRF Token", http.StatusForbidden)
			return
		}

		log.Debug().Msg("CSRF token validated successfully")
		next.ServeHTTP(w, r)
	})
}

// GetSession retrieves the session from the request context
func GetSession(r *http.Request) *scs.SessionManager {
	log := logger.Get().With().Str("function", "GetSession").Str("path", r.URL.Path).Logger()
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
