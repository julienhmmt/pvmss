package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/store"
	"testing"
)

// adminISODTOWithMissing mirrors adminISODTO plus the missing field the
// orphan-surfacing change added.
type adminISODTOWithMissing struct {
	Storage   string `json:"storage"`
	Node      string `json:"node"`
	File      string `json:"file"`
	SizeBytes int64  `json:"sizeBytes"`
	Enabled   bool   `json:"enabled"`
	Missing   bool   `json:"missing"`
}

// emptyDiscoveryListClient reports no nodes, storages, bridges, or ISOs so
// every stored approval is an orphan. It embeds cluster.Fake so Snapshot
// returns a valid (empty-overridden) snapshot.
type emptyDiscoveryListClient struct {
	cluster.Fake
}

func (emptyDiscoveryListClient) Snapshot(_ context.Context) (cluster.Snapshot, error) {
	return cluster.Snapshot{}, nil
}

func (emptyDiscoveryListClient) ListBridges(_ context.Context) ([]cluster.Bridge, error) {
	return nil, nil
}

func (emptyDiscoveryListClient) ListISOs(_ context.Context) ([]cluster.ISOImage, error) {
	return nil, nil
}

// newAdminHandlerWithEmptyDiscovery builds an AdminCatalog handler backed by a
// client that reports no discovered resources — every stored approval is an
// orphan. Returns the store so tests can seed approvals directly.
func newAdminHandlerWithEmptyDiscovery(t *testing.T) (*httpapi.AdminCatalog, *httpapi.Auth, *store.Store) {
	t.Helper()
	authHandler := newAuthHandler(t)
	st := newAdminStore(t)
	registry := emptyDiscoveryRegistry{client: emptyDiscoveryListClient{}}
	handler := httpapi.NewAdminCatalogWithRegistry(authHandler, st, registry, nil, testLogger(t))
	return handler, authHandler, st
}

// emptyDiscoveryRegistry is a ClientProvider that always returns the same
// empty-discovery client regardless of the cluster name.
type emptyDiscoveryRegistry struct {
	client emptyDiscoveryListClient
}

func (r emptyDiscoveryRegistry) List() []string { return []string{auditTestCluster} }
func (r emptyDiscoveryRegistry) Client(_ string) (cluster.Client, error) {
	return r.client, nil
}

// TestAdminISOs_DeleteOrphan: DELETE /api/v1/admin/isos/{cluster}/{node}/{storage}/{file}
// removes a disabled orphan ISO approval.
func TestAdminISOs_DeleteOrphan(t *testing.T) {
	handler, authHandler, st := newAdminHandlerWithEmptyDiscovery(t)
	cookie := adminCookie(t, authHandler)
	ctx := context.Background()

	if err := st.SetISOEnabled(ctx, "default", "pve-node-01", "local", "ghost.iso", false); err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := adminDelete(t, handler, authHandler, cookie, "/api/v1/admin/isos/default/pve-node-01/local/ghost.iso")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want %d: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}

	rec2 := adminDelete(t, handler, authHandler, cookie, "/api/v1/admin/isos/default/pve-node-01/local/ghost.iso")
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("second delete status = %d, want %d", rec2.Code, http.StatusNotFound)
	}
}

// TestAdminISOs_DisabledOrphanSurfacedAsMissing: GET /admin/isos surfaces a
// disabled orphan with missing=true.
func TestAdminISOs_DisabledOrphanSurfacedAsMissing(t *testing.T) {
	handler, authHandler, st := newAdminHandlerWithEmptyDiscovery(t)
	cookie := adminCookie(t, authHandler)
	ctx := context.Background()

	if err := st.SetISOEnabled(ctx, "default", "pve-node-01", "local", "ghost.iso", false); err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/isos?cluster=default")
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d: %s", rec.Code, rec.Body.String())
	}

	var isos []adminISODTOWithMissing
	if err := json.Unmarshal(rec.Body.Bytes(), &isos); err != nil {
		t.Fatalf("decode: %v", err)
	}

	found := false
	for _, iso := range isos {
		if iso.File == "ghost.iso" && iso.Node == "pve-node-01" {
			found = true
			if !iso.Missing {
				t.Error("disabled orphan ISO should have missing=true")
			}
		}
	}
	if !found {
		t.Fatal("disabled orphan ISO should be surfaced in the list")
	}
}

// TestAdminISOs_EnabledOrphanAutoRemoved: GET /admin/isos auto-removes an
// enabled orphan — it does not appear in the list at all.
func TestAdminISOs_EnabledOrphanAutoRemoved(t *testing.T) {
	handler, authHandler, st := newAdminHandlerWithEmptyDiscovery(t)
	cookie := adminCookie(t, authHandler)
	ctx := context.Background()

	if err := st.SetISOEnabled(ctx, "default", "pve-node-01", "local", "ghost.iso", true); err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/isos?cluster=default")
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d: %s", rec.Code, rec.Body.String())
	}

	var isos []adminISODTOWithMissing
	if err := json.Unmarshal(rec.Body.Bytes(), &isos); err != nil {
		t.Fatalf("decode: %v", err)
	}

	for _, iso := range isos {
		if iso.File == "ghost.iso" {
			t.Errorf("enabled orphan ISO should have been auto-removed, got %+v", iso)
		}
	}
}

// TestAdminNodes_DeleteOrphan: DELETE /api/v1/admin/nodes/{cluster}/{name}
// removes a disabled orphan node approval.
func TestAdminNodes_DeleteOrphan(t *testing.T) {
	handler, authHandler, st := newAdminHandlerWithEmptyDiscovery(t)
	cookie := adminCookie(t, authHandler)
	ctx := context.Background()

	if err := st.SetNodeEnabled(ctx, "default", "ghost-node", false); err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := adminDelete(t, handler, authHandler, cookie, "/api/v1/admin/nodes/default/ghost-node")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want %d: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}

	rec2 := adminDelete(t, handler, authHandler, cookie, "/api/v1/admin/nodes/default/ghost-node")
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("second delete status = %d, want %d", rec2.Code, http.StatusNotFound)
	}
}

// TestAdminStorages_DeleteOrphan: DELETE removes a disabled orphan storage.
func TestAdminStorages_DeleteOrphan(t *testing.T) {
	handler, authHandler, st := newAdminHandlerWithEmptyDiscovery(t)
	cookie := adminCookie(t, authHandler)
	ctx := context.Background()

	if err := st.SetStorageEnabled(ctx, "default", "ghost-storage", "ghost-node", false); err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := adminDelete(t, handler, authHandler, cookie, "/api/v1/admin/storages/default/ghost-node/ghost-storage")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want %d: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
}

// TestAdminBridges_DeleteOrphan: DELETE removes a disabled orphan bridge.
func TestAdminBridges_DeleteOrphan(t *testing.T) {
	handler, authHandler, st := newAdminHandlerWithEmptyDiscovery(t)
	cookie := adminCookie(t, authHandler)
	ctx := context.Background()

	if err := st.SetBridgeEnabled(ctx, "default", "ghost-node", "ghost-bridge", false); err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := adminDelete(t, handler, authHandler, cookie, "/api/v1/admin/bridges/default/ghost-node/ghost-bridge")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want %d: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
}
