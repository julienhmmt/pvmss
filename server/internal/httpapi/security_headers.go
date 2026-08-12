package httpapi

import "net/http"

// apiSecurityHeaders is the strict CSP for API responses — no inline scripts
// or styles are ever served on /api/* routes.
const apiSecurityHeaders = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: https:; " +
	"font-src 'self' data:; " +
	"connect-src 'self'; " +
	"frame-ancestors 'none'; " +
	"base-uri 'self'; " +
	"form-action 'self'"

// spaSecurityHeaders is the CSP for the SPA shell. SvelteKit's adapter-static
// injects an inline bootstrap script to load the app bundle — 'unsafe-inline'
// is required for script-src so the browser executes it. The inline script is
// build-generated, not user-controlled, so the XSS risk is minimal. API
// responses keep the stricter apiSecurityHeaders.
const spaSecurityHeaders = "default-src 'self'; " +
	"script-src 'self' 'unsafe-inline'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: https:; " +
	"font-src 'self' data:; " +
	"connect-src 'self'; " +
	"frame-ancestors 'none'; " +
	"base-uri 'self'; " +
	"form-action 'self'"

// withSecurityHeaders wraps the given handler with a middleware that sets
// security headers on every response. API routes get the strict CSP (no
// inline scripts); SPA routes get the relaxed CSP (inline bootstrap script
// allowed).
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		if isAPIPath(r.URL.Path) {
			h.Set("Content-Security-Policy", apiSecurityHeaders)
		} else {
			h.Set("Content-Security-Policy", spaSecurityHeaders)
		}
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
