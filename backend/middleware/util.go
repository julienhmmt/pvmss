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
