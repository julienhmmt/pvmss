package security

import (
	"context"
	"net/http"
	"pvmss/logger"
)

// CSRFGeneratorMiddleware generates CSRF tokens for GET requests
// This middleware should be placed after the session middleware but before the CSRF validation middleware
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
			next.ServeHTTP(w, r) // Continue to next handler without failing
			return
		}

		// Check if we have a valid session by trying to get the session ID
		sessionToken, err := r.Cookie(sessionManager.Cookie.Name)
		if err != nil || sessionToken == nil || sessionToken.Value == "" {
			// No session cookie found - skip CSRF token generation
			log.Debug().Msg("No active session, skipping CSRF token generation")
			next.ServeHTTP(w, r)
			return
		}

		// Generate a new CSRF token
		csrfToken, err := GenerateCSRFToken()
		if err != nil {
			log.Error().Err(err).Msg("Failed to generate CSRF token")
			next.ServeHTTP(w, r) // Continue without CSRF token rather than failing
			return
		}

		// Store the CSRF token in the session
		err = sessionManager.RenewToken(r.Context())
		if err != nil {
			log.Error().Err(err).Msg("Failed to renew session token")
			next.ServeHTTP(w, r) // Continue without CSRF token rather than failing
			return
		}

		sessionManager.Put(r.Context(), "csrf_token", csrfToken)

		// Add the token to the request context
		ctx := context.WithValue(r.Context(), CSRFTokenContextKey, csrfToken)
		r = r.WithContext(ctx)

		log.Debug().Str("token", csrfToken).Msg("CSRF token generated and stored in session")
		next.ServeHTTP(w, r)
	})
}
