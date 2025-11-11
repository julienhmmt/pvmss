// Package constants defines application-wide constants for timeouts, limits, and configuration values.
package constants

import "time"

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

	// ProxmoxOfflineThreshold is how long to wait before switching to offline mode
	// If no connection for 2 minutes, app goes offline automatically
	ProxmoxOfflineThreshold = 2 * time.Minute
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
