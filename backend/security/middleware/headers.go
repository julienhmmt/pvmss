package middleware

import (
	"net/http"
	"os"
	"strings"
)

var (
	// isProduction is set once at startup via SetProductionMode.
	isProduction bool
)

// SetProductionMode configures security headers for the given environment.
// Call once after loading EnvConfig.
func SetProductionMode(env string) {
	isProduction = env == "production" || env == "prod"
}

func init() {
	// Fallback: read PVMSS_ENV directly if SetProductionMode hasn't been called yet.
	// This ensures headers work even before full startup completes.
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("PVMSS_ENV"))); v == "production" || v == "prod" {
		isProduction = true
	}
}

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

	// Content Security Policy — hardened: no unsafe-inline or unsafe-eval.
	// The SvelteKit SPA does not require inline scripts or eval for its
	// compiled bundle. Inline styles are still allowed because Svelte
	// injects scoped styles into <style> tags at runtime.
	csp := "default-src 'self'; " +
		"script-src 'self'; " +
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

	// Set CORS origin: in development, allow localhost origins for the dev
	// server. In production, only allow origins explicitly configured via
	// PVMSS_CORS_ORIGIN (comma-separated exact matches).
	if origin := r.Header.Get("Origin"); origin != "" {
		allowed := false
		if !isProduction {
			allowed = strings.HasPrefix(origin, "http://localhost") || strings.HasPrefix(origin, "http://127.0.0.1")
		}
		if !allowed {
			for _, cfg := range strings.Split(os.Getenv("PVMSS_CORS_ORIGIN"), ",") {
				cfg = strings.TrimSpace(cfg)
				if cfg != "" && origin == cfg {
					allowed = true
					break
				}
			}
		}
		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
	}

	// ===== HSTS in production =====
	if isProduction {
		// 1 year HSTS with subdomains and preload
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
	}
}
