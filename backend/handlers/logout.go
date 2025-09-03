package handlers

import (
	"net/http"
)

// LogoutHandler handles user logout
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	ctx := NewHandlerContext(w, r, "LogoutHandler")

	if !ctx.ValidateSessionManager() {
		return
	}

	// Clear session data
	ctx.SessionManager.Clear(r.Context())

	// Regenerate session token to prevent session fixation
	err := ctx.SessionManager.RenewToken(r.Context())
	if err != nil {
		ctx.Log.Error().Err(err).Msg("Failed to renew session token during logout")
	}

	ctx.Log.Info().Msg("User logged out successfully")

	// Redirect to home page
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
