//nolint:goconst // test fixtures reuse cluster/storage string literals across seed and assertion sites
package recovery_test

import (
	"context"
	"database/sql"
	"pvmss/server/internal/recovery"
	"testing"
)

// snapshotCounts returns the row count of every catalog_* table in a v0.4 DB,
// keyed by table name. Used to assert two recovery runs produce identical
// persisted state (T013: sftp_config has no effect).
func snapshotCounts(t *testing.T, db *sql.DB) map[string]int {
	t.Helper()

	tables := []string{
		"catalog_nodes", "catalog_storages", "catalog_bridges", "catalog_isos",
		"catalog_profiles", "catalog_tags", "vm_limits", "node_limits",
	}

	out := make(map[string]int, len(tables)+1)
	for _, table := range tables {
		out[table] = countRows(t, db, `SELECT COUNT(*) FROM `+table+` WHERE cluster = ?`, "default")
	}

	out["clusters"] = countRows(t, db, `SELECT COUNT(*) FROM clusters`)

	return out
}

// TestRun_FullPipeline exercises the whole recovery sequence (data-model.md
// "Sequence"): every step maps its legacy table into the v0.4 catalog and
// accumulates the summary. DryRun=true maps without writing, covering stepNodes,
// stepBridges, stepISOs, stepProfiles, stepTags, stepVMLimits and stepNodeLimits
// (storage expansion is skipped with a nil resolver, as the CLI does when no
// Proxmox credentials are configured).
func TestRun_FullPipeline(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, defaultSeed())

	v04DB := openV04DB(t)
	ctx := context.Background()

	sum, err := recovery.Run(ctx, legacyDB, v04DB, recovery.RunOptions{
		ClusterName: "default",
		DryRun:      true,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if sum.Cluster.Written != 1 {
		t.Errorf("cluster written = %d, want 1", sum.Cluster.Written)
	}

	if sum.CatalogNodes.Written == 0 {
		t.Error("expected catalog_nodes to be written")
	}

	if sum.CatalogBridges.Written == 0 {
		t.Error("expected catalog_bridges to be written")
	}

	if sum.CatalogISOs.Written == 0 {
		t.Error("expected catalog_isos to be written")
	}

	if sum.CatalogProfiles.Written == 0 {
		t.Error("expected catalog_profiles to be written")
	}

	if sum.CatalogTags.Written == 0 {
		t.Error("expected catalog_tags to be written")
	}

	// RenderSummary must not panic on a populated summary.
	out := recovery.RenderSummary(sum, "legacy.db", "v04.db", "default")
	if out == "" {
		t.Error("RenderSummary returned empty output")
	}
}

// TestRun_WritePersists verifies that a non-dry run actually upserts rows into
// the v0.4 catalog, covering the upsert paths of every step (the DryRun branch
// skips the writes).
func TestRun_WritePersists(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, defaultSeed())

	v04DB := openV04DB(t)
	ctx := context.Background()

	if _, err := recovery.Run(ctx, legacyDB, v04DB, recovery.RunOptions{
		ClusterName: "default",
		DryRun:      false,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if n := countRows(t, v04DB, `SELECT COUNT(*) FROM catalog_nodes WHERE cluster = ?`, "default"); n == 0 {
		t.Errorf("catalog_nodes rows = %d, want > 0", n)
	}

	if n := countRows(t, v04DB, `SELECT COUNT(*) FROM catalog_profiles WHERE cluster = ?`, "default"); n == 0 {
		t.Errorf("catalog_profiles rows = %d, want > 0", n)
	}

	if n := countRows(t, v04DB, `SELECT COUNT(*) FROM catalog_tags WHERE cluster = ?`, "default"); n == 0 {
		t.Errorf("catalog_tags rows = %d, want > 0", n)
	}
}
