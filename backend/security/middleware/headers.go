package middleware

import (
	"net/http"
	"os"
	"pvmss/logger"
	"strings"
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
	headers := getSecurityHeaders(r)

	// Set all headers
	for k, v := range headers {
		w.Header().Set(k, v)
	}

	// Log request
	logger.Get().Debug().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Msg("Processing request with security headers")
}

// getSecurityHeaders returns a map of standard security headers.
// These headers help mitigate common web vulnerabilities like XSS and clickjacking.
func getSecurityHeaders(r *http.Request) map[string]string {
	headers := map[string]string{
		// "X-Content-Type-Options": "nosniff",
		// "Referrer-Policy":        "strict-origin-when-cross-origin",
		"Permissions-Policy": "camera=(), microphone=(), geolocation=()",
	}

	// Add CORS headers for API and WebSocket endpoints
	headers["Access-Control-Allow-Methods"] = "GET, POST, OPTIONS"
	headers["Access-Control-Allow-Headers"] = "Content-Type, Authorization, X-Requested-With"
	headers["Access-Control-Allow-Credentials"] = "true"

	// Set CORS origin based on request
	origin := r.Header.Get("Origin")
	if origin != "" {
		// For development, allow localhost origins
		if strings.HasPrefix(origin, "http://localhost") ||
			strings.HasPrefix(origin, "http://127.0.0.1") {
			headers["Access-Control-Allow-Origin"] = origin
		}
	}

	if os.Getenv("ENV") == "production" {
		headers["Strict-Transport-Security"] = "max-age=31536000; includeSubDomains; preload"
	}
	return headers
}
