// Package httpapi provides HTTP API handlers and types.
package httpapi

import "time"

// ClusterFreshness is one configured cluster's freshness signal — its name and
// the time its inventory projection was last successfully refreshed. The health
// handler compares RefreshedAt against twice the inventory refresh interval to
// decide staleness (data-model.md). The Name is never disclosed in the public
// health response (FR-012) — only the count of stale clusters is.
type ClusterFreshness struct {
	Name        string
	RefreshedAt time.Time
}

// ClusterFreshnessChecker supplies the health handler with per-cluster
// freshness data and the demonstration-mode flag. The health handler never
// calls cluster.Client — it reads state the inventory refresh goroutines
// already maintain (constitution IV, FR-010).
type ClusterFreshnessChecker interface {
	// Clusters returns the freshness snapshot for every configured cluster.
	Clusters() []ClusterFreshness
	// DemoMode reports whether the instance is wired to the fake cluster
	// client (Configuration.ClusterSource == "fake").
	DemoMode() bool
}
