package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"pvmss/logger"
	"pvmss/security"
)

// contextKey is a custom type for context keys
type contextKey string

// String implements the Stringer interface
func (c contextKey) String() string {
	return "context_key_" + string(c)
}

// CSRFContextKey is the key used to store CSRF token in context
var CSRFContextKey = contextKey("csrf_token")

// CompareTokens performs a constant time comparison of two tokens
// to prevent timing attacks.
func CompareTokens(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// GetCSRFToken retrieves the CSRF token from the request context.
// It returns an empty string if the token is not found.
func GetCSRFToken(r *http.Request) string {
	if token, ok := r.Context().Value(CSRFContextKey).(string); ok {
		return token
	}
	return ""
}

// CSRF provides CSRF protection
func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logger.Get().With().
			Str("middleware", "security.CSRFValidation").
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Logger()

		// Centralized skip logic
		if shouldSkipCSRF(r) {
			next.ServeHTTP(w, r)
			return
		}

		// Retrieve session manager. Use security.GetSession so validation works
		// even if SessionMiddleware hasn't injected the manager into context yet.
		sessionManager := security.GetSession(r)

		// Extract token from request (header or form)
		token := r.Header.Get("X-CSRF-Token")
		if token == "" {
			token = r.FormValue("csrf_token")
		}

		if token == "" {
			// This is an unsafe request and token is missing
			log.Warn().Msg("Missing CSRF token in request (unsafe method)")
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Helper: safely read token from session without panicking if no session data
		safeGet := func() (string, bool) {
			if sessionManager == nil {
				return "", false
			}
			defer func() {
				if rec := recover(); rec != nil {
					log.Debug().Interface("recover", rec).Msg("CSRFValidation: session Get panicked; treating as missing token")
				}
			}()
			if v, ok := sessionManager.Get(r.Context(), "csrf_token").(string); ok && v != "" {
				return v, true
			}
			return "", false
		}

		sessionToken, ok := safeGet()
		if !ok {
			log.Warn().Msg("Missing CSRF token in session during validation")
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Validate
		if !CompareTokens(token, sessionToken) {
			log.Warn().Msg("Invalid CSRF token")
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		log.Debug().Msg("CSRF token validated successfully")
		next.ServeHTTP(w, r)
	})
}

// shouldSkipCSRF determines if CSRF validation should be skipped for the request
func shouldSkipCSRF(r *http.Request) bool {
	// Skip health checks
	if r.URL.Path == "/health" || r.URL.Path == "/api/health" || r.URL.Path == "/api/healthz" {
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
	if _, isSafe := safeMethods[r.Method]; isSafe {
		return true
	}

	// Static asset paths — no CSRF needed
	if strings.HasPrefix(r.URL.Path, "/css/") ||
		strings.HasPrefix(r.URL.Path, "/js/") ||
		strings.HasPrefix(r.URL.Path, "/webfonts/") ||
		r.URL.Path == "/favicon.ico" {
		return true
	}

	return false
}
