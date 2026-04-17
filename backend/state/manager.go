package state

import (
	"errors"
	"sync"
	"time"

	"github.com/alexedwards/scs/v2"

	"pvmss/constants"
	"pvmss/database"
	envpkg "pvmss/env"
	"pvmss/logger"
	"pvmss/proxmox"
)

// NOTE: Proxmox connection methods (StartOnlineMode, SetOfflineMode, IsOfflineMode,
// GetProxmoxStatus, CheckProxmoxConnection, etc.) are in manager_proxmox.go

// appState is the concrete implementation of StateManager
type appState struct {
	sessionManager *scs.SessionManager

	// settings cache – protected by settingsMu (separate from mu so settings
	// writes never block session/env/frontend reads and vice-versa).
	settings   *AppSettings
	settingsMu sync.RWMutex

	// db is the SQLite database backing the settings cache.
	// nil when running without a DB (unit tests only).
	db database.DB

	mu sync.RWMutex

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

	// Environment configuration (loaded once at startup)
	envConfig *envpkg.EnvConfig

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

// MakeAppState creates a new StateManager without a database.
// Suitable for unit tests and legacy settings.json operation.
func MakeAppState() StateManager {
	return newAppState(nil)
}

// MakeAppStateWithDB creates a StateManager backed by the provided SQLite DB.
// The DB must already be open and migrated; the caller retains ownership and
// is responsible for closing it after the StateManager is done.
func MakeAppStateWithDB(db database.DB) StateManager {
	return newAppState(db)
}

// newAppState is the shared constructor used by both public constructors.
func newAppState(db database.DB) *appState {
	s := &appState{
		settings:   &AppSettings{},
		db:         db,
		csrfTokens: make(map[string]time.Time),
	}
	go s.cleanupSecurityData()
	go s.cleanupGuestAgentCache()
	return s
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

// SetEnvConfig stores the validated environment configuration.
func (s *appState) SetEnvConfig(cfg *envpkg.EnvConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.envConfig = cfg
}

// GetEnvConfig returns the stored environment configuration.
// Panics if SetEnvConfig has not been called; this is a programming error.
func (s *appState) GetEnvConfig() *envpkg.EnvConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.envConfig == nil {
		panic("state: GetEnvConfig called before SetEnvConfig — call SetEnvConfig during startup")
	}
	return s.envConfig
}

// Settings methods moved to manager_settings.go
