package middleware

import (
	"net/http"

	"pvmss/logger"
	"pvmss/security"
)

// GetCSRFToken retrieves the CSRF token from the request context.
// It returns an empty string if the token is not found.
func GetCSRFToken(r *http.Request) string {
	if token, ok := security.CSRFTokenFromContext(r.Context()); ok {
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
		if security.ShouldSkipCSRF(r) {
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

		// Retrieve the expected token from the session.
		// Using GetString is safer than a raw type assertion and avoids panics.
		sessionToken := sessionManager.GetString(r.Context(), "csrf_token")
		if sessionToken == "" {
			log.Warn().Msg("Missing CSRF token in session during validation")
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Validate
		if !security.CompareTokens(token, sessionToken) {
			log.Warn().Msg("Invalid CSRF token")
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		log.Debug().Msg("CSRF token validated successfully")
		next.ServeHTTP(w, r)
	})
}
