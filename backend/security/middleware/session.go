package middleware

import (
	"net/http"

	"github.com/alexedwards/scs/v2"
	"pvmss/security"
)

// SessionMiddleware injects the session manager into the request context, making it
// accessible to downstream handlers. It should be placed early in the middleware
// chain, before any handlers that need session access.
func SessionMiddleware(sm *scs.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add the session manager to the context for easy access.
			ctx := security.WithSessionManager(r.Context(), sm)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
