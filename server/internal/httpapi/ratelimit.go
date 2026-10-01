package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"pvmss/server/internal/store"
	"strconv"
	"sync"
	"time"
)

// windowLimiter is the fixed-window counter shared by both limiters: at most
// max hits per key inside window. Keys are pruned on access and idle keys are
// swept, so the map cannot grow for the life of the process.
type windowLimiter struct {
	mu               sync.Mutex
	hits             map[string][]time.Time
	lastSweep        time.Time
	max              int
	window           time.Duration
	trustedProxyHops int
	store            *store.Store
}

func newWindowLimiter(maxRequests int, window time.Duration, trustedProxyHops int, st *store.Store) *windowLimiter {
	return &windowLimiter{hits: make(map[string][]time.Time), max: maxRequests, window: window, trustedProxyHops: trustedProxyHops, store: st}
}

// allow reports whether key may make another request now, recording the hit
// if so. When it may not, the duration is how long until the oldest hit in the
// current window expires.
func (l *windowLimiter) allow(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := now.Add(-l.window)
	l.lastSweep = sweepIdle(l.hits, l.lastSweep, now, l.window)

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

		remaining := oldest.Add(l.window).Sub(now)
		if oldest.IsZero() || remaining <= 0 {
			return false, l.window
		}

		return false, remaining
	}

	l.hits[key] = append(kept, now)

	return true, 0
}

// sweepIdle drops keys whose hits have all left the window, at most once per
// window, and returns the time of the last sweep. Without it a key that never
// returns would stay in the map for the life of the process. The caller holds
// the limiter's lock.
func sweepIdle(hits map[string][]time.Time, lastSweep, now time.Time, window time.Duration) time.Time {
	if now.Sub(lastSweep) < window {
		return lastSweep
	}

	cutoff := now.Add(-window)

	for key, times := range hits {
		if len(times) == 0 || !times[len(times)-1].After(cutoff) {
			delete(hits, key)
		}
	}

	return now
}

func (l *windowLimiter) recordRateLimited(ctx context.Context, r *http.Request, actor, keyType string) {
	if l.store == nil {
		return
	}

	body, _ := json.Marshal(map[string]any{
		auditKeySummary: fmt.Sprintf("rate limited on %s key", keyType),
		auditKeyChanges: []any{map[string]any{"keyType": keyType, "actor": actor}},
	})
	_ = l.store.RecordAdminAction(ctx, actor, "auth.rate_limited", "auth", actor, string(body), clientIP(r, l.trustedProxyHops))
}

// ipRateLimiter is a per-IP limiter for unauthenticated endpoints (login,
// admin-login) that would otherwise let an attacker brute-force credentials
// with no backoff.
type ipRateLimiter struct{ *windowLimiter }

func newIPRateLimiter(maxRequests int, window time.Duration, trustedProxyHops int, st *store.Store) *ipRateLimiter {
	return &ipRateLimiter{newWindowLimiter(maxRequests, window, trustedProxyHops, st)}
}

// middleware wraps next, rejecting requests over the limit with 429.
func (l *ipRateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r, l.trustedProxyHops)
		if allowed, _ := l.allow(ip, time.Now()); !allowed {
			l.recordRateLimited(r.Context(), r, ip, "ip")
			writeAuthError(w, http.StatusTooManyRequests, "rate_limited", "too many requests, try again later")

			return
		}

		next.ServeHTTP(w, r)
	})
}

// userRateLimiter is a per-user limiter for authenticated state-changing
// endpoints. It is keyed by username, falling back to client IP when no
// principal can be resolved.
type userRateLimiter struct{ *windowLimiter }

func newUserRateLimiter(maxRequests int, window time.Duration, trustedProxyHops int, st *store.Store) *userRateLimiter {
	return &userRateLimiter{newWindowLimiter(maxRequests, window, trustedProxyHops, st)}
}

// middleware wraps next, rejecting requests over the limit with 429 and a
// Retry-After header. The caller is identified by Auth.Principal; when the
// request is unauthenticated, client IP is used as a fallback key.
func (l *userRateLimiter) middleware(authHandler *Auth, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := clientIP(r, l.trustedProxyHops)
		actor := ""

		identity, err := authHandler.Principal(r)
		if err == nil {
			key = identity.Username
			actor = identity.Username
		}

		allowed, retryAfter := l.allow(key, time.Now())
		if !allowed {
			l.recordRateLimited(r.Context(), r, actor, "user")
			writeRateLimitError(w, retryAfter)

			return
		}

		next.ServeHTTP(w, r)
	})
}

// writeRateLimitError writes a 429 response with a Retry-After header and a
// JSON body that includes the remaining seconds. The web client reads
// retryAfterSeconds to re-enable gated controls.
func writeRateLimitError(w http.ResponseWriter, retryAfter time.Duration) {
	seconds := max(int(retryAfter.Seconds()), 1)

	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeAuthJSON(w, http.StatusTooManyRequests, authError{Code: "rate_limited", Message: "too many requests, try again later", RetryAfterSeconds: seconds})
}
