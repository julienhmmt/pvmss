package state

import (
	"errors"
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

// MakeAppState creates a new instance of the application state manager
func MakeAppState() StateManager {
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

// Settings methods moved to manager_settings.go
