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
		log := logger.Get().With().Str("middleware", "CSRFGeneratorMiddleware").Logger()

		// Get the session manager from the request context
		sessionManager := GetSession(r)
		if sessionManager == nil {
			log.Error().Msg("Session manager not available in CSRF generator")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// For GET requests, generate a new token and add it to the context
		if r.Method == http.MethodGet {
			// Check if we have a valid session by trying to get the session ID
			sessionToken, err := r.Cookie(sessionManager.Cookie.Name)
			if err != nil || sessionToken == nil || sessionToken.Value == "" {
				log.Error().Err(err).Msg("No active session found in context")
				http.Error(w, "Session not available", http.StatusUnauthorized)
				return
			}

			// Generate a new CSRF token
			csrfToken, err := GenerateCSRFToken()
			if err != nil {
				log.Error().Err(err).Msg("Failed to generate CSRF token")
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			// Store the CSRF token in the session
			sessionManager.Put(r.Context(), "csrf_token", csrfToken)

			// Add the token to the request context
			ctx := context.WithValue(r.Context(), CSRFTokenContextKey, csrfToken)
			r = r.WithContext(ctx)

			log.Debug().Str("token", csrfToken).Msg("CSRF token generated and stored in session")
		}

		next.ServeHTTP(w, r)
	})
}
