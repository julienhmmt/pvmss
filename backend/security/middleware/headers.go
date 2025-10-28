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
	// ===== CRITICAL SECURITY HEADERS =====

	// Prevent clickjacking attacks
	w.Header().Set("X-Frame-Options", "DENY")

	// Prevent MIME-sniffing attacks
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// Enable XSS filter in older browsers
	w.Header().Set("X-XSS-Protection", "1; mode=block")

	// Content Security Policy - balanced between security and functionality
	csp := "default-src 'self'; " +
		"script-src 'self' 'unsafe-inline' 'unsafe-eval'; " + // unsafe-inline/eval needed for Go templates
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: https:; " +
		"font-src 'self' data:; " +
		"connect-src 'self'; " +
		"frame-ancestors 'none'; " +
		"base-uri 'self'; " +
		"form-action 'self'"
	w.Header().Set("Content-Security-Policy", csp)

	// Referrer policy - don't leak URLs to external sites
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

	// Permissions policy - disable dangerous features
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")

	// ===== CORS HEADERS =====
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, X-CSRF-Token")
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	// Set CORS origin based on request (allow localhost in development)
	if origin := r.Header.Get("Origin"); origin != "" {
		if strings.HasPrefix(origin, "http://localhost") || strings.HasPrefix(origin, "http://127.0.0.1") {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
	}

	// ===== HSTS in production =====
	if isProduction {
		// 1 year HSTS with subdomains and preload
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
	}
}
