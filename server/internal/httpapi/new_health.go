package httpapi

import (
	"log/slog"
	"time"
)

// NewHealth creates a health handler for the given store and cluster freshness
// source. staleThreshold is the duration beyond which a cluster's
// RefreshedAt is declared stale — typically 2 * InventoryRefreshInterval
// (clusterStaleMultiplier). A nil freshness checker produces a healthy
// clusters check and demoMode=false (useful for T00's original tests that
// predate the clusters aggregate).
func NewHealth(store Pinger, log *slog.Logger, freshness ClusterFreshnessChecker, staleThreshold time.Duration) *Health {
	return &Health{store: store, freshness: freshness, staleThreshold: staleThreshold, log: log}
}
