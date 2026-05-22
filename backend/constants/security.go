// Package constants defines application-wide constants for timeouts, limits, and configuration values.
package constants

import "time"

// Background cleanup
const (
	// GuestAgentCacheCleanupInterval is how often to clean expired guest agent cache entries.
	GuestAgentCacheCleanupInterval = 30 * time.Minute
)

// Rate Limiting
const (
	// RateLimitWindow is the time window for rate limiting
	RateLimitWindow = 5 * time.Minute

	// RateLimitCleanup is how often to clean rate limiter data
	RateLimitCleanup = 15 * time.Minute

	// LoginRateLimitCapacity is the max login attempts allowed
	LoginRateLimitCapacity = 5

	// LoginRateLimitRefill is how often a login attempt token is refilled
	LoginRateLimitRefill = 12 * time.Second
)
