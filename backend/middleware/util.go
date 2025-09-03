package middleware

import (
	"net"
	"net/http"
	"strings"

	"github.com/rs/zerolog"
	"pvmss/logger"
)

// clientIP extracts the client's IP address from the request, respecting common proxy headers.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can be a comma-separated list of IPs. The first one is the original client.
		if parts := strings.Split(xff, ","); len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	if xr := r.Header.Get("X-Real-IP"); xr != "" {
		return xr
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr // Fallback to RemoteAddr if parsing fails.
	}
	return host
}

// NewMiddlewareLogger creates a new contextual logger for a middleware component.
func NewMiddlewareLogger(name string) *zerolog.Logger {
	log := logger.Get().With().Str("middleware", name).Logger()
	return &log
}

// IsStaticOrHealthPath checks if the request path corresponds to a static asset or a health check endpoint.
// These paths are often exempt from certain middleware processing like CSRF checks or session loading.
func IsStaticOrHealthPath(path string) bool {
	// Check for static asset paths.
	staticPrefixes := []string{"/css/", "/js/", "/webfonts/", "/favicon.ico"}
	for _, prefix := range staticPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	// Check for health check endpoints.
	healthEndpoints := []string{"/health", "/api/health", "/api/healthz"}
	for _, endpoint := range healthEndpoints {
		if path == endpoint {
			return true
		}
	}

	return false
}
