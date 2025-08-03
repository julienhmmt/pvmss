// security/csrfgen.go
package security

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"

	"pvmss/logger"
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

// ValidateCSRFToken validates a CSRF token from the request against the session
func ValidateCSRFToken(r *http.Request) bool {
	// Skip CSRF check for safe methods
	if r.Method == http.MethodGet || r.Method == http.MethodHead ||
		r.Method == http.MethodOptions || r.Method == http.MethodTrace {
		return true
	}

	log := logger.Get().With().
		Str("function", "ValidateCSRFToken").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Logger()

	// Get token from header or form
	token := r.Header.Get("X-CSRF-Token")
	if token == "" {
		token = r.FormValue("csrf_token")
	}

	if token == "" {
		log.Warn().Msg("CSRF token is missing")
		return false
	}

	// Get session manager
	sessionManager := GetSession(r)
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
		log.Warn().Msg("CSRF token validation failed")
	}
	return valid
}

// CSRFGeneratorMiddleware generates CSRF tokens for GET requests
func CSRFGeneratorMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip CSRF for health endpoints and non-GET requests
		if r.URL.Path == "/health" || r.URL.Path == "/api/health" || r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		log := logger.Get().With().
			Str("middleware", "CSRFGeneratorMiddleware").
			Str("path", r.URL.Path).
			Logger()

		// Get the session manager from the request context
		sessionManager := GetSession(r)
		if sessionManager == nil {
			log.Error().Msg("Session manager not available in CSRF generator")
			next.ServeHTTP(w, r)
			return
		}

		// Check for existing session
		sessionToken, err := r.Cookie(sessionManager.Cookie.Name)
		if err != nil || sessionToken == nil || sessionToken.Value == "" {
			// No active session, skip CSRF token generation
			log.Debug().Msg("No active session, skipping CSRF token generation")
			next.ServeHTTP(w, r)
			return
		}

		// Ensure we have a valid session ID
		_, err = sessionManager.Load(r.Context(), sessionToken.Value)
		if err != nil {
			log.Error().Err(err).Msg("Failed to load session")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Generate a new CSRF token
		csrfToken, err := GenerateCSRFToken()
		if err != nil {
			log.Error().Err(err).Msg("Failed to generate CSRF token")
			next.ServeHTTP(w, r)
			return
		}

		// Store the CSRF token in the session
		err = sessionManager.RenewToken(r.Context())
		if err != nil {
			log.Error().Err(err).Msg("Failed to renew session token")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		sessionManager.Put(r.Context(), "csrf_token", csrfToken)

		// Add the token to the request context
		ctx := context.WithValue(r.Context(), CSRFTokenContextKey, csrfToken)
		r = r.WithContext(ctx)

		log.Debug().Msg("CSRF token generated and stored in session")
		next.ServeHTTP(w, r)
	})
}
