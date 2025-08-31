// security/csrfgen.go
package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"pvmss/logger"
)

// CSRF token length in bytes
const csrfTokenLength = 32

// GenerateCSRFToken generates a new CSRF token
func GenerateCSRFToken() (string, error) {
	b := make([]byte, csrfTokenLength)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// CompareTokens performs a constant time comparison of two tokens.
func CompareTokens(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// ShouldSkipCSRF determines if CSRF validation should be skipped for the request.
// Rules: safe HTTP methods, health endpoints, and static asset paths.
func ShouldSkipCSRF(r *http.Request) bool {
	// Health checks
	if r.URL.Path == "/health" || r.URL.Path == "/api/health" || r.URL.Path == "/api/healthz" {
		return true
	}

	// Safe methods
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	}

	// Static assets
	if strings.HasPrefix(r.URL.Path, "/css/") ||
		strings.HasPrefix(r.URL.Path, "/js/") ||
		strings.HasPrefix(r.URL.Path, "/webfonts/") ||
		r.URL.Path == "/favicon.ico" {
		return true
	}
	return false
}

// CSRFGeneratorMiddleware generates CSRF tokens for GET requests
func CSRFGeneratorMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip CSRF for health endpoints, API endpoints, and non-GET requests
		if r.URL.Path == "/health" || r.URL.Path == "/api/health" ||
			r.URL.Path == "/api/healthz" ||
			strings.HasPrefix(r.URL.Path, "/api/") ||
			r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		log := logger.Get().With().
			Str("middleware", "CSRFGeneratorMiddleware").
			Str("path", r.URL.Path).
			Logger()

		// Get the session manager (may not have data loaded yet for some routes)
		sessionManager := GetSession(r)

		// Helper: safe session get to avoid panics when no session data in context
		safeGet := func() (string, bool) {
			if sessionManager == nil {
				return "", false
			}
			defer func() {
				if rec := recover(); rec != nil {
					// scs panicked due to missing session data in context
					log.Debug().Interface("recover", rec).Msg("CSRFGenerator: session Get panicked; treating as no token")
				}
			}()
			if v, ok := sessionManager.Get(r.Context(), "csrf_token").(string); ok && v != "" {
				return v, true
			}
			return "", false
		}

		// Helper: safe session put to avoid panics when no session data in context
		safePut := func(token string, issuedAt time.Time) {
			if sessionManager == nil {
				return
			}
			defer func() {
				if rec := recover(); rec != nil {
					log.Debug().Interface("recover", rec).Msg("CSRFGenerator: session Put panicked; skipping persist")
				}
			}()
			sessionManager.Put(r.Context(), "csrf_token", token)
			sessionManager.Put(r.Context(), "csrf_issued_at", issuedAt.UTC().Format(time.RFC3339))
		}

		// Reuse existing token if safely retrievable; else generate
		var csrfToken string
		var issuedAt time.Time
		if existing, ok := safeGet(); ok {
			// Try to read issued_at and decide if still valid
			if sessionManager != nil {
				if v, ok := sessionManager.Get(r.Context(), "csrf_issued_at").(string); ok && v != "" {
					if t, err := time.Parse(time.RFC3339, v); err == nil {
						issuedAt = t
					}
				}
			}
			// Fallback: if no issued_at, consider now
			if issuedAt.IsZero() {
				issuedAt = time.Now().UTC()
			}
			// Determine TTL from config if available
			ttl := CSRFTokenTTL
			if cfg := GetConfig(); cfg.CSRFTokenTTL > 0 {
				ttl = cfg.CSRFTokenTTL
			}
			if time.Since(issuedAt) < ttl {
				csrfToken = existing
				log.Debug().Msg("CSRF token found in session; reusing (within TTL)")
			}
		}
		if csrfToken == "" {
			newToken, err := GenerateCSRFToken()
			if err != nil {
				log.Error().Err(err).Msg("Failed to generate CSRF token")
				next.ServeHTTP(w, r)
				return
			}
			csrfToken = newToken
			issuedAt = time.Now().UTC()
			// Try to persist in session if available (safe against panic)
			safePut(csrfToken, issuedAt)
			log.Debug().Msg("CSRF token generated; persisted to session if available")
		}

		// Add the token to the request context using the new helper function.
		ctx := WithCSRFToken(r.Context(), csrfToken)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}
