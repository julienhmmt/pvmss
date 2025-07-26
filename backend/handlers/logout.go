package handlers

import (
	"net/http"

	"pvmss/logger"
	"pvmss/state"
)

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	stateManager := state.GetGlobalState()
	sessionManager := stateManager.GetSessionManager()
	_ = sessionManager.Destroy(r.Context())
	logger.Get().Info().Msg("User logged out")
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
