package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"pvmss/logger"
)

// Simple in-memory token bucket per IP+path.
// Not distributed; suitable for a single-instance deployment.

type bucket struct {
	capacity   int
	tokens     float64
	ratePerSec float64
	lastRefill time.Time
}

type limiter struct {
	mu      sync.Mutex
	rules   map[string]rule // path -> rule
	buckets map[string]*bucket
}

type rule struct {
	Capacity int
	Refill   time.Duration // time to refill 1 token
}

var defaultLimiter = &limiter{
	rules:   make(map[string]rule),
	buckets: make(map[string]*bucket),
}

// ConfigureLoginRateLimit sets a sane default for POST /login, e.g., 5 req/min per IP.
func ConfigureLoginRateLimit() {
	// 5 tokens capacity, refill 12s per token ~ 5/minute
	defaultLimiter.mu.Lock()
	defer defaultLimiter.mu.Unlock()
	defaultLimiter.rules["POST /login"] = rule{Capacity: 5, Refill: 12 * time.Second}
}

// RateLimitMiddleware enforces per-IP rate limits for configured routes.
func RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		defaultLimiter.mu.Lock()
		rl, ok := defaultLimiter.rules[key]
		defaultLimiter.mu.Unlock()
		if !ok {
			// No rule, just pass through
			next.ServeHTTP(w, r)
			return
		}

		ip := clientIP(r)
		bkKey := key + "|" + ip

		defaultLimiter.mu.Lock()
		bk, exists := defaultLimiter.buckets[bkKey]
		if !exists {
			bk = &bucket{
				capacity:   rl.Capacity,
				tokens:     float64(rl.Capacity),
				ratePerSec: 1.0 / rl.Refill.Seconds(),
				lastRefill: time.Now(),
			}
			defaultLimiter.buckets[bkKey] = bk
		}

		// Refill
		now := time.Now()
		elapsed := now.Sub(bk.lastRefill).Seconds()
		bk.tokens = minFloat(float64(bk.capacity), bk.tokens+elapsed*bk.ratePerSec)
		bk.lastRefill = now

		allowed := bk.tokens >= 1.0
		if allowed {
			bk.tokens -= 1.0
		}
		defaultLimiter.mu.Unlock()

		if !allowed {
			logger.Get().Warn().Str("ip", ip).Str("path", r.URL.Path).Msg("Rate limit exceeded")
			w.Header().Set("Retry-After", "10")
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	// Try common proxy headers, then RemoteAddr
	// Note: You mentioned not behind proxy for HTTPS currently; still safe.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	if xr := r.Header.Get("X-Real-IP"); xr != "" {
		return xr
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
