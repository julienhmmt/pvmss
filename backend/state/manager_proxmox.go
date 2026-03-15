package state

import (
	"context"
	"fmt"
	"time"

	"pvmss/constants"
	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/proxmox"
)

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

// StartOnlineMode transitions to online mode and starts background monitors.
func (s *appState) StartOnlineMode() error {
	s.mu.Lock()
	s.offlineMode = false
	s.mu.Unlock()

	// Reset manual offline mode tracking
	s.proxmoxMu.Lock()
	s.proxmoxConnectionLostTime = time.Time{}
	s.proxmoxConnectionFailureCount = 0
	s.proxmoxMu.Unlock()

	s.CheckProxmoxConnection()
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
	logger.ProxmoxEvent("offline_mode_activated").
		Msg("Offline mode activated")
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
func (s *appState) CheckProxmoxConnection() bool {
	s.mu.RLock()
	offline := s.offlineMode
	s.mu.RUnlock()

	// Check if we're in manual offline mode (PVMSS_OFFLINE=true)
	// In this case, we don't try to recover
	if offline && s.isManualOfflineMode() {
		s.updateProxmoxStatus(false, translateProxmoxMessage(constants.MsgProxmoxOfflineMode))
		return false
	}

	// Try to get node names as a simple connection test
	ctx, cancel := context.WithTimeout(context.Background(), constants.ProxmoxConnectionCheckTimeout)
	defer cancel()

	restyClient, err := proxmox.MakeRestyClientFromEnv(constants.ProxmoxConnectionCheckTimeout)
	if err != nil {
		logger.ProxmoxFailure("connection_check", "client_error").
			Err(err).
			Msg("Proxmox connection check failed: could not create resty client")
		s.handleConnectionFailure()
		return false
	}

	logger.Get().Debug().Msg("Starting Proxmox connection check")
	nodes, err := proxmox.GetNodeNamesResty(ctx, restyClient)

	if err != nil {
		logger.ProxmoxFailure("connection_check", "api_error").
			Err(err).
			Msg("Proxmox connection check failed")
		s.handleConnectionFailure()
		return false
	}

	if len(nodes) == 0 {
		logger.ProxmoxFailure("connection_check", "empty_node_list").
			Msg("Proxmox connection check returned empty node list")
		s.handleConnectionFailure()
		return false
	}

	logger.ProxmoxEvent("connection_check_success").
		Int("node_count", len(nodes)).
		Msg("Proxmox connection check successful")

	// If we got here, the connection is good - attempt recovery
	s.handleConnectionRecovery()
	return true
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
