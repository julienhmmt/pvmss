// Package state provides centralized management of application state and dependencies.
// It uses dependency injection and interface-based design for better testability and maintainability.
package state

import (
	"html/template"
	"time"

	"pvmss/proxmox"

	"github.com/alexedwards/scs/v2"
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
	GetProxmoxStatus() (bool, string) // Returns (connected, errorMessage)
	CheckProxmoxConnection() bool     // Performs a connection check and updates status

	// Settings management
	GetSettings() *AppSettings
	SetSettings(settings *AppSettings) error
	SetSettingsWithoutSave(settings *AppSettings)
	GetAdminPassword() string
	GetTags() []string
	GetISOs() []string
	GetVMBRs() []string
	GetLimits() map[string]interface{}

	// Security management
	// CSRF token management
	AddCSRFToken(token string, expiry time.Time) error
	ValidateAndRemoveCSRFToken(token string) bool
	CleanExpiredCSRFTokens()
}
