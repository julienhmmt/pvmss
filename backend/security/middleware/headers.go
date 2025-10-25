package middleware

import (
	"net/http"
	"strings"
	"pvmss/utils"
)

var (
	// isProduction is cached at initialization to avoid repeated PVMSS_ENV lookups
	isProduction = utils.IsProduction()
)

// Headers adds security headers to all responses
func Headers(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w, r)
		next.ServeHTTP(w, r)
	})
}

// setSecurityHeaders applies a set of security-related HTTP headers to the response.
func setSecurityHeaders(w http.ResponseWriter, r *http.Request) {
	// Set standard security headers
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

	// CORS headers for API and WebSocket endpoints
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	// Set CORS origin based on request (allow localhost in development)
	if origin := r.Header.Get("Origin"); origin != "" {
		if strings.HasPrefix(origin, "http://localhost") || strings.HasPrefix(origin, "http://127.0.0.1") {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
	}

	// Add HSTS header in production
	if isProduction {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
	}
}
