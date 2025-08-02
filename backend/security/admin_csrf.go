package security

import (
	"net/http"
	"strings"

	"pvmss/logger"
)

// AdminCSRFMiddleware is a CSRF protection middleware that only applies to admin routes
func AdminCSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip CSRF check for safe methods
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions || r.Method == http.MethodTrace {
			next.ServeHTTP(w, r)
			return
		}

		// Only apply CSRF protection to admin routes
		if !strings.HasPrefix(r.URL.Path, "/admin/") {
			next.ServeHTTP(w, r)
			return
		}

		// Get the CSRF token from the request
		token := r.Header.Get("X-CSRF-Token")
		if token == "" {
			token = r.FormValue("csrf_token")
		}

		// Validate the CSRF token
		if !ValidateCSRFToken(r) {
			log := logger.Get().With().
				Str("handler", "AdminCSRFMiddleware").
				Str("path", r.URL.Path).
				Str("method", r.Method).
				Str("remote_addr", r.RemoteAddr).
				Logger()

			log.Warn().
				Str("token", token).
				Msg("CSRF token validation failed")

			http.Error(w, "Invalid CSRF token", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
