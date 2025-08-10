package middleware

import (
	"net/http"

	"pvmss/logger"
	"pvmss/security"
)

// GetCSRFToken retrieves the CSRF token from the request context.
// It returns an empty string if the token is not found.
func GetCSRFToken(r *http.Request) string {
	if token, ok := r.Context().Value(security.CSRFTokenContextKey).(string); ok {
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
		if !security.CompareTokens(token, sessionToken) {
			log.Warn().Msg("Invalid CSRF token")
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		log.Debug().Msg("CSRF token validated successfully")
		next.ServeHTTP(w, r)
	})
}
