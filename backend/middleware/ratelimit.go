package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"pvmss/logger"
)

// Package middleware provides rate-limiting functionality using an in-memory token bucket algorithm.
// This implementation is designed for single-instance deployments and is not distributed.

// bucket represents a token bucket for a specific client and route.
type bucket struct {
	capacity   int
	tokens     float64
	ratePerSec float64
	lastAccess time.Time // Used to identify and clean up stale buckets.
	rejects    uint64    // Counter for rejected requests (for monitoring).
}

// Rule defines the rate-limiting parameters for a specific route.
type Rule struct {
	Capacity int           // The maximum number of tokens the bucket can hold.
	Refill   time.Duration // The time it takes to generate one new token.
}

// Limiter manages rate-limiting rules and active token buckets.
type Limiter struct {
	mu      sync.RWMutex
	rules   map[string]Rule    // Key: "METHOD /path"
	buckets map[string]*bucket // Key: "METHOD /path|ip_address"
}

// NewRateLimiter creates a new Limiter and starts a background goroutine to clean up stale buckets.
func NewRateLimiter(cleanupInterval, staleThreshold time.Duration) *Limiter {
	l := &Limiter{
		rules:   make(map[string]Rule),
		buckets: make(map[string]*bucket),
	}

	// Start a background goroutine to periodically remove old buckets.
	go l.cleanupStaleBuckets(cleanupInterval, staleThreshold)

	return l
}

// AddRule adds a new rate-limiting rule for a given method and path.
func (l *Limiter) AddRule(method, path string, rule Rule) {
	l.mu.Lock()
	defer l.mu.Unlock()
	key := method + " " + path

	if rule.Capacity <= 0 {
		logger.Get().Warn().Str("method", method).Str("path", path).
			Msg("Rate limit capacity must be positive; defaulting to 1")
		rule.Capacity = 1
	}

	if rule.Refill <= 0 {
		logger.Get().Warn().Str("method", method).Str("path", path).
			Msg("Rate limit refill duration must be positive; defaulting to 1s")
		rule.Refill = time.Second
	}

	l.rules[key] = rule
	logger.Get().Debug().Str("method", method).Str("path", path).
		Int("capacity", rule.Capacity).Dur("refill", rule.Refill).
		Msg("Rate limit rule added")
}

// Allow checks if a request is permitted under the configured rate limits.
// It returns true if the request should be allowed, and false otherwise.
func (l *Limiter) Allow(method, path, ip string) bool {
	key := method + " " + path

	// Use RLock first to check if rule exists
	l.mu.RLock()
	rule, ok := l.rules[key]
	l.mu.RUnlock()

	if !ok {
		return true // No rule for this path, so allow the request.
	}

	// Now acquire write lock for bucket operations
	l.mu.Lock()
	defer l.mu.Unlock()

	bucketKey := key + "|" + ip
	bk, exists := l.buckets[bucketKey]
	if !exists {
		ratePerSec := 1.0 / rule.Refill.Seconds()
		bk = &bucket{
			capacity:   rule.Capacity,
			tokens:     float64(rule.Capacity),
			ratePerSec: ratePerSec,
			rejects:    0,
		}
		bk.lastAccess = time.Now()
		l.buckets[bucketKey] = bk
	}

	if bk.capacity != rule.Capacity {
		bk.capacity = rule.Capacity
		if bk.tokens > float64(bk.capacity) {
			bk.tokens = float64(bk.capacity)
		}
	}

	ratePerSec := 1.0 / rule.Refill.Seconds()
	bk.ratePerSec = ratePerSec

	// Refill the bucket with new tokens based on the elapsed time.
	now := time.Now()
	if !bk.lastAccess.IsZero() {
		elapsed := now.Sub(bk.lastAccess).Seconds()
		if elapsed > 0 && bk.ratePerSec > 0 {
			bk.tokens += elapsed * bk.ratePerSec
			if bk.tokens > float64(bk.capacity) {
				bk.tokens = float64(bk.capacity)
			}
		}
	}
	bk.lastAccess = now

	// Check if there are enough tokens to allow the request.
	if bk.tokens >= 1.0 {
		bk.tokens--
		return true
	}

	// Track rejection for monitoring
	bk.rejects++
	return false
}

// Rule returns the configured rate limiting rule for a given method and path, if any.
func (l *Limiter) Rule(method, path string) (Rule, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	rule, ok := l.rules[method+" "+path]
	return rule, ok
}

// cleanupStaleBuckets periodically removes buckets that haven't been accessed
// for a duration greater than the staleThreshold.
func (l *Limiter) cleanupStaleBuckets(interval, threshold time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log := logger.Get().With().Str("component", "rate_limiter_cleanup").Logger()

	for range ticker.C {
		l.mu.Lock()
		cleaned := 0
		for key, bk := range l.buckets {
			if time.Since(bk.lastAccess) > threshold {
				delete(l.buckets, key)
				cleaned++
			}
		}
		remainingBuckets := len(l.buckets)
		l.mu.Unlock()

		if cleaned > 0 {
			log.Debug().Int("cleaned", cleaned).Int("remaining", remainingBuckets).
				Msg("Cleaned up stale rate limit buckets")
		}
	}
}

// GetStats returns statistics about the current state of the rate limiter.
// Useful for monitoring and debugging.
func (l *Limiter) GetStats() map[string]interface{} {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return map[string]interface{}{
		"active_buckets": len(l.buckets),
		"rules_count":    len(l.rules),
	}
}

// RateLimitMiddleware returns a middleware that enforces rate limits using the provided Limiter.
func RateLimitMiddleware(limiter *Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rule, hasRule := limiter.Rule(r.Method, r.URL.Path)
			ip := clientIP(r)

			if hasRule && rule.Capacity > 0 {
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rule.Capacity))
			}

			if !limiter.Allow(r.Method, r.URL.Path, ip) {
				logger.Get().Warn().
					Str("ip", ip).
					Str("method", r.Method).
					Str("path", r.URL.Path).
					Str("user_agent", r.UserAgent()).
					Msg("Rate limit exceeded")
				retryAfter := 1
				if hasRule {
					if seconds := int(rule.Refill / time.Second); seconds > 0 {
						retryAfter = seconds
					}
				}
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
