package catalog_test

import (
	"context"
	"path/filepath"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/store"
	"testing"
)

// openAdminStore opens a fully-migrated store (V9) with the T06 seed and the
// pvmss tag, ready for admin catalog operations.
func openAdminStore(t *testing.T) *store.Store {
	t.Helper()

	st, err := store.Open(config.Configuration{
		DBPath:    filepath.Join(t.TempDir(), "admin.db"),
		LogLevel:  "info",
		LogFormat: "json",
		LogOutput: "stdout",
	})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	return st
}

// findApprovalNode returns the NodeApproval with the given name, or fails.
func findApprovalNode(t *testing.T, nodes []catalog.NodeApproval, name string) catalog.NodeApproval {
	t.Helper()

	for _, n := range nodes {
		if n.Name == name {
			return n
		}
	}

	t.Fatalf("node %q not found in approval list", name)

	return catalog.NodeApproval{}
}

// findApprovalStorage returns the StorageApproval matching (name, node).
func findApprovalStorage(t *testing.T, storages []catalog.StorageApproval, name, node string) catalog.StorageApproval {
	t.Helper()

	for _, s := range storages {
		if s.Name == name && s.Node == node {
			return s
		}
	}

	t.Fatalf("storage %q@%q not found in approval list", name, node)

	return catalog.StorageApproval{}
}

// TestAdminListNodes_IncludesAllDiscoveredNodes verifies that AdminListNodes
// returns every node from the cluster snapshot (3 fake nodes), not just the
// approved ones — the unapproved node reports enabled=false.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminListNodes_IncludesAllDiscoveredNodes(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	nodes, err := catalog.AdminListNodes(ctx, st, cluster.Fake{}, "default")
	if err != nil {
		t.Fatalf("AdminListNodes: %v", err)
	}

	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d: %+v", len(nodes), nodes)
	}

	approved := findApprovalNode(t, nodes, "pve-node-01")
	if !approved.Enabled {
		t.Error("pve-node-01 should be enabled (T06 seed)")
	}

	approved2 := findApprovalNode(t, nodes, "pve-node-02")
	if !approved2.Enabled {
		t.Error("pve-node-02 should be enabled (T06 seed)")
	}

	unapproved := findApprovalNode(t, nodes, "pve-node-03")
	if unapproved.Enabled {
		t.Error("pve-node-03 should not be enabled (not in T06 seed)")
	}
}

// TestSetNodeEnabled_UpsertNeverDeletes verifies toggling is an idempotent
// upsert — a resource discovered but never toggled reports enabled=false;
// toggling on then off persists the row.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestSetNodeEnabled_UpsertNeverDeletes(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	// Before toggle: pve-node-03 is discovered but not approved.
	nodes, err := catalog.AdminListNodes(ctx, st, cluster.Fake{}, "default")
	if err != nil {
		t.Fatalf("AdminListNodes: %v", err)
	}

	before := findApprovalNode(t, nodes, "pve-node-03")
	if before.Enabled {
		t.Fatal("pve-node-03 should start disabled")
	}

	// Toggle on.
	if err := catalog.SetNodeEnabled(ctx, st, cluster.Fake{}, "default", "pve-node-03", true); err != nil {
		t.Fatalf("SetNodeEnabled true: %v", err)
	}

	after, err := catalog.AdminListNodes(ctx, st, cluster.Fake{}, "default")
	if err != nil {
		t.Fatalf("AdminListNodes after toggle: %v", err)
	}

	on := findApprovalNode(t, after, "pve-node-03")
	if !on.Enabled {
		t.Fatal("pve-node-03 should be enabled after toggle on")
	}

	// Toggle off — row persists, enabled=false.
	if err := catalog.SetNodeEnabled(ctx, st, cluster.Fake{}, "default", "pve-node-03", false); err != nil {
		t.Fatalf("SetNodeEnabled false: %v", err)
	}

	off, err := catalog.AdminListNodes(ctx, st, cluster.Fake{}, "default")
	if err != nil {
		t.Fatalf("AdminListNodes after toggle off: %v", err)
	}

	offNode := findApprovalNode(t, off, "pve-node-03")
	if offNode.Enabled {
		t.Fatal("pve-node-03 should be disabled after toggle off")
	}

	// Cross-tranche proof: ApprovedResources (T06) excludes disabled nodes.
	resources, err := catalog.ApprovedResources(ctx, st, "default")
	if err != nil {
		t.Fatalf("ApprovedResources: %v", err)
	}

	for _, n := range resources.Nodes {
		if n.Name == "pve-node-03" {
			t.Fatal("pve-node-03 should not be in ApprovedResources when disabled")
		}
	}
}

// TestSetStorageEnabled_PerPairIsolation verifies toggling one storage+node
// pair does not affect a same-named pair on a different node (Acceptance
// Scenario 1.3).
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestSetStorageEnabled_PerPairIsolation(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	// local@pve-node-02 is approved (T06 seed); local@pve-node-01 is not.
	storages, err := catalog.AdminListStorages(ctx, st, cluster.Fake{}, "default")
	if err != nil {
		t.Fatalf("AdminListStorages: %v", err)
	}

	node02Local := findApprovalStorage(t, storages, "local", "pve-node-02")
	if !node02Local.Enabled {
		t.Error("local@pve-node-02 should be enabled (T06 seed)")
	}

	node01Local := findApprovalStorage(t, storages, "local", "pve-node-01")
	if node01Local.Enabled {
		t.Error("local@pve-node-01 should not be enabled")
	}

	// Toggle local@pve-node-02 off.
	if err := catalog.SetStorageEnabled(ctx, st, cluster.Fake{}, "default", "local", "pve-node-02", false); err != nil {
		t.Fatalf("SetStorageEnabled: %v", err)
	}

	// local@pve-node-01 is unaffected — still disabled (its own state).
	if err := catalog.SetStorageEnabled(ctx, st, cluster.Fake{}, "default", "local", "pve-node-01", true); err != nil {
		t.Fatalf("SetStorageEnabled pve-node-01: %v", err)
	}

	after, err := catalog.AdminListStorages(ctx, st, cluster.Fake{}, "default")
	if err != nil {
		t.Fatalf("AdminListStorages after: %v", err)
	}

	node02After := findApprovalStorage(t, after, "local", "pve-node-02")
	if node02After.Enabled {
		t.Error("local@pve-node-02 should be disabled after toggle off")
	}

	node01After := findApprovalStorage(t, after, "local", "pve-node-01")
	if !node01After.Enabled {
		t.Error("local@pve-node-01 should be enabled after toggle on")
	}
}

// TestAdminListBridges_IncludesSuperset verifies the admin bridge list
// includes the fake superset (vmbr0, vmbr1 approved; vmbr2 not).
//
//nolint:paralleltest,dupl // serial: shared fake dataset; intentionally parallel to TestAdminListISOs_IncludesSuperset
func TestAdminListBridges_IncludesSuperset(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	bridges, err := catalog.AdminListBridges(ctx, st, cluster.Fake{}, "default")
	if err != nil {
		t.Fatalf("AdminListBridges: %v", err)
	}

	if len(bridges) != 3 {
		t.Fatalf("expected 3 bridges, got %d: %+v", len(bridges), bridges)
	}

	for _, b := range bridges {
		switch b.Name {
		case "vmbr0", "vmbr1":
			if !b.Enabled {
				t.Errorf("bridge %q should be enabled (T06 seed)", b.Name)
			}
		case "vmbr2":
			if b.Enabled {
				t.Error("vmbr2 should not be enabled")
			}
		}
	}
}

// TestSetBridgeEnabled_ToggleIsolatesByBridgeName verifies toggling one bridge
// does not affect another.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestSetBridgeEnabled_ToggleIsolatesByBridgeName(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	if err := catalog.SetBridgeEnabled(ctx, st, cluster.Fake{}, "default", "vmbr2", true); err != nil {
		t.Fatalf("SetBridgeEnabled vmbr2: %v", err)
	}

	bridges, err := catalog.AdminListBridges(ctx, st, cluster.Fake{}, "default")
	if err != nil {
		t.Fatalf("AdminListBridges: %v", err)
	}

	for _, b := range bridges {
		if b.Name == "vmbr2" && !b.Enabled {
			t.Error("vmbr2 should be enabled after toggle")
		}

		if b.Name == "vmbr0" && !b.Enabled {
			t.Error("vmbr0 should still be enabled (unaffected)")
		}
	}
}

// TestAdminListISOs_IncludesSuperset verifies the admin ISO list includes the
// fake superset keyed by (storage, file).
//
//nolint:paralleltest,dupl // serial: shared fake dataset; intentionally parallel to TestAdminListBridges_IncludesSuperset
func TestAdminListISOs_IncludesSuperset(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	isos, err := catalog.AdminListISOs(ctx, st, cluster.Fake{}, "default")
	if err != nil {
		t.Fatalf("AdminListISOs: %v", err)
	}

	if len(isos) != 3 {
		t.Fatalf("expected 3 ISOs, got %d: %+v", len(isos), isos)
	}

	for _, i := range isos {
		switch i.File {
		case "debian-12-generic-amd64.iso", "ubuntu-24.04-server-amd64.iso":
			if !i.Enabled {
				t.Errorf("ISO %q should be enabled (T06 seed)", i.File)
			}
		case "rocky-9-generic-x86_64.iso":
			if i.Enabled {
				t.Error("rocky-9 should not be enabled")
			}
		}
	}
}

// TestSetISOEnabled_ToggleIsolatesByStorageFile verifies toggling one ISO does
// not affect another.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestSetISOEnabled_ToggleIsolatesByStorageFile(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	if err := catalog.SetISOEnabled(ctx, st, cluster.Fake{}, "default", "local", "rocky-9-generic-x86_64.iso", true); err != nil {
		t.Fatalf("SetISOEnabled rocky-9: %v", err)
	}

	isos, err := catalog.AdminListISOs(ctx, st, cluster.Fake{}, "default")
	if err != nil {
		t.Fatalf("AdminListISOs: %v", err)
	}

	for _, i := range isos {
		if i.File == "rocky-9-generic-x86_64.iso" && !i.Enabled {
			t.Error("rocky-9 should be enabled after toggle")
		}

		if i.File == "debian-12-generic-amd64.iso" && !i.Enabled {
			t.Error("debian-12 should still be enabled (unaffected)")
		}
	}
}

// TestSetNodeEnabled_UnknownNodeReturnsError verifies that toggling a node not
// in the current discovery set returns an error (404 at the handler level).
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestSetNodeEnabled_UnknownNodeReturnsError(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	err := catalog.SetNodeEnabled(ctx, st, cluster.Fake{}, "default", "pve-node-99", true)
	if err == nil {
		t.Fatal("expected error for unknown node, got nil")
	}
}
