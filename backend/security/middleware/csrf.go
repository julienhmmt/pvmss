package middleware

import (
	"net/http"
	"pvmss/logger"
	"pvmss/security"
)

// contextKey is used to create context keys that are safe from collisions.
type contextKey string

// CSRFTokenContextKey is the key used to store the CSRF token in the request context.
const CSRFTokenContextKey contextKey = "csrf_token"

// CSRF provides CSRF protection
func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logger.Get().With().
			Str("middleware", "CSRF").
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote_addr", r.RemoteAddr).
			Str("user_agent", r.UserAgent()).
			Logger()

		// Skip CSRF checks for safe methods and health endpoints
		if shouldSkipCSRF(r) {
			log.Debug().Msg("Skipping CSRF validation for safe method or health check")
			next.ServeHTTP(w, r)
			return
		}

		// Log CSRF token validation attempt
		log.Debug().Msg("Validating CSRF token")

		if !security.ValidateCSRFToken(r) {
			log.Warn().Msg("CSRF token validation failed")
			http.Error(w, "Invalid CSRF Token: request rejected for security reasons", http.StatusForbidden)
			return
		}

		log.Debug().Msg("CSRF token validated successfully")
		next.ServeHTTP(w, r)
	})
}

// shouldSkipCSRF determines if CSRF validation should be skipped for the request
func shouldSkipCSRF(r *http.Request) bool {
	// Skip health checks
	if r.URL.Path == "/health" || r.URL.Path == "/api/health" {
		return true
	}

	// Define safe methods that don't need CSRF protection
	safeMethods := map[string]bool{
		http.MethodGet:     true, // Safe, read-only
		http.MethodHead:    true, // Safe, read-only
		http.MethodOptions: true, // Safe, preflight requests
		http.MethodTrace:   true, // Safe, diagnostic
	}

	// Check if the current method is in the safe methods list
	_, isSafe := safeMethods[r.Method]
	return isSafe
}

// GetCSRFToken retrieves the CSRF token from the request context.
// It returns an empty string if the token is not found.
func GetCSRFToken(r *http.Request) string {
	if token, ok := r.Context().Value(CSRFTokenContextKey).(string); ok {
		return token
	}
	return ""
}
