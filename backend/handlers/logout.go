package handlers

import (
	"net/http"

	"pvmss/logger"
	"pvmss/state"
)

// LogoutHandler handles user logout
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	log := logger.Get().With().Str("handler", "LogoutHandler").Logger()

	// Get session manager
	sessionManager := state.GetGlobalState().GetSessionManager()

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
