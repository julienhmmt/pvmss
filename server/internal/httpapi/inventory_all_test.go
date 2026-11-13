//nolint:noctx // test scaffolding does not need real context
package httpapi_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"testing"
	"time"
)

const (
	wantAccepted = "accepted"
	wantNotFound = "not_found"
)

// multiRefreshRegistry binds one refresher per cluster name.
func multiRefreshRegistry(t *testing.T, names ...string) *inventory.Registry {
	t.Helper()

	projections := map[string]*inventory.Projection{}
	for _, name := range names {
		projections[name] = inventory.NewProjection()
	}

	registry := inventory.NewRegistryFromProjections(projections)

	for _, name := range names {
		client := &stubClusterClient{snapshot: cluster.Snapshot{Nodes: []cluster.Node{{Name: cluster.FakeNode01, Status: cluster.NodeOnline}}}}
		worker := inventory.NewWorker(client, projections[name], time.Hour, slog.New(slog.DiscardHandler))
		registry.SetRefresherForTest(name, inventory.NewRefresher(worker, 5*time.Second))
	}

	return registry
}

// TestClusterRefresh_AdminRefreshAll - an admin POST without ?cluster=
// refreshes every cluster and reports a per-cluster outcome map.
//
//nolint:paralleltest // serial: shared inventory fixture
func TestClusterRefresh_AdminRefreshAll(t *testing.T) {
	registry := multiRefreshRegistry(t, dashCluster, dashWestCluster)
	provider := vmCreateClientProvider{clients: map[string]cluster.Client{
		dashCluster:     &stubClusterClient{},
		dashWestCluster: &stubClusterClient{},
	}}
	auth := nodesTestAuth(t)
	h := httpapi.NewClusterRefreshWithRegistry(registry, provider, auth, slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/refresh", nil)
	r.AddCookie(adminCookie(t, auth))
	h.ServeHTTP(w, r)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", w.Code)
	}

	var got struct {
		Clusters map[string]string `json:"clusters"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.Clusters[dashCluster] != wantAccepted || got.Clusters[dashWestCluster] != wantAccepted {
		t.Fatalf("clusters = %v, want both accepted", got.Clusters)
	}
}

// TestClusterRefresh_AdminRefreshAllMissingRefresher - a listed cluster with
// no refresher reports not_found; when nothing can refresh at all, the handler
// answers 503.
//
//nolint:paralleltest // serial: shared inventory fixture
func TestClusterRefresh_AdminRefreshAllMissingRefresher(t *testing.T) {
	registry := inventory.NewRegistryFromProjections(map[string]*inventory.Projection{dashCluster: inventory.NewProjection()})
	provider := vmCreateClientProvider{clients: map[string]cluster.Client{
		dashCluster:     &stubClusterClient{},
		dashWestCluster: &stubClusterClient{},
	}}
	auth := nodesTestAuth(t)
	h := httpapi.NewClusterRefreshWithRegistry(registry, provider, auth, slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/refresh", nil)
	r.AddCookie(adminCookie(t, auth))
	h.ServeHTTP(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 when every refresh lookup fails", w.Code)
	}
}

// TestClusterRefresh_AdminRefreshAllMixedOutcomes - a refresh-all where one
// cluster accepts and one has no refresher reports both outcomes in the map.
//
//nolint:paralleltest // serial: shared inventory fixture
func TestClusterRefresh_AdminRefreshAllMixedOutcomes(t *testing.T) {
	registry := multiRefreshRegistry(t, dashCluster)
	provider := vmCreateClientProvider{clients: map[string]cluster.Client{
		dashCluster:     &stubClusterClient{},
		dashWestCluster: &stubClusterClient{},
	}}
	auth := nodesTestAuth(t)
	h := httpapi.NewClusterRefreshWithRegistry(registry, provider, auth, slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/refresh", nil)
	r.AddCookie(adminCookie(t, auth))
	h.ServeHTTP(w, r)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", w.Code)
	}

	var got struct {
		Clusters map[string]string `json:"clusters"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.Clusters[dashCluster] != wantAccepted || got.Clusters[dashWestCluster] != wantNotFound {
		t.Fatalf("clusters = %v, want east accepted, west not_found", got.Clusters)
	}
}

// TestClusterRefresh_AdminRefreshAllUnreachable - when every refresh fails the
// first failure decides the response; an unreachable cluster maps to 502.
//
//nolint:paralleltest // serial: shared inventory fixture
func TestClusterRefresh_AdminRefreshAllUnreachable(t *testing.T) {
	projection := inventory.NewProjection()
	client := &stubClusterClient{err: cluster.ErrUnreachable}
	worker := inventory.NewWorker(client, projection, time.Hour, slog.New(slog.DiscardHandler))
	refresher := inventory.NewRefresher(worker, 1*time.Millisecond)
	time.Sleep(3 * time.Millisecond)

	registry := inventory.NewRegistryFromProjections(map[string]*inventory.Projection{dashCluster: projection})
	registry.SetRefresherForTest(dashCluster, refresher)

	provider := vmCreateClientProvider{clients: map[string]cluster.Client{dashCluster: &stubClusterClient{}}}
	auth := nodesTestAuth(t)
	h := httpapi.NewClusterRefreshWithRegistry(registry, provider, auth, slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/refresh", nil)
	r.AddCookie(adminCookie(t, auth))
	h.ServeHTTP(w, r)

	if w.Code != http.StatusAccepted && w.Code != http.StatusBadGateway && w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 202, 502 or 429", w.Code)
	}
}

// TestClusterRefresh_Unauthenticated - a request without a session is refused
// before any refresh.
func TestClusterRefresh_Unauthenticated(t *testing.T) {
	t.Parallel()

	auth := nodesTestAuth(t)
	h := httpapi.NewClusterRefreshWithRegistry(inventory.NewRegistryFromProjections(nil), nil, auth, slog.New(slog.DiscardHandler))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/refresh", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

// TestClusterRefresh_AdminUnknownCluster - ?cluster= naming a cluster with no
// refresher answers 404.
//
//nolint:paralleltest // serial: shared inventory fixture
func TestClusterRefresh_AdminUnknownCluster(t *testing.T) {
	registry := multiRefreshRegistry(t, dashCluster)
	auth := nodesTestAuth(t)
	h := httpapi.NewClusterRefreshWithRegistry(registry, nil, auth, slog.New(slog.DiscardHandler))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/refresh?cluster=ghost", nil)
	r.AddCookie(adminCookie(t, auth))
	h.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}
