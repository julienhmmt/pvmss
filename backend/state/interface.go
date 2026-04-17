// Package state provides centralized management of application state and dependencies.
// It uses dependency injection and interface-based design for better testability and maintainability.
//
// Usage:
//   - Use StateManager interface with dependency injection (RECOMMENDED)

package state

import (
	"context"
	"time"

	"github.com/alexedwards/scs/v2"

	"pvmss/database"
	envpkg "pvmss/env"
	"pvmss/proxmox"
)

// StateManager defines the interface for managing application state
type StateManager interface {
	// Session management
	GetSessionManager() *scs.SessionManager
	SetSessionManager(sm *scs.SessionManager) error

	// Proxmox connection management
	StartOnlineMode() error
	SetOfflineMode() // Enable offline mode (no Proxmox client)
	IsOfflineMode() bool
	GetProxmoxStatus() (bool, string) // Returns (connected, errorMessage)
	CheckProxmoxConnection() bool
	GetNodeCache() ([]*proxmox.NodeDetails, time.Time)
	RefreshNodeCache(ctx context.Context)
	GetProxmoxSnapshot() *ProxmoxClusterSnapshot
	RequestSnapshotRefresh()

	// Settings management
	GetSettings() *AppSettings
	SetSettings(settings *AppSettings) error
	SetSettingsWithoutSave(settings *AppSettings)
	GetTags() []string
	GetISOs() []string
	GetVMBRs() []string
	GetLimits() map[string]interface{}
	GetStorages() []string

	// Security management
	AddCSRFToken(token string, expiry time.Time) error
	ValidateAndRemoveCSRFToken(token string) bool
	CleanExpiredCSRFTokens()

	// Environment configuration
	GetEnvConfig() *envpkg.EnvConfig
	SetEnvConfig(cfg *envpkg.EnvConfig)

	// Frontend configuration
	GetFrontendPath() string
	SetFrontendPath(path string)

	// Cleanup callbacks
	SetGuestAgentCleanupFunc(cleanupFunc func())

	// DB-backed settings writers.
	// changedBy must be the authenticated admin username (from JWT claims).
	// All methods call reloadSettingsCache() on success so the in-memory
	// cache is immediately consistent with the database.

	// HasDB reports whether the state manager is backed by a database.
	// When true, callers must use the fine-grained DB setters (SetTags, etc.)
	// instead of SetSettings() for persistence.
	HasDB() bool

	// LoadSettingsFromDB loads all settings from the database into the cache.
	// Must be called once during startup after DB initialisation.
	LoadSettingsFromDB() error

	// SetVMLimits persists updated VM resource limits.
	SetVMLimits(limits *database.VMLimits, changedBy string) error

	// GetNodeLimitFromDB retrieves a single node's limits directly from the database.
	// Returns the limit, true if found, or false if not found.
	GetNodeLimitFromDB(node string) (database.NodeLimit, bool, error)

	// SetNodeLimit upserts all capacity limits for a single node.
	SetNodeLimit(limit database.NodeLimit, changedBy string) error

	// DeleteNodeLimit removes a per-node VM count override.
	DeleteNodeLimit(node string, changedBy string) error

	// SetEnabledNodes replaces the full list of enabled Proxmox nodes.
	SetEnabledNodes(nodes []string, changedBy string) error

	// SetEnabledStorages replaces the full list of enabled storages.
	SetEnabledStorages(storages []string, changedBy string) error

	// SetEnabledISOs replaces the full list of available ISO images.
	SetEnabledISOs(isos []string, changedBy string) error

	// SetEnabledVMBRs replaces the full list of allowed network bridges.
	SetEnabledVMBRs(vmbrs []string, changedBy string) error

	// SetTags replaces the full list of VM tags.
	SetTags(tags []string, changedBy string) error

	// CreateCloudInitTemplate inserts a new cloud-init template.
	CreateCloudInitTemplate(t *database.CloudInitTemplate, changedBy string) error

	// UpdateCloudInitTemplate updates an existing cloud-init template.
	UpdateCloudInitTemplate(t *database.CloudInitTemplate, changedBy string) error

	// DeleteCloudInitTemplate removes a cloud-init template by ID.
	DeleteCloudInitTemplate(id string, changedBy string) error

	// CreateVMProfile inserts a new VM profile.
	CreateVMProfile(p *database.VMProfile, changedBy string) error

	// UpdateVMProfile updates an existing VM profile.
	UpdateVMProfile(p *database.VMProfile, changedBy string) error

	// DeleteVMProfile removes a VM profile by ID.
	DeleteVMProfile(id string, changedBy string) error

	// SetSFTPConfig persists the SFTP/SSH configuration.
	SetSFTPConfig(cfg *database.SFTPConfig, changedBy string) error
}

// SettingsProvider exposes read access to application settings.
type SettingsProvider interface {
	GetSettings() *AppSettings
}

// ProxmoxStatusProvider exposes the Proxmox connection status.
type ProxmoxStatusProvider interface {
	GetProxmoxStatus() (bool, string)
}
