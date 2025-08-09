package handlers

import (
	"net/http"

	"pvmss/logger"
	"pvmss/security"
)

// LogoutHandler handles user logout
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	log := logger.Get().With().Str("handler", "LogoutHandler").Logger()

	// Prefer session from middleware context
	sessionManager := security.GetSession(r)
	if sessionManager == nil {
		// Fallback to state manager from context (injected by handlers.InitHandlers)
		sm := getStateManager(r)
		sessionManager = sm.GetSessionManager()
	}

	// Clear session data
	sessionManager.Clear(r.Context())

	// Regenerate session token to prevent session fixation
	err := sessionManager.RenewToken(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to renew session token during logout")
	}

	log.Info().Msg("User logged out successfully")

	// Redirect to login page
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
