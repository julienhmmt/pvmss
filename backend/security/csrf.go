package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"pvmss/logger"
	"pvmss/state"
)

// CSRF token length in bytes
const csrfTokenLength = 32

// GenerateCSRFToken generates a new CSRF token
func GenerateCSRFToken() (string, error) {
	b := make([]byte, csrfTokenLength)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// GetSessionManager returns the session manager from the global state
func GetSessionManager() *scs.SessionManager {
	stateManager := state.GetGlobalState()
	if stateManager == nil {
		logger.Get().Error().Msg("State manager not initialized")
		return nil
	}

	sessionManager := stateManager.GetSessionManager()
	if sessionManager == nil {
		logger.Get().Error().Msg("Session manager not initialized")
		return nil
	}

	return sessionManager
}

// ValidateCSRFToken validates a CSRF token from the request against the session
func ValidateCSRFToken(r *http.Request) bool {
	// Skip CSRF check for safe methods
	if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions || r.Method == http.MethodTrace {
		return true
	}

	log := logger.Get().With().
		Str("function", "ValidateCSRFToken").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	// Get token from header or form
	token := r.Header.Get("X-CSRF-Token")
	tokenSource := "header"

	// If not in header, check form
	if token == "" {
		token = r.FormValue("csrf_token")
		tokenSource = "form"
	}

	if token == "" {
		log.Warn().Msg("CSRF token is missing")
		return false
	}

	// Get session manager
	sessionManager := GetSessionManager()
	if sessionManager == nil {
		log.Error().Msg("Failed to get session manager")
		return false
	}

	// Get token from session
	sessionToken, ok := sessionManager.Get(r.Context(), "csrf_token").(string)
	if !ok || sessionToken == "" {
		log.Warn().Msg("CSRF token not found in session")
		return false
	}

	// Compare tokens
	valid := subtle.ConstantTimeCompare([]byte(token), []byte(sessionToken)) == 1
	if !valid {
		log.Warn().
			Str("token_source", tokenSource).
			Str("session_token", sessionToken).
			Str("provided_token", token).
			Msg("CSRF token validation failed")
	}

	return valid
}

// SetCSRFToken generates a new CSRF token and stores it in the session
func SetCSRFToken(r *http.Request) (string, error) {
	token, err := GenerateCSRFToken()
	if err != nil {
		logger.Get().Error().
			Err(err).
			Msg("Failed to generate CSRF token")
		return "", err
	}

	sessionManager := GetSessionManager()
	if sessionManager == nil {
		logger.Get().Error().Msg("Failed to get session manager")
		return "", nil
	}

	// Store the token in the session
	sessionManager.Put(r.Context(), "csrf_token", token)

	return token, nil
}
