package state

import (
	"errors"
	"fmt"
	"html/template"
	"sync"
	"time"

	"github.com/alexedwards/scs/v2"

	"pvmss/constants"
	"pvmss/logger"
	"pvmss/proxmox"
)

// NOTE: Proxmox client methods (GetProxmoxClient, SetProxmoxClient, SetOfflineMode, IsOfflineMode,
// GetProxmoxStatus, CheckProxmoxConnection, etc.) are in manager_proxmox.go

// appState is the concrete implementation of StateManager
type appState struct {
	templates      *template.Template
	sessionManager *scs.SessionManager
	// TODO Telmate migration: this field stores the Telmate ClientInterface; remove it once all handlers use Resty-based helpers.
	proxmoxClient proxmox.ClientInterface
	settings      *AppSettings
	mu            sync.RWMutex

	// Proxmox connection status
	proxmoxConnected bool
	proxmoxError     string
	proxmoxMu        sync.RWMutex

	// Background monitor control
	proxmoxMonitorStarted  bool
	nodeCacheWorkerStarted bool

	// Offline mode flag
	offlineMode bool

	// Connection failure tracking for automatic offline mode
	proxmoxConnectionLostTime     time.Time
	proxmoxConnectionFailureCount int

	// Security-related fields
	csrfTokens map[string]time.Time
	securityMu sync.RWMutex // Mutex for CSRF token operations

	// Frontend configuration
	frontendPath string

	// Cached node details for admin views
	nodeCache       []*proxmox.NodeDetails
	nodeCacheMu     sync.RWMutex
	nodeCacheUpdate time.Time

	// Cached cluster snapshot for frequently accessed Proxmox data
	clusterSnapshot            *ProxmoxClusterSnapshot
	clusterSnapshotMu          sync.RWMutex
	clusterSnapshotWorkerStart bool
	clusterSnapshotRefreshMu   sync.RWMutex
	clusterSnapshotRefreshing  bool

	// Cleanup callbacks
	guestAgentCleanupFunc func()
	cleanupMu             sync.RWMutex
}

// NewAppState creates a new instance of the application state manager
func NewAppState() StateManager {
	state := &appState{
		settings:   &AppSettings{},
		csrfTokens: make(map[string]time.Time),
	}

	// Start background cleanup goroutines
	go state.cleanupSecurityData()
	go state.cleanupGuestAgentCache()

	return state
}

// cleanupSecurityData runs periodic cleanup of expired security data
func (s *appState) cleanupSecurityData() {
	ticker := time.NewTicker(constants.CSRFCleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		s.CleanExpiredCSRFTokens()
	}
}

// cleanupGuestAgentCache runs periodic cleanup of expired guest agent cache entries
func (s *appState) cleanupGuestAgentCache() {
	ticker := time.NewTicker(constants.CSRFCleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		s.cleanupMu.RLock()
		cleanupFunc := s.guestAgentCleanupFunc
		s.cleanupMu.RUnlock()

		if cleanupFunc != nil {
			cleanupFunc()
		}
	}
}

// SetGuestAgentCleanupFunc registers a cleanup function for guest agent caches.
// This avoids circular dependencies with handlers.
func (s *appState) SetGuestAgentCleanupFunc(cleanupFunc func()) {
	s.cleanupMu.Lock()
	defer s.cleanupMu.Unlock()
	s.guestAgentCleanupFunc = cleanupFunc
	logger.Get().Debug().Msg("Guest agent cleanup function registered")
}

// GetTemplates returns the template cache
func (s *appState) GetTemplates() *template.Template {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.templates
}

// SetTemplates sets the template cache
func (s *appState) SetTemplates(t *template.Template) error {
	if t == nil {
		return errors.New("templates cannot be nil")
	}
	s.mu.Lock()
	s.templates = t
	s.mu.Unlock()
	return nil
}

// GetSessionManager returns the session manager
func (s *appState) GetSessionManager() *scs.SessionManager {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessionManager
}

// SetSessionManager sets the session manager
func (s *appState) SetSessionManager(sm *scs.SessionManager) error {
	if sm == nil {
		return errors.New("session manager cannot be nil")
	}
	s.mu.Lock()
	s.sessionManager = sm
	s.mu.Unlock()
	return nil
}

// GetSettings returns the application settings
func (s *appState) GetSettings() *AppSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

// SetSettingsWithoutSave updates the application settings in memory without saving them to file
func (s *appState) SetSettingsWithoutSave(settings *AppSettings) {
	if settings == nil {
		logger.Get().Warn().Msg("Attempted to set nil settings without saving")
		return
	}
	s.mu.Lock()
	s.settings = settings
	s.mu.Unlock()
	logger.Get().Debug().Msg("Application settings updated in memory only")
}

// SetSettings updates the application settings and saves them to the settings file
func (s *appState) SetSettings(settings *AppSettings) error {
	if settings == nil {
		return errors.New("settings cannot be nil")
	}

	s.mu.Lock()
	s.settings = settings
	s.mu.Unlock()

	// Save the settings to the settings file
	if err := WriteSettings(settings); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to save settings to file")
		return fmt.Errorf("failed to save settings: %w", err)
	}

	logger.Get().Info().Msg("Application settings updated and saved to file")
	return nil
}

// GetTags returns the list of available tags
func (s *appState) GetTags() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.settings == nil || s.settings.Tags == nil {
		return []string{}
	}
	return s.settings.Tags
}

// GetISOs returns the list of available ISO files
func (s *appState) GetISOs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.settings == nil || s.settings.ISOs == nil {
		return []string{}
	}
	return s.settings.ISOs
}

// GetVMBRs returns the list of available network bridges
func (s *appState) GetVMBRs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.settings == nil || s.settings.VMBRs == nil {
		return []string{}
	}
	return s.settings.VMBRs
}

// GetLimits returns the resource limits as a map for backward compatibility
func (s *appState) GetLimits() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.settings == nil {
		return make(map[string]interface{})
	}

	// Convert LimitsConfig to map[string]interface{} for backward compatibility
	limits := make(map[string]interface{})

	// Convert VM limits
	vmLimits := make(map[string]interface{})
	vmLimits["sockets"] = map[string]int{"min": s.settings.Limits.VM.Sockets.Min, "max": s.settings.Limits.VM.Sockets.Max}
	vmLimits["cores"] = map[string]int{"min": s.settings.Limits.VM.Cores.Min, "max": s.settings.Limits.VM.Cores.Max}
	vmLimits["ram"] = map[string]int{"min": s.settings.Limits.VM.RAM.Min, "max": s.settings.Limits.VM.RAM.Max}
	vmLimits["disk"] = map[string]int{"min": s.settings.Limits.VM.Disk.Min, "max": s.settings.Limits.VM.Disk.Max}
	limits["vm"] = vmLimits

	// Convert node limits
	if len(s.settings.Limits.Nodes) > 0 {
		nodesLimits := make(map[string]interface{})
		for nodeName, nodeLimits := range s.settings.Limits.Nodes {
			nodeMap := map[string]interface{}{
				"sockets": map[string]int{"min": nodeLimits.Sockets.Min, "max": nodeLimits.Sockets.Max},
				"cores":   map[string]int{"min": nodeLimits.Cores.Min, "max": nodeLimits.Cores.Max},
				"ram":     map[string]int{"min": nodeLimits.RAM.Min, "max": nodeLimits.RAM.Max},
				"disk":    map[string]int{"min": nodeLimits.Disk.Min, "max": nodeLimits.Disk.Max},
			}
			nodesLimits[nodeName] = nodeMap
		}
		limits["nodes"] = nodesLimits
	}

	return limits
}

// GetStorages returns the list of available storages
func (s *appState) GetStorages() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.settings == nil {
		return []string{}
	}
	return s.settings.EnabledStorages
}

// Security Methods
// AddCSRFToken adds a new CSRF token with an expiry time
func (s *appState) AddCSRFToken(token string, expiry time.Time) error {
	s.securityMu.Lock()
	defer s.securityMu.Unlock()
	s.csrfTokens[token] = expiry
	return nil
}

// ValidateAndRemoveCSRFToken validates a CSRF token and removes it if valid
func (s *appState) ValidateAndRemoveCSRFToken(token string) bool {
	s.securityMu.Lock()
	defer s.securityMu.Unlock()

	expiry, exists := s.csrfTokens[token]
	if !exists {
		return false
	}

	// Remove the token (one-time use)
	delete(s.csrfTokens, token)

	// Check if token is expired
	if time.Now().After(expiry) {
		return false
	}

	return true
}

// CleanExpiredCSRFTokens removes all expired CSRF tokens
func (s *appState) CleanExpiredCSRFTokens() {
	s.securityMu.Lock()
	defer s.securityMu.Unlock()

	now := time.Now()
	expiredCount := 0
	for token, expiry := range s.csrfTokens {
		if now.After(expiry) {
			delete(s.csrfTokens, token)
			expiredCount++
		}
	}

	if expiredCount > 0 {
		logger.Get().Debug().
			Int("expired_count", expiredCount).
			Int("remaining_count", len(s.csrfTokens)).
			Msg("Cleaned expired CSRF tokens")
	}
}

// Frontend Configuration Methods

// GetFrontendPath returns the frontend path for static file serving
func (s *appState) GetFrontendPath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.frontendPath
}

// SetFrontendPath sets the frontend path for static file serving
func (s *appState) SetFrontendPath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.frontendPath = path
	logger.Get().Debug().Str("path", path).Msg("Frontend path configured")
}
