package security

import (
	"net/http"
	"os"

	"pvmss/logger"
)

// HeadersMiddleware adds security headers to all responses
func HeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Security headers
		headers := map[string]string{
			"X-Content-Type-Options": "nosniff",
			"X-Frame-Options":        "DENY",
			"X-XSS-Protection":       "1; mode=block",
			"Referrer-Policy":        "strict-origin-when-cross-origin",
			"Permissions-Policy":     "camera=(), microphone=(), geolocation=()",
		}

		// Add HSTS in production
		if os.Getenv("ENV") == "production" {
			headers["Strict-Transport-Security"] = "max-age=31536000; includeSubDomains; preload"

			// Add CSP header in production
			csp := "default-src 'self'; " +
				"script-src 'self' 'unsafe-inline' 'unsafe-eval'; " +
				"style-src 'self' 'unsafe-inline'; " +
				"img-src 'self' data:; " +
				"font-src 'self'; " +
				"connect-src 'self'"
			headers["Content-Security-Policy"] = csp
		}

		// Set all headers
		for k, v := range headers {
			w.Header().Set(k, v)
		}

		// Log request details for security monitoring
		log := logger.Get().With().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote_addr", r.RemoteAddr).
			Logger()

		log.Debug().Msg("Processing request with security headers")

		next.ServeHTTP(w, r)
	})
}
