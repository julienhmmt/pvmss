package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/store"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

type adminNodeDTO struct {
	Name         string  `json:"name"`
	Status       string  `json:"status"`
	CPUCores     int     `json:"cpuCores"`
	CPUUsage     float64 `json:"cpuUsage"`
	MemoryTotal  int64   `json:"memoryTotal"`
	MemoryUsed   int64   `json:"memoryUsed"`
	StorageTotal int64   `json:"storageTotal"`
	StorageUsed  int64   `json:"storageUsed"`
	VMCount      int     `json:"vmCount"`
	Enabled      bool    `json:"enabled"`
}

type adminToggleResponse struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// TestAdminNodes_ListAsAdmin_ReturnsAllNodes — T010: GET /admin/nodes as admin
// returns every fake node (3), with correct enabled per T06's seed.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminNodes_ListAsAdmin_ReturnsAllNodes(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/nodes?cluster=default")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var nodes []adminNodeDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &nodes); err != nil {
		t.Fatalf("decode nodes: %v", err)
	}

	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(nodes))
	}

	enabledByName := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		enabledByName[n.Name] = n.Enabled
	}

	if !enabledByName["pve-node-01"] {
		t.Error("pve-node-01 should be enabled")
	}

	if !enabledByName["pve-node-02"] {
		t.Error("pve-node-02 should be enabled")
	}

	if enabledByName["pve-node-03"] {
		t.Error("pve-node-03 should not be enabled")
	}
}

// TestAdminNodes_ListAsNonAdmin_Returns403 — T011: GET /admin/nodes as a
// non-admin identity returns 403 (FR-008).
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminNodes_ListAsNonAdmin_Returns403(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	aliceCookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	rec := adminGet(t, handler, authHandler, aliceCookie, "/api/v1/admin/nodes?cluster=default")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// TestAdminNodes_ToggleUnapprovedNode — T012: POST /admin/nodes/toggle on the
// unapproved node returns 200, and a subsequent GET reflects it.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminNodes_ToggleUnapprovedNode(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/nodes/toggle",
		`{"cluster":"default","name":"pve-node-03","enabled":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var toggleResp adminToggleResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &toggleResp); err != nil {
		t.Fatalf("decode toggle response: %v", err)
	}

	if toggleResp.Name != "pve-node-03" || !toggleResp.Enabled {
		t.Fatalf("toggle response = %+v", toggleResp)
	}

	// Subsequent GET reflects the change.
	list := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/nodes?cluster=default")
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d", list.Code)
	}

	var nodes []adminNodeDTO
	if err := json.Unmarshal(list.Body.Bytes(), &nodes); err != nil {
		t.Fatalf("decode nodes: %v", err)
	}

	for _, n := range nodes {
		if n.Name == "pve-node-03" && !n.Enabled {
			t.Error("pve-node-03 should be enabled after toggle")
		}
	}
}

// TestAdminNodes_ToggleUnknownNode_Returns404 — T013: toggling a node not in
// the discovery set returns 404.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminNodes_ToggleUnknownNode_Returns404(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/nodes/toggle",
		`{"cluster":"default","name":"pve-node-99","enabled":true}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

// newMultiClusterAdminCatalogHandler builds an AdminCatalog handler backed by
// a real cluster.Registry seeded with the store's default/secondary/
// offline-demo rows (T15), instead of the single fixed cluster.Fake the
// single-cluster newAdminHandler above uses — needed to prove SC-004's
// cross-cluster catalog isolation, which requires two distinct clusters'
// discovery sets and two distinct catalog_nodes rows to exist at once.
func newMultiClusterAdminCatalogHandler(t *testing.T) (*httpapi.AdminCatalog, *httpapi.Auth) {
	t.Helper()
	t.Cleanup(cluster.ResetFake)

	const secret = "admin-catalog-cross-cluster-secret-32b" //nolint:gosec // deterministic test secret
	st, err := store.Open(config.Configuration{DBPath: filepath.Join(t.TempDir(), "catalog-cross-cluster.db"), ClusterSource: "fake", SessionSecret: secret})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	rows, err := st.ListClusters(context.Background())
	if err != nil {
		t.Fatalf("ListClusters: %v", err)
	}
	registry, err := cluster.NewRegistry("fake", rows)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	sessions, err := auth.NewSessionManager(st, secret, false)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("pvmss-local-admin"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))
	authHandler := httpapi.NewAuthWithRegistry(registry, st, sessions, string(hash), auth.NewTokenService(st), logger)
	catalog := httpapi.NewAdminCatalogWithRegistry(authHandler, st, registry, nil, logger)

	return catalog, authHandler
}

// TestAdminNodes_CrossClusterApprovalIsolation — T033/SC-004: a node with an
// identical name (pve-node-01, present in both default's and secondary's
// fake discovery sets) is approved independently per cluster. Toggling it on
// one cluster must never affect the other's row — proven in both directions,
// not just observed as an accident of seed data (default's pve-node-01/02
// start pre-approved by T06's seed; secondary's do not).
//
//nolint:paralleltest // shared fake dataset and database fixture
func TestAdminNodes_CrossClusterApprovalIsolation(t *testing.T) {
	handler, authHandler := newMultiClusterAdminCatalogHandler(t)
	cookie := adminCookie(t, authHandler)

	nodeEnabled := func(cluster, name string) bool {
		t.Helper()
		rec := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/nodes?cluster="+cluster)
		if rec.Code != http.StatusOK {
			t.Fatalf("list cluster=%s status = %d: %s", cluster, rec.Code, rec.Body.String())
		}
		var nodes []adminNodeDTO
		if err := json.Unmarshal(rec.Body.Bytes(), &nodes); err != nil {
			t.Fatalf("decode nodes: %v", err)
		}
		for _, n := range nodes {
			if n.Name == name {
				return n.Enabled
			}
		}
		t.Fatalf("node %q not reported by cluster=%s", name, cluster)
		return false
	}

	// Starting state, from T06's seed (default only) plus the absence of any
	// secondary row: identical name, already-diverging approval state.
	if !nodeEnabled("default", "pve-node-01") {
		t.Fatal("default:pve-node-01 should start enabled (T06 seed)")
	}
	if nodeEnabled("secondary", "pve-node-01") {
		t.Fatal("secondary:pve-node-01 should start disabled — cluster isolation, not shared with default's seed")
	}

	// Explicitly approve pve-node-01 for secondary only.
	toggle := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/nodes/toggle",
		`{"cluster":"secondary","name":"pve-node-01","enabled":true}`)
	if toggle.Code != http.StatusOK {
		t.Fatalf("toggle secondary:pve-node-01 status = %d: %s", toggle.Code, toggle.Body.String())
	}
	if !nodeEnabled("secondary", "pve-node-01") {
		t.Fatal("secondary:pve-node-01 should be enabled after its own toggle")
	}
	if !nodeEnabled("default", "pve-node-01") {
		t.Fatal("default:pve-node-01 should remain enabled — secondary's toggle must not touch it")
	}

	// Reverse direction: explicitly revoke pve-node-01 on default only.
	revoke := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/nodes/toggle",
		`{"cluster":"default","name":"pve-node-01","enabled":false}`)
	if revoke.Code != http.StatusOK {
		t.Fatalf("revoke default:pve-node-01 status = %d: %s", revoke.Code, revoke.Body.String())
	}
	if nodeEnabled("default", "pve-node-01") {
		t.Fatal("default:pve-node-01 should be disabled after its own revoke")
	}
	if !nodeEnabled("secondary", "pve-node-01") {
		t.Fatal("secondary:pve-node-01 should remain enabled — default's revoke must not touch it")
	}
}

// TestAdminNodes_ClusterRequiredOnceMultipleConfigured — T032/FR-021 at the
// nodes-endpoint level: once 2+ clusters are configured, omitting ?cluster=
// on GET /admin/nodes returns 400 cluster_required rather than silently
// defaulting to an arbitrary cluster.
//
//nolint:paralleltest // shared fake dataset and database fixture
func TestAdminNodes_ClusterRequiredOnceMultipleConfigured(t *testing.T) {
	handler, authHandler := newMultiClusterAdminCatalogHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/nodes")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body.Code != "cluster_required" {
		t.Fatalf("error code = %q, want cluster_required", body.Code)
	}
}
