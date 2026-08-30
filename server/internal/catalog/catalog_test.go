package catalog_test

import (
	"context"
	"path/filepath"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/config"
	"pvmss/server/internal/store"
	"testing"
)

// openCatalogStore opens a migrated store in a temp dir — the catalog fixture
// is seeded by migration version 7, so a fresh DB carries it.
func openCatalogStore(t *testing.T) *store.Store {
	t.Helper()

	st, err := store.Open(config.Configuration{
		DBPath:    filepath.Join(t.TempDir(), "catalog.db"),
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

// TestApprovedResources_SeedInvariants pins the fixture's cross-table
// invariants (T06 data-model.md): every approved storage's node is itself
// approved, and every seeded row belongs to the configured cluster.
//
//nolint:paralleltest // serial: shared database fixture
func TestApprovedResources_SeedInvariants(t *testing.T) {
	st := openCatalogStore(t)

	resources, err := catalog.ApprovedResources(context.Background(), st, "default")
	if err != nil {
		t.Fatalf("ApprovedResources: %v", err)
	}

	if len(resources.Nodes) == 0 {
		t.Fatalf("seed has no approved nodes")
	}

	approvedNodes := make(map[string]bool, len(resources.Nodes))
	for _, node := range resources.Nodes {
		approvedNodes[node.Name] = true
	}

	for _, storage := range resources.Storages {
		if !approvedNodes[storage.Node] {
			t.Errorf("storage %q is approved on node %q, which is not itself approved", storage.Name, storage.Node)
		}
	}

	if len(resources.Bridges) != 0 {
		t.Errorf("node-aware migration should reset ambiguous bridge approvals, got %+v", resources.Bridges)
	}
}

// TestProfiles_SeedInvariants pins the profile fixture: unique IDs and rows
// scoped to the configured cluster.
//
//nolint:paralleltest // serial: shared database fixture
func TestProfiles_SeedInvariants(t *testing.T) {
	st := openCatalogStore(t)

	profiles, err := catalog.Profiles(context.Background(), st, "default")
	if err != nil {
		t.Fatalf("Profiles: %v", err)
	}

	if len(profiles) == 0 {
		t.Fatalf("seed has no profiles")
	}

	seen := make(map[string]bool, len(profiles))
	for _, profile := range profiles {
		if seen[profile.ID] {
			t.Errorf("duplicate profile id %q", profile.ID)
		}

		seen[profile.ID] = true
		if profile.CPUCores < 1 || profile.MemoryMB < 1 || profile.DiskGB < 1 {
			t.Errorf("profile %q has non-positive hardware values: %+v", profile.ID, profile)
		}
	}
}

// TestApprovedResources_ISONodePopulated — the store query returns one row per
// node (D1b), and ApprovedResources must copy iso.Node so HasISO can validate
// node locality (US1). V17 drops seed ISOs, so this test seeds them directly.
//
//nolint:paralleltest // serial: shared database fixture
func TestApprovedResources_ISONodePopulated(t *testing.T) {
	st := openCatalogStore(t)
	ctx := context.Background()

	// Seed one ISO on pve-node-01 and the same file on pve-node-02 (shared
	// storage pattern — one row per node, D1b).
	for _, row := range []struct{ node, storage, file string }{
		{"pve-node-01", "local", "debian-12.iso"},
		{"pve-node-02", "local", "debian-12.iso"},
	} {
		if err := st.SetISOEnabled(ctx, "default", row.node, row.storage, row.file, true); err != nil {
			t.Fatalf("SetISOEnabled: %v", err)
		}
	}

	resources, err := catalog.ApprovedResources(ctx, st, "default")
	if err != nil {
		t.Fatalf("ApprovedResources: %v", err)
	}

	if len(resources.ISOs) != 2 {
		t.Fatalf("ISOs = %d, want 2 (one row per node)", len(resources.ISOs))
	}

	for _, iso := range resources.ISOs {
		if iso.Node == "" {
			t.Errorf("ISO %q on storage %q has empty Node — store query must populate it", iso.File, iso.Storage)
		}

		if !resources.HasISO(iso.Storage, iso.File, iso.Node) {
			t.Errorf("HasISO(%q, %q, %q) = false, want true (seeded ISO must match its own node)", iso.Storage, iso.File, iso.Node)
		}
	}
}

// TestApprovedResources_UnknownClusterReturnsEmpty — a cluster with no seeded
// rows yields an empty catalog, not an error (membership checks then reject
// everything, which is the correct failure mode for a misconfigured cluster).
//
//nolint:paralleltest // serial: shared database fixture
func TestApprovedResources_UnknownClusterReturnsEmpty(t *testing.T) {
	st := openCatalogStore(t)

	resources, err := catalog.ApprovedResources(context.Background(), st, "no-such-cluster")
	if err != nil {
		t.Fatalf("ApprovedResources: %v", err)
	}

	if len(resources.Nodes) != 0 || len(resources.Storages) != 0 || len(resources.Bridges) != 0 || len(resources.ISOs) != 0 {
		t.Errorf("expected empty catalog for unknown cluster, got %+v", resources)
	}

	profiles, err := catalog.Profiles(context.Background(), st, "no-such-cluster")
	if err != nil {
		t.Fatalf("Profiles: %v", err)
	}

	if len(profiles) != 0 {
		t.Errorf("expected no profiles for unknown cluster, got %+v", profiles)
	}
}
