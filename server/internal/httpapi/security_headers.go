package httpapi

import "net/http"

// securityHeaders is the set of HTTP security headers applied to every
// response served by the v0.4 router. CSP excludes unsafe-inline and
// unsafe-eval for scripts; inline styles are retained because Svelte injects
// scoped styles into <style> tags at runtime.
const securityHeaders = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: https:; " +
	"font-src 'self' data:; " +
	"connect-src 'self'; " +
	"frame-ancestors 'none'; " +
	"base-uri 'self'; " +
	"form-action 'self'"

// withSecurityHeaders wraps the given handler with a middleware that sets
// security headers on every response. It is applied to both API and SPA
// routes so the headers are present consistently.
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", securityHeaders)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		// HSTS is only meaningful over TLS; setting it on a plain-HTTP
		// connection is ignored by browsers but harmless.
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		// Prevent caching of authenticated API responses. Static assets are
		// served with hash-based filenames by Vite and cached separately by
		// the browser via the SPA handler.
		if isAPIPath(r.URL.Path) {
			h.Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
		}

		next.ServeHTTP(w, r)
	})
}

// isAPIPath reports whether the request targets an API endpoint (vs. the SPA).
func isAPIPath(p string) bool {
	return len(p) >= 4 && p[:4] == "/api"
}
