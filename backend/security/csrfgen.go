package security

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"pvmss/logger"
)

// Constants for CSRF protection.
const (
	// csrfTokenLength is the byte length of the CSRF token.
	csrfTokenLength = 32
	// CSRFTokenTTL defines the default lifetime for CSRF tokens.
	// This can be overridden by the CSRF_TOKEN_TTL environment variable.
	CSRFTokenTTL = 30 * time.Minute
	// csrfSessionKey is the key used to store the token in the session.
	csrfSessionKey = "csrf_token"
	// csrfHeader is the HTTP header to check for the token.
	csrfHeader = "X-CSRF-Token"
	// csrfFormKey is the form field to check for the token.
	csrfFormKey = "csrf_token"
)

// csrfContextKey is an unexported type for the context key to avoid collisions.
type csrfContextKey struct{}

// WithCSRFToken returns a new context with the provided CSRF token.
func WithCSRFToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, csrfContextKey{}, token)
}

// CSRFTokenFromContext extracts a CSRF token from the context.
// It returns the token and a boolean indicating if it was found.
func CSRFTokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(csrfContextKey{}).(string)
	return token, ok
}

// GenerateCSRFToken creates a new, cryptographically secure CSRF token.
func GenerateCSRFToken() (string, error) {
	b := make([]byte, csrfTokenLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// CompareTokens performs a constant-time comparison of two tokens.
func CompareTokens(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// CSRF is a middleware that provides Cross-Site Request Forgery protection.
// It handles both generating tokens for safe requests (GET, HEAD, etc.) and
// validating them for unsafe requests (POST, PUT, etc.).
// This middleware must be placed *after* the session middleware.
func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logger.Get().With().Str("middleware", "security.CSRF").Logger()

		// Get the session manager. This must be available in the context.
		sessionManager := GetSession(r)
		if sessionManager == nil {
			log.Error().Msg("CSRF middleware requires session middleware to be active")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// For safe methods (GET, HEAD, etc.), generate a token if needed and add it to the context.
		if isSafeMethod(r) {
			token := getOrCreateSessionToken(r, sessionManager)
			ctx := WithCSRFToken(r.Context(), token)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// For unsafe methods, validate the token.
		requestToken := r.Header.Get(csrfHeader)
		if requestToken == "" {
			requestToken = r.FormValue(csrfFormKey)
		}

		if requestToken == "" {
			log.Warn().Msg("Missing CSRF token in unsafe request")
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		sessionToken := sessionManager.GetString(r.Context(), csrfSessionKey)
		if sessionToken == "" {
			log.Warn().Msg("Missing CSRF token in session for validation")
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		if !CompareTokens(requestToken, sessionToken) {
			log.Warn().Msg("Invalid CSRF token")
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		log.Debug().Msg("CSRF token validated successfully")
		next.ServeHTTP(w, r)
	})
}

// getOrCreateSessionToken retrieves a CSRF token from the session or generates a new one.
func getOrCreateSessionToken(r *http.Request, sm sessionManager) string {
	token := sm.GetString(r.Context(), csrfSessionKey)
	if token != "" {
		return token // Reuse existing token
	}

	// Generate a new token if one doesn't exist.
	newToken, err := GenerateCSRFToken()
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to generate CSRF token")
		return "" // Return empty on error
	}

	sm.Put(r.Context(), csrfSessionKey, newToken)
	return newToken
}

// isSafeMethod checks if the request uses a safe HTTP method (e.g., GET, HEAD).
func isSafeMethod(r *http.Request) bool {
	// Skip CSRF for health checks and static assets.
	if r.URL.Path == "/health" || r.URL.Path == "/api/health" || r.URL.Path == "/api/healthz" ||
		strings.HasPrefix(r.URL.Path, "/css/") ||
		strings.HasPrefix(r.URL.Path, "/js/") ||
		strings.HasPrefix(r.URL.Path, "/webfonts/") ||
		r.URL.Path == "/favicon.ico" {
		return true
	}

	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

// sessionManager is an interface to abstract the session manager's methods.
// This helps in testing and keeps the dependency on scs contained.
type sessionManager interface {
	Put(ctx context.Context, key string, val interface{})
	GetString(ctx context.Context, key string) string
}
