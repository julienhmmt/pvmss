package middleware

import (
	"context"
	"net/http"
	"strings"

	"pvmss/logger"
	"pvmss/security"
)

// GetCSRFToken retrieves the CSRF token from the request context only.
// IMPORTANT: Do not touch the session here — some routes (e.g. /health,
// static, or early middlewares) may not have scs session data loaded into
// the context, and calling SessionManager.Get would panic.
// It returns an empty string if the token is not found.
func GetCSRFToken(r *http.Request) string {
	if token, ok := r.Context().Value(security.CSRFTokenContextKey).(string); ok {
		return token
	}
	return ""
}

// CSRFMiddleware is a middleware that adds CSRF protection to routes
func CSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logger.Get().With().
			Str("middleware", "CSRF").
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Logger()

		// Get CSRF token from request context (do not access session here)
		token := GetCSRFToken(r)
		// Determine if request is safe/static where token absence is expected
		isSafeMethod := r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions || r.Method == http.MethodTrace
		isStaticPath := strings.HasPrefix(r.URL.Path, "/css/") ||
			strings.HasPrefix(r.URL.Path, "/js/") ||
			strings.HasPrefix(r.URL.Path, "/webfonts/") ||
			r.URL.Path == "/favicon.ico"

		if token == "" {
			if isSafeMethod || isStaticPath || r.URL.Path == "/health" || r.URL.Path == "/api/health" || r.URL.Path == "/api/healthz" {
				log.Debug().Msg("No CSRF token in context (expected for safe/static/health)")
			} else {
				log.Warn().Msg("No CSRF token found in request context for potentially unsafe request")
			}
		} else {
			log.Debug().Msg("CSRF token present in request context")
		}

		// Add CSRF token to context (idempotent)
		ctx := context.WithValue(r.Context(), security.CSRFTokenContextKey, token)
		r = r.WithContext(ctx)

		// For GET requests, expose token via header so clients can fetch it easily
		if isSafeMethod && token != "" {
			w.Header().Set("X-CSRF-Token", token)
		}

		next.ServeHTTP(w, r)
	})
}
