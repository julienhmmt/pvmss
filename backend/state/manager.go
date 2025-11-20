package state

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"sync"
	"time"

	"github.com/alexedwards/scs/v2"

	"pvmss/constants"
	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/proxmox"
)

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
	clusterSnapshotRefreshMu   sync.Mutex
	clusterSnapshotRefreshing  bool

	// Cleanup callbacks
	guestAgentCleanupFunc func()
	cleanupMu             sync.RWMutex
}

func translateProxmoxMessage(messageID string) string {
	return i18n.Localize(i18n.GetLocalizer(i18n.DefaultLang), messageID)
}

// startProxmoxMonitor starts a non-blocking background goroutine that periodically
// checks the Proxmox connectivity and updates the shared status. Runs every 30 seconds.
func (s *appState) startProxmoxMonitor() {
	s.proxmoxMu.Lock()
	if s.proxmoxMonitorStarted {
		s.proxmoxMu.Unlock()
		return
	}
	s.proxmoxMonitorStarted = true
	s.proxmoxMu.Unlock()

	log := logger.Get().With().Str("component", "ProxmoxMonitor").Logger()
	go func() {
		// Immediate check to ensure status freshness
		s.CheckProxmoxConnection()

		ticker := time.NewTicker(constants.ProxmoxConnectionCheckInterval)
		defer ticker.Stop()
		for range ticker.C {
			ok := s.CheckProxmoxConnection()
			if !ok {
				_, errMsg := s.GetProxmoxStatus()
				log.Debug().Str("error", errMsg).Msg("Proxmox connectivity check failed")
			}
		}
	}()
}

// startNodeCacheWorker launches a background worker that refreshes node details at regular intervals.
func (s *appState) startNodeCacheWorker() {
	s.mu.Lock()
	if s.nodeCacheWorkerStarted {
		s.mu.Unlock()
		return
	}
	s.nodeCacheWorkerStarted = true
	s.mu.Unlock()

	log := logger.Get().With().Str("component", "NodeCacheWorker").Logger()
	go func() {
		log.Info().Dur("interval", constants.NodeCacheRefreshInterval).Msg("Node cache worker started")
		s.refreshNodeCache(context.Background())
		ticker := time.NewTicker(constants.NodeCacheRefreshInterval)
		defer ticker.Stop()
		for range ticker.C {
			s.refreshNodeCache(context.Background())
		}
	}()
}

// startClusterSnapshotWorker launches a background worker that keeps a warm snapshot of cluster resources.
func (s *appState) startClusterSnapshotWorker() {
	s.mu.Lock()
	if s.clusterSnapshotWorkerStart {
		s.mu.Unlock()
		return
	}
	s.clusterSnapshotWorkerStart = true
	s.mu.Unlock()

	log := logger.Get().With().Str("component", "ClusterSnapshotWorker").Logger()
	go func() {
		log.Info().Dur("interval", constants.ClusterCacheRefreshInterval).Msg("Cluster snapshot worker started")
		s.triggerSnapshotRefresh("worker_init")
		ticker := time.NewTicker(constants.ClusterCacheRefreshInterval)
		defer ticker.Stop()
		for range ticker.C {
			s.triggerSnapshotRefresh("worker")
		}
	}()
}

// triggerSnapshotRefresh schedules an asynchronous refresh if one isn't already in progress.
func (s *appState) triggerSnapshotRefresh(trigger string) {
	if s.IsOfflineMode() {
		return
	}

	s.clusterSnapshotRefreshMu.Lock()
	if s.clusterSnapshotRefreshing {
		s.clusterSnapshotRefreshMu.Unlock()
		return
	}
	s.clusterSnapshotRefreshing = true
	s.clusterSnapshotRefreshMu.Unlock()

	go func() {
		s.refreshProxmoxSnapshot(trigger)
		s.clusterSnapshotRefreshMu.Lock()
		s.clusterSnapshotRefreshing = false
		s.clusterSnapshotRefreshMu.Unlock()
	}()
}

// refreshProxmoxSnapshot refreshes the cached cluster snapshot.
func (s *appState) refreshProxmoxSnapshot(trigger string) {
	if s.IsOfflineMode() {
		return
	}

	log := logger.Get().With().
		Str("component", "ClusterSnapshotWorker").
		Str("trigger", trigger).
		Logger()

	client, err := proxmox.NewRestyClientFromEnv(constants.ClusterCacheRequestTimeout)
	if err != nil {
		log.Warn().Err(err).Msg("Unable to create resty client for snapshot refresh")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), constants.ClusterCacheRequestTimeout)
	defer cancel()

	snapshot, err := buildProxmoxSnapshot(ctx, client)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to refresh Proxmox snapshot")
		return
	}

	s.clusterSnapshotMu.Lock()
	s.clusterSnapshot = snapshot
	s.clusterSnapshotMu.Unlock()

	log.Debug().
		Time("generated_at", snapshot.GeneratedAt).
		Dur("duration", snapshot.Duration).
		Int("nodes", len(snapshot.NodeNames)).
		Int("storages", len(snapshot.NodeStorages)).
		Msg("Proxmox snapshot updated")
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

// refreshNodeCache fetches node information from Proxmox and stores it for fast access.
func (s *appState) refreshNodeCache(ctx context.Context) {
	log := logger.Get().With().Str("component", "NodeCacheWorker").Logger()
	if s.IsOfflineMode() {
		log.Debug().Msg("Skipping node cache refresh: offline mode enabled")
		return
	}

	connected, _ := s.GetProxmoxStatus()
	if !connected {
		log.Debug().Msg("Skipping node cache refresh: Proxmox disconnected")
		return
	}

	restyClient, err := proxmox.NewRestyClientFromEnv(constants.NodeCacheRequestTimeout)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to create resty client for node cache refresh")
		return
	}

	refreshCtx, cancel := context.WithTimeout(ctx, constants.NodeCacheRequestTimeout)
	defer cancel()

	details, err := proxmox.FetchAllNodeDetailsResty(refreshCtx, restyClient)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to refresh node cache")
		return
	}

	s.nodeCacheMu.Lock()
	s.nodeCache = cloneNodeDetails(details)
	s.nodeCacheUpdate = time.Now()
	s.nodeCacheMu.Unlock()

	log.Debug().Int("nodes", len(details)).Msg("Node cache refreshed")
}

// SetGuestAgentCleanupFunc registers a cleanup function for guest agent caches
// This allows handlers package to register its cleanup without circular dependencies
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

// GetProxmoxClient returns the Proxmox client
// TODO Telmate migration: this getter is only needed for the Telmate ClientInterface; delete it after the migration and rely on Resty-based helpers instead.
func (s *appState) GetProxmoxClient() proxmox.ClientInterface {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.proxmoxClient
}

// SetProxmoxClient sets the Proxmox client
// TODO Telmate migration: this setter is only needed for the Telmate ClientInterface; delete it after the migration and rely on Resty-based helpers instead.
func (s *appState) SetProxmoxClient(pc proxmox.ClientInterface) error {
	if pc == nil {
		return errors.New("proxmox client cannot be nil")
	}
	s.mu.Lock()
	s.proxmoxClient = pc
	s.offlineMode = false
	s.mu.Unlock()

	// Check connection when setting a new client
	s.CheckProxmoxConnection()

	// Start background monitor (only once)
	s.startProxmoxMonitor()
	s.startNodeCacheWorker()
	s.startClusterSnapshotWorker()
	return nil
}

// SetOfflineMode enables offline mode (no Proxmox client)
func (s *appState) SetOfflineMode() {
	s.mu.Lock()
	s.offlineMode = true
	s.mu.Unlock()

	// Reset failure tracking when manually setting offline mode
	s.proxmoxMu.Lock()
	s.proxmoxConnectionLostTime = time.Time{}
	s.proxmoxConnectionFailureCount = 0
	s.proxmoxMu.Unlock()

	// Update status to reflect offline mode
	s.updateProxmoxStatus(false, translateProxmoxMessage(constants.MsgProxmoxOfflineMode))
	logger.Get().Info().Msg("Offline mode activated")
}

// IsOfflineMode returns true if offline mode is enabled
func (s *appState) IsOfflineMode() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.offlineMode
}

// GetProxmoxStatus returns the current Proxmox connection status
func (s *appState) GetProxmoxStatus() (bool, string) {
	s.proxmoxMu.RLock()
	defer s.proxmoxMu.RUnlock()
	return s.proxmoxConnected, s.proxmoxError
}

// CheckProxmoxConnection checks the connection to the Proxmox server and updates the status
// TODO Telmate migration: reimplement this Proxmox health check using only Resty-based helpers instead of the Telmate client.
func (s *appState) CheckProxmoxConnection() bool {
	s.mu.RLock()
	client := s.proxmoxClient
	offline := s.offlineMode
	s.mu.RUnlock()

	// Check if we're in manual offline mode (PVMSS_OFFLINE=true)
	// In this case, we don't try to recover
	if offline && s.isManualOfflineMode() {
		s.updateProxmoxStatus(false, translateProxmoxMessage(constants.MsgProxmoxOfflineMode))
		return false
	}

	if client == nil {
		s.updateProxmoxStatus(false, translateProxmoxMessage(constants.MsgProxmoxClientNil))
		return false
	}

	// Try to get node names as a simple connection test
	ctx, cancel := context.WithTimeout(context.Background(), constants.ProxmoxConnectionCheckTimeout)
	defer cancel()

	logger.Get().Debug().Msg("Starting Proxmox connection check")
	nodes, err := proxmox.GetNodeNamesWithContext(ctx, client)

	if err != nil {
		logger.Get().Error().Err(err).Msg("Proxmox connection check failed with error")
		s.handleConnectionFailure()
		return false
	}

	if len(nodes) == 0 {
		logger.Get().Error().Msg("Proxmox connection check returned empty node list")
		s.handleConnectionFailure()
		return false
	}

	logger.Get().Debug().Int("node_count", len(nodes)).Msg("Proxmox connection check successful")

	// If we got here, the connection is good - attempt recovery
	s.handleConnectionRecovery()
	return true
}

// GetNodeCache returns a copy of cached node details and the last refresh timestamp.
func (s *appState) GetNodeCache() ([]*proxmox.NodeDetails, time.Time) {
	s.nodeCacheMu.RLock()
	defer s.nodeCacheMu.RUnlock()

	if len(s.nodeCache) == 0 {
		return []*proxmox.NodeDetails{}, s.nodeCacheUpdate
	}

	return cloneNodeDetails(s.nodeCache), s.nodeCacheUpdate
}

// GetProxmoxSnapshot returns the cached cluster snapshot (if available).
func (s *appState) GetProxmoxSnapshot() *ProxmoxClusterSnapshot {
	s.clusterSnapshotMu.RLock()
	defer s.clusterSnapshotMu.RUnlock()
	return cloneProxmoxSnapshot(s.clusterSnapshot)
}

// RequestSnapshotRefresh schedules a snapshot refresh if possible.
func (s *appState) RequestSnapshotRefresh() {
	if s.IsOfflineMode() {
		return
	}
	s.clusterSnapshotMu.RLock()
	snapshot := s.clusterSnapshot
	s.clusterSnapshotMu.RUnlock()
	if snapshot != nil {
		if time.Since(snapshot.GeneratedAt) < constants.ClusterCacheRequestMinRefreshInterval {
			return
		}
	}
	s.triggerSnapshotRefresh("request")
}

// isManualOfflineMode checks if offline mode was set manually (via PVMSS_OFFLINE)
// vs automatic offline mode due to connection failures
func (s *appState) isManualOfflineMode() bool {
	// If we're in offline mode but have no failure tracking, it's manual
	s.proxmoxMu.RLock()
	defer s.proxmoxMu.RUnlock()
	return s.proxmoxConnectionLostTime.IsZero()
}

// handleConnectionFailure manages connection failures and automatic offline mode
func (s *appState) handleConnectionFailure() {
	s.proxmoxMu.Lock()
	defer s.proxmoxMu.Unlock()

	now := time.Now()

	// Track failure time and count
	if s.proxmoxConnectionLostTime.IsZero() {
		s.proxmoxConnectionLostTime = now
		logger.Get().Warn().Time("failure_started", now).Msg("Proxmox connection failures detected")
	}

	s.proxmoxConnectionFailureCount++

	// Check if we've exceeded the threshold for automatic offline mode
	if now.Sub(s.proxmoxConnectionLostTime) >= constants.ProxmoxOfflineThreshold {
		if !s.offlineMode {
			logger.Get().Warn().
				Dur("failure_duration", now.Sub(s.proxmoxConnectionLostTime)).
				Int("failure_count", s.proxmoxConnectionFailureCount).
				Msg("Proxmox connection failed for 2 minutes, switching to offline mode automatically")

			// Switch to offline mode
			s.offlineMode = true
			s.updateProxmoxStatusInternal(false, translateProxmoxMessage(constants.MsgProxmoxAutoOfflineMode))
		}
	} else {
		// Still within threshold, update error message
		errMsg := fmt.Sprintf("Failed to connect to Proxmox (failure #%d, duration: %v)",
			s.proxmoxConnectionFailureCount, now.Sub(s.proxmoxConnectionLostTime).Round(time.Second))
		s.updateProxmoxStatusInternal(false, errMsg)
	}
}

// handleConnectionRecovery resets failure tracking when connection is restored
func (s *appState) handleConnectionRecovery() {
	s.proxmoxMu.Lock()
	defer s.proxmoxMu.Unlock()

	// Reset failure tracking if we were in failure state
	if !s.proxmoxConnectionLostTime.IsZero() {
		logger.Get().Info().
			Time("failure_started", s.proxmoxConnectionLostTime).
			Int("failure_count", s.proxmoxConnectionFailureCount).
			Dur("total_downtime", time.Since(s.proxmoxConnectionLostTime)).
			Msg("Proxmox connection restored after failures")

		// Reset tracking
		s.proxmoxConnectionLostTime = time.Time{}
		s.proxmoxConnectionFailureCount = 0

		// Exit automatic offline mode
		if s.offlineMode {
			s.mu.Lock()
			s.offlineMode = false
			s.mu.Unlock()
			logger.Get().Info().Msg("Exiting automatic offline mode - back to online")
		}
	}

	// Update status to connected (using internal method, already holding lock)
	s.updateProxmoxStatusInternal(true, "")
}

// updateProxmoxStatusInternal updates status without locking (caller must hold lock)
func (s *appState) updateProxmoxStatusInternal(connected bool, errorMsg string) {
	// Only log if status changed
	if s.proxmoxConnected != connected || s.proxmoxError != errorMsg {
		status := "connected"
		if !connected {
			status = fmt.Sprintf("disconnected: %s", errorMsg)
		}
		logger.Get().Debug().
			Bool("connected", connected).
			Str("status", status).
			Str("error", errorMsg).
			Msg("Proxmox status updated")
	}

	s.proxmoxConnected = connected
	s.proxmoxError = errorMsg
}

// updateProxmoxStatus updates the Proxmox connection status in a thread-safe way
func (s *appState) updateProxmoxStatus(connected bool, errorMsg string) {
	s.proxmoxMu.Lock()
	defer s.proxmoxMu.Unlock()
	s.updateProxmoxStatusInternal(connected, errorMsg)
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

// cloneNodeDetails creates deep copies of node details slices to avoid data races.
func cloneNodeDetails(details []*proxmox.NodeDetails) []*proxmox.NodeDetails {
	if len(details) == 0 {
		return []*proxmox.NodeDetails{}
	}
	cloned := make([]*proxmox.NodeDetails, len(details))
	for i, detail := range details {
		if detail == nil {
			continue
		}
		copyDetail := *detail
		cloned[i] = &copyDetail
	}
	return cloned
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
