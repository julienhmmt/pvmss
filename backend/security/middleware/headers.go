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
		"X-XSS-Protection":       "1; mode=block",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"Permissions-Policy":     "camera=(), microphone=(), geolocation=()",
		// Apply a reasonable default CSP in all environments to reduce risk.
		// In development, this policy is permissive enough for inline styles/scripts already configured below.
		"Content-Security-Policy": getCSP(),
	}

	if os.Getenv("ENV") == "production" {
		headers["Strict-Transport-Security"] = "max-age=31536000; includeSubDomains; preload"
	}
	return headers
}

func getCSP() string {
	return "default-src 'self'; " +
		"script-src 'self' 'unsafe-inline' 'unsafe-eval'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data:; " +
		"font-src 'self'; " +
		"connect-src 'self'"
}
