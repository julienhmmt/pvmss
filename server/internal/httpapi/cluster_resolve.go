package httpapi

import (
	"fmt"
	"net/http"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/vm"
)

// resolveCapability resolves a per-cluster client capability (Writer,
// SnapshotReader, ConsoleRelay, ...) from a registry-backed ClientProvider,
// keyed on the request's own :cluster path/query value. When clients is nil
// (legacy single-cluster constructors and unit tests that construct a
// handler directly with a bound capability), it returns fallback unchanged —
// every WithRegistry constructor sets clients so the per-request path is the
// one main.go actually exercises.
//
// Without this, a handler bound once at startup to the "default" cluster's
// client would keep serving every cluster's requests through that one
// client — a cross-cluster data leak when node names or vmids collide
// between clusters (root cause behind the metrics-history ticket).
func resolveCapability[T any](clients cluster.ClientProvider, fallback T, clusterName, capability string) (T, error) {
	if clients == nil {
		return fallback, nil
	}

	client, err := clients.Client(clusterName)
	if err != nil {
		var zero T
		return zero, err
	}

	value, ok := client.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("cluster %q client does not implement %s", clusterName, capability)
	}

	return value, nil
}

// ClusterRefresherResolver resolves the IndexRefresher for a named cluster —
// the write-side sibling of registryResolver. Without it, a handler bound once
// at startup to the default cluster's *inventory.Worker would refresh the
// default cluster's projection after every write, even writes targeting a
// non-default cluster (the same class of bug registryResolver closed for
// reads).
type ClusterRefresherResolver interface {
	RefresherFor(cluster string) (vm.IndexRefresher, error)
}

// registryRefresherResolver adapts the inventory Registry to
// ClusterRefresherResolver — each cluster name resolves to that cluster's own
// *inventory.Worker, which satisfies vm.IndexRefresher.
type registryRefresherResolver struct {
	registry *inventory.Registry
}

func (r registryRefresherResolver) RefresherFor(clusterName string) (vm.IndexRefresher, error) {
	return r.registry.Worker(clusterName)
}

// loadClusterIndex resolves the current Index for clusterName via resolver,
// writing the appropriate error response and returning ok=false on any
// failure: an unknown cluster name (404 cluster_not_found) or a known
// cluster whose inventory has not been populated yet (503
// inventory_not_ready).
func loadClusterIndex(resolver vm.ClusterIndexResolver, clusterName string, writeErr func(status int, code, message string)) (*inventory.Index, bool) {
	index, err := resolver.IndexFor(clusterName)
	if err != nil {
		writeErr(http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return nil, false
	}

	if index == nil {
		writeErr(http.StatusServiceUnavailable, "inventory_not_ready", msgInventoryNotReady)
		return nil, false
	}

	return index, true
}
