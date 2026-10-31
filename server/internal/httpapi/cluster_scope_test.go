//nolint:noctx // test scaffolding does not need real context
package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	scopeDefaultNode   = "default-node"
	scopeSecondaryNode = "secondary-node"
)

type nodesTestResponse struct {
	Nodes []struct {
		Name    string `json:"name"`
		Cluster string `json:"cluster"`
	} `json:"nodes"`
}

// newScopedAuthFixture builds a registry-backed Auth over the seeded fake
// clusters plus a bootstrap admin, so tests can mint both a scoped user
// session (alice on secondary) and an admin session.
func newScopedAuthFixture(t *testing.T) *httpapi.Auth {
	t.Helper()

	secret := "scope-test-secret-with-at-least-32-bytes" //nolint:gosec // deterministic test secret

	st, err := store.Open(config.Configuration{DBPath: filepath.Join(t.TempDir(), "scope.db"), ClusterSource: cluster.SourceFake, SessionSecret: secret})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	rows, err := st.ListClusters(context.Background())
	if err != nil {
		t.Fatalf("ListClusters: %v", err)
	}

	registry, err := cluster.NewRegistry(cluster.SourceFake, rows)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	sessions, err := auth.NewSessionManager(st, secret, false)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("pvmss-local-admin"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	return httpapi.NewAuthWithRegistry(registry, st, sessions, string(hash), slog.Default())
}

func secondaryAliceCookie(t *testing.T, authHandler *httpapi.Auth) *http.Cookie {
	t.Helper()

	login := loginRequest(t, authHandler, `{"username":"alice","password":"pvmss-alice","cluster":"secondary"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("alice login status = %d: %s", login.Code, login.Body.String())
	}

	return login.Result().Cookies()[0]
}

func scopedAdminCookie(t *testing.T, authHandler *httpapi.Auth) *http.Cookie {
	t.Helper()

	login := serveJSON(authHandler.AdminLogin, "/api/v1/auth/admin-login", `{"password":"pvmss-local-admin"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("admin login status = %d: %s", login.Code, login.Body.String())
	}

	return login.Result().Cookies()[0]
}

// newScopedInventoryRegistry returns a two-cluster inventory registry whose
// workers wrap the given stub clients, so a manual refresh leaves a Snapshot
// call count behind.
func newScopedInventoryRegistry(t *testing.T, clients map[string]*stubClusterClient) (*inventory.Registry, cluster.ClientProvider) {
	t.Helper()

	rows := make([]store.ClusterRow, 0, len(clients))
	for name := range clients {
		rows = append(rows, store.ClusterRow{Name: name})
	}

	clusterRegistry, err := cluster.NewRegistryWithFactory(func(row store.ClusterRow) (cluster.Client, error) {
		client, ok := clients[row.Name]
		if !ok {
			return nil, cluster.ErrClusterNotFound
		}

		return client, nil
	}, rows)
	if err != nil {
		t.Fatalf("NewRegistryWithFactory: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(testWriter{t}, nil))

	return inventory.NewRegistry(clusterRegistry, time.Hour, logger), clusterRegistry
}

// waitForIndex polls until the named cluster's projection is populated - the
// atomic store establishes happens-before with the worker goroutine, so
// reading stub call counts after this returns is race-free.
func waitForIndex(t *testing.T, registry *inventory.Registry, name string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if index, _ := registry.Index(name); index != nil {
			return
		}

		time.Sleep(5 * time.Millisecond)
	}

	t.Fatalf("index for %q did not populate within timeout", name)
}

func nodesRequest(t *testing.T, handler *httpapi.ClusterNodes, cookie *http.Cookie, target string) (int, nodesTestResponse) {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.AddCookie(cookie)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	var got nodesTestResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode nodes response: %v", err)
	}

	return response.Code, got
}

//nolint:paralleltest // serial: shared fake auth fixture
func TestClusterNodes_ScopedToIdentityCluster(t *testing.T) {
	defaultIndex := inventory.BuildIndexForCluster(auditTestCluster, cluster.Snapshot{Nodes: []cluster.Node{{Name: scopeDefaultNode, Status: cluster.NodeOnline}}})
	secondaryIndex := inventory.BuildIndexForCluster(crossSecondaryCluster, cluster.Snapshot{Nodes: []cluster.Node{{Name: scopeSecondaryNode, Status: cluster.NodeOnline}}})
	registry := inventory.NewRegistryFromIndexes(map[string]*inventory.Index{
		auditTestCluster:      &defaultIndex,
		crossSecondaryCluster: &secondaryIndex,
	})

	authHandler := newScopedAuthFixture(t)
	handler := httpapi.NewClusterNodesWithRegistry(registry, authHandler, slog.Default())
	alice := secondaryAliceCookie(t, authHandler)

	for _, target := range []string{"/api/v1/cluster/nodes", "/api/v1/cluster/nodes?cluster=default"} {
		code, got := nodesRequest(t, handler, alice, target)
		if code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200", target, code)
		}

		if len(got.Nodes) != 1 || got.Nodes[0].Name != scopeSecondaryNode || got.Nodes[0].Cluster != crossSecondaryCluster {
			t.Fatalf("GET %s nodes = %+v, want only secondary-node on secondary", target, got.Nodes)
		}
	}
}

//nolint:paralleltest // serial: shared fake auth fixture
func TestClusterNodes_AdminUnionAndParam(t *testing.T) {
	defaultIndex := inventory.BuildIndexForCluster(auditTestCluster, cluster.Snapshot{Nodes: []cluster.Node{{Name: scopeDefaultNode, Status: cluster.NodeOnline}}})
	secondaryIndex := inventory.BuildIndexForCluster(crossSecondaryCluster, cluster.Snapshot{Nodes: []cluster.Node{{Name: scopeSecondaryNode, Status: cluster.NodeOnline}}})
	registry := inventory.NewRegistryFromIndexes(map[string]*inventory.Index{
		auditTestCluster:      &defaultIndex,
		crossSecondaryCluster: &secondaryIndex,
	})

	authHandler := newScopedAuthFixture(t)
	handler := httpapi.NewClusterNodesWithRegistry(registry, authHandler, slog.Default())
	admin := scopedAdminCookie(t, authHandler)

	code, got := nodesRequest(t, handler, admin, "/api/v1/cluster/nodes")
	if code != http.StatusOK {
		t.Fatalf("union status = %d, want 200", code)
	}

	clusters := make(map[string]string, len(got.Nodes))
	for _, node := range got.Nodes {
		clusters[node.Name] = node.Cluster
	}

	if clusters[scopeDefaultNode] != auditTestCluster || clusters[scopeSecondaryNode] != crossSecondaryCluster {
		t.Fatalf("union nodes = %+v, want one node per cluster", got.Nodes)
	}

	code, got = nodesRequest(t, handler, admin, "/api/v1/cluster/nodes?cluster=secondary")
	if code != http.StatusOK {
		t.Fatalf("param status = %d, want 200", code)
	}

	if len(got.Nodes) != 1 || got.Nodes[0].Cluster != crossSecondaryCluster {
		t.Fatalf("?cluster=secondary nodes = %+v, want only secondary", got.Nodes)
	}
}

//nolint:paralleltest // serial: shared fake auth fixture
func TestClusterRefresh_ScopedToIdentityCluster(t *testing.T) {
	defaultClient := &stubClusterClient{snapshot: cluster.Snapshot{Nodes: []cluster.Node{{Name: scopeDefaultNode}}}}
	secondaryClient := &stubClusterClient{snapshot: cluster.Snapshot{Nodes: []cluster.Node{{Name: scopeSecondaryNode}}}}
	registry, clusters := newScopedInventoryRegistry(t, map[string]*stubClusterClient{
		auditTestCluster:      defaultClient,
		crossSecondaryCluster: secondaryClient,
	})

	authHandler := newScopedAuthFixture(t)
	handler := httpapi.NewClusterRefreshWithRegistry(registry, clusters, authHandler, slog.Default())
	alice := secondaryAliceCookie(t, authHandler)

	for _, target := range []string{"/api/v1/cluster/refresh", "/api/v1/cluster/refresh?cluster=default"} {
		request := httptest.NewRequest(http.MethodPost, target, nil)
		request.AddCookie(alice)

		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		// 429 is also correct: the first async refresh may have populated the
		// index before the second request's guard check runs.
		if response.Code != http.StatusAccepted && response.Code != http.StatusTooManyRequests {
			t.Fatalf("POST %s status = %d, want 202 or 429", target, response.Code)
		}
	}

	waitForIndex(t, registry, crossSecondaryCluster)

	if secondaryClient.calls == 0 {
		t.Fatal("secondary client calls = 0, want at least 1")
	}

	if defaultClient.calls != 0 {
		t.Fatalf("default client calls = %d, want 0 - a scoped user must not refresh another cluster", defaultClient.calls)
	}
}

//nolint:paralleltest // serial: shared fake auth fixture
func TestClusterRefresh_AdminAllAndParam(t *testing.T) {
	defaultClient := &stubClusterClient{snapshot: cluster.Snapshot{Nodes: []cluster.Node{{Name: scopeDefaultNode}}}}
	secondaryClient := &stubClusterClient{snapshot: cluster.Snapshot{Nodes: []cluster.Node{{Name: scopeSecondaryNode}}}}
	registry, clusters := newScopedInventoryRegistry(t, map[string]*stubClusterClient{
		auditTestCluster:      defaultClient,
		crossSecondaryCluster: secondaryClient,
	})

	authHandler := newScopedAuthFixture(t)
	handler := httpapi.NewClusterRefreshWithRegistry(registry, clusters, authHandler, slog.Default())
	admin := scopedAdminCookie(t, authHandler)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/refresh?cluster=secondary", nil)
	request.AddCookie(admin)
	handler.ServeHTTP(httptest.NewRecorder(), request)
	waitForIndex(t, registry, crossSecondaryCluster)

	if secondaryClient.calls != 1 || defaultClient.calls != 0 {
		t.Fatalf("?cluster=secondary calls = default:%d secondary:%d, want 0/1", defaultClient.calls, secondaryClient.calls)
	}

	// Admin without a parameter refreshes every cluster. The one already
	// refreshed reports too_soon (the guard still applies per cluster) while
	// the untouched one is accepted.
	request = httptest.NewRequest(http.MethodPost, "/api/v1/cluster/refresh", nil)
	request.AddCookie(admin)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("refresh-all status = %d, want 202", response.Code)
	}

	var got struct {
		Clusters map[string]string `json:"clusters"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode refresh-all: %v", err)
	}

	waitForIndex(t, registry, auditTestCluster)

	if defaultClient.calls != 1 {
		t.Fatalf("default client calls = %d, want 1", defaultClient.calls)
	}

	if got.Clusters[auditTestCluster] != "accepted" || got.Clusters[crossSecondaryCluster] != "too_soon" {
		t.Fatalf("outcomes = %+v, want default accepted, secondary too_soon", got.Clusters)
	}
}
