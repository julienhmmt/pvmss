package middleware

import (
	"net/http"
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
}

// Rule defines the rate-limiting parameters for a specific route.
type Rule struct {
	Capacity int           // The maximum number of tokens the bucket can hold.
	Refill   time.Duration // The time it takes to generate one new token.
}

// Limiter manages rate-limiting rules and active token buckets.
type Limiter struct {
	mu      sync.Mutex
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
	l.rules[key] = rule
}

// Allow checks if a request is permitted under the configured rate limits.
// It returns true if the request should be allowed, and false otherwise.
func (l *Limiter) Allow(method, path, ip string) bool {
	key := method + " " + path
	l.mu.Lock()
	defer l.mu.Unlock()

	rule, ok := l.rules[key]
	if !ok {
		return true // No rule for this path, so allow the request.
	}

	bucketKey := key + "|" + ip
	bk, exists := l.buckets[bucketKey]
	if !exists {
		bk = &bucket{
			capacity:   rule.Capacity,
			tokens:     float64(rule.Capacity),
			ratePerSec: 1.0 / rule.Refill.Seconds(),
		}
		l.buckets[bucketKey] = bk
	}

	// Refill the bucket with new tokens based on the elapsed time.
	now := time.Now()
	elapsed := now.Sub(bk.lastAccess).Seconds()
	bk.tokens += elapsed * bk.ratePerSec
	if bk.tokens > float64(bk.capacity) {
		bk.tokens = float64(bk.capacity)
	}
	bk.lastAccess = now

	// Check if there are enough tokens to allow the request.
	if bk.tokens >= 1.0 {
		bk.tokens--
		return true
	}

	return false
}

// cleanupStaleBuckets periodically removes buckets that haven't been accessed
// for a duration greater than the staleThreshold.
func (l *Limiter) cleanupStaleBuckets(interval, threshold time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		l.mu.Lock()
		for key, bk := range l.buckets {
			if time.Since(bk.lastAccess) > threshold {
				delete(l.buckets, key)
			}
		}
		l.mu.Unlock()
	}
}

// RateLimitMiddleware returns a middleware that enforces rate limits using the provided Limiter.
func RateLimitMiddleware(limiter *Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			if !limiter.Allow(r.Method, r.URL.Path, ip) {
				logger.Get().Warn().Str("ip", ip).Str("path", r.URL.Path).Msg("Rate limit exceeded")
				w.Header().Set("Retry-After", "10") // Inform the client to wait.
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
