package security

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"pvmss/logger"
	"pvmss/state"
)

// GenerateCSRFToken generates a new CSRF token using the state manager
func GenerateCSRFToken(r *http.Request) string {
	// Generate random token
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to generate CSRF token")
		return ""
	}

	token := hex.EncodeToString(bytes)

	// Get state manager
	stateManager := state.GetGlobalState()
	if stateManager == nil {
		logger.Get().Error().Msg("State manager not initialized")
		return ""
	}

	// Store token with expiry
	expiry := time.Now().Add(csrfTTL)
	if err := stateManager.AddCSRFToken(token, expiry); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to store CSRF token")
		return ""
	}

	return token
}

// ValidateCSRFToken validates a CSRF token using the state manager
func ValidateCSRFToken(r *http.Request) bool {
	token := r.FormValue("csrf_token")
	if token == "" {
		token = r.Header.Get("X-CSRF-Token")
	}

	if token == "" {
		return false
	}

	// Get state manager
	stateManager := state.GetGlobalState()
	if stateManager == nil {
		logger.Get().Error().Msg("State manager not initialized")
		return false
	}

	// Validate and remove the token
	return stateManager.ValidateAndRemoveCSRFToken(token)
}
