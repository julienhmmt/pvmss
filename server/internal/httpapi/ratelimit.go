package httpapi

import (
	"net"
	"net/http"
	"strconv"
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

// userRateLimiter is a per-user fixed-window request limiter for authenticated
// state-changing endpoints. It is keyed by username, falling back to client IP
// when no principal can be resolved.
type userRateLimiter struct {
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

func newUserRateLimiter(maxRequests int, window time.Duration) *userRateLimiter {
	return &userRateLimiter{hits: make(map[string][]time.Time), max: maxRequests, window: window}
}

// allow reports whether key may make another request now and, if not, how long
// until the oldest hit in the current window expires.
func (l *userRateLimiter) allow(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := now.Add(-l.window)

	hits := l.hits[key]
	kept := hits[:0]

	var oldest time.Time

	for _, t := range hits {
		if t.After(cutoff) {
			kept = append(kept, t)
			if oldest.IsZero() || t.Before(oldest) {
				oldest = t
			}
		}
	}

	if len(kept) >= l.max {
		l.hits[key] = kept
		if oldest.IsZero() {
			return false, l.window
		}

		remaining := oldest.Add(l.window).Sub(now)
		if remaining <= 0 {
			return false, l.window
		}

		return false, remaining
	}

	l.hits[key] = append(kept, now)

	return true, 0
}

// middleware wraps next, rejecting requests over the limit with 429 and a
// Retry-After header. The caller is identified by Auth.Principal; when the
// request is unauthenticated, client IP is used as a fallback key.
func (l *userRateLimiter) middleware(authHandler *Auth, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := clientIP(r)

		identity, err := authHandler.Principal(r)
		if err == nil {
			key = identity.Username
		}

		allowed, retryAfter := l.allow(key, time.Now())
		if !allowed {
			writeRateLimitError(w, retryAfter)

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

// writeRateLimitError writes a 429 response with a Retry-After header and a
// JSON body that includes the remaining seconds. The web client reads
// retryAfterSeconds to re-enable gated controls.
func writeRateLimitError(w http.ResponseWriter, retryAfter time.Duration) {
	seconds := max(int(retryAfter.Seconds()), 1)

	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeAuthJSON(w, http.StatusTooManyRequests, authError{Code: "rate_limited", Message: "too many requests, try again later", RetryAfterSeconds: seconds})
}
