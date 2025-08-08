// security/csrfgen.go
package security

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"

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
		// Skip CSRF for health endpoints, API endpoints, and non-GET requests
		if r.URL.Path == "/health" || r.URL.Path == "/api/health" ||
			r.URL.Path == "/api/healthz" ||
			strings.HasPrefix(r.URL.Path, "/api/") ||
			r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		log := logger.Get().With().
			Str("middleware", "CSRFGeneratorMiddleware").
			Str("path", r.URL.Path).
			Logger()

		// Get the session manager (may not have data loaded yet for some routes)
		sessionManager := GetSession(r)

		// Helper: safe session get to avoid panics when no session data in context
		safeGet := func() (string, bool) {
			if sessionManager == nil {
				return "", false
			}
			defer func() {
				if rec := recover(); rec != nil {
					// scs panicked due to missing session data in context
					log.Debug().Interface("recover", rec).Msg("CSRFGenerator: session Get panicked; treating as no token")
				}
			}()
			if v, ok := sessionManager.Get(r.Context(), "csrf_token").(string); ok && v != "" {
				return v, true
			}
			return "", false
		}

		// Helper: safe session put to avoid panics when no session data in context
		safePut := func(token string) {
			if sessionManager == nil {
				return
			}
			defer func() {
				if rec := recover(); rec != nil {
					log.Debug().Interface("recover", rec).Msg("CSRFGenerator: session Put panicked; skipping persist")
				}
			}()
			sessionManager.Put(r.Context(), "csrf_token", token)
		}

		// Reuse existing token if safely retrievable; else generate
		var csrfToken string
		if existing, ok := safeGet(); ok {
			csrfToken = existing
			log.Debug().Msg("CSRF token found in session; reusing")
		} else {
			newToken, err := GenerateCSRFToken()
			if err != nil {
				log.Error().Err(err).Msg("Failed to generate CSRF token")
				next.ServeHTTP(w, r)
				return
			}
			csrfToken = newToken
			// Try to persist in session if available (safe against panic)
			safePut(csrfToken)
			log.Debug().Msg("CSRF token generated; persisted to session if available")
		}

		// Add the token to the request context
		ctx := context.WithValue(r.Context(), CSRFTokenContextKey, csrfToken)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}
