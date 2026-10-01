//nolint:wsl_v5 // multi-cluster fixture setup keeps sequential assignments and assertions readable
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
	"pvmss/server/internal/inventory"
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

// TestAdminNodes_ListAsAdmin_ReturnsAllNodes - GET /admin/nodes as admin
// returns every fake node, with correct enabled per the seed.
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

//nolint:gocyclo,paralleltest // explicit field assertions document the HTTP contract; the fake fixture is shared
func TestAdminNodeDetails_AsAdmin_ReturnsSelectedNode(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)
	rec := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/nodes/"+auditTestCluster+"/pve-node-01")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var detail struct {
		ClusterKey string `json:"clusterKey"`
		Name       string `json:"name"`
		Health     struct {
			Status   string `json:"status"`
			Stale    bool   `json:"stale"`
			CPUModel string `json:"cpuModel"`
		} `json:"health"`
		Network struct {
			Available  bool `json:"available"`
			Interfaces []struct {
				CIDR string `json:"cidr"`
			} `json:"interfaces"`
		} `json:"network"`
		PCI struct {
			Available bool `json:"available"`
			Devices   []struct {
				DeviceName string `json:"deviceName"`
			} `json:"devices"`
		} `json:"pci"`
		Containers struct {
			Available  bool `json:"available"`
			Containers []struct {
				Name string `json:"name"`
			} `json:"containers"`
		} `json:"containers"`
		Inventory struct {
			VMs      []json.RawMessage `json:"vms"`
			Storages []json.RawMessage `json:"storages"`
		} `json:"inventory"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode node details: %v", err)
	}
	if detail.ClusterKey != auditTestCluster || detail.Name != "pve-node-01" {
		t.Fatalf("node identity = cluster:%q name:%q", detail.ClusterKey, detail.Name)
	}
	if detail.Health.Status != "online" || detail.Health.Stale || detail.Health.CPUModel == "" {
		t.Fatalf("health = %+v", detail.Health)
	}
	if !detail.Network.Available || len(detail.Network.Interfaces) == 0 || detail.Network.Interfaces[0].CIDR == "" {
		t.Fatalf("network = %+v", detail.Network)
	}
	if !detail.PCI.Available || len(detail.PCI.Devices) == 0 || detail.PCI.Devices[0].DeviceName == "" {
		t.Fatalf("PCI = %+v", detail.PCI)
	}
	if !detail.Containers.Available || len(detail.Containers.Containers) == 0 {
		t.Fatalf("containers = %+v", detail.Containers)
	}
	if len(detail.Inventory.VMs) == 0 || len(detail.Inventory.Storages) == 0 {
		t.Fatalf("inventory = %+v", detail.Inventory)
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminNodeDetails_PartialFailureKeepsCachedSections(t *testing.T) {
	t.Cleanup(cluster.ResetFake)
	authHandler := newAuthHandler(t)
	st := newAdminStore(t)
	fake := cluster.Fake{}
	snapshot, _ := fake.Snapshot(context.Background())
	index := inventory.BuildIndex(snapshot)
	projection := inventory.NewProjectionFromIndex(&index)
	client := nodeDetailFailureClient{Fake: fake, failHealth: true, failNetwork: true}
	handler := httpapi.NewAdminCatalog(authHandler, st, client, projection, slog.New(slog.DiscardHandler))
	cookie := adminCookie(t, authHandler)
	rec := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/nodes/"+auditTestCluster+"/pve-node-01")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var detail struct {
		Health struct {
			Stale       bool  `json:"stale"`
			MemoryTotal int64 `json:"memoryTotalBytes"`
		} `json:"health"`
		Network struct {
			Available bool `json:"available"`
		} `json:"network"`
		PCI struct {
			Available bool `json:"available"`
		} `json:"pci"`
		Inventory struct {
			VMs []json.RawMessage `json:"vms"`
		} `json:"inventory"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode node details: %v", err)
	}
	if !detail.Health.Stale || detail.Health.MemoryTotal == 0 || detail.Network.Available || !detail.PCI.Available || len(detail.Inventory.VMs) == 0 {
		t.Fatalf("partial detail = %+v", detail)
	}
}

// TestAdminNodes_ListAsNonAdmin_Returns403 - GET /admin/nodes as a
// non-admin identity returns 403.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminNodes_ListAsNonAdmin_Returns403(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	aliceCookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)
	rec := adminGet(t, handler, authHandler, aliceCookie, "/api/v1/admin/nodes?cluster="+auditTestCluster)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminNodeDetails_AsNonAdmin_Returns403(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	aliceCookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)
	rec := adminGet(t, handler, authHandler, aliceCookie, "/api/v1/admin/nodes/"+auditTestCluster+"/pve-node-01")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

type nodeDetailFailureClient struct {
	cluster.Fake
	failHealth  bool
	failNetwork bool
}

func (client nodeDetailFailureClient) ReadNodeHealth(ctx context.Context, node string) (cluster.NodeHealth, error) {
	if client.failHealth {
		return cluster.NodeHealth{}, cluster.ErrUnreachable
	}
	return client.Fake.ReadNodeHealth(ctx, node)
}

func (client nodeDetailFailureClient) ReadNodeNetwork(ctx context.Context, node string) ([]cluster.NodeNetworkInterface, error) {
	if client.failNetwork {
		return nil, cluster.ErrUnreachable
	}
	return client.Fake.ReadNodeNetwork(ctx, node)
}

// TestAdminNodes_ToggleUnapprovedNode - POST /admin/nodes/toggle on the
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

// TestAdminNodes_ToggleUnknownNode_Returns404 - toggling a node not in
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
// offline-demo rows, instead of the single fixed cluster.Fake the
// single-cluster newAdminHandler above uses - needed to prove the cross-cluster catalog
// isolation, which requires two distinct clusters'
// discovery sets and two distinct catalog_nodes rows to exist at once.
func newMultiClusterAdminCatalogHandler(t *testing.T) (*httpapi.AdminCatalog, *httpapi.Auth) {
	t.Helper()
	t.Cleanup(cluster.ResetFake)

	const secret = "admin-catalog-cross-cluster-secret-32b" // deterministic test secret
	st, err := store.Open(config.Configuration{DBPath: filepath.Join(t.TempDir(), "catalog-cross-cluster.db"), ClusterSource: cluster.SourceFake, SessionSecret: secret})
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
	authHandler := httpapi.NewAuthWithRegistry(registry, st, sessions, string(hash), logger)
	catalog := httpapi.NewAdminCatalogWithRegistry(authHandler, st, registry, nil, logger)

	return catalog, authHandler
}

// TestAdminNodes_CrossClusterApprovalIsolation - a node with an
// identical name (pve-node-01, present in both default's and secondary's
// fake discovery sets) is approved independently per cluster. Toggling it on
// one cluster must never affect the other's row - proven in both directions,
// not just observed as an accident of seed data (default's pve-node-01/02
// start pre-approved by the seed; secondary's do not).
//
//nolint:paralleltest // shared fake dataset and database fixture
func TestAdminNodes_CrossClusterApprovalIsolation(t *testing.T) {
	handler, authHandler := newMultiClusterAdminCatalogHandler(t)
	cookie := adminCookie(t, authHandler)

	nodeEnabled := func(clusterName, name string) bool {
		t.Helper()
		rec := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/nodes?cluster="+clusterName)
		if rec.Code != http.StatusOK {
			t.Fatalf("list cluster=%s status = %d: %s", clusterName, rec.Code, rec.Body.String())
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
		t.Fatalf("node %q not reported by cluster=%s", name, clusterName)
		return false
	}

	// Starting state, from the seed (default only) plus the absence of any
	// secondary row: identical name, already-diverging approval state.
	if !nodeEnabled(auditTestCluster, "pve-node-01") {
		t.Fatal("default:pve-node-01 should start enabled (T06 seed)")
	}
	if nodeEnabled(crossSecondaryCluster, "pve-node-01") {
		t.Fatal("secondary:pve-node-01 should start disabled - cluster isolation, not shared with default's seed")
	}

	assertCrossClusterToggleIsolation(t, handler, authHandler, cookie, nodeEnabled)
}

// assertCrossClusterToggleIsolation verifies that toggling a node on one
// cluster does not affect the same node on the other cluster, in both
// directions. Extracted from TestAdminNodes_CrossClusterApprovalIsolation to
// keep its Cognitive Complexity under the SonarQube go:S3776 threshold.
func assertCrossClusterToggleIsolation(
	t *testing.T,
	handler *httpapi.AdminCatalog,
	authHandler *httpapi.Auth,
	cookie *http.Cookie,
	nodeEnabled func(string, string) bool,
) {
	t.Helper()

	// Explicitly approve pve-node-01 for secondary only.
	toggle := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/nodes/toggle",
		`{"cluster":"secondary","name":"pve-node-01","enabled":true}`)
	if toggle.Code != http.StatusOK {
		t.Fatalf("toggle secondary:pve-node-01 status = %d: %s", toggle.Code, toggle.Body.String())
	}
	if !nodeEnabled(crossSecondaryCluster, "pve-node-01") {
		t.Fatal("secondary:pve-node-01 should be enabled after its own toggle")
	}
	if !nodeEnabled(auditTestCluster, "pve-node-01") {
		t.Fatal("default:pve-node-01 should remain enabled - secondary's toggle must not touch it")
	}

	// Reverse direction: explicitly revoke pve-node-01 on default only.
	revoke := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/nodes/toggle",
		`{"cluster":"default","name":"pve-node-01","enabled":false}`)
	if revoke.Code != http.StatusOK {
		t.Fatalf("revoke default:pve-node-01 status = %d: %s", revoke.Code, revoke.Body.String())
	}
	if nodeEnabled(auditTestCluster, "pve-node-01") {
		t.Fatal("default:pve-node-01 should be disabled after its own revoke")
	}
	if !nodeEnabled(crossSecondaryCluster, "pve-node-01") {
		t.Fatal("secondary:pve-node-01 should remain enabled - default's revoke must not touch it")
	}
}

// TestAdminNodes_ClusterRequiredOnceMultipleConfigured - at the
//
//	nodes-endpoint level: once 2+ clusters are configured, omitting ?cluster=
//
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
	if body.Code != apiCodeClusterRequired {
		t.Fatalf("error code = %q, want cluster_required", body.Code)
	}
}
