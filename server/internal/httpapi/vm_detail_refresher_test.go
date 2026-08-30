//nolint:wsl_v5 // cross-cluster HTTP assertions stay in one scenario
package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"strings"
	"testing"
	"time"
)

// refresherSecondaryCluster is the non-default cluster name used in the
// per-cluster refresher tests.
const refresherSecondaryCluster = "secondary"

// newTwoClusterRefresherFixture builds a VMDetail handler wired to a
// two-cluster inventory registry (default + secondary), each with its own
// worker. It returns the handler, the auth cookie for alice, the inventory
// registry (so tests can read each cluster's projection), and the default
// cluster's worker (used as the fallback Refresher — same wiring as main.go).
func newTwoClusterRefresherFixture(t *testing.T) (*httpapi.VMDetail, *http.Cookie, *inventory.Registry) {
	t.Helper()
	t.Cleanup(cluster.ResetFake)
	cluster.ResetFake()

	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))

	clusterRows := []store.ClusterRow{
		{Name: auditTestCluster},
		{Name: refresherSecondaryCluster},
	}
	clusterRegistry, err := cluster.NewRegistry("fake", clusterRows)
	if err != nil {
		t.Fatalf("cluster.NewRegistry: %v", err)
	}

	inventoryRegistry := inventory.NewRegistry(clusterRegistry, time.Hour, logger)
	for _, name := range []string{auditTestCluster, refresherSecondaryCluster} {
		if _, err := inventoryRegistry.Refresh(context.Background(), name); err != nil {
			t.Fatalf("initial Refresh(%s): %v", name, err)
		}
	}

	defaultProjection, err := inventoryRegistry.Projection(auditTestCluster)
	if err != nil {
		t.Fatalf("Projection(default): %v", err)
	}

	st, err := store.Open(config.Configuration{
		Port:      50001,
		DBPath:    filepath.Join(t.TempDir(), "refresher.db"),
		LogLevel:  snapshotTestLogLevel,
		LogFormat: snapshotTestLogFormat,
		LogOutput: snapshotTestLogOutput,
	})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authHandler := newAuthHandler(t)

	defaultWorker, err := inventoryRegistry.Worker(auditTestCluster)
	if err != nil {
		t.Fatalf("Worker(default): %v", err)
	}

	handler := httpapi.NewVMDetailWithRegistry(httpapi.VMDetailDeps{
		Source:     inventoryRegistry,
		Projection: defaultProjection,
		Auth:       authHandler,
		Writer:     cluster.Fake{},
		Clients:    clusterRegistry,
		Store:      st,
		Refresher:  defaultWorker,
		Log:        logger,
	})

	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	return handler, cookie, inventoryRegistry
}

// projectionRefreshedAt returns the RefreshedAt timestamp of the named
// cluster's projection, failing the test if the projection or index is nil.
func projectionRefreshedAt(t *testing.T, registry *inventory.Registry, clusterName string) time.Time {
	t.Helper()

	projection, err := registry.Projection(clusterName)
	if err != nil {
		t.Fatalf("Projection(%s): %v", clusterName, err)
	}

	index := projection.Load()
	if index == nil {
		t.Fatalf("index for %s is nil", clusterName)
	}

	return index.RefreshedAt
}

// TestVMDetail_ActionOnSecondaryClusterRefreshesOnlySecondaryProjection is the
// acceptance test for ticket 02: an action on a non-default cluster must
// refresh that cluster's projection, not the default cluster's. Before the
// fix, the handler used the default cluster's worker for every cluster, so
// the default projection was refreshed instead.
//
//nolint:paralleltest // serial: shared fake cluster and database fixtures
func TestVMDetail_ActionOnSecondaryClusterRefreshesOnlySecondaryProjection(t *testing.T) {
	handler, cookie, inventoryRegistry := newTwoClusterRefresherFixture(t)

	defaultBefore := projectionRefreshedAt(t, inventoryRegistry, auditTestCluster)
	secondaryBefore := projectionRefreshedAt(t, inventoryRegistry, refresherSecondaryCluster)

	// VM 101 is stopped in both clusters, owned by alice (pool-alice, tagged pvmss).
	// Start it on the secondary cluster.
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/vms/"+refresherSecondaryCluster+"/101/actions", strings.NewReader(`{"action":"start"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	req.SetPathValue("cluster", refresherSecondaryCluster)
	req.SetPathValue("vmid", "101")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("action on secondary: status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	defaultAfter := projectionRefreshedAt(t, inventoryRegistry, auditTestCluster)
	secondaryAfter := projectionRefreshedAt(t, inventoryRegistry, refresherSecondaryCluster)

	// The secondary projection must have been refreshed.
	if !secondaryAfter.After(secondaryBefore) {
		t.Errorf("secondary projection RefreshedAt not updated: before=%s after=%s (the target cluster's projection must be refreshed after an action on it)", secondaryBefore, secondaryAfter)
	}

	// The default projection must NOT have been refreshed.
	if defaultAfter.After(defaultBefore) {
		t.Errorf("default projection RefreshedAt was updated: before=%s after=%s (a non-default-cluster action must not refresh the default cluster's projection)", defaultBefore, defaultAfter)
	}
}

// TestVMDetail_SingleClusterFallbackUsesBoundRefresher is the non-regression
// test: when deps.Source is not a *inventory.Registry (single-cluster mode),
// the fallback refresher is used for every cluster name.
//
//nolint:paralleltest // serial: shared fake cluster and database fixtures
func TestVMDetail_SingleClusterFallbackUsesBoundRefresher(t *testing.T) {
	t.Cleanup(cluster.ResetFake)
	cluster.ResetFake()

	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))

	snap, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	index := inventory.BuildIndex(snap)
	index.RefreshedAt = time.Now()
	projection := inventory.NewProjectionFromIndex(&index)

	st, err := store.Open(config.Configuration{
		Port:      50001,
		DBPath:    filepath.Join(t.TempDir(), "single-cluster.db"),
		LogLevel:  snapshotTestLogLevel,
		LogFormat: snapshotTestLogFormat,
		LogOutput: snapshotTestLogOutput,
	})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authHandler := newAuthHandler(t)
	worker := inventory.NewWorker(cluster.Fake{}, projection, time.Hour, logger)

	// No Source (no registry) — single-cluster mode, fallback refresher is the worker.
	handler := httpapi.NewVMDetail(projection, authHandler, cluster.Fake{}, st, worker, logger)

	cookie := aliceCookie(t, authHandler)

	before := projection.Load().RefreshedAt

	// VM 101 is stopped, owned by alice. Start it.
	req := detailRequest(http.MethodPost, "/api/v1/vms/default/101/actions", `{"action":"start"}`, cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	after := projection.Load().RefreshedAt
	if !after.After(before) {
		t.Errorf("fallback refresher was not used: RefreshedAt before=%s after=%s (single-cluster mode must use the bound refresher)", before, after)
	}
}

// TestVMDetail_StatusOnSecondaryClusterResolvesPerClusterReader is the
// regression test for the cross-cluster bug in handleStatus: before the fix,
// the handler used the fallback statusReader (bound to the default cluster)
// instead of resolving the per-cluster reader via Clients. In multi-cluster
// mode where deps.StatusReader is nil (the fallback), the old code returned
// 503 no_status_reader for every cluster; the fix resolves the secondary
// cluster's own client via statusReaderFor.
//
//nolint:paralleltest // serial: shared fake cluster and database fixtures
func TestVMDetail_StatusOnSecondaryClusterResolvesPerClusterReader(t *testing.T) {
	handler, cookie, _ := newTwoClusterRefresherFixture(t)

	// VM 101 is stopped in both clusters, owned by alice. Read live status
	// on the secondary cluster — the handler must resolve the secondary's
	// own VMStatusReader via Clients, not fall back to the default's.
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/vms/"+refresherSecondaryCluster+"/101/status", nil)
	req.AddCookie(cookie)
	req.SetPathValue("cluster", refresherSecondaryCluster)
	req.SetPathValue("vmid", "101")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status on secondary: status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body.Status == "" {
		t.Errorf("live status = empty, want a valid status (per-cluster reader must be resolved, not 503 no_status_reader)")
	}
}
