package httpapi_test

import (
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"testing"
)

// projectionRegistry wraps the test's live projection under the "default"
// cluster name the requests use - the registry shares the same projection
// handle, so mid-test store() calls stay visible, matching the removed
// single-cluster resolver.
func projectionRegistry(projection *inventory.Projection) *inventory.Registry {
	return inventory.NewRegistryFromProjections(map[string]*inventory.Projection{auditTestCluster: projection})
}

// singleIndexRegistry registers one index under the "default" cluster name;
// nil keeps the entry unpopulated, matching the "not ready" (503) state an
// empty inventory.NewProjection() had.
func singleIndexRegistry(index *inventory.Index) *inventory.Registry {
	return inventory.NewRegistryFromIndexes(map[string]*inventory.Index{auditTestCluster: index})
}

// emptyRegistry replaces the inventory.NewProjection() call sites: the
// "default" entry stays unpopulated so reads answer inventory_not_ready.
func emptyRegistry() *inventory.Registry {
	return singleIndexRegistry(nil)
}

// singleClientProvider wraps one cluster client so the registry constructors
// get a ClientProvider that serves every request for "default". A nil client
// leaves the map empty, so resolution fails with cluster_not_found - what the
// nil-client error tests expect.
func singleClientProvider(client cluster.Client) cluster.ClientProvider {
	clients := map[string]cluster.Client{}
	if client != nil {
		clients[auditTestCluster] = client
	}

	return vmCreateClientProvider{clients: clients}
}

// refreshRegistryFor shares the test's projection under "default" and binds
// its hand-built refresher to it, so NewClusterRefreshWithRegistry triggers
// the same worker the legacy single-refresher wiring used.
func refreshRegistryFor(projection *inventory.Projection, refresher *inventory.Refresher) *inventory.Registry {
	registry := inventory.NewRegistryFromProjections(map[string]*inventory.Projection{auditTestCluster: projection})
	registry.SetRefresherForTest(auditTestCluster, refresher)

	return registry
}

// nodesTestAuth keeps a per-test auth handler so helper calls do not build a
// fresh session manager per constructor argument.
func nodesTestAuth(t *testing.T) *httpapi.Auth {
	t.Helper()
	return newAuthHandler(t)
}
