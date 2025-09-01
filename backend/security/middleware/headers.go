package middleware

import (
	"net/http"
	"os"
	"pvmss/logger"
)

// Headers adds security headers to all responses
func Headers(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w, r)
		next.ServeHTTP(w, r)
	})
}

func setSecurityHeaders(w http.ResponseWriter, r *http.Request) {
	headers := getSecurityHeaders()

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

func getSecurityHeaders() map[string]string {
	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"Permissions-Policy":     "camera=(), microphone=(), geolocation=()",
	}

	// Add CORS headers for API and WebSocket endpoints
	headers["Access-Control-Allow-Methods"] = "GET, POST, OPTIONS"
	headers["Access-Control-Allow-Headers"] = "Content-Type, Authorization, X-Requested-With"
	headers["Access-Control-Allow-Credentials"] = "true"

	if os.Getenv("ENV") == "production" {
		headers["Strict-Transport-Security"] = "max-age=31536000; includeSubDomains; preload"
	}
	return headers
}
