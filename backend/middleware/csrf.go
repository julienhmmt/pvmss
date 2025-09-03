package middleware

import (
	"net/http"

	"pvmss/logger"
	"pvmss/security"
)

// GetCSRFToken retrieves the CSRF token from the request context only.
// It's crucial this function does not access the session, as some routes
// (e.g., /health, static assets) are processed before session data is loaded,
// and attempting to access it would cause a panic.
// It returns an empty string if no token is found in the context.
func GetCSRFToken(r *http.Request) string {
	if token, ok := security.CSRFTokenFromContext(r.Context()); ok {
		return token
	}
	return ""
}

// CSRFMiddleware provides Cross-Site Request Forgery protection.
// It performs the following actions:
//  1. Extracts the CSRF token from the request context.
//  2. Logs whether a token is present, warning if it's missing on potentially unsafe requests.
//  3. Re-embeds the token into the context to ensure it's available for subsequent handlers.
//  4. For safe methods (GET, HEAD, etc.), it sets the X-CSRF-Token header in the response,
//     allowing client-side JavaScript to easily retrieve it for subsequent state-changing requests.
func CSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logger.Get().With().
			Str("middleware", "CSRF").
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Logger()

		// Retrieve the token from the context. It's safe to do this early because
		// GetCSRFToken does not access the session.
		token := GetCSRFToken(r)

		// Define conditions under which a CSRF token is not required.
		isSafeMethod := r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions || r.Method == http.MethodTrace

		// Log the status of the CSRF token.
		if token == "" {
			if isSafeMethod || IsStaticOrHealthPath(r.URL.Path) {
				log.Debug().Msg("No CSRF token in context (expected for safe/static/health request)")
			} else {
				log.Warn().Msg("No CSRF token found in context for potentially unsafe request")
			}
		} else {
			log.Debug().Msg("CSRF token present in request context")
		}

		// Ensure the token is always passed down in the context.
		ctx := security.WithCSRFToken(r.Context(), token)
		r = r.WithContext(ctx)

		// For safe methods, expose the token in a header so client-side code can access it.
		if isSafeMethod && token != "" {
			w.Header().Set("X-CSRF-Token", token)
		}

		next.ServeHTTP(w, r)
	})
}
