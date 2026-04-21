package handlers

import (
	"net/http"
)

// IsAuthenticated checks if the user is authenticated.
func IsAuthenticated(r *http.Request) bool {
	log := CreateHandlerLogger("IsAuthenticated", r).With().Str("remote_addr", r.RemoteAddr).Logger()
	stateManager := getStateManager(r)
	if stateManager == nil {
		log.Error().Msg("State manager not available in IsAuthenticated")
		return false
	}
	sessionManager := stateManager.GetSessionManager()
	if sessionManager == nil {
		log.Error().Msg("Session manager not available in IsAuthenticated")
		return false
	}
	sessionToken := sessionManager.Token(r.Context())
	log.Debug().Str("session_token", sessionToken).Msg("Session token in IsAuthenticated")
	authenticated, ok := sessionManager.Get(r.Context(), "authenticated").(bool)
	sessionData := map[string]interface{}{
		"authenticated_found": ok,
		"authenticated_value": authenticated,
	}
	if username, ok := sessionManager.Get(r.Context(), "username").(string); ok {
		sessionData["username"] = username
	}
	if isAdmin, ok := sessionManager.Get(r.Context(), "is_admin").(bool); ok {
		sessionData["is_admin"] = isAdmin
	}
	if !ok || !authenticated {
		log.Debug().Bool("authenticated", false).Interface("session_data", sessionData).Str("session_id", sessionToken).Msg("Access denied: user not authenticated")
		return false
	}
	log.Debug().Bool("authenticated", true).Interface("session_data", sessionData).Str("session_id", sessionToken).Msg("Access granted: user authenticated")
	return true
}

// IsAdmin checks if the current user is an admin.
func IsAdmin(r *http.Request) bool {
	log := CreateHandlerLogger("IsAdmin", r)
	stateManager := getStateManager(r)
	if stateManager == nil {
		return false
	}
	sessionManager := stateManager.GetSessionManager()
	if sessionManager == nil {
		return false
	}
	isAdmin, ok := sessionManager.Get(r.Context(), "is_admin").(bool)
	if !ok || !isAdmin {
		log.Debug().Bool("is_admin", false).Msg("User is authenticated but not admin")
		return false
	}
	log.Debug().Bool("is_admin", true).Str("session_id", sessionManager.Token(r.Context())).Msg("Admin access verified")
	return true
}
