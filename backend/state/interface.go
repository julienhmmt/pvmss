// Package state provides centralized management of application state and dependencies.
// It uses dependency injection and interface-based design for better testability and maintainability.
//
// Usage:
//   - Use StateManager interface with dependency injection (RECOMMENDED)

package state

import (
	"html/template"
	"time"

	"github.com/alexedwards/scs/v2"

	"pvmss/proxmox"
)

// StateManager defines the interface for managing application state
type StateManager interface {
	// Templates management
	GetTemplates() *template.Template
	SetTemplates(t *template.Template) error

	// Session management
	GetSessionManager() *scs.SessionManager
	SetSessionManager(sm *scs.SessionManager) error

	// Proxmox client management
	GetProxmoxClient() proxmox.ClientInterface
	SetProxmoxClient(pc proxmox.ClientInterface) error
	SetOfflineMode() // Enable offline mode (no Proxmox client)
	IsOfflineMode() bool
	GetProxmoxStatus() (bool, string) // Returns (connected, errorMessage)
	CheckProxmoxConnection() bool
	GetNodeCache() ([]*proxmox.NodeDetails, time.Time)
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

	// Frontend configuration
	GetFrontendPath() string
	SetFrontendPath(path string)

	// Cleanup callbacks
	SetGuestAgentCleanupFunc(cleanupFunc func())
}

// SettingsProvider exposes read access to application settings.
type SettingsProvider interface {
	GetSettings() *AppSettings
}

// ProxmoxClientProvider exposes access to the Proxmox API client.
type ProxmoxClientProvider interface {
	GetProxmoxClient() proxmox.ClientInterface
}

// ProxmoxStatusProvider exposes the Proxmox connection status.
type ProxmoxStatusProvider interface {
	GetProxmoxStatus() (bool, string)
}
