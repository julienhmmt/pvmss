package middleware

import (
	"net/http"

	"pvmss/security"
)

// SessionMiddleware injects the session manager into the request context. This makes
// the session manager accessible to downstream handlers.
func SessionMiddleware(sm *security.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add the session manager to the context for easy access.
			ctx := security.WithSessionManager(r.Context(), sm)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
