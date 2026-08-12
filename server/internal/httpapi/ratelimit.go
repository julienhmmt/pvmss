package httpapi

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// ipRateLimiter is a per-IP fixed-window request limiter for unauthenticated
// endpoints (login, admin-login) that would otherwise let an attacker
// brute-force credentials with no backoff.
type ipRateLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	max    int
	window time.Duration
}

func newIPRateLimiter(maxRequests int, window time.Duration) *ipRateLimiter {
	return &ipRateLimiter{hits: make(map[string][]time.Time), max: maxRequests, window: window}
}

// allow reports whether ip may make another request now, recording the hit
// if so. Old hits outside the window are pruned on every call so the map
// never grows unbounded for a given key.
func (l *ipRateLimiter) allow(ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := now.Add(-l.window)

	hits := l.hits[ip]
	kept := hits[:0]

	for _, t := range hits {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}

	if len(kept) >= l.max {
		l.hits[ip] = kept
		return false
	}

	l.hits[ip] = append(kept, now)

	return true
}

// middleware wraps next, rejecting requests over the limit with 429.
func (l *ipRateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(clientIP(r), time.Now()) {
			writeAuthError(w, http.StatusTooManyRequests, "rate_limited", "too many requests, try again later")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// clientIP extracts the request's source IP, stripping the port. Falls back
// to the raw RemoteAddr if it isn't in host:port form.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}
