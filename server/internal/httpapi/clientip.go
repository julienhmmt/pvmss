package httpapi

import (
	"net"
	"net/http"
	"strings"
)

// clientIP extracts the request's source IP. PVMSS deploys behind a Kubernetes
// ingress, so RemoteAddr is the proxy's IP, not the user's. When
// trustedProxyHops > 0, the X-Forwarded-For header is parsed and the IP at
// position len(hops) - trustedProxyHops is selected — the first untrusted hop
// from the right. With trustedProxyHops == 0 (no trusted proxy), XFF is
// ignored and RemoteAddr is used directly. In both cases the port is stripped
// from the result.
//
// Example: XFF = "203.0.113.5, 10.0.0.1" (client, proxy1)
//   - trustedProxyHops=1 → index 1 → "10.0.0.1" (trust proxy1, take its claim)
//   - trustedProxyHops=2 → index 0 → "203.0.113.5" (trust both, take the client)
//
// If the XFF chain is shorter than trustedProxyHops, the leftmost entry
// (original client claim) is returned. If XFF is absent or
// trustedProxyHops is 0, RemoteAddr is used.
func clientIP(r *http.Request, trustedProxyHops int) string {
	if trustedProxyHops > 0 {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			hops := strings.Split(xff, ",")
			for i := range hops {
				hops[i] = strings.TrimSpace(hops[i])
			}

			idx := len(hops) - trustedProxyHops
			if idx < 0 {
				idx = 0
			}

			if idx < len(hops) && hops[idx] != "" {
				return hops[idx]
			}
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}
