// Package constants defines application-wide constants for timeouts, limits, and configuration values.
package constants

import "time"

// Proxmox naming conventions used by PVMSS. These three values define the
// namespace PVMSS owns inside a shared Proxmox cluster:
//
//   - PoolPrefix is the prefix attached to every PVMSS-managed pool. A pool
//     named `pvmss_alice` belongs to user `alice@pve`.
//   - UserSuffix is the Proxmox realm suffix used for PVMSS-provisioned users.
//   - RequiredTag is the mandatory tag every PVMSS-managed VM carries; the
//     API filters out VMs that do not have it.
const (
	PoolPrefix  = "pvmss_"
	UserSuffix  = "@pve"
	RequiredTag = "pvmss"
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

	// ProxmoxOfflineThreshold is how long to wait before switching to offline mode
	// If no connection for 2 minutes, app goes offline automatically
	ProxmoxOfflineThreshold = 2 * time.Minute

	// NodeCacheRefreshInterval controls how often we refresh cached node data
	NodeCacheRefreshInterval = 30 * time.Second

	// NodeCacheRequestTimeout is the maximum time we wait when refreshing the cache
	NodeCacheRequestTimeout = 10 * time.Second

	// NodeCacheStaleThreshold indicates when cached data should be considered stale
	NodeCacheStaleThreshold = 2 * time.Minute

	// ClusterCacheRefreshInterval controls how often the background snapshot worker refreshes Proxmox data
	ClusterCacheRefreshInterval           = 45 * time.Second
	ClusterCacheRequestMinRefreshInterval = 30 * time.Second

	// ClusterCacheRequestTimeout bounds the overall duration of a single snapshot refresh cycle
	ClusterCacheRequestTimeout = 20 * time.Second
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
	GuestAgentCacheTTL = 1 * time.Minute

	// GuestAgentTimeout is the maximum time to wait for guest agent responses
	GuestAgentTimeout = 3 * time.Second

	// GuestAgentShutdownPollInterval is the delay between status checks after an agent-based shutdown
	GuestAgentShutdownPollInterval = 3 * time.Second

	// GuestAgentShutdownMaxAttempts is the maximum number of status checks after an agent-based shutdown
	GuestAgentShutdownMaxAttempts = 5
)
