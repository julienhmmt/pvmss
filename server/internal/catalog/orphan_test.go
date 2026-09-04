package catalog_test

import (
	"context"
	"errors"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"testing"
)

// emptyDiscoveryClient reports no nodes, storages, bridges, or ISOs — every
// stored approval is an orphan. It embeds cluster.Fake so Snapshot returns a
// valid (empty-overridden) snapshot for the storage path.
type emptyDiscoveryClient struct {
	cluster.Fake
}

func (emptyDiscoveryClient) Snapshot(_ context.Context) (cluster.Snapshot, error) {
	return cluster.Snapshot{}, nil
}

func (emptyDiscoveryClient) ListBridges(_ context.Context) ([]cluster.Bridge, error) {
	return nil, nil
}

func (emptyDiscoveryClient) ListISOs(_ context.Context) ([]cluster.ISOImage, error) {
	return nil, nil
}

// TestAdminListISOs_EnabledOrphanAutoRemoved: an enabled ISO approval whose
// file Proxmox no longer reports is silently removed — it would otherwise be
// offered to users on a file that no longer exists.
func TestAdminListISOs_EnabledOrphanAutoRemoved(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	// Approve an ISO that the fake cluster reports, then switch to a client
	// that reports nothing — the approval becomes an enabled orphan.
	if err := catalog.SetISOEnabled(ctx, st, cluster.Fake{}, "default",
		catalog.ISORef{Node: node01, Storage: storageLocal, File: debianGenericISO}, true); err != nil {
		t.Fatalf("SetISOEnabled: %v", err)
	}

	isos, err := catalog.AdminListISOs(ctx, st, emptyDiscoveryClient{}, "default")
	if err != nil {
		t.Fatalf("AdminListISOs: %v", err)
	}

	for _, iso := range isos {
		if iso.File == debianGenericISO {
			t.Errorf("enabled orphan ISO %q should have been auto-removed, got %+v", debianGenericISO, iso)
		}
	}

	// The row must be gone from the store too.
	rows, err := st.CatalogISOsEnabled(ctx, "default")
	if err != nil {
		t.Fatalf("CatalogISOsEnabled: %v", err)
	}
	for _, row := range rows {
		if row.File == debianGenericISO {
			t.Errorf("enabled orphan ISO %q should have been deleted from the store", debianGenericISO)
		}
	}
}

// TestAdminListISOs_DisabledOrphanSurfacedAsMissing: a disabled ISO approval
// whose file is gone is surfaced with Missing=true so the admin can remove it
// manually — auto-removing a disabled row would lose the admin's intent if the
// file comes back.
func TestAdminListISOs_DisabledOrphanSurfacedAsMissing(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	// Approve then disable — the row persists with enabled=false.
	if err := catalog.SetISOEnabled(ctx, st, cluster.Fake{}, "default",
		catalog.ISORef{Node: node01, Storage: storageLocal, File: debianGenericISO}, true); err != nil {
		t.Fatalf("SetISOEnabled true: %v", err)
	}
	if err := st.SetISOEnabled(ctx, "default", node01, storageLocal, debianGenericISO, false); err != nil {
		t.Fatalf("SetISOEnabled false: %v", err)
	}

	isos, err := catalog.AdminListISOs(ctx, st, emptyDiscoveryClient{}, "default")
	if err != nil {
		t.Fatalf("AdminListISOs: %v", err)
	}

	found := false
	for _, iso := range isos {
		if iso.File == debianGenericISO && iso.Node == node01 {
			found = true
			if !iso.Missing {
				t.Error("disabled orphan ISO should have Missing=true")
			}
			if iso.Enabled {
				t.Error("disabled orphan ISO should still be disabled")
			}
		}
	}
	if !found {
		t.Fatalf("disabled orphan ISO %q should be surfaced as missing", debianGenericISO)
	}
}

// TestDeleteISO_RemovesOrphan: the admin can remove a disabled orphan approval.
func TestDeleteISO_RemovesOrphan(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	if err := st.SetISOEnabled(ctx, "default", node01, storageLocal, debianGenericISO, false); err != nil {
		t.Fatalf("seed disabled ISO: %v", err)
	}

	if err := catalog.DeleteISO(ctx, st, "default", node01, storageLocal, debianGenericISO); err != nil {
		t.Fatalf("DeleteISO: %v", err)
	}

	if err := catalog.DeleteISO(ctx, st, "default", node01, storageLocal, debianGenericISO); !errors.Is(err, catalog.ErrISONotFound) {
		t.Fatalf("second DeleteISO: got %v, want ErrISONotFound", err)
	}
}

// TestAdminListNodes_EnabledOrphanAutoRemoved: an enabled node approval whose
// node Proxmox no longer reports is auto-removed.
func TestAdminListNodes_EnabledOrphanAutoRemoved(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	// pve-node-01 is seeded enabled. With empty discovery it becomes an
	// enabled orphan and should be auto-removed.
	nodes, err := catalog.AdminListNodes(ctx, st, emptyDiscoveryClient{}, "default")
	if err != nil {
		t.Fatalf("AdminListNodes: %v", err)
	}

	for _, n := range nodes {
		if n.Name == "pve-node-01" {
			t.Errorf("enabled orphan node pve-node-01 should have been auto-removed, got %+v", n)
		}
	}

	rows, err := st.CatalogNodesEnabled(ctx, "default")
	if err != nil {
		t.Fatalf("CatalogNodesEnabled: %v", err)
	}
	for _, row := range rows {
		if row.Name == "pve-node-01" && row.Enabled {
			t.Errorf("enabled orphan node pve-node-01 should have been deleted from the store")
		}
	}
}

// TestAdminListNodes_DisabledOrphanSurfacedAsMissing: a disabled node approval
// whose node is gone is surfaced with Missing=true.
func TestAdminListNodes_DisabledOrphanSurfacedAsMissing(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	// Seed a disabled approval for a node that empty discovery does not report.
	if err := st.SetNodeEnabled(ctx, "default", "ghost-node", false); err != nil {
		t.Fatalf("seed disabled node: %v", err)
	}

	nodes, err := catalog.AdminListNodes(ctx, st, emptyDiscoveryClient{}, "default")
	if err != nil {
		t.Fatalf("AdminListNodes: %v", err)
	}

	found := false
	for _, n := range nodes {
		if n.Name == "ghost-node" {
			found = true
			if !n.Missing {
				t.Error("disabled orphan node should have Missing=true")
			}
		}
	}
	if !found {
		t.Fatal("disabled orphan node ghost-node should be surfaced as missing")
	}
}

// TestDeleteNode_RemovesOrphan: the admin can remove a disabled orphan node.
func TestDeleteNode_RemovesOrphan(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	if err := st.SetNodeEnabled(ctx, "default", "ghost-node", false); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := catalog.DeleteNode(ctx, st, "default", "ghost-node"); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}

	if err := catalog.DeleteNode(ctx, st, "default", "ghost-node"); !errors.Is(err, catalog.ErrNodeNotFound) {
		t.Fatalf("second DeleteNode: got %v, want ErrNodeNotFound", err)
	}
}

// TestAdminListStorages_EnabledOrphanAutoRemoved: an enabled storage approval
// whose storage is gone is auto-removed.
func TestAdminListStorages_EnabledOrphanAutoRemoved(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	// local@pve-node-02 is seeded enabled. With empty discovery it becomes an
	// enabled orphan and should be auto-removed.
	storages, err := catalog.AdminListStorages(ctx, st, emptyDiscoveryClient{}, "default")
	if err != nil {
		t.Fatalf("AdminListStorages: %v", err)
	}

	for _, s := range storages {
		if s.Name == "local" && s.Node == "pve-node-02" {
			t.Errorf("enabled orphan storage local@pve-node-02 should have been auto-removed, got %+v", s)
		}
	}
}

// TestAdminListStorages_DisabledOrphanSurfacedAsMissing: a disabled storage
// approval whose storage is gone is surfaced with Missing=true.
func TestAdminListStorages_DisabledOrphanSurfacedAsMissing(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	if err := st.SetStorageEnabled(ctx, "default", "ghost-storage", "ghost-node", false); err != nil {
		t.Fatalf("seed: %v", err)
	}

	storages, err := catalog.AdminListStorages(ctx, st, emptyDiscoveryClient{}, "default")
	if err != nil {
		t.Fatalf("AdminListStorages: %v", err)
	}

	found := false
	for _, s := range storages {
		if s.Name == "ghost-storage" && s.Node == "ghost-node" {
			found = true
			if !s.Missing {
				t.Error("disabled orphan storage should have Missing=true")
			}
		}
	}
	if !found {
		t.Fatal("disabled orphan storage should be surfaced as missing")
	}
}

// TestDeleteStorage_RemovesOrphan: the admin can remove a disabled orphan storage.
func TestDeleteStorage_RemovesOrphan(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	if err := st.SetStorageEnabled(ctx, "default", "ghost-storage", "ghost-node", false); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := catalog.DeleteStorage(ctx, st, "default", "ghost-storage", "ghost-node"); err != nil {
		t.Fatalf("DeleteStorage: %v", err)
	}

	if err := catalog.DeleteStorage(ctx, st, "default", "ghost-storage", "ghost-node"); !errors.Is(err, catalog.ErrStorageNotFound) {
		t.Fatalf("second DeleteStorage: got %v, want ErrStorageNotFound", err)
	}
}

// TestAdminListBridges_EnabledOrphanAutoRemoved: an enabled bridge approval
// whose bridge is gone is auto-removed.
func TestAdminListBridges_EnabledOrphanAutoRemoved(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	// Approve a bridge the fake reports, then switch to empty discovery.
	if err := catalog.SetBridgeEnabled(ctx, st, cluster.Fake{}, "default", node01, bridgeVMbr0, true); err != nil {
		t.Fatalf("SetBridgeEnabled: %v", err)
	}

	bridges, err := catalog.AdminListBridges(ctx, st, emptyDiscoveryClient{}, "default")
	if err != nil {
		t.Fatalf("AdminListBridges: %v", err)
	}

	for _, b := range bridges {
		if b.Name == bridgeVMbr0 && b.Node == node01 {
			t.Errorf("enabled orphan bridge vmbr0@pve-node-01 should have been auto-removed, got %+v", b)
		}
	}
}

// TestAdminListBridges_DisabledOrphanSurfacedAsMissing: a disabled bridge
// approval whose bridge is gone is surfaced with Missing=true.
func TestAdminListBridges_DisabledOrphanSurfacedAsMissing(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	if err := st.SetBridgeEnabled(ctx, "default", "ghost-node", "ghost-bridge", false); err != nil {
		t.Fatalf("seed: %v", err)
	}

	bridges, err := catalog.AdminListBridges(ctx, st, emptyDiscoveryClient{}, "default")
	if err != nil {
		t.Fatalf("AdminListBridges: %v", err)
	}

	found := false
	for _, b := range bridges {
		if b.Name == "ghost-bridge" && b.Node == "ghost-node" {
			found = true
			if !b.Missing {
				t.Error("disabled orphan bridge should have Missing=true")
			}
		}
	}
	if !found {
		t.Fatal("disabled orphan bridge should be surfaced as missing")
	}
}

// TestDeleteBridge_RemovesOrphan: the admin can remove a disabled orphan bridge.
func TestDeleteBridge_RemovesOrphan(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	if err := st.SetBridgeEnabled(ctx, "default", "ghost-node", "ghost-bridge", false); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := catalog.DeleteBridge(ctx, st, "default", "ghost-node", "ghost-bridge"); err != nil {
		t.Fatalf("DeleteBridge: %v", err)
	}

	if err := catalog.DeleteBridge(ctx, st, "default", "ghost-node", "ghost-bridge"); !errors.Is(err, catalog.ErrBridgeNotFound) {
		t.Fatalf("second DeleteBridge: got %v, want ErrBridgeNotFound", err)
	}
}
