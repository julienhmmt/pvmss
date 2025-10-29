// Package constants defines application-wide constants for timeouts, limits, and configuration values.
package constants

import "time"

// Security Configuration
const (
	// CSRFTokenLength is the byte length of CSRF tokens
	CSRFTokenLength = 32

	// CSRFTokenTTL is the default lifetime for CSRF tokens
	CSRFTokenTTL = 30 * time.Minute

	// CSRFCleanupInterval is how often to clean expired CSRF tokens
	CSRFCleanupInterval = 30 * time.Minute

	// SessionCleanupInterval is how often to clean expired sessions
	SessionCleanupInterval = 30 * time.Minute
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

// Session Keys
const (
	// SessionKeyAuthenticated is the session key for authentication status
	SessionKeyAuthenticated = "authenticated"

	// SessionKeyIsAdmin is the session key for admin status
	SessionKeyIsAdmin = "is_admin"

	// SessionKeyUsername is the session key for username
	SessionKeyUsername = "username"

	// SessionKeyCSRFToken is the session key for CSRF token
	SessionKeyCSRFToken = "csrf_token"

	// SessionKeyPVEAuthCookie is the session key for Proxmox auth cookie
	SessionKeyPVEAuthCookie = "pve_auth_cookie" // #nosec G101 - This is a session key name, not a credential

	// SessionKeyPVECSRFToken is the session key for Proxmox CSRF token
	SessionKeyPVECSRFToken = "pve_csrf_token" // #nosec G101 - This is a session key name, not a credential

	// SessionKeyPVEUsername is the session key for Proxmox username
	SessionKeyPVEUsername = "pve_username" // #nosec G101 - This is a session key name, not a credential

	// SessionKeyPVETicketCreated is the session key for ticket creation timestamp
	SessionKeyPVETicketCreated = "pve_ticket_created"
)
