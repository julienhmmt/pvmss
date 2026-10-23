package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"regexp"
	"strings"
)

// apiSecurityHeaders is the strict CSP for API responses - no inline scripts
// or styles are ever served on /api/* routes.
const apiSecurityHeaders = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data:; " +
	"font-src 'self' data:; " +
	"connect-src 'self'; " +
	"frame-ancestors 'none'; " +
	"base-uri 'self'; " +
	"form-action 'self'"

// spaSecurityHeadersTemplate is the CSP for the SPA shell. %s is the shell's
// script-src sources: 'self' plus the hash of each inline script index.html
// carries (the theme bootstrap from app.html and SvelteKit's start script),
// so no other inline script can run. img-src stays same-origin: the UI loads
// no remote image, and the docs renderer emits none.
const spaSecurityHeadersTemplate = "default-src 'self'; " +
	"script-src %s; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data:; " +
	"font-src 'self' data:; " +
	"connect-src 'self'; " +
	"frame-ancestors 'none'; " +
	"base-uri 'self'; " +
	"form-action 'self'"

// scriptRe matches a <script> element, capturing its attributes and its exact
// text, which is what the browser hashes.
var scriptRe = regexp.MustCompile(`(?s)<script(\s[^>]*)?>(.*?)</script>`)

// spaSecurityPolicy builds the SPA CSP from the built index.html. Without a
// readable shell (no web dir) no inline script is allowed.
func spaSecurityPolicy(webBuildDir string) string {
	sources := []string{"'self'"}

	if webBuildDir != "" {
		if index, err := fs.ReadFile(os.DirFS(webBuildDir), "index.html"); err == nil {
			for _, match := range scriptRe.FindAllSubmatch(index, -1) {
				if bytes.Contains(match[1], []byte("src=")) {
					continue // external script, covered by 'self'
				}

				sum := sha256.Sum256(match[2])
				sources = append(sources, "'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'")
			}
		}
	}

	return fmt.Sprintf(spaSecurityHeadersTemplate, strings.Join(sources, " "))
}

// withSecurityHeaders wraps the given handler with a middleware that sets
// security headers on every response. API routes get the strict CSP (no
// inline scripts); SPA routes get spaPolicy (the shell's inline scripts by
// hash).
func withSecurityHeaders(spaPolicy string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		if isAPIPath(r.URL.Path) {
			h.Set("Content-Security-Policy", apiSecurityHeaders)
		} else {
			h.Set("Content-Security-Policy", spaPolicy)
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
