//nolint:goconst // test fixtures reuse cluster/tag/profile string literals across seed and assertion sites
package recovery_test

import (
	"context"
	"database/sql"
	"pvmss/server/internal/recovery"
	"strings"
	"testing"
)

// T005: Run calls each per-table function in data-model.md's Sequence order
// (clusters → nodes → storages → vmbrs → isos → profiles → tags → vm_limits → node_limits).
// T012: idempotence — running Run twice produces identical row counts (SC-003).
func TestRun_FullSequence_WritesAllTables(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, defaultSeed())

	v04DB := openV04DB(t)
	ctx := context.Background()

	env := stubEnviron{ //nolint:gosec // test fixture credential
		"PROXMOX_URL":             "https://pve.example.com:8006",
		"PROXMOX_API_TOKEN_NAME":  "pvmss@pve!service",
		"PROXMOX_API_TOKEN_VALUE": "secret-token-1234567890",
	}
	opts := recovery.RunOptions{
		ClusterName:   "test-cluster",
		Environ:       env,
		SessionSecret: "test-session-secret-at-least-32-bytes!!",
	}

	sum, err := recovery.Run(ctx, legacyDB, v04DB, opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Verify summary counts
	if sum.Cluster.Written != 1 {
		t.Errorf("Cluster.Written = %d, want 1", sum.Cluster.Written)
	}

	if sum.CatalogNodes.Written != 2 {
		t.Errorf("CatalogNodes.Written = %d, want 2", sum.CatalogNodes.Written)
	}

	if sum.CatalogBridges.Written != 1 {
		t.Errorf("CatalogBridges.Written = %d, want 1", sum.CatalogBridges.Written)
	}

	if sum.CatalogISOs.Written != 2 {
		t.Errorf("CatalogISOs.Written = %d, want 2", sum.CatalogISOs.Written)
	}

	if sum.CatalogProfiles.Written != 2 {
		t.Errorf("CatalogProfiles.Written = %d, want 2", sum.CatalogProfiles.Written)
	}

	if sum.CatalogTags.Written != 3 {
		t.Errorf("CatalogTags.Written = %d, want 3", sum.CatalogTags.Written)
	}

	if sum.VMLimits.Written != 1 {
		t.Errorf("VMLimits.Written = %d, want 1", sum.VMLimits.Written)
	}

	if sum.NodeLimits.Written != 2 {
		t.Errorf("NodeLimits.Written = %d, want 2", sum.NodeLimits.Written)
	}
	// Storages skipped (no resolver)
	if sum.CatalogStorages.Skipped != 2 {
		t.Errorf("CatalogStorages.Skipped = %d, want 2 (no resolver)", sum.CatalogStorages.Skipped)
	}

	// Verify database contents directly (using "test-cluster" to avoid v0.4 seed data)
	assertRowCount(t, v04DB, `SELECT COUNT(*) FROM clusters WHERE name = ?`, 1, "test-cluster")
	assertRowCount(t, v04DB, `SELECT COUNT(*) FROM catalog_nodes WHERE cluster = ?`, 2, "test-cluster")
	assertRowCount(t, v04DB, `SELECT COUNT(*) FROM catalog_bridges WHERE cluster = ?`, 1, "test-cluster")
	assertRowCount(t, v04DB, `SELECT COUNT(*) FROM catalog_isos WHERE cluster = ?`, 2, "test-cluster")
	assertRowCount(t, v04DB, `SELECT COUNT(*) FROM catalog_profiles WHERE cluster = ?`, 2, "test-cluster")
	assertRowCount(t, v04DB, `SELECT COUNT(*) FROM catalog_tags WHERE cluster = ?`, 3, "test-cluster")
	assertRowCount(t, v04DB, `SELECT COUNT(*) FROM node_limits WHERE cluster = ?`, 2, "test-cluster")
}

// T012 / SC-003: idempotence — running Run twice produces identical row counts.
func TestRun_Idempotent(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, defaultSeed())

	v04DB := openV04DB(t)
	ctx := context.Background()

	env := stubEnviron{ //nolint:gosec // test fixture credential
		"PROXMOX_URL":             "https://pve.example.com:8006",
		"PROXMOX_API_TOKEN_NAME":  "pvmss@pve!service",
		"PROXMOX_API_TOKEN_VALUE": "secret-token-1234567890",
	}
	opts := recovery.RunOptions{
		ClusterName:   "test-cluster",
		Environ:       env,
		SessionSecret: "test-session-secret-at-least-32-bytes!!",
	}

	if _, err := recovery.Run(ctx, legacyDB, v04DB, opts); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	// Capture row counts after first run (using "test-cluster" to avoid seed data)
	countsAfterFirst := snapshotCountsNamed(t, v04DB, "test-cluster")

	if _, err := recovery.Run(ctx, legacyDB, v04DB, opts); err != nil {
		t.Fatalf("second Run: %v", err)
	}

	countsAfterSecond := snapshotCountsNamed(t, v04DB, "test-cluster")

	for table, c1 := range countsAfterFirst {
		c2 := countsAfterSecond[table]
		if c1 != c2 {
			t.Errorf("idempotence violated: %s had %d rows after first run, %d after second", table, c1, c2)
		}
	}
}

func TestRun_DryRun_WritesNothing(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, defaultSeed())

	v04DB := openV04DB(t)
	ctx := context.Background()

	env := stubEnviron{ //nolint:gosec // test fixture credential
		"PROXMOX_URL":             "https://pve.example.com:8006",
		"PROXMOX_API_TOKEN_NAME":  "pvmss@pve!service",
		"PROXMOX_API_TOKEN_VALUE": "secret-token-1234567890",
	}
	opts := recovery.RunOptions{
		ClusterName:   "test-cluster",
		Environ:       env,
		SessionSecret: "test-session-secret-at-least-32-bytes!!",
		DryRun:        true,
	}

	sum, err := recovery.Run(ctx, legacyDB, v04DB, opts)
	if err != nil {
		t.Fatalf("Run dry-run: %v", err)
	}

	// Summary should still report counts
	if sum.CatalogNodes.Written != 2 {
		t.Errorf("CatalogNodes.Written = %d, want 2 (counted even in dry-run)", sum.CatalogNodes.Written)
	}

	// But the database should have no rows for the test cluster
	if count := countRows(t, v04DB, `SELECT COUNT(*) FROM clusters WHERE name = ?`, "test-cluster"); count != 0 {
		t.Errorf("clusters count = %d, want 0 (dry-run)", count)
	}

	if count := countRows(t, v04DB, `SELECT COUNT(*) FROM catalog_nodes WHERE cluster = ?`, "test-cluster"); count != 0 {
		t.Errorf("catalog_nodes count = %d, want 0 (dry-run)", count)
	}
}

func TestRun_WithStorageResolver(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, defaultSeed())

	v04DB := openV04DB(t)
	ctx := context.Background()

	env := stubEnviron{ //nolint:gosec // test fixture credential
		"PROXMOX_URL":             "https://pve.example.com:8006",
		"PROXMOX_API_TOKEN_NAME":  "pvmss@pve!service",
		"PROXMOX_API_TOKEN_VALUE": "secret-token-1234567890",
	}
	resolver := stubStorageResolver{
		nodes: map[string][]string{
			"local-lvm": {"pve-a", "pve-b"},
			"nfs-share": {"pve-a"},
		},
	}
	opts := recovery.RunOptions{
		ClusterName:     "test-cluster",
		Environ:         env,
		SessionSecret:   "test-session-secret-at-least-32-bytes!!",
		StorageResolver: resolver,
	}

	sum, err := recovery.Run(ctx, legacyDB, v04DB, opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if sum.CatalogStorages.Written != 3 {
		t.Errorf("CatalogStorages.Written = %d, want 3 (2+1 node expansions)", sum.CatalogStorages.Written)
	}

	if sum.CatalogStorages.Skipped != 0 {
		t.Errorf("CatalogStorages.Skipped = %d, want 0", sum.CatalogStorages.Skipped)
	}
}

func TestRenderSummary_OutputShape(t *testing.T) {
	t.Parallel()

	sum := recovery.Summary{
		Cluster:         recovery.TableResult{Written: 1},
		CatalogNodes:    recovery.TableResult{Read: 4, Written: 4},
		CatalogStorages: recovery.TableResult{Read: 6, Written: 5, Skipped: 1, SkipReasons: []string{`storage "old-nfs": no node reports it`}},
		CatalogBridges:  recovery.TableResult{Read: 2, Written: 2},
		CatalogISOs:     recovery.TableResult{Read: 3, Written: 2, Skipped: 1, SkipReasons: []string{`row "bad": does not match volid shape`}},
		CatalogProfiles: recovery.TableResult{Read: 5, Written: 4, Skipped: 1, SkipReasons: []string{`profile "bad": JSON parse error`}},
		CatalogTags:     recovery.TableResult{Read: 4, Written: 4},
		VMLimits:        recovery.TableResult{Read: 1, Written: 1, Note: "max_disk_per_vm_gb, max_network_cards, max_snapshots, max_vm_per_user, allow_custom_yaml — max_sockets/max_cores/max_memory_mb left at shipped defaults, no legacy source"},
		NodeLimits:      recovery.TableResult{Read: 3, Written: 3},
	}

	out := recovery.RenderSummary(sum, "/legacy.db", "/v04.db", "default")
	if !strings.Contains(out, "pvmss-recover: legacy=/legacy.db v0.4=/v04.db cluster=default") {
		t.Error("output missing header line")
	}

	if !strings.Contains(out, "SUMMARY: written=") {
		t.Error("output missing SUMMARY line")
	}

	if !strings.Contains(out, "clusters") {
		t.Error("output missing clusters line")
	}
}

// assertRowCount runs query (with args) against db and fails the test when the
// returned count differs from want. It wraps countRows so call sites stay flat.
func assertRowCount(t *testing.T, db *sql.DB, query string, want int, args ...any) {
	t.Helper()

	if got := countRows(t, db, query, args...); got != want {
		t.Errorf("%s: count = %d, want %d", query, got, want)
	}
}

// snapshotCounts captures row counts for all v0.4 tables for the "default" cluster.
func snapshotCounts(t *testing.T, db *sql.DB) map[string]int {
	t.Helper()
	return snapshotCountsNamed(t, db, "default")
}

// snapshotCountsNamed captures row counts for all v0.4 tables for a given cluster.
func snapshotCountsNamed(t *testing.T, db *sql.DB, cluster string) map[string]int {
	t.Helper()

	ctx := context.Background()
	queries := map[string]string{
		"clusters":         `SELECT COUNT(*) FROM clusters WHERE name = '` + cluster + `'`,
		"catalog_nodes":    `SELECT COUNT(*) FROM catalog_nodes WHERE cluster = '` + cluster + `'`,
		"catalog_storages": `SELECT COUNT(*) FROM catalog_storages WHERE cluster = '` + cluster + `'`,
		"catalog_bridges":  `SELECT COUNT(*) FROM catalog_bridges WHERE cluster = '` + cluster + `'`,
		"catalog_isos":     `SELECT COUNT(*) FROM catalog_isos WHERE cluster = '` + cluster + `'`,
		"catalog_profiles": `SELECT COUNT(*) FROM catalog_profiles WHERE cluster = '` + cluster + `'`,
		"catalog_tags":     `SELECT COUNT(*) FROM catalog_tags WHERE cluster = '` + cluster + `'`,
		"vm_limits":        `SELECT COUNT(*) FROM vm_limits WHERE cluster = '` + cluster + `'`,
		"node_limits":      `SELECT COUNT(*) FROM node_limits WHERE cluster = '` + cluster + `'`,
	}

	counts := make(map[string]int, len(queries))
	for name, query := range queries {
		var count int
		if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
			t.Fatalf("snapshot %s: %v", name, err)
		}

		counts[name] = count
	}

	return counts
}
