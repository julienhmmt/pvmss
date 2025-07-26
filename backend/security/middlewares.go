package security

import (
	"net/http"
	"strings"

	"pvmss/logger"
)

// CSRFMiddleware provides CSRF protection
func CSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip CSRF for GET requests, health checks, and static files
		if r.Method == "GET" ||
			r.URL.Path == "/health" ||
			strings.HasPrefix(r.URL.Path, "/css/") ||
			strings.HasPrefix(r.URL.Path, "/js/") ||
			strings.HasPrefix(r.URL.Path, "/favicon") {
			next.ServeHTTP(w, r)
			return
		}

		// Skip CSRF for API GET requests
		if strings.HasPrefix(r.URL.Path, "/api/") && r.Method == "GET" {
			next.ServeHTTP(w, r)
			return
		}

		if !ValidateCSRFToken(r) {
			logger.Get().Warn().
				Str("ip", r.RemoteAddr).
				Str("path", r.URL.Path).
				Msg("CSRF token validation failed")
			http.Error(w, "Invalid CSRF token", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// HeadersMiddleware adds security headers
// func HeadersMiddleware(next http.Handler) http.Handler {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		// Security headers
// 		w.Header().Set("X-Content-Type-Options", "nosniff")
// 		w.Header().Set("X-Frame-Options", "DENY")
// 		w.Header().Set("X-XSS-Protection", "1; mode=block")
// 		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

// 		// Content Security Policy
// 		csp := "default-src 'self'; " +
// 			"script-src 'self' 'unsafe-inline'; " +
// 			"style-src 'self' 'unsafe-inline'; " +
// 			"img-src 'self' data:; " +
// 			"font-src 'self'; " +
// 			"connect-src 'self'"
// 		w.Header().Set("Content-Security-Policy", csp)

// 		next.ServeHTTP(w, r)
// 	})
// }
