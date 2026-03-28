package state

import (
	"context"
	"time"

	"pvmss/constants"
	"pvmss/logger"
	"pvmss/proxmox"
)

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

	client, err := proxmox.MakeRestyClientFromEnv(constants.ClusterCacheRequestTimeout)
	if err != nil {
		log.Warn().
			Err(err).
			Str("component", "state_manager").
			Str("operation", "snapshot_refresh").
			Str("reason", "client_creation_failed").
			Msg("Unable to create resty client for snapshot refresh")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), constants.ClusterCacheRequestTimeout)
	defer cancel()

	snapshot, err := buildProxmoxSnapshot(ctx, client)
	if err != nil {
		log.Warn().
			Err(err).
			Str("component", "state_manager").
			Str("operation", "snapshot_refresh").
			Str("reason", "snapshot_build_failed").
			Msg("Failed to refresh Proxmox snapshot")
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

// RefreshNodeCache is the public entry point for an on-demand synchronous cache refresh.
// It is a no-op when offline mode is active or Proxmox is not reachable.
func (s *appState) RefreshNodeCache(ctx context.Context) {
	s.refreshNodeCache(ctx)
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

	restyClient, err := proxmox.MakeRestyClientFromEnv(constants.NodeCacheRequestTimeout)
	if err != nil {
		log.Warn().
			Err(err).
			Str("component", "state_manager").
			Str("operation", "node_cache_refresh").
			Str("reason", "client_creation_failed").
			Msg("Failed to create resty client for node cache refresh")
		return
	}

	refreshCtx, cancel := context.WithTimeout(ctx, constants.NodeCacheRequestTimeout)
	defer cancel()

	// Fetch latest node details using the shared Resty-based aggregator.
	details, err := proxmox.FetchAllNodeDetailsResty(refreshCtx, restyClient)
	if err != nil {
		log.Warn().
			Err(err).
			Str("component", "state_manager").
			Str("operation", "node_cache_refresh").
			Str("reason", "node_details_failed").
			Msg("Failed to refresh node cache")
		return
	}

	// Merge the freshly fetched details with the existing cache so that when a node
	// becomes offline we still keep the last known resource metrics instead of
	// wiping them out. This makes the admin nodes page more informative for
	// temporarily unreachable nodes.
	s.nodeCacheMu.Lock()
	defer s.nodeCacheMu.Unlock()

	previousByName := make(map[string]*proxmox.NodeDetails, len(s.nodeCache))
	for _, existing := range s.nodeCache {
		if existing == nil || existing.Node == "" {
			continue
		}
		previousByName[existing.Node] = existing
	}

	merged := make([]*proxmox.NodeDetails, 0, len(details))
	for _, current := range details {
		if current == nil || current.Node == "" {
			continue
		}

		if prev, ok := previousByName[current.Node]; ok {
			// If the node is now reported as offline but we previously had a snapshot
			// with real resource metrics (online or already-offline with data),
			// preserve those metrics so we can still display "last known" values in
			// the UI, even across multiple refresh cycles.
			if current.Status == "offline" && (prev.MaxCPU != 0 || prev.MaxMemory != 0 || prev.MaxDisk != 0) {
				log.Debug().
					Str("node", current.Node).
					Str("component", "state_manager").
					Str("operation", "node_cache_refresh").
					Msg("Preserving last known metrics for offline node in cache")
				current.CPU = prev.CPU
				current.MaxCPU = prev.MaxCPU
				current.Sockets = prev.Sockets
				current.Memory = prev.Memory
				current.MaxMemory = prev.MaxMemory
				current.Disk = prev.Disk
				current.MaxDisk = prev.MaxDisk
				current.Uptime = prev.Uptime
			}
		}

		merged = append(merged, current)
	}

	s.nodeCache = cloneNodeDetails(merged)
	s.nodeCacheUpdate = time.Now()

	log.Debug().Int("nodes", len(merged)).Msg("Node cache refreshed")
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
