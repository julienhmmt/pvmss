package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"

	"pvmss/logger"
)

// trustedProxyConfig holds the parsed trusted-proxy CIDRs. When empty,
// X-Forwarded-For and X-Real-IP are ignored and r.RemoteAddr is used directly.
// This prevents IP spoofing by direct clients.
var (
	trustedProxyMu         sync.RWMutex
	trustedProxyCIDRs      []*net.IPNet
	trustedProxyConfigured bool
)

// SetTrustedProxies configures the list of trusted proxy CIDRs. Only requests
// whose RemoteAddr matches one of these CIDRs will have their X-Forwarded-For
// / X-Real-IP headers honored by clientIP. An empty list (the default) means
// no proxies are trusted and forwarding headers are always ignored.
//
// Must be called once at startup before the server accepts traffic.
func SetTrustedProxies(cidrs []string) error {
	parsed := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		// If no / prefix, treat as a single IP (/32 for IPv4, /128 for IPv6).
		if !strings.Contains(c, "/") {
			ip := net.ParseIP(c)
			if ip == nil {
				return &net.ParseError{Type: "IP address", Text: c}
			}
			if ip.To4() != nil {
				c += "/32"
			} else {
				c += "/128"
			}
		}
		_, ipNet, err := net.ParseCIDR(c)
		if err != nil {
			return err
		}
		parsed = append(parsed, ipNet)
	}

	trustedProxyMu.Lock()
	trustedProxyCIDRs = parsed
	trustedProxyConfigured = len(parsed) > 0
	trustedProxyMu.Unlock()
	return nil
}

// isTrustedProxy reports whether ip belongs to one of the configured trusted
// proxy CIDRs.
func isTrustedProxy(ip net.IP) bool {
	trustedProxyMu.RLock()
	defer trustedProxyMu.RUnlock()
	if !trustedProxyConfigured {
		return false
	}
	for _, cidr := range trustedProxyCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// clientIP extracts the client's IP address from the request. Forwarding
// headers (X-Forwarded-For, X-Real-IP) are only honored when the request's
// RemoteAddr belongs to a configured trusted proxy. Otherwise RemoteAddr is
// used directly, preventing IP spoofing by direct clients.
func clientIP(r *http.Request) string {
	remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteHost = r.RemoteAddr
	}
	remoteIP := net.ParseIP(remoteHost)

	if remoteIP != nil && isTrustedProxy(remoteIP) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// X-Forwarded-For can be a comma-separated list of IPs. The first
			// one is the original client.
			if parts := strings.Split(xff, ","); len(parts) > 0 {
				return strings.TrimSpace(parts[0])
			}
		}
		if xr := r.Header.Get("X-Real-IP"); xr != "" {
			return strings.TrimSpace(xr)
		}
	}

	return remoteHost
}

// MakeMiddlewareLogger creates a new contextual logger for a middleware component.
func MakeMiddlewareLogger(name string) *logger.Logger {
	log := logger.Get().With().Str("middleware", name).Logger()
	return &log
}
