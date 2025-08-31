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
		// Apply a reasonable default CSP. In production, this is made stricter.
		"Content-Security-Policy": getCSP(),
	}

	if os.Getenv("ENV") == "production" {
		headers["Strict-Transport-Security"] = "max-age=31536000; includeSubDomains; preload"
	}
	return headers
}

func getCSP() string {
	// Base CSP with settings for development and production.
	baseSrc := "'self'"
	scriptSrc := "'self' 'unsafe-inline'"
	styleSrc := "'self' 'unsafe-inline'"
	connectSrc := "'self'"

	// In development, allow 'unsafe-eval' for faster development cycles (e.g., for hot-reloading or certain libraries).
	// In production, this is removed to enhance security.
	if os.Getenv("ENV") != "production" {
		scriptSrc += " 'unsafe-eval'"
	}

	return "default-src " + baseSrc + "; " +
		"script-src " + scriptSrc + "; " +
		"style-src " + styleSrc + "; " +
		"img-src " + baseSrc + " data:; " +
		"font-src " + baseSrc + "; " +
		"connect-src " + connectSrc
}
