// Package constants defines application-wide constants for timeouts, limits, and configuration values.
// This centralizes magic numbers and makes them easier to tune and maintain.
package constants

import "time"

// HTTP and Form Configuration
const (
	// MaxFormSize is the maximum size for form submissions (10 MB)
	MaxFormSize = 10 * 1024 * 1024

	// MaxHeaderBytes is the maximum size for HTTP headers (1 MB)
	MaxHeaderBytes = 1 << 20
)

// Server Timeouts
const (
	// ServerReadTimeout is the maximum duration for reading the entire request
	ServerReadTimeout = 10 * time.Second

	// ServerWriteTimeout is the maximum duration before timing out writes of the response
	ServerWriteTimeout = 30 * time.Second

	// ServerIdleTimeout is the maximum amount of time to wait for the next request
	ServerIdleTimeout = 120 * time.Second

	// ServerReadHeaderTimeout is the amount of time allowed to read request headers
	ServerReadHeaderTimeout = 5 * time.Second
)

// Proxmox API Configuration
const (
	// ProxmoxDefaultTimeout is the default timeout for Proxmox API calls
	ProxmoxDefaultTimeout = 10 * time.Second

	// ProxmoxCacheTTL is the time-to-live for cached Proxmox API responses
	ProxmoxCacheTTL = 2 * time.Minute

	// ProxmoxConnectionCheckInterval is how often to check Proxmox connectivity in background
	ProxmoxConnectionCheckInterval = 30 * time.Second

	// ProxmoxConnectionCheckTimeout is the timeout for connectivity checks
	ProxmoxConnectionCheckTimeout = 5 * time.Second
)

// Console Session Configuration
const (
	// ConsoleSessionTTL is the lifetime of a temporary console session
	// This provides a safe margin before VNC ticket expiration (10s window)
	ConsoleSessionTTL = 8 * time.Second

	// VNCTicketValidityDuration is how long VNC tickets remain valid
	VNCTicketValidityDuration = 2 * time.Hour

	// VNCTicketSafetyMargin is the buffer before ticket expiration to consider it invalid
	VNCTicketSafetyMargin = 5 * time.Minute
)

// Cache Configuration
const (
	// GuestAgentCacheTTL is how long to cache "guest agent unavailable" status
	// This prevents repeated slow API calls to VMs without guest agent
	GuestAgentCacheTTL = 5 * time.Minute

	// GuestAgentTimeout is the maximum time to wait for guest agent responses
	GuestAgentTimeout = 1 * time.Second
)

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

// HTTP Transport Configuration
const (
	// HTTPMaxIdleConns is the maximum number of idle connections across all hosts
	HTTPMaxIdleConns = 100

	// HTTPMaxIdleConnsPerHost is the maximum number of idle connections per host
	HTTPMaxIdleConnsPerHost = 50

	// HTTPIdleConnTimeout is the maximum amount of time an idle connection will remain idle
	HTTPIdleConnTimeout = 90 * time.Second

	// HTTPTLSHandshakeTimeout is the maximum amount of time to wait for a TLS handshake
	HTTPTLSHandshakeTimeout = 10 * time.Second

	// HTTPExpectContinueTimeout is the maximum amount of time to wait for a server's first
	// response headers after fully writing the request headers if the request has "Expect: 100-continue"
	HTTPExpectContinueTimeout = 1 * time.Second

	// HTTPResponseHeaderTimeout is the amount of time to wait for a server's response headers
	HTTPResponseHeaderTimeout = 15 * time.Second
)

// Context Timeouts
const (
	// DefaultContextTimeout is the default timeout for context operations
	DefaultContextTimeout = 10 * time.Second

	// LongContextTimeout is used for long-running operations
	LongContextTimeout = 30 * time.Second

	// ShortContextTimeout is used for quick operations
	ShortContextTimeout = 5 * time.Second

	// FetchVMsTimeout is the timeout for fetching VM lists
	FetchVMsTimeout = 15 * time.Second
)

// Validation Limits
const (
	// MaxUsernameLength is the maximum allowed username length
	MaxUsernameLength = 100

	// MaxPasswordLength is the maximum allowed password length
	MaxPasswordLength = 200

	// MinPasswordLength is the minimum required password length
	MinPasswordLength = 5

	// MaxVMNameLength is the maximum VM name length
	MaxVMNameLength = 100
)

// Default Values
const (
	// DefaultPort is the default HTTP server port
	DefaultPort = "50000"

	// DefaultLogLevel is the default logging level
	DefaultLogLevel = "info"

	// DefaultLanguage is the default application language
	DefaultLanguage = "en"

	// DefaultLoginRealm is the default Proxmox authentication realm
	DefaultLoginRealm = "pve"

	// AppVersion is the current application version
	AppVersion = "0.2.0"
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
